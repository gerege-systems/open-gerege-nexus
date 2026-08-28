/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package auth

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

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/eid"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/eidmongolia"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/time/rate"
)

const (
	maxLoginBody       = 8 << 10
	maxLoginFailures   = 5
	loginLockoutWindow = 15 * time.Minute
)

var dummyPasswordHash = func() string {
	hash, err := HashPassword("constant-time-missing-user-placeholder")
	if err != nil {
		panic(err)
	}
	return hash
}()

func (h *Handlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBody)
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid login credentials")
		return
	}

	// The account, and nothing about where it works.
	//
	// This used to be one statement joining memberships, which picked the
	// oldest one and read is_admin from it. That was wrong from the day
	// migration 00085 landed: the join is inner, so an account belonging to no
	// organisation matched no row and the answer was "invalid email or
	// password" — for the exact people 00085 exists to let in. The eID and
	// federated paths never had the bug, because they ask FirstTenantFor
	// instead of joining; this one carried its own copy of the question and so
	// missed the change to the answer.
	//
	// Which workspace this session opens in is asked below, once the password
	// is known to be right, and by the one function that knows the rule.
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	var userID, passwordHash, tenantID, name string
	var isAdmin bool
	var lockedUntil pgtype.Timestamptz
	err := h.db.QueryRow(r.Context(),
		`SELECT u.id, u.password_hash, u.name, u.locked_until
		   FROM registry.users u
		  WHERE lower(u.email) = $1`, req.Email).Scan(&userID, &passwordHash, &name, &lockedUntil)

	if errors.Is(err, pgx.ErrNoRows) {
		passwordHash = dummyPasswordHash
	} else if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "login service unavailable")
		return
	}
	passwordOK := CheckPasswordHash(req.Password, passwordHash)
	locked := lockedUntil.Valid && lockedUntil.Time.After(time.Now())
	if !passwordOK || locked {
		if userID != "" && !locked {
			h.recordLoginFailure(r.Context(), userID)
		}
		audit.Record(r.Context(), audit.NoTenant, audit.Anonymous, "auth.login_failed", "user",
			map[string]any{"email": req.Email})
		telemetry.RecordLogin(telemetry.LoginPassword, false)
		httpx.Error(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	_, _ = h.db.Exec(r.Context(), `UPDATE registry.users SET failed_login_attempts=0, locked_until=NULL WHERE id=$1`, userID)

	// Where this session opens: the oldest organisation they belong to, or a
	// workspace of their own. Asked after the password rather than with it, so
	// a wrong password cannot make a workspace as a side effect of being typed.
	tenantID, err = h.FirstTenantFor(r.Context(), userID)
	if err != nil {
		slog.Error("could not open a workspace for a signed-in account", "user_id", userID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, "login service unavailable")
		return
	}
	// Administrator of the workspace being opened, which is a question about
	// that workspace and not about the account. In a personal workspace the
	// answer is no: nobody administers a space with one person in it.
	//
	// Guarded against an empty id even though FirstTenantFor no longer returns
	// one. An empty string is not a uuid, and Postgres refuses the whole
	// statement rather than the row — which reached production as a 500 on the
	// sign-in of everybody who belonged to no organisation, for the day the
	// workspace was allowed to be absent. The guard costs a comparison and
	// makes the failure impossible rather than unlikely.
	if tenantID != "" {
		if err := h.db.QueryRow(r.Context(),
			`SELECT EXISTS (
			     SELECT 1 FROM workspace.memberships m
			     JOIN workspace.membership_roles mr ON mr.membership_id = m.id
			     JOIN workspace.roles ro ON ro.id = mr.role_id
			    WHERE m.user_id = $1::uuid AND m.tenant_id = $2::uuid
			      AND ro.tenant_id = m.tenant_id AND ro.code = 'admin' AND ro.active)`,
			userID, tenantID).Scan(&isAdmin); err != nil {
			slog.Error("could not read the signed-in account's roles", "user_id", userID, "error", err)
			httpx.Error(w, http.StatusInternalServerError, "login service unavailable")
			return
		}
	}

	token, expiresAt, err := h.IssueSession(r, userID, tenantID, "password")
	if err != nil {
		ReportSessionFailure(w, err)
		return
	}
	SetSessionCookie(w, token, expiresAt)

	audit.Record(r.Context(), tenantID, userID, "auth.login_success", "user", map[string]any{"email": req.Email})
	telemetry.RecordLogin(telemetry.LoginPassword, true)

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
// Callers must not use it on an account that is currently locked; HandleLogin
// checks that before calling, so a live lockout is never extended by attempts
// made during it.
const loginFailureStatement = `
	UPDATE registry.users u SET
	   failed_login_attempts = next.count,
	   locked_until = CASE WHEN next.count >= $2 THEN NOW() + $3::interval END
	  FROM (
	      SELECT CASE WHEN locked_until IS NOT NULL AND locked_until <= NOW()
	                  THEN 1 ELSE failed_login_attempts + 1 END AS count
	        FROM registry.users WHERE id = $1
	  ) AS next
	 WHERE u.id = $1`

// recordLoginFailure applies loginFailureStatement. A failure to record one is
// logged rather than surfaced: the caller is already answering 401, and turning
// a bookkeeping error into a 500 would tell whoever is guessing that this
// address exists.
func (h *Handlers) recordLoginFailure(ctx context.Context, userID string) {
	if _, err := h.db.Exec(ctx, loginFailureStatement,
		userID, maxLoginFailures, loginLockoutWindow.String()); err != nil {
		slog.Error("failed to record a login failure", "user_id", userID, "error", err)
	}
}

func (h *Handlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// Logout previously only cleared the cookie; the token stayed valid
	// forever for anyone who had captured it.
	if token := TokenFromRequest(r); token != "" {
		if err := h.sessions.Revoke(r.Context(), token); err != nil {
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
	if h.endSession != nil {
		endSession = h.endSession(w, r)
	}

	ClearSessionCookie(w)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "logged_out",
		// Empty unless this deployment federates and its provider offers
		// RP-initiated logout. A client that finds it empty is already done.
		"end_session_url": endSession,
	})
}

// HandleTenants answers which tenants the caller may act for.
//
// A user with one membership gets a list of one. That is not a wasted answer:
// it is what lets the client show where it is without claiming there is
// somewhere else to go.
func (h *Handlers) HandleTenants(w http.ResponseWriter, r *http.Request) {
	claims, err := UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	options, err := h.sessions.TenantsForUser(nexus.WithoutWorkspace(r.Context()), claims.UserID)
	if err != nil {
		slog.Error("failed to list the tenants a user belongs to", "user_id", claims.UserID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, "failed to list tenants")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"current": claims.WorkspaceID,
		"tenants": options,
		// Which of them this session is reading across. Empty would be
		// ambiguous on the client — "none" and "just the current one" look the
		// same — so the acting organisation is always named.
		"active": activeOrCurrent(claims),
	})
}

func activeOrCurrent(claims UserClaims) []string {
	if len(claims.AllowedWorkspaceIDs) > 0 {
		return claims.AllowedWorkspaceIDs
	}
	return []string{claims.WorkspaceID}
}

// HandleSetActiveTenants chooses which organisations this session reads across.
//
// The list is a request, not an instruction: every id is held against an active
// membership and the ones that do not survive are dropped rather than refused,
// so a tab left open since before somebody changed jobs narrows quietly instead
// of failing. The organisation being acted in is always kept.
func (h *Handlers) HandleSetActiveTenants(w http.ResponseWriter, r *http.Request) {
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

	token := TokenFromRequest(r)
	if token == "" {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	active, err := h.sessions.SetActiveTenants(r.Context(), token, body.TenantIDs)
	if err != nil {
		if errors.Is(err, ErrSessionInvalid) {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		slog.Error("could not set the active organisations", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "could not save the selection")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"active": active})
}

// HandleSwitchTenant moves the caller's session to another of their tenants.
//
// Until this existed, which tenant a person acted in was decided once, at
// login, by whichever membership was oldest — someone who works for two
// organisations could reach only one of them, and the only way to change that
// was to edit the database. Signing out and back in would not have helped: the
// login picks the same oldest membership every time, deliberately.
func (h *Handlers) HandleSwitchTenant(w http.ResponseWriter, r *http.Request) {
	claims, err := UserFromContext(r.Context())
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
	if req.TenantID == claims.WorkspaceID {
		httpx.JSON(w, http.StatusOK, map[string]any{"tenant_id": claims.WorkspaceID, "switched": false})
		return
	}

	token, expiresAt, err := h.sessions.SwitchTenant(nexus.WithoutWorkspace(r.Context()), TokenFromRequest(r), req.TenantID)
	switch {
	case errors.Is(err, ErrNotAMember):
		// Not 404: whether that tenant exists is not this caller's business.
		httpx.Error(w, http.StatusForbidden, "no membership in that tenant")
		return
	case errors.Is(err, ErrSessionInvalid):
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	case err != nil:
		slog.Error("failed to switch tenant", "user_id", claims.UserID, "tenant_id", req.TenantID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, "failed to switch tenant")
		return
	}

	SetSessionCookie(w, token, expiresAt)
	// Recorded against the tenant being left, because that is the trail an
	// administrator of it reads: this person stopped acting here, and when.
	audit.Record(r.Context(), claims.WorkspaceID, claims.UserID, "auth.tenant_switched", "session",
		map[string]any{"from": claims.WorkspaceID, "to": req.TenantID})
	httpx.JSON(w, http.StatusOK, map[string]any{
		"tenant_id":  req.TenantID,
		"switched":   true,
		"expires_at": expiresAt,
	})
}

// HandleLogoutEverywhere ends every session the caller holds.
//
// Logging out ends the session in front of you. This is for the one you left
// somewhere else — a machine in an office you have left, a desktop client on a
// laptop that was taken — and it is the only thing on the platform that answers
// that, short of an administrator disabling the account.
//
// It is inside the authenticated group, so the caller is proving they hold one
// of the sessions they are ending. The cookie here is cleared too: the session
// it names is among those just revoked.
func (h *Handlers) HandleLogoutEverywhere(w http.ResponseWriter, r *http.Request) {
	claims, err := UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	revoked, err := h.sessions.RevokeAllForUser(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("failed to revoke every session", "user_id", claims.UserID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, "failed to end the other sessions")
		return
	}

	ClearSessionCookie(w)
	audit.Record(r.Context(), claims.WorkspaceID, claims.UserID, "auth.logout_everywhere", "session",
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
	LoginRatePerMinute = 5
	LoginBurst         = 5
	PollRatePerMinute  = 60
	PollBurst          = 15

	// The AI budget was here until 2026-08-23. It moved to the app that spends
	// it — internal/apps/ai — which asks for the same numbers through
	// nexus.RateLimit, so the deployment-wide bucket is still one bucket.
	//
	// The verification endpoint spends a mailbox credential the whole platform
	// shares, which is why it is budgeted and why it stays here: it is the
	// platform's own route.
	VerifyRatePerMinute = 60
	VerifyBurst         = 20
)

func NewLoginLimiter() *security.IPRateLimiter {
	return security.NewIPRateLimiter(rate.Limit(float64(LoginRatePerMinute)/60.0), LoginBurst)
}

func NewPollLimiter() *security.IPRateLimiter {
	return security.NewIPRateLimiter(rate.Limit(float64(PollRatePerMinute)/60.0), PollBurst)
}

// IssueSession creates a persisted session bound to the caller's IP and agent.
// IssueSession is the one funnel every way of signing in passes through —
// password, eID, ДАН, Google, a federated provider, a staff PIN — which is why
// the suspension check lives here rather than in each of them. An organisation
// the control plane has closed cannot be signed in to by any route, including
// one added next year by somebody who never read this file.
func (h *Handlers) IssueSession(r *http.Request, userID, tenantID, method string) (string, time.Time, error) {
	if suspended, reason := h.TenantSuspended(r.Context(), tenantID); suspended {
		if reason != "" {
			return "", time.Time{}, fmt.Errorf("%w: %s", ErrTenantSuspended, reason)
		}
		return "", time.Time{}, ErrTenantSuspended
	}
	return h.sessions.Create(r.Context(), userID, tenantID, method,
		r.UserAgent(), security.ClientIP(r))
}

// ReportSessionFailure answers a sign-in that could not produce a session.
//
// A suspended organisation is the caller's to know about — they will otherwise
// try the same password all afternoon — and everything else is ours. 403
// rather than 401 for the same reason Middleware uses it: signing in again
// is not the remedy.
func ReportSessionFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrTenantSuspended) {
		httpx.Error(w, http.StatusForbidden, err.Error())
		return
	}
	slog.Error("could not establish a session", "error", err)
	httpx.Error(w, http.StatusInternalServerError, "failed to establish session")
}

// SignInError carries a reason that is meant for the person signing in. Account
// linking also fails for reasons that are ours alone — a missing key, a broken
// query, a rejected hash — and those are logged, never rendered: the citizen
// once saw "bcrypt: password length exceeds 72 bytes" in the eID card.
type SignInError struct{ msg string }

func (e SignInError) Error() string { return e.msg }

// NewSignInError carries a reason that is meant for the person signing in. The
// field stays unexported so that the only way to make one is to decide that the
// sentence is theirs to read.
func NewSignInError(msg string) SignInError { return SignInError{msg: msg} }

// ReportSignInFailure answers with the reason when it is the caller's to act
// on, and with a stable message otherwise.
func ReportSignInFailure(w http.ResponseWriter, err error) {
	var visible SignInError
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

// LinkEIDIdentity records who a signed-in user is to eID Mongolia.
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
func (h *Handlers) LinkEIDIdentity(ctx context.Context, userID string, identity *eid.EIDIdentity) {
	if err := h.LinkEIDIdentityStrict(ctx, userID, identity); err != nil {
		slog.Warn("could not link the eID identity to the platform account",
			"user_id", userID, "error", err)
	}
}

// ErrEIDBelongsToSomebodyElse is this citizen's eID already being an account
// here, and not this one.
//
// One eID is one person. The unique index on person_etsi says so, and this is
// what that refusal reads as by the time somebody sees it: not "database
// error" but "this identity is already somebody's here", which is a thing they
// can act on — by signing in as that account instead.
var ErrEIDBelongsToSomebodyElse = errors.New("that eID identity is already linked to another account")

// LinkEIDIdentityStrict is the same write, for a caller who needs to know.
//
// The wrapper above is deliberately best-effort: it runs after a sign-in has
// already succeeded, and failing the login because a convenience row could not
// be written would trade a working session for a missing one. Somebody who
// pressed "link my eID" on their profile is in the opposite position — the
// write is the entire point of what they asked for, and silence would leave
// them looking at a screen that never changes.
func (h *Handlers) LinkEIDIdentityStrict(ctx context.Context, userID string, identity *eid.EIDIdentity) error {
	if identity == nil {
		return errors.New("no eID identity to link")
	}
	subject := strings.TrimSpace(identity.CivilID)
	if subject == "" {
		subject = strings.TrimSpace(identity.RegNumber)
	}
	if subject == "" {
		return errors.New("the eID identity carries neither a civil ID nor a registration number")
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
	if _, err := h.db.Exec(ctx,
		`INSERT INTO registry.user_eid_identities
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
		// The unique index on person_etsi, arriving as words. ON CONFLICT
		// names user_id, so a row held by a different account is not updated —
		// it collides, which is the schema refusing to split one citizen's
		// history across two accounts.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrEIDBelongsToSomebodyElse
		}
		return fmt.Errorf("link the eID identity: %w", err)
	}
	// The Gerege number goes on the account itself, not only into the claims
	// blob beside it: it is what the OIDC provider hands downstream and what a
	// second sign-in finds this citizen by, and neither can read a JSON column.
	h.rememberGeID(ctx, userID, identity)
	return nil
}

// ResolveOrProvisionEIDUser links an eID subject to a stable, non-PII local
// identifier. JIT provisioning is opt-in per tenant and always receives the
// standard user role through the membership_default_role database trigger.
func (h *Handlers) ResolveOrProvisionEIDUser(ctx context.Context, identity *eid.EIDIdentity) (userID, tenantID string, err error) {
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
	err = h.db.QueryRow(ctx,
		`SELECT user_id::text FROM registry.user_eid_identities WHERE person_etsi=$1`, personEtsi).Scan(&userID)
	if err == nil {
		tenantID, err = h.FirstTenantFor(ctx, userID)
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
	// The address this account is known by.
	//
	// It used to be `eid+<32 hex>@identity.invalid` — correct, unique, and
	// meaningless to everybody who ever read it, including the relying parties
	// this platform hands it to over OIDC. When eID gives us the citizen's
	// Gerege number the address is that number instead, which is the same
	// person written in a way somebody can recognise.
	//
	// The old form is still computed, because accounts created under it exist
	// and are found by it below.
	syntheticEmail := "eid+" + digest[:32] + "@identity.invalid"
	email := syntheticEmail
	if identity.GeID != 0 {
		email = GeIDEmail(identity.GeID)
	}

	// Neither lookup below joins memberships, and that is the whole of this
	// paragraph. Both did, and it made "who is this person" depend on where
	// they happen to work: an account with no membership matched no row, was
	// read as a stranger, and fell through to the provisioning branch — which
	// on a private deployment refuses, so the citizen could not sign in at all.
	//
	// It is the third time this repository has made this mistake. HandleLogin
	// carried an inner join onto memberships and told people with none
	// "invalid email or password"; the same shape appeared here twice, in two
	// copies that had to be found separately. Identity is answered by identity;
	// where somebody stands is a second question with one answer, and
	// FirstTenantFor is it.

	// By the Gerege number first: it is the identifier the ecosystem shares, so
	// an account that already carries it is this citizen whatever address it
	// was opened under.
	if identity.GeID != 0 {
		if err = h.db.QueryRow(ctx,
			`SELECT id::text FROM registry.users WHERE ge_id=$1`,
			identity.GeID).Scan(&userID); err == nil {
			tenantID, err = h.FirstTenantFor(ctx, userID)
			if err != nil {
				return "", "", err
			}
			return userID, tenantID, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return "", "", err
		}
	}

	// Then by either address. An account opened before the Gerege number
	// existed keeps its row and is upgraded in place by rememberGeID, rather
	// than being left behind as a second copy of the same citizen.
	if err = h.db.QueryRow(ctx,
		`SELECT id::text FROM registry.users WHERE email = ANY($1) ORDER BY created_at LIMIT 1`,
		[]string{syntheticEmail, email}).Scan(&userID); err == nil {
		tenantID, err = h.FirstTenantFor(ctx, userID)
		if err != nil {
			return "", "", err
		}
		return userID, tenantID, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", err
	}

	// Whether this deployment opens accounts to strangers at all. It is the
	// only question left here: where the account then lives used to be the
	// second one, answered by EID_JIT_TENANT_SLUG, and migration 00085 removed
	// the need to ask it.
	if err := MayProvisionAccount("eid"); err != nil {
		return "", "", err
	}
	name := strings.TrimSpace(identity.LastName + " " + identity.FirstName)
	if name == "" {
		name = "eID Mongolia хэрэглэгч"
	}

	// The address the new account is opened under.
	//
	// eID hands over an email and a telephone number along with the name, and
	// until now both were thrown away: the account was opened under a
	// synthesised address and the citizen's own details survived only inside
	// the claims blob, where no screen and no query can reach them. Somebody
	// signing in for the first time landed on a profile that knew their name
	// and nothing else they had just been asked to share.
	//
	// The address eID gives is used only to OPEN an account, never to FIND
	// one. That distinction is the whole of the safety here: the lookups above
	// stay on the Gerege number and the synthesised address, both of which
	// this platform derives itself. eID's email is what the citizen told the
	// civil registry, not proof they control that mailbox today — matching on
	// it would let anybody holding an eID walk into an account opened by
	// somebody who once used the same address.
	//
	// And only when it is free. registry.users.email is unique, so a collision
	// would fail the insert; falling back keeps the sign-in working and leaves
	// the address with whoever already had it.
	accountEmail := email
	if candidate := strings.ToLower(strings.TrimSpace(identity.Email)); candidate != "" {
		var taken bool
		if err := h.db.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM registry.users WHERE lower(email) = $1)`, candidate).Scan(&taken); err == nil && !taken {
			accountEmail = candidate
		}
	}
	// The synthetic account has no password login path. Keep the random-looking
	// preimage within bcrypt's strict 72-byte input limit.
	passwordHash, err := HashPassword(digest)
	if err != nil {
		return "", "", err
	}
	tx, err := h.db.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = tx.QueryRow(ctx,
		`INSERT INTO registry.users(email,password_hash,name,phone,is_admin,ge_id)
		 VALUES($1,$2,$3,COALESCE(NULLIF($4,''),''),FALSE,NULLIF($5,0)::bigint)
		 ON CONFLICT(email) DO UPDATE SET
		     name  = EXCLUDED.name,
		     -- Байгаа дугаарыг хоосноор дарахгүй: eID энэ удаа утас
		     -- буцаагаагүй нь хүн түүнийгээ устгасан гэсэн үг биш.
		     phone = COALESCE(NULLIF(EXCLUDED.phone,''), registry.users.phone)
		 RETURNING id::text`,
		accountEmail, passwordHash, name, identity.Phone, identity.GeID).Scan(&userID); err != nil {
		return "", "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", err
	}

	// The citizen lands in a workspace of their own, and there is no
	// organisation's quota to check because no organisation gained a member.
	// That check used to be here with a comment saying what was wrong with the
	// thing it was guarding:
	//
	//	provisioning somebody on their first eID sign-in is exactly the path by
	//	which an organisation grows without anybody choosing to add a person.
	//
	// The path is gone rather than guarded. A personal workspace is one person
	// by construction — registry.tenants.owner_user_id, one row per person — so
	// there is nothing there for it to grow.
	tenantID, err = h.FirstTenantFor(ctx, userID)
	if err != nil {
		return "", "", err
	}
	return userID, tenantID, nil
}

// WorkspaceKindNone is what /api/v1/auth/me reports for a session standing in
// no workspace: signed in, belonging to no organisation.
//
// A named value because it crosses the wire into the shell, where it decides
// which rail is drawn. Exported so the tests on both sides of that wire can
// name it rather than spelling the string twice and finding out in a browser.
const WorkspaceKindNone = "none"

func (h *Handlers) HandleMe(w http.ResponseWriter, r *http.Request) {
	claims, err := UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var name, email string
	var tenantName, workspaceKind string
	_ = h.db.QueryRow(r.Context(), `SELECT name, email FROM registry.users WHERE id = $1`, claims.UserID).Scan(&name, &email)
	// kind comes back with the name because the shell decides which of two
	// layouts to draw from it, and a second round trip for one word on the
	// request every screen makes first is a second round trip on every screen.
	if claims.WorkspaceID != "" {
		_ = h.db.QueryRow(r.Context(), `SELECT name, kind FROM registry.tenants WHERE id = $1`,
			claims.WorkspaceID).Scan(&tenantName, &workspaceKind)
	} else {
		// Signed in and standing in no workspace, which since 00094 is what
		// belonging to no organisation looks like.
		//
		// A word rather than an empty string, because the shell has to tell
		// three states apart and two of them are falsy: "the answer has not
		// arrived", "there is no workspace", and "there is one". An empty
		// string collapses the first two, and the screen it would draw for a
		// person who is merely still loading is a citizen's — briefly, on every
		// sign-in, including an administrator's.
		workspaceKind = WorkspaceKindNone
	}

	// The effective grant of every role the member holds, so a screen can hide
	// what the caller may not do. Administrators bypass the check, so their
	// list stays empty rather than enumerating the whole catalog.
	granted := make([]string, 0)
	if !claims.IsAdmin {
		if permissions, permErr := h.permissions.GetUserPermissions(r.Context(), claims.WorkspaceID, claims.UserID); permErr == nil {
			for code := range permissions {
				granted = append(granted, code)
			}
			sort.Strings(granted)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":          claims.UserID,
		"tenant_id":   claims.WorkspaceID,
		"tenant_name": tenantName,
		// "organisation", "personal", or "none" for a session standing in no
		// workspace at all. The wire keeps saying tenant for the two fields
		// above, which existed before the word changed; this one is new, so it
		// is named for what the code now calls the thing.
		"workspace_kind": workspaceKind,
		"name":           name,
		"email":          email,
		"is_admin":       claims.IsAdmin,
		"permissions":    granted,
		// What the platform wants to tell this person right now: a Maintenance
		// window, or an announcement an operator broadcast.
		"Notices": h.Notices(r.Context(), claims.WorkspaceID),
		// Whether a platform operator is inside this account right now. The
		// shell draws a banner from it that cannot be dismissed — the person
		// whose screen this is has a right to know, and so does anybody
		// looking over their shoulder.
		"impersonated": claims.Impersonated,
	})
}
