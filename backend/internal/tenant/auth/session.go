package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/async"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SessionCookieName is the cookie carrying the opaque session token. The name
// itself is declared in kernel/security, beside the console's, because that is
// where the CSRF middleware decides which cookies carry ambient authority —
// and a kernel package cannot import a plane to ask.
const SessionCookieName = security.TenantSessionCookie

// DefaultSessionTTL bounds how long a single login stays valid.
const DefaultSessionTTL = 12 * time.Hour

// DefaultIdleTimeout ends a session nobody has used.
//
// It answers a different question from DefaultSessionTTL: not "how long may
// one sign-in last" but "how long may an unattended machine keep somebody
// signed in". Eight hours of work should never be interrupted, and a screen
// left in a shared office should not still be signed in after lunch.
//
// The desktop clients are why this is a duration rather than something
// tighter: a native client sitting open on somebody's second monitor is doing
// nothing for long stretches and must not be signed out for it. Deployments
// that want a stricter rule set SESSION_IDLE_TIMEOUT; setting it to 0 turns
// the timeout off and leaves only the absolute lifetime.
const DefaultIdleTimeout = 90 * time.Minute

// touchInterval is how stale last_seen_at may get before it is written again.
// Without it every authenticated read would carry a write.
const touchInterval = time.Minute

// IdleTimeoutFromEnv reads SESSION_IDLE_TIMEOUT as a Go duration ("45m", "2h").
// An unset or unreadable value gives the default; "0" turns it off.
//
// Still exported and still reading the environment, because the environment is
// still the fallback — but the value the platform actually uses now comes from
// idleTimeout below, which asks the settings registry first. A deployment that
// only sets the variable behaves exactly as it did.
func IdleTimeoutFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("SESSION_IDLE_TIMEOUT"))
	if raw == "" {
		return DefaultIdleTimeout
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed < 0 {
		slog.Warn("SESSION_IDLE_TIMEOUT is not a duration; using the default",
			"value", raw, "default", DefaultIdleTimeout)
		return DefaultIdleTimeout
	}
	return parsed
}

// idleTimeout is what Resolve enforces, read per request rather than captured
// at construction.
//
// Per request because the point of making it a setting is that changing it
// takes effect — an operator who shortens the timeout during an incident
// should not have to restart the platform to mean it. The read is a map lookup
// behind a read lock, which is what the store exists to make cheap.
func (s *SessionStore) idleTimeout() time.Duration {
	if settings.Default == nil {
		// No store: the value captured at construction, which is the
		// environment or the default. This is every test and the first moments
		// of a process.
		return s.idle
	}
	return settings.Duration(settings.SessionIdleTimeout)
}

var ErrSessionInvalid = errors.New("session is invalid or expired")

// ErrNotAMember is returned when somebody asks to act for a tenant they hold no
// membership in. It is deliberately distinct from ErrSessionInvalid: the
// session is fine, the destination is not.
var ErrNotAMember = errors.New("no membership in that tenant")

// SessionStore issues and resolves opaque server-side session tokens.
//
// Tokens are 256 bits of crypto/rand output; only their SHA-256 digest is
// persisted, so a database leak cannot be replayed as a login.
type SessionStore struct {
	db   *pgxpool.Pool
	ttl  time.Duration
	idle time.Duration
}

func NewSessionStore(db *pgxpool.Pool, ttl time.Duration) *SessionStore {
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	return &SessionStore{db: db, ttl: ttl, idle: IdleTimeoutFromEnv()}
}

// HashSessionToken is the digest a session row is found by.
//
// Exported because the control plane's impersonation writes a session row of
// its own — with an expiry and an operator id this store knows nothing about —
// and it must produce exactly the digest Resolve looks for. A second hashing
// helper elsewhere would be a second place for the two to drift apart.
func HashSessionToken(token string) string { return hashToken(token) }

// NewSessionToken mints a token in the same shape as this package's own.
func NewSessionToken() (string, error) { return newToken() }

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// Create issues a new session and returns the plaintext token. The token is
// shown to the caller exactly once — only its digest is stored.
func (s *SessionStore) Create(ctx context.Context, userID, tenantID, authMethod, userAgent, ip string) (string, time.Time, error) {
	token, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt := time.Now().Add(s.ttl)
	if authMethod == "" {
		authMethod = "password"
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO tenant.sessions (token_hash, user_id, tenant_id, auth_method, user_agent, ip_address, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		hashToken(token), userID, tenantID, authMethod, userAgent, ip, expiresAt)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("persist session: %w", err)
	}

	return token, expiresAt, nil
}

// Resolve validates a token and returns the claims bound to it.
func (s *SessionStore) Resolve(ctx context.Context, token string) (UserClaims, error) {
	if token == "" {
		return UserClaims{}, ErrSessionInvalid
	}

	// Validating and touching in one statement, because this runs in front of
	// every authenticated request and a second round trip here is a second
	// round trip everywhere. The UPDATE writes at most once a minute per
	// session; the rest of the time its WHERE matches nothing and the CTE costs
	// an index lookup.
	//
	// idleCutoff is passed rather than computed in SQL so that turning the
	// timeout off is a value this query does not have to branch on.
	idleCutoff := time.Time{}
	if idle := s.idleTimeout(); idle > 0 {
		idleCutoff = time.Now().Add(-idle)
	}

	var claims UserClaims
	err := s.db.QueryRow(ctx,
		`WITH live AS (
		    SELECT id FROM tenant.sessions
		     WHERE token_hash = $1
		       AND revoked_at IS NULL
		       AND expires_at > NOW()
		       AND ($2::timestamptz IS NULL OR last_seen_at > $2)
		), touched AS (
		    UPDATE tenant.sessions SET last_seen_at = NOW()
		     WHERE id IN (SELECT id FROM live)
		       AND last_seen_at < NOW() - $3::interval
		)
		SELECT s.user_id::text, s.tenant_id::text, u.email,
		        ARRAY(SELECT a::text FROM unnest(s.allowed_tenant_ids) a) AS allowed,
		        COALESCE(s.impersonated_by::text, '') AS impersonated_by,
		        EXISTS (
		            SELECT 1 FROM tenant.memberships m
		            JOIN tenant.membership_roles mr ON mr.membership_id=m.id
		            JOIN tenant.roles r ON r.id=mr.role_id
		            WHERE m.tenant_id=s.tenant_id AND m.user_id=s.user_id
		              AND r.tenant_id=s.tenant_id AND r.code='admin' AND r.active
		        ) AS is_admin
		   FROM tenant.sessions s
		   JOIN live ON live.id = s.id
		   JOIN registry.users u ON u.id = s.user_id
		   JOIN tenant.memberships sm ON sm.tenant_id=s.tenant_id AND sm.user_id=s.user_id`,
		hashToken(token), nullableTime(idleCutoff), touchInterval.String()).
		Scan(&claims.UserID, &claims.TenantID, &claims.Email, &claims.AllowedTenantIDs,
			&claims.ImpersonatedBy, &claims.IsAdmin)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserClaims{}, ErrSessionInvalid
		}
		return UserClaims{}, fmt.Errorf("resolve session: %w", err)
	}

	claims.Impersonated = claims.ImpersonatedBy != ""
	return claims, nil
}

// Revoke invalidates a single session. Revoking an unknown token is a no-op so
// that logout never leaks whether a token existed.
func (s *SessionStore) Revoke(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.Exec(ctx,
		`UPDATE tenant.sessions SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`,
		hashToken(token))
	return err
}

// TenantOption is one tenant a person may act for.
type TenantOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// TenantsForUser lists the tenants a person holds a membership in, by name.
//
// The caller must pass a context with no tenant in it. memberships carries a
// tenant_id and is therefore under the row-level policy, so a context bound to
// the current tenant would answer this question with the one tenant the caller
// is already in — a switcher that only ever offers where you already are.
func (s *SessionStore) TenantsForUser(ctx context.Context, userID string) ([]TenantOption, error) {
	rows, err := s.db.Query(ctx,
		`SELECT t.id::text, t.name, t.slug
		   FROM tenant.memberships m
		   JOIN registry.tenants t ON t.id = m.tenant_id
		  WHERE m.user_id = $1
		  ORDER BY t.name, t.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list tenants for user: %w", err)
	}
	defer rows.Close()

	options := make([]TenantOption, 0, 2)
	for rows.Next() {
		var option TenantOption
		if err := rows.Scan(&option.ID, &option.Name, &option.Slug); err != nil {
			return nil, fmt.Errorf("read tenant option: %w", err)
		}
		options = append(options, option)
	}
	return options, rows.Err()
}

// SwitchTenant moves a live session to another tenant the same person belongs
// to, and returns the token that replaces the old one.
//
// The token is rotated rather than the row updated. A session token is the
// authority to act inside one tenant, and the tenant is what changes here; a
// token that silently means something different from what it meant a moment ago
// is the thing rotation exists to prevent. It also means the audit trail keeps
// one row per tenant a person acted in, rather than one row whose tenant column
// is only ever the last one they chose.
//
// The membership check is the authorization, and it is made here rather than by
// the caller so that no route can reach the insert without it.
//
// Like TenantsForUser, this needs a context with no tenant: sessions carries a
// tenant_id, and the policy's WITH CHECK would refuse a row belonging to the
// tenant being moved to.
func (s *SessionStore) SwitchTenant(ctx context.Context, token, tenantID string) (string, time.Time, error) {
	if token == "" {
		return "", time.Time{}, ErrSessionInvalid
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("switch tenant: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID string
	err = tx.QueryRow(ctx,
		`SELECT user_id::text FROM tenant.sessions
		  WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()`,
		hashToken(token)).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", time.Time{}, ErrSessionInvalid
		}
		return "", time.Time{}, fmt.Errorf("switch tenant: %w", err)
	}

	var member bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM tenant.memberships WHERE user_id = $1 AND tenant_id = $2)`,
		userID, tenantID).Scan(&member); err != nil {
		return "", time.Time{}, fmt.Errorf("switch tenant: %w", err)
	}
	if !member {
		return "", time.Time{}, ErrNotAMember
	}

	next, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	// A switch is not a fresh sign-in: the new session expires when the old one
	// would have. Otherwise moving between two tenants every so often would
	// extend a session indefinitely without anybody proving who they are again.
	var expiresAt time.Time
	if err := tx.QueryRow(ctx,
		`INSERT INTO tenant.sessions (token_hash, user_id, tenant_id, auth_method, user_agent, ip_address, expires_at)
		 SELECT $2, user_id, $3, auth_method, user_agent, ip_address, expires_at
		   FROM tenant.sessions WHERE token_hash = $1
		 RETURNING expires_at`,
		hashToken(token), hashToken(next), tenantID).Scan(&expiresAt); err != nil {
		return "", time.Time{}, fmt.Errorf("switch tenant: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE tenant.sessions SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`,
		hashToken(token)); err != nil {
		return "", time.Time{}, fmt.Errorf("switch tenant: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", time.Time{}, fmt.Errorf("switch tenant: %w", err)
	}
	return next, expiresAt, nil
}

// SetActiveTenants records which organisations a session reads across.
//
// Odoo calls this the allowed companies, and the shape is the same: a person
// ticks several of the organisations they belong to and every list that opts in
// spans them, while new rows still go to the one they are acting in. That
// asymmetry is the safety of it, and it lives in the policies (see migration
// 00037) rather than being remembered by each caller.
//
// Every id is checked against an active membership before it is written, and
// the acting tenant is always included — a session that could read a set not
// containing the organisation it writes into would be able to create rows it
// then could not see.
func (s *SessionStore) SetActiveTenants(ctx context.Context, token string, tenantIDs []string) ([]string, error) {
	if token == "" {
		return nil, ErrSessionInvalid
	}
	// Crossing tenants is the point, so this runs on the platform path: under
	// the caller's own policies, memberships in another organisation are not
	// visible and every id would look like one they do not hold.
	ctx = nexus.WithoutTenant(ctx)

	var current string
	if err := s.db.QueryRow(ctx,
		`SELECT tenant_id::text FROM tenant.sessions
		  WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()`,
		hashToken(token)).Scan(&current); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionInvalid
		}
		return nil, fmt.Errorf("read session: %w", err)
	}

	wanted := map[string]bool{current: true}
	for _, id := range tenantIDs {
		if id != "" {
			wanted[id] = true
		}
	}
	candidates := make([]string, 0, len(wanted))
	for id := range wanted {
		candidates = append(candidates, id)
	}

	// One query rather than one per id, and it answers with what the person
	// actually holds — an id they do not is dropped rather than refused, so a
	// stale tab that still lists an organisation somebody has left narrows
	// quietly instead of failing.
	rows, err := s.db.Query(ctx,
		`SELECT m.tenant_id::text FROM tenant.memberships m
		  JOIN tenant.sessions s ON s.token_hash = $1 AND s.user_id = m.user_id
		 WHERE m.tenant_id = ANY($2::uuid[]) AND m.active`,
		hashToken(token), candidates)
	if err != nil {
		return nil, fmt.Errorf("check memberships: %w", err)
	}
	defer rows.Close()

	allowed := make([]string, 0, len(candidates))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("check memberships: %w", err)
		}
		allowed = append(allowed, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("check memberships: %w", err)
	}
	sort.Strings(allowed)

	if _, err := s.db.Exec(ctx,
		`UPDATE tenant.sessions SET allowed_tenant_ids = $2::uuid[] WHERE token_hash = $1`,
		hashToken(token), allowed); err != nil {
		return nil, fmt.Errorf("save the active organisations: %w", err)
	}
	return allowed, nil
}

// RevokeAllForUser ends every session a person holds, on every device.
//
// It is what somebody needs after leaving a session open on a machine they no
// longer have, and what an operator needs when an account is suspected. The
// caller's own session goes with the rest: "sign out everywhere" that leaves
// the browser it was pressed in signed in is not what it says.
func (s *SessionStore) RevokeAllForUser(ctx context.Context, userID string) (int64, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE tenant.sessions SET revoked_at = NOW()
		  WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > NOW()`, userID)
	if err != nil {
		return 0, fmt.Errorf("revoke every session: %w", err)
	}
	return tag.RowsAffected(), nil
}

// nullableTime turns the zero time into a SQL NULL, which is how the resolve
// query says "no idle timeout" without a second version of itself.
func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// DeleteExpired purges sessions that can no longer be used.
//
// The seven-day grace is not caution about the expiry itself — an expired row
// stops authenticating the moment it expires — but about the audit question
// "when did this person last sign in", which is only answerable while the row
// is still there.
func (s *SessionStore) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM tenant.sessions WHERE expires_at < NOW() - INTERVAL '7 days'`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// StartHousekeeping purges long-expired sessions until ctx is cancelled.
//
// Without it the table only ever grows: every sign-in writes a row and nothing
// ever removed one, so the cost of resolving a token climbed with the lifetime
// of the deployment rather than with the number of people using it.
func (s *SessionStore) StartHousekeeping(ctx context.Context) {
	async.Go("session-housekeeping", func() {
		ticker := time.NewTicker(sessionSweepInterval)
		defer ticker.Stop()
		for {
			s.sweep(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
}

// sessionSweepInterval is deliberately long. Nothing depends on a dead row
// being gone promptly — it cannot authenticate anyone — so this is a size
// concern, and sweeping hourly would be a repeated table scan bought for
// nothing.
const sessionSweepInterval = 6 * time.Hour

func (s *SessionStore) sweep(ctx context.Context) {
	sweepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	purged, err := s.DeleteExpired(sweepCtx)
	if err != nil {
		slog.Warn("auth: could not purge expired sessions", "error", err)
		return
	}
	if purged > 0 {
		slog.Info("auth: purged expired sessions", "count", purged)
	}
}

// TokenFromRequest extracts a session token from the cookie or the
// Authorization header, in that order.
func TokenFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(SessionCookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	const prefix = "Bearer "
	if header := r.Header.Get("Authorization"); len(header) > len(prefix) && header[:len(prefix)] == prefix {
		return header[len(prefix):]
	}
	return ""
}

// SetSessionCookie writes the session cookie with hardened attributes.
//
// SameSite is Lax rather than Strict, and that is what makes single sign-on
// single. A relying party signing somebody in sends the browser to
// /oauth2/auth, which is a top-level navigation arriving from another site; a
// Strict cookie is not sent on one, so the authorization endpoint saw no
// session, decided nobody was signed in, and showed a login screen to a person
// who had signed in a minute earlier. The same is true in the other direction,
// for a deployment that federates: every return from its provider looked like a
// first visit.
//
// It costs nothing in CSRF terms, because the cookie was never the defence.
// Lax adds exactly one thing over Strict — the cookie rides along on a
// cross-site *top-level GET navigation* — and every state-changing request on
// this platform goes through security.CSRFMiddleware, which requires positive
// evidence that a page of ours made it (an allowlisted Origin, or
// Sec-Fetch-Site saying same-origin or none). A cross-site POST is refused
// there whatever the cookie policy says.
func SetSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   IsProduction(),
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie expires the session cookie on the client. Its attributes
// have to match the ones it was set with, or the browser treats it as a
// different cookie and leaves the original in place.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   IsProduction(),
		SameSite: http.SameSiteLaxMode,
	})
}

// IsProduction reports whether the process runs with ENVIRONMENT=production.
func IsProduction() bool {
	return os.Getenv("ENVIRONMENT") == "production"
}
