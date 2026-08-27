package operator

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/async"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SessionCookieName is the cookie carrying the operator's session token.
//
// Different from the tenant side's, and that is the point rather than a
// detail: the two must never be interchangeable, and a browser that holds both
// (an operator who also administers an organisation) must not have one of them
// answer for the other.
const SessionCookieName = security.ControlPlaneSessionCookie

// ErrSessionInvalid covers every reason a token does not authenticate: unknown,
// expired, idle, revoked, or belonging to an account that has been disabled.
// The caller cannot tell which, and neither can whoever holds the token.
var ErrSessionInvalid = errors.New("operator session is invalid or expired")

// Session is a signed-in operator.
type Session struct {
	Operator
	// Token is the plaintext token, kept only so that logout and step-up can
	// name the row. It is never sent anywhere but the cookie it arrived in.
	Token string `json:"-"`
	// SteppedUpAt is when the second factor was last re-confirmed, or the zero
	// time. requireStepUp is what turns it into a decision.
	SteppedUpAt time.Time `json:"stepped_up_at,omitzero"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// SteppedUp reports whether a dangerous action may proceed without asking for
// the code again.
func (s Session) SteppedUp(now time.Time) bool {
	return !s.SteppedUpAt.IsZero() && now.Sub(s.SteppedUpAt) < StepUpWindow
}

// SessionStore issues and resolves operator session tokens. It is the tenant
// side's SessionStore with the tenant machinery removed — no organisation to
// act for, no switching, no allowed set.
type SessionStore struct {
	db *pgxpool.Pool
}

func NewSessionStore(db *pgxpool.Pool) *SessionStore { return &SessionStore{db: db} }

// touchInterval is how stale last_seen_at may get before it is written again,
// so that the idle timeout does not put a write on every request.
const touchInterval = time.Minute

// HashToken is how a token becomes the row that stands for it. Shared because
// the support screen issues links the same way a session is issued.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate an operator session token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// create issues a session inside an existing transaction.
//
// Only ever called from the sign-in path, which writes the audit row in the
// same transaction — so a session that exists is a session whose creation was
// recorded, with no window between the two.
func (s *SessionStore) create(ctx context.Context, tx pgx.Tx, operatorID, userAgent, ip string, steppedUp bool) (string, time.Time, error) {
	token, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(SessionTTL)

	// Signing in has just proved the second factor, so the session starts
	// stepped up. Without this the first dangerous action of every session
	// would ask for a code that was typed seconds earlier, and an operator
	// asked for the same code twice in a row learns to keep the authenticator
	// open — which is the habit step-up exists to avoid.
	var steppedUpAt any
	if steppedUp {
		steppedUpAt = time.Now()
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO operator.operator_sessions (token_hash, operator_id, user_agent, ip_address, expires_at, stepped_up_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		HashToken(token), operatorID, userAgent, ip, expiresAt, steppedUpAt); err != nil {
		return "", time.Time{}, fmt.Errorf("persist the operator session: %w", err)
	}
	return token, expiresAt, nil
}

// Resolve validates a token and returns who holds it.
//
// Idle and absolute expiry are both enforced here, in the statement that also
// touches the row, for the reason the tenant side gives: this runs in front of
// every console request and a second round trip here is a second round trip
// everywhere.
func (s *SessionStore) Resolve(ctx context.Context, token string) (Session, error) {
	if token == "" {
		return Session{}, ErrSessionInvalid
	}
	ctx = Scoped(ctx)

	var (
		sess        Session
		role        string
		steppedUpAt *time.Time
	)
	err := s.db.QueryRow(ctx,
		`WITH live AS (
		    SELECT s.id FROM operator.operator_sessions s
		      JOIN operator.operator_accounts a ON a.id = s.operator_id
		     WHERE s.token_hash = $1
		       AND s.revoked_at IS NULL
		       AND s.expires_at > NOW()
		       AND s.last_seen_at > NOW() - $2::interval
		       AND a.disabled_at IS NULL
		), touched AS (
		    UPDATE operator.operator_sessions SET last_seen_at = NOW()
		     WHERE id IN (SELECT id FROM live)
		       AND last_seen_at < NOW() - $3::interval
		)
		SELECT a.id::text, a.email, a.name, a.role, s.stepped_up_at, s.expires_at
		  FROM operator.operator_sessions s
		  JOIN live ON live.id = s.id
		  JOIN operator.operator_accounts a ON a.id = s.operator_id`,
		HashToken(token), SessionIdleTimeout.String(), touchInterval.String()).
		Scan(&sess.ID, &sess.Email, &sess.Name, &role, &steppedUpAt, &sess.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrSessionInvalid
		}
		return Session{}, fmt.Errorf("resolve the operator session: %w", err)
	}

	sess.Role = Role(role)
	sess.Token = token
	if steppedUpAt != nil {
		sess.SteppedUpAt = *steppedUpAt
	}
	return sess, nil
}

// MarkSteppedUp records that the second factor has just been re-confirmed.
func (s *SessionStore) MarkSteppedUp(ctx context.Context, tx pgx.Tx, token string) error {
	_, err := tx.Exec(ctx,
		`UPDATE operator.operator_sessions SET stepped_up_at = NOW()
		  WHERE token_hash = $1 AND revoked_at IS NULL`, HashToken(token))
	if err != nil {
		return fmt.Errorf("record the step-up: %w", err)
	}
	return nil
}

// Revoke ends one session. Revoking an unknown token is a no-op, so logout
// never says whether a token existed.
func (s *SessionStore) Revoke(ctx context.Context, tx pgx.Tx, token string) error {
	if token == "" {
		return nil
	}
	_, err := tx.Exec(ctx,
		`UPDATE operator.operator_sessions SET revoked_at = NOW()
		  WHERE token_hash = $1 AND revoked_at IS NULL`, HashToken(token))
	return err
}

// RevokeAllForOperator ends every session an account holds. It is what
// disabling an operator has to do to take effect immediately rather than
// whenever their current session happens to expire.
func (s *SessionStore) RevokeAllForOperator(ctx context.Context, tx pgx.Tx, operatorID string) (int64, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE operator.operator_sessions SET revoked_at = NOW()
		  WHERE operator_id = $1 AND revoked_at IS NULL AND expires_at > NOW()`, operatorID)
	if err != nil {
		return 0, fmt.Errorf("revoke the operator's sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

// sweepInterval and the grace below follow the tenant side's reasoning: an
// expired row authenticates nobody, so removing it promptly buys nothing, and
// keeping it a while answers "when was this operator last here".
const (
	sweepInterval = 6 * time.Hour
	sweepGrace    = "30 days"
)

// StartHousekeeping purges long-expired operator sessions until ctx is done.
func (s *SessionStore) StartHousekeeping(ctx context.Context) {
	async.Go("control-plane-session-housekeeping", func() {
		ticker := time.NewTicker(sweepInterval)
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

// sweep deletes on the platform path rather than as the operator role.
//
// That is not an oversight: DELETE is a privilege migration 00049 deliberately
// does not grant the console, because nothing an operator does through the
// console should be able to remove a session row. Housekeeping is not a console
// request — it is the process tidying up after itself — and it runs where the
// rest of this platform's sweeps run.
func (s *SessionStore) sweep(ctx context.Context) {
	sweepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tag, err := s.db.Exec(sweepCtx,
		`DELETE FROM operator.operator_sessions WHERE expires_at < NOW() - INTERVAL '`+sweepGrace+`'`)
	if err != nil {
		slog.Warn("control plane: could not purge expired operator sessions", "error", err)
		return
	}
	if purged := tag.RowsAffected(); purged > 0 {
		slog.Info("control plane: purged expired operator sessions", "count", purged)
	}
}

// TokenFromRequest reads the console's session cookie.
//
// Cookie only, and no Authorization header: the console is a browser
// application on one hostname behind an address allowlist, and a bearer token
// is the shape of credential that ends up in a script, a shell history and a
// CI variable.
func TokenFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(SessionCookieName); err == nil {
		return cookie.Value
	}
	return ""
}

// SetSessionCookie writes the console's cookie.
//
// SameSite is Strict, where the tenant side has to be Lax. That cookie rides
// along on single sign-on navigations arriving from other sites; this one has
// no such journey to make — nothing off this hostname ever links into the
// console — so it takes the stricter setting.
func SetSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   config.IsProduction(),
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearSessionCookie expires the console's cookie. The attributes have to match
// the ones it was set with or the browser keeps the original.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   config.IsProduction(),
		SameSite: http.SameSiteStrictMode,
	})
}
