/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Package platform provides the core HTTP Server orchestrator, routing table,
 * authentication middleware, and app installer wiring.
 */

package platform

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/eid"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/eidmongolia"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/observability"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/ssoclient"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/tenant"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/time/rate"
)

const (
	maxLoginBody       = 8 << 10
	maxLoginFailures   = 5
	loginLockoutWindow = 15 * time.Minute
)

var dummyPasswordHash = func() string {
	hash, err := auth.HashPassword("constant-time-missing-user-placeholder")
	if err != nil {
		panic(err)
	}
	return hash
}()

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBody)
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid login credentials")
		return
	}

	// A user can hold memberships in several tenants. LIMIT 1 without an ORDER
	// BY let Postgres pick, so the same credentials could land in a different
	// tenant from one sign-in to the next — and the session, its audit trail
	// and every subsequent read are scoped to whichever it picked. Oldest
	// membership first makes it the same tenant every time.
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	var userID, passwordHash, tenantID, name string
	var isAdmin bool
	var lockedUntil pgtype.Timestamptz
	err := s.db.QueryRow(r.Context(),
		`SELECT u.id, u.password_hash, u.name,
		        EXISTS (
		            SELECT 1 FROM membership_roles mr
		            JOIN roles r ON r.id=mr.role_id
		            WHERE mr.membership_id=m.id AND r.tenant_id=m.tenant_id
		              AND r.code='admin' AND r.active
		        ) AS is_admin,
		        m.tenant_id, u.locked_until
		 FROM users u
		 JOIN memberships m ON m.user_id = u.id
		 WHERE lower(u.email) = $1
		 ORDER BY m.created_at, m.tenant_id
		 LIMIT 1`, req.Email).Scan(&userID, &passwordHash, &name, &isAdmin, &tenantID, &lockedUntil)

	if errors.Is(err, pgx.ErrNoRows) {
		passwordHash = dummyPasswordHash
	} else if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "login service unavailable")
		return
	}
	passwordOK := auth.CheckPasswordHash(req.Password, passwordHash)
	locked := lockedUntil.Valid && lockedUntil.Time.After(time.Now())
	if !passwordOK || locked {
		if userID != "" && !locked {
			s.recordLoginFailure(r.Context(), userID)
		}
		audit.Record(r.Context(), "unknown", "anonymous", "auth.login_failed", "user", map[string]any{"email": req.Email})
		observability.RecordLogin(observability.LoginPassword, false)
		httpx.Error(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	_, _ = s.db.Exec(r.Context(), `UPDATE users SET failed_login_attempts=0, locked_until=NULL WHERE id=$1`, userID)

	token, expiresAt, err := s.issueSession(r, userID, tenantID, "password")
	if err != nil {
		reportSessionFailure(w, err)
		return
	}
	auth.SetSessionCookie(w, token, expiresAt)

	audit.Record(r.Context(), tenantID, userID, "auth.login_success", "user", map[string]any{"email": req.Email})
	observability.RecordLogin(observability.LoginPassword, true)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"expires_at": expiresAt,
		"user": map[string]any{
			"id":        userID,
			"tenant_id": tenantID,
			"name":      name,
			"email":     req.Email,
			"is_admin":  isAdmin,
		},
	})
}

// loginFailureStatement counts one failed sign-in and locks the account once
// the count reaches maxLoginFailures.
//
// A lockout that has run its course starts the count again. Adding to a counter
// that stopped at maxLoginFailures meant the threshold was still met by the
// next single failure: after one lockout, every subsequent mistyped password
// re-locked the account for another full window, and anybody who knew the
// address could hold it shut with one request every loginLockoutWindow. The
// lapsed lock is cleared in the same statement — the CASE has no ELSE, so a
// count below the threshold writes NULL — rather than left to assert a lockout
// that has expired.
//
// Callers must not use it on an account that is currently locked; handleLogin
// checks that before calling, so a live lockout is never extended by attempts
// made during it.
const loginFailureStatement = `
	UPDATE users u SET
	   failed_login_attempts = next.count,
	   locked_until = CASE WHEN next.count >= $2 THEN NOW() + $3::interval END
	  FROM (
	      SELECT CASE WHEN locked_until IS NOT NULL AND locked_until <= NOW()
	                  THEN 1 ELSE failed_login_attempts + 1 END AS count
	        FROM users WHERE id = $1
	  ) AS next
	 WHERE u.id = $1`

// recordLoginFailure applies loginFailureStatement. A failure to record one is
// logged rather than surfaced: the caller is already answering 401, and turning
// a bookkeeping error into a 500 would tell whoever is guessing that this
// address exists.
func (s *Server) recordLoginFailure(ctx context.Context, userID string) {
	if _, err := s.db.Exec(ctx, loginFailureStatement,
		userID, maxLoginFailures, loginLockoutWindow.String()); err != nil {
		slog.Error("failed to record a login failure", "user_id", userID, "error", err)
	}
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Logout previously only cleared the cookie; the token stayed valid
	// forever for anyone who had captured it.
	if token := auth.TokenFromRequest(r); token != "" {
		if err := s.sessions.Revoke(r.Context(), token); err != nil {
			slog.Error("failed to revoke session", "error", err)
		}
	}

	// On a deployment that federates, ending the session here is half the job.
	// The provider still holds one, and a person who pressed "sign out" and can
	// be signed straight back in by clicking "sign in" has not signed out in
	// any sense they would recognise. So the answer carries where to go to
	// finish it, and the browser is sent there; the provider ends its own
	// session and returns them to this deployment's post-logout address.
	//
	// Read before the cookie is cleared, and cleared after: the hint is spent
	// here and is of no use to the next session.
	endSession := ""
	if s.ssoClientEnabled() {
		endSession = s.ssoClient.EndSessionURL(r.Context(), ssoclient.IDTokenFromRequest(r))
		ssoclient.ClearIDTokenCookie(w)
	}

	auth.ClearSessionCookie(w)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "logged_out",
		// Empty unless this deployment federates and its provider offers
		// RP-initiated logout. A client that finds it empty is already done.
		"end_session_url": endSession,
	})
}

// handleTenants answers which tenants the caller may act for.
//
// A user with one membership gets a list of one. That is not a wasted answer:
// it is what lets the client show where it is without claiming there is
// somewhere else to go.
func (s *Server) handleTenants(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	options, err := s.sessions.TenantsForUser(tenant.Without(r.Context()), claims.UserID)
	if err != nil {
		slog.Error("failed to list the tenants a user belongs to", "user_id", claims.UserID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, "failed to list tenants")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"current": claims.TenantID,
		"tenants": options,
		// Which of them this session is reading across. Empty would be
		// ambiguous on the client — "none" and "just the current one" look the
		// same — so the acting organisation is always named.
		"active": activeOrCurrent(claims),
	})
}

func activeOrCurrent(claims auth.UserClaims) []string {
	if len(claims.AllowedTenantIDs) > 0 {
		return claims.AllowedTenantIDs
	}
	return []string{claims.TenantID}
}

// handleSetActiveTenants chooses which organisations this session reads across.
//
// The list is a request, not an instruction: every id is held against an active
// membership and the ones that do not survive are dropped rather than refused,
// so a tab left open since before somebody changed jobs narrows quietly instead
// of failing. The organisation being acted in is always kept.
func (s *Server) handleSetActiveTenants(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TenantIDs []string `json:"tenant_ids"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "malformed request body")
		return
	}
	// A group with this many organisations in it is not what this feature is
	// for, and an unbounded array becomes an unbounded query parameter.
	if len(body.TenantIDs) > 64 {
		httpx.Error(w, http.StatusBadRequest, "too many organisations")
		return
	}

	token := auth.TokenFromRequest(r)
	if token == "" {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	active, err := s.sessions.SetActiveTenants(r.Context(), token, body.TenantIDs)
	if err != nil {
		if errors.Is(err, auth.ErrSessionInvalid) {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		slog.Error("could not set the active organisations", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "could not save the selection")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"active": active})
}

// handleSwitchTenant moves the caller's session to another of their tenants.
//
// Until this existed, which tenant a person acted in was decided once, at
// login, by whichever membership was oldest — someone who works for two
// organisations could reach only one of them, and the only way to change that
// was to edit the database. Signing out and back in would not have helped: the
// login picks the same oldest membership every time, deliberately.
func (s *Server) handleSwitchTenant(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBody)
	var req struct {
		TenantID string `json:"tenant_id"`
	}
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil || strings.TrimSpace(req.TenantID) == "" {
		httpx.Error(w, http.StatusBadRequest, "tenant_id is required")
		return
	}
	req.TenantID = strings.TrimSpace(req.TenantID)

	// Already there. Answering rather than rotating keeps a double-click on the
	// tenant you are in from replacing a working session with another one.
	if req.TenantID == claims.TenantID {
		httpx.JSON(w, http.StatusOK, map[string]any{"tenant_id": claims.TenantID, "switched": false})
		return
	}

	token, expiresAt, err := s.sessions.SwitchTenant(tenant.Without(r.Context()), auth.TokenFromRequest(r), req.TenantID)
	switch {
	case errors.Is(err, auth.ErrNotAMember):
		// Not 404: whether that tenant exists is not this caller's business.
		httpx.Error(w, http.StatusForbidden, "no membership in that tenant")
		return
	case errors.Is(err, auth.ErrSessionInvalid):
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	case err != nil:
		slog.Error("failed to switch tenant", "user_id", claims.UserID, "tenant_id", req.TenantID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, "failed to switch tenant")
		return
	}

	auth.SetSessionCookie(w, token, expiresAt)
	// Recorded against the tenant being left, because that is the trail an
	// administrator of it reads: this person stopped acting here, and when.
	audit.Record(r.Context(), claims.TenantID, claims.UserID, "auth.tenant_switched", "session",
		map[string]any{"from": claims.TenantID, "to": req.TenantID})
	httpx.JSON(w, http.StatusOK, map[string]any{
		"tenant_id":  req.TenantID,
		"switched":   true,
		"expires_at": expiresAt,
	})
}

// handleLogoutEverywhere ends every session the caller holds.
//
// Logging out ends the session in front of you. This is for the one you left
// somewhere else — a machine in an office you have left, a desktop client on a
// laptop that was taken — and it is the only thing on the platform that answers
// that, short of an administrator disabling the account.
//
// It is inside the authenticated group, so the caller is proving they hold one
// of the sessions they are ending. The cookie here is cleared too: the session
// it names is among those just revoked.
func (s *Server) handleLogoutEverywhere(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	revoked, err := s.sessions.RevokeAllForUser(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("failed to revoke every session", "user_id", claims.UserID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, "failed to end the other sessions")
		return
	}

	auth.ClearSessionCookie(w)
	audit.Record(r.Context(), claims.TenantID, claims.UserID, "auth.logout_everywhere", "session",
		map[string]any{"revoked": revoked})
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "logged_out_everywhere", "revoked": revoked})
}

// Sign-in and eID polling are budgeted separately, because they are different
// things happening at different rates.
//
// Signing in is the endpoint worth guessing against, and starting an eID
// session pushes a notification to a real person's phone, so it stays tight.
//
// Polling is neither: it needs a session ID the relying party only ever handed
// to whoever started that session, and it cannot be turned into an attempt at a
// second one. What it does do is repeat — once every eid.PollWindow for as long
// as a citizen takes to reach their phone. Sharing the sign-in budget meant a
// few citizens waiting behind one office or NAT address spent it between them,
// and the next person there could not sign in at all. So this is budgeted by
// how many of them may plausibly be waiting behind a single address at once.
const (
	loginRatePerMinute = 5
	loginBurst         = 5
	pollRatePerMinute  = 60
	pollBurst          = 15

	// The AI budget was here until 2026-08-23. It moved to the app that spends
	// it — internal/apps/ai — which asks for the same numbers through
	// nexus.RateLimit, so the deployment-wide bucket is still one bucket.
	//
	// The verification endpoint spends a mailbox credential the whole platform
	// shares, which is why it is budgeted and why it stays here: it is the
	// platform's own route.
	verifyRatePerMinute = 60
	verifyBurst         = 20
)

func newLoginLimiter() *security.IPRateLimiter {
	return security.NewIPRateLimiter(rate.Limit(float64(loginRatePerMinute)/60.0), loginBurst)
}

func newPollLimiter() *security.IPRateLimiter {
	return security.NewIPRateLimiter(rate.Limit(float64(pollRatePerMinute)/60.0), pollBurst)
}

// issueSession creates a persisted session bound to the caller's IP and agent.
// issueSession is the one funnel every way of signing in passes through —
// password, eID, ДАН, Google, a federated provider, a staff PIN — which is why
// the suspension check lives here rather than in each of them. An organisation
// the control plane has closed cannot be signed in to by any route, including
// one added next year by somebody who never read this file.
func (s *Server) issueSession(r *http.Request, userID, tenantID, method string) (string, time.Time, error) {
	if suspended, reason := s.tenantSuspended(r.Context(), tenantID); suspended {
		if reason != "" {
			return "", time.Time{}, fmt.Errorf("%w: %s", ErrTenantSuspended, reason)
		}
		return "", time.Time{}, ErrTenantSuspended
	}
	return s.sessions.Create(r.Context(), userID, tenantID, method,
		r.UserAgent(), security.ClientIP(r))
}

// reportSessionFailure answers a sign-in that could not produce a session.
//
// A suspended organisation is the caller's to know about — they will otherwise
// try the same password all afternoon — and everything else is ours. 403
// rather than 401 for the same reason authMiddleware uses it: signing in again
// is not the remedy.
func reportSessionFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrTenantSuspended) {
		httpx.Error(w, http.StatusForbidden, err.Error())
		return
	}
	slog.Error("could not establish a session", "error", err)
	httpx.Error(w, http.StatusInternalServerError, "failed to establish session")
}

// signInError carries a reason that is meant for the person signing in. Account
// linking also fails for reasons that are ours alone — a missing key, a broken
// query, a rejected hash — and those are logged, never rendered: the citizen
// once saw "bcrypt: password length exceeds 72 bytes" in the eID card.
type signInError struct{ msg string }

func (e signInError) Error() string { return e.msg }

// reportSignInFailure answers with the reason when it is the caller's to act
// on, and with a stable message otherwise.
func reportSignInFailure(w http.ResponseWriter, err error) {
	var visible signInError
	if errors.As(err, &visible) {
		httpx.Error(w, http.StatusForbidden, visible.Error())
		return
	}
	slog.Error("failed to link verified national identity", "error", err)
	httpx.Error(w, http.StatusInternalServerError, "Баталгаажсан eID хэрэглэгчийг "+config.BrandName()+" бүртгэлтэй холбож чадсангүй")
}

// eidLinkingDigest derives the stable, non-PII handle for an eID subject. It
// doubles as the synthetic account's password preimage, so its length is not
// cosmetic: bcrypt rejects anything over 72 bytes outright, and a suffix that
// pushed it to 73 once failed every first-time eID sign-in.
func eidLinkingDigest(linkingKey, subject string) string {
	mac := hmac.New(sha256.New, []byte(linkingKey))
	_, _ = mac.Write([]byte("eid-mn:" + subject))
	return fmt.Sprintf("%x", mac.Sum(nil))
}

// linkEIDIdentity records who a signed-in user is to eID Mongolia.
//
// Qualified remote signing addresses the citizen by their ETSI semantics
// identifier, and until this row exists nothing on the platform knows how to
// reach the phone of the person who just authenticated. Without it every
// signature would make the citizen retype the registration number they had
// just proved — and a typo there would push a PIN2 prompt at somebody else.
//
// It is best effort on purpose. Sign-in has already succeeded by this point,
// and failing the login because a convenience row could not be written would
// trade a working session for a missing one.
func (s *Server) linkEIDIdentity(ctx context.Context, userID string, identity *eid.EIDIdentity) {
	if identity == nil {
		return
	}
	subject := strings.TrimSpace(identity.CivilID)
	if subject == "" {
		subject = strings.TrimSpace(identity.RegNumber)
	}
	if subject == "" {
		return
	}
	personEtsi := eidmongolia.PersonEtsi(subject)

	// The conflict target is person_etsi as well as user_id: one eID citizen
	// resolves to one platform account, and a second account claiming the same
	// identifier would silently split that person's signing history in two.
	// The whole identity as eID returned it, beside the columns the sign-in
	// path reads. A person is entitled to see what was handed over about them,
	// and eID adds fields faster than this schema would follow.
	claims, err := json.Marshal(identity)
	if err != nil {
		slog.Warn("could not record what eID returned", "error", err)
		claims = []byte("{}")
	}
	if _, err := s.db.Exec(ctx,
		`INSERT INTO user_eid_identities
		     (user_id, civil_id, reg_number, person_etsi, given_name, surname, claims, last_seen_at)
		 VALUES ($1, NULLIF($2,''), NULLIF($3,''), $4, NULLIF($5,''), NULLIF($6,''), $7, NOW())
		 ON CONFLICT (user_id) DO UPDATE SET
		     civil_id     = COALESCE(EXCLUDED.civil_id, user_eid_identities.civil_id),
		     reg_number   = COALESCE(EXCLUDED.reg_number, user_eid_identities.reg_number),
		     person_etsi  = EXCLUDED.person_etsi,
		     given_name   = COALESCE(EXCLUDED.given_name, user_eid_identities.given_name),
		     surname      = COALESCE(EXCLUDED.surname, user_eid_identities.surname),
		     claims       = EXCLUDED.claims,
		     last_seen_at = NOW()`,
		userID, identity.CivilID, identity.RegNumber, personEtsi,
		identity.FirstName, identity.LastName, claims); err != nil {
		slog.Warn("could not link the eID identity to the platform account",
			"user_id", userID, "error", err)
	}
}

// eidPlaceholderName нь нэрээ олж чадаагүй eID дансны түр нэр. Дараагийн
// амжилттай нэрийн лавлагаагаар adoptEIDName үүнийг жинхэнэ нэрээр солино.
const eidPlaceholderName = "eID Mongolia хэрэглэгч"

// fillEIDNameFromLookup нь identity нэргүй ирсэн үед иргэнийг eID-ийн
// person/summary-гаас олж нэрийг нь identity дээр нөхнө. Best effort:
// лавлагаа унасан ч нэвтрэлт зогсохгүй — нэр нь л түр хоосон үлдэнэ.
func (s *Server) fillEIDNameFromLookup(ctx context.Context, identity *eid.EIDIdentity, subject string) {
	if identity == nil || strings.TrimSpace(identity.FirstName+identity.LastName) != "" {
		return
	}
	given, surname, err := s.eidSvc.PersonName(ctx, subject)
	if err != nil {
		slog.Warn("eID person lookup failed; the account keeps its placeholder name",
			"error", err)
		return
	}
	identity.FirstName, identity.LastName = given, surname
}

// adoptEIDName нь нэрээ олоогүй төрсөн дансыг дараагийн нэвтрэлтээр засна:
// users.name нь placeholder хэвээр л бол жинхэнэ нэрийг олж бичнэ. Хүний
// өөрөө засаад тавьсан нэрэнд хэзээ ч хүрэхгүй.
func (s *Server) adoptEIDName(ctx context.Context, userID, currentName string, identity *eid.EIDIdentity, subject string) {
	trimmed := strings.TrimSpace(currentName)
	if trimmed != "" && trimmed != eidPlaceholderName {
		return
	}
	s.fillEIDNameFromLookup(ctx, identity, subject)
	name := strings.TrimSpace(identity.LastName + " " + identity.FirstName)
	if name == "" {
		return
	}
	if _, err := s.db.Exec(ctx, `UPDATE users SET name=$2 WHERE id=$1`, userID, name); err != nil {
		slog.Warn("could not adopt the eID name", "user_id", userID, "error", err)
	}
}

// resolveOrProvisionEIDUser links an eID subject to a stable, non-PII local
// identifier. JIT provisioning is opt-in per tenant and always receives the
// standard user role through the membership_default_role database trigger.
func (s *Server) resolveOrProvisionEIDUser(ctx context.Context, identity *eid.EIDIdentity) (userID, tenantID string, err error) {
	subject := strings.TrimSpace(identity.CivilID)
	if subject == "" {
		subject = strings.TrimSpace(identity.RegNumber)
	}
	if subject == "" {
		return "", "", errors.New("eID identity carries neither a civil ID nor a registration number")
	}
	personEtsi := eidmongolia.PersonEtsi(subject)
	// Asked without joining memberships, for the reason firstTenantFor
	// explains: a citizen whose eID is linked here is the same citizen whether
	// or not they currently belong to an organisation, and letting the
	// membership decide made a linked identity read as an unknown one.
	var currentName string
	err = s.db.QueryRow(ctx,
		`SELECT ui.user_id::text, u.name FROM user_eid_identities ui
		   JOIN users u ON u.id = ui.user_id
		  WHERE ui.person_etsi=$1`, personEtsi).Scan(&userID, &currentName)
	if err == nil {
		s.adoptEIDName(ctx, userID, currentName, identity, subject)
		tenantID, err = s.firstTenantFor(ctx, userID)
		if err != nil {
			return "", "", err
		}
		return userID, tenantID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", err
	}
	linkingKey := os.Getenv("EID_RP_SECRET")
	if linkingKey == "" {
		return "", "", errors.New("EID_RP_SECRET is unset, so no account-linking key is available")
	}
	digest := eidLinkingDigest(linkingKey, subject)
	syntheticEmail := "eid+" + digest[:32] + "@identity.invalid"
	if err = s.db.QueryRow(ctx,
		`SELECT u.id::text, m.tenant_id::text, u.name FROM users u JOIN memberships m ON m.user_id=u.id WHERE u.email=$1
		 ORDER BY m.created_at, m.tenant_id LIMIT 1`,
		syntheticEmail).Scan(&userID, &tenantID, &currentName); err == nil {
		s.adoptEIDName(ctx, userID, currentName, identity, subject)
		return userID, tenantID, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", err
	}

	// The platform's access mode, before the environment's provisioning
	// tenant: a deployment that set EID_JIT_TENANT_SLUG once and has since
	// been closed should stop provisioning, and this is the order in which
	// that reads correctly.
	if err := mayProvisionAccount("eid"); err != nil {
		return "", "", err
	}
	tenantSlug := strings.TrimSpace(os.Getenv("EID_JIT_TENANT_SLUG"))
	if tenantSlug == "" {
		return "", "", signInError{"eID identity is verified but account provisioning is disabled"}
	}
	if err = s.db.QueryRow(ctx, `SELECT id::text FROM tenants WHERE slug=$1`, tenantSlug).Scan(&tenantID); err != nil {
		return "", "", fmt.Errorf("eID provisioning tenant %q is unavailable: %w", tenantSlug, err)
	}
	// Session нэр өгөөгүй бол иргэнээ person/summary-гаас олж авна («user
	// find») — эс бөгөөс данс нэргүй, «eID Mongolia хэрэглэгч» гэж төрнө.
	s.fillEIDNameFromLookup(ctx, identity, subject)
	name := strings.TrimSpace(identity.LastName + " " + identity.FirstName)
	if name == "" {
		name = eidPlaceholderName
	}
	// The synthetic account has no password login path. Keep the random-looking
	// preimage within bcrypt's strict 72-byte input limit.
	passwordHash, err := auth.HashPassword(digest)
	if err != nil {
		return "", "", err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = tx.QueryRow(ctx,
		`INSERT INTO users(email,password_hash,name,is_admin) VALUES($1,$2,$3,FALSE)
		 ON CONFLICT(email) DO UPDATE SET name=EXCLUDED.name RETURNING id::text`,
		syntheticEmail, passwordHash, name).Scan(&userID); err != nil {
		return "", "", err
	}
	// See the same check in sso_client_handlers.go: provisioning somebody on
	// their first eID sign-in is exactly the path by which an organisation
	// grows without anybody choosing to add a person.
	if err = s.checkUserQuota(ctx, tenantID); err != nil {
		return "", "", signInError{"Энэ байгууллага хэрэглэгчийн тооны хязгаартаа хүрсэн байна"}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO memberships(tenant_id,user_id) VALUES($1,$2) ON CONFLICT(tenant_id,user_id) DO NOTHING`, tenantID, userID); err != nil {
		return "", "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", err
	}
	return userID, tenantID, nil
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var name, email string
	var tenantName string
	_ = s.db.QueryRow(r.Context(), `SELECT name, email FROM users WHERE id = $1`, claims.UserID).Scan(&name, &email)
	_ = s.db.QueryRow(r.Context(), `SELECT name FROM tenants WHERE id = $1`, claims.TenantID).Scan(&tenantName)

	// The effective grant of every role the member holds, so a screen can hide
	// what the caller may not do. Administrators bypass the check, so their
	// list stays empty rather than enumerating the whole catalog.
	granted := make([]string, 0)
	if !claims.IsAdmin {
		if permissions, permErr := s.permissions.GetUserPermissions(r.Context(), claims.TenantID, claims.UserID); permErr == nil {
			for code := range permissions {
				granted = append(granted, code)
			}
			sort.Strings(granted)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":          claims.UserID,
		"tenant_id":   claims.TenantID,
		"tenant_name": tenantName,
		"name":        name,
		"email":       email,
		"is_admin":    claims.IsAdmin,
		"permissions": granted,
		// What the platform wants to tell this person right now: a maintenance
		// window, or an announcement an operator broadcast.
		"notices": s.notices(r.Context(), claims.TenantID),
		// Whether a platform operator is inside this account right now. The
		// shell draws a banner from it that cannot be dismissed — the person
		// whose screen this is has a right to know, and so does anybody
		// looking over their shoulder.
		"impersonated": claims.Impersonated,
	})
}
