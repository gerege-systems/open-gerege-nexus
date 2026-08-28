package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/dbguard"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// The console's guarantees are database guarantees, so most of them cannot be
// tested without one: that the operator role sees organisations but cannot
// write to them, that the audit table refuses to be edited, that a code works
// once. These skip without a database, the way the rest of this repository's
// database tests do — and CI sets DATABASE_URL, so they run there.

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("CONTROLPLANE_TEST_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Skip("neither CONTROLPLANE_TEST_DATABASE_URL nor DATABASE_URL is set")
	}

	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse the database url: %v", err)
	}
	// With the guard installed and probed, exactly as the running process has
	// it. Without this the operator role would never be assumed and every
	// assertion below about what the console cannot do would pass for the wrong
	// reason.
	guard := &dbguard.Guard{}
	guard.Install(config)

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("database unreachable: %v", err)
	}
	if err := guard.Probe(context.Background(), pool); err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !guard.OperatorReady() {
		t.Skip("the control plane's database role is not installed (run the migrations)")
	}
	return pool
}

// codeAt renders the code an authenticator would show at that moment.
func codeAt(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	code, err := totp.GenerateCodeCustom(secret, at, totp.ValidateOpts{
		Period: TOTPPeriod, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("generate a code: %v", err)
	}
	return code
}

// newOperator creates a confirmed account and returns it with its secret.
//
// The address carries the test's name and the clock, because operator_audit is
// append-only — the rows this leaves behind cannot be deleted afterwards, so
// every run has to be able to tell its own rows from the last run's. That is
// not a wart in the test, it is the table doing what it was built to do.
func newOperator(t *testing.T, pool *pgxpool.Pool, role Role) (Operator, string) {
	t.Helper()
	ctx := context.Background()

	email := fmt.Sprintf("%s+%d@controlplane.test", strings.ToLower(t.Name()), time.Now().UnixNano())
	operator, enrolment, err := CreateOperator(ctx, pool, NewOperator{
		Email:    email,
		Name:     "Test Operator",
		Role:     role,
		Password: "correct horse battery",
	})
	if err != nil {
		t.Fatalf("create the operator: %v", err)
	}
	t.Cleanup(func() {
		// The account goes; its audit rows stay, and cannot be removed.
		_, _ = pool.Exec(context.Background(), `DELETE FROM operator.operator_accounts WHERE id = $1::uuid`, operator.ID)
	})

	// Confirmed with the previous step's code, which the skew window accepts.
	// It leaves the account's high-water mark one step behind now, so the tests
	// below can sign in with the current code and step up with the next one —
	// without waiting thirty seconds between each, and without pretending a
	// code can be reused, which is the property being protected.
	//
	// Tried twice, and the second attempt is not superstition. This is the one
	// call in the file that asks for a code at the *edge* of the skew window,
	// and the edge can move underneath it: the code is generated here and
	// judged inside ConfirmSecondFactor against that function's own clock, so
	// a thirty-second boundary falling in the gap makes "one step back" arrive
	// as two, which is exactly what the window refuses. The gap is a database
	// round trip — microseconds on a laptop, long enough on a loaded CI runner
	// to catch a boundary every few hundred runs.
	//
	// Recomputing after the boundary has passed lands in a fresh step, so one
	// retry is enough; a second failure is a real one and is reported as
	// before. The alternative, sleeping until the step has room, costs every
	// test in this file up to thirty seconds to fix a race that costs nothing
	// to retry.
	//
	// The other edge case, `now + TOTPPeriod` in the step-up test, is safe for
	// the opposite reason: a boundary crossing there moves the code from one
	// step ahead to the current one, and the window accepts both.
	var confirmErr error
	for attempt := range 2 {
		confirmErr = ConfirmSecondFactor(ctx, pool, operator.ID,
			codeAt(t, enrolment.Secret, time.Now().Add(-TOTPPeriod*time.Second)))
		if confirmErr == nil {
			break
		}
		if attempt == 0 {
			t.Logf("confirming the second factor missed the skew window; a "+
				"thirty-second boundary fell inside the call. Retrying: %v", confirmErr)
		}
	}
	if confirmErr != nil {
		t.Fatalf("confirm the second factor: %v", confirmErr)
	}
	return operator, enrolment.Secret
}

// An account that exists but never confirmed its authenticator cannot sign in,
// whatever password is typed. It is the state an interrupted bootstrap leaves
// behind, and it must be a locked door rather than a password-only one.
func TestSignInRefusesAnUnconfirmedAccount(t *testing.T) {
	pool := openPool(t)
	service := New(pool)

	email := fmt.Sprintf("unconfirmed+%d@controlplane.test", time.Now().UnixNano())
	operator, enrolment, err := CreateOperator(context.Background(), pool, NewOperator{
		Email: email, Name: "Unconfirmed", Role: RoleOperator, Password: "correct horse battery",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operator.operator_accounts WHERE id = $1::uuid`, operator.ID)
	})

	recorder := signIn(t, service, email, "correct horse battery", codeAt(t, enrolment.Secret, time.Now()))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("an unconfirmed account signed in: %d", recorder.Code)
	}
}

// The whole sign-in path, end to end: password, code, session, audit row — and
// then the same code again, which must not work twice.
func TestSignInIssuesASessionAndRecordsIt(t *testing.T) {
	pool := openPool(t)
	service := New(pool)
	operator, secret := newOperator(t, pool, RoleOperator)

	// The current code, one step past the one enrolment consumed.
	code := codeAt(t, secret, time.Now())

	recorder := signIn(t, service, operator.Email, "correct horse battery", code)
	if recorder.Code != http.StatusOK {
		t.Fatalf("sign-in answered %d: %s", recorder.Code, recorder.Body.String())
	}

	token := sessionCookie(t, recorder)
	session, err := service.sessions.Resolve(context.Background(), token)
	if err != nil {
		t.Fatalf("the issued token does not resolve: %v", err)
	}
	if session.ID != operator.ID || session.Role != RoleOperator {
		t.Fatalf("the session names the wrong operator: %+v", session)
	}
	// Signing in proves the second factor, so the session starts stepped up —
	// otherwise the first action of every session asks for a code typed
	// seconds earlier.
	if !session.SteppedUp(time.Now()) {
		t.Fatal("a fresh session was not stepped up")
	}

	if got := auditCount(t, pool, operator.ID, "operator.session.begin"); got != 1 {
		t.Fatalf("sign-in wrote %d audit rows, want 1", got)
	}

	// The same code, again. A code that stays usable for the rest of its
	// thirty seconds is a code somebody standing behind you can use.
	replay := signIn(t, service, operator.Email, "correct horse battery", code)
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("a code was accepted twice: %d", replay.Code)
	}
	if got := auditCount(t, pool, operator.ID, "operator.session.denied"); got != 1 {
		t.Fatalf("the refused attempt wrote %d audit rows, want 1", got)
	}
}

func TestSignInRefusesTheWrongPasswordAndLocksOut(t *testing.T) {
	pool := openPool(t)
	service := New(pool)
	operator, secret := newOperator(t, pool, RoleOperator)

	for attempt := 0; attempt < maxLoginFailures; attempt++ {
		recorder := signIn(t, service, operator.Email, "not the password", codeAt(t, secret, time.Now()))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d answered %d", attempt, recorder.Code)
		}
	}

	var locked bool
	if err := pool.QueryRow(context.Background(),
		`SELECT locked_until IS NOT NULL AND locked_until > NOW() FROM operator.operator_accounts WHERE id = $1::uuid`,
		operator.ID).Scan(&locked); err != nil {
		t.Fatalf("read the lockout: %v", err)
	}
	if !locked {
		t.Fatalf("%d failures did not lock the account", maxLoginFailures)
	}

	// The right password now too, because the lockout is about the account and
	// not about the guess.
	recorder := signIn(t, service, operator.Email, "correct horse battery", codeAt(t, secret, time.Now()))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("a locked account signed in: %d", recorder.Code)
	}
}

// §2.5: the audit trail is append-only, and it is the database that says so —
// not a convention in the Go code, which is exactly what a compromised or
// careless writer would ignore.
func TestOperatorAuditCannotBeRewritten(t *testing.T) {
	pool := openPool(t)
	operator, _ := newOperator(t, pool, RoleOperator)
	ctx := context.Background()

	// As the owning login role, which outranks every GRANT. If this is refused,
	// nothing in the process can rewrite the trail.
	if _, err := pool.Exec(ctx,
		`UPDATE operator.operator_audit SET reason = 'rewritten' WHERE operator_id = $1::uuid`,
		operator.ID); err == nil {
		t.Fatal("an audit row was updated")
	}
	if _, err := pool.Exec(ctx,
		`DELETE FROM operator.operator_audit WHERE operator_id = $1::uuid`, operator.ID); err == nil {
		t.Fatal("an audit row was deleted")
	}
}

// What the operator's database role may and may not do, asked of the database
// rather than of the handlers. This is the check that keeps "the console reads
// every organisation" from quietly becoming "the console can do anything".
func TestOperatorRoleReadsButCannotWrite(t *testing.T) {
	pool := openPool(t)
	ctx := dbguard.AsOperator(context.Background())

	// Reads across organisations: the row-level policies admit this role for
	// SELECT, on the tables migration 00049 names.
	var tenants int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM registry.tenants`).Scan(&tenants); err != nil {
		t.Fatalf("the operator role cannot read organisations: %v", err)
	}
	var memberships int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM workspace.memberships`).Scan(&memberships); err != nil {
		t.Fatalf("the operator role cannot read memberships: %v", err)
	}

	// And what it may not write, it may not write — refused by the grants
	// rather than by a WHERE clause somebody has to remember.
	//
	// Creating an organisation is *not* on this list, because CP-2 gave the
	// console that on purpose: opening one is reversible, and removing one is
	// the thing that is not.
	if _, err := pool.Exec(ctx, `DELETE FROM registry.tenants WHERE slug = 'nothing-matching'`); err == nil {
		t.Fatal("the operator role can delete organisations")
	}
	// The column grant is the sharp one: the console may write a person's
	// lockout state and nothing else about them, so this is refused even
	// though the row is one it can read and partly update.
	if _, err := pool.Exec(ctx, `UPDATE registry.users SET name = name`); err == nil {
		t.Fatal("the operator role updated a person's record")
	}

	// A table nobody granted it — the tenants' own data, which the console has
	// no business reading. Reaching for it fails at the door.
	if _, err := pool.Exec(ctx, `SELECT count(*) FROM contacts`); err == nil {
		t.Fatal("the operator role read a tenant's contacts")
	}
}

// Step-up is what CP-2's dangerous actions will sit behind, so the mechanism is
// exercised now: a code moves the session into the window, and the same code
// cannot be used to do it twice.
func TestStepUpConfirmsAndCannotBeReplayed(t *testing.T) {
	pool := openPool(t)
	service := New(pool)
	operator, secret := newOperator(t, pool, RoleSuperadmin)

	signInRecorder := signIn(t, service, operator.Email, "correct horse battery",
		codeAt(t, secret, time.Now()))
	if signInRecorder.Code != http.StatusOK {
		t.Fatalf("sign-in answered %d: %s", signInRecorder.Code, signInRecorder.Body.String())
	}
	token := sessionCookie(t, signInRecorder)

	session, err := service.sessions.Resolve(context.Background(), token)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// The next step: later than the one sign-in consumed, and still inside the
	// window verifyTOTP accepts.
	code := codeAt(t, secret, time.Now().Add(TOTPPeriod*time.Second))
	recorder := stepUp(t, service, session, code)
	if recorder.Code != http.StatusOK {
		t.Fatalf("step-up answered %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := auditCount(t, pool, operator.ID, "operator.step_up"); got != 1 {
		t.Fatalf("step-up wrote %d audit rows, want 1", got)
	}

	if replay := stepUp(t, service, session, code); replay.Code != http.StatusUnauthorized {
		t.Fatalf("a step-up code was accepted twice: %d", replay.Code)
	}
}

// Signing out ends the session immediately, rather than leaving a token that
// keeps working until it expires eight hours later.
func TestSignOutRevokesTheSession(t *testing.T) {
	pool := openPool(t)
	service := New(pool)
	operator, secret := newOperator(t, pool, RoleSupport)

	recorder := signIn(t, service, operator.Email, "correct horse battery", codeAt(t, secret, time.Now()))
	if recorder.Code != http.StatusOK {
		t.Fatalf("sign-in answered %d: %s", recorder.Code, recorder.Body.String())
	}
	token := sessionCookie(t, recorder)
	session, err := service.sessions.Resolve(context.Background(), token)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	request := httptest.NewRequest(http.MethodDelete, "/api/platform/v1/session", nil)
	request = request.WithContext(withSession(request.Context(), session))
	out := httptest.NewRecorder()
	service.HandleLogout(out, request)
	if out.Code != http.StatusOK {
		t.Fatalf("sign-out answered %d", out.Code)
	}

	if _, err := service.sessions.Resolve(context.Background(), token); err == nil {
		t.Fatal("a revoked token still resolves")
	}
}

// Helpers.

func signIn(t *testing.T, service *Console, email, password, code string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": password, "code": code})
	request := httptest.NewRequest(http.MethodPost, "/api/platform/v1/session", strings.NewReader(string(body)))
	recorder := httptest.NewRecorder()
	service.HandleLogin(recorder, request)
	return recorder
}

func stepUp(t *testing.T, service *Console, session Session, code string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"code": code})
	request := httptest.NewRequest(http.MethodPost, "/api/platform/v1/step-up", strings.NewReader(string(body)))
	request = request.WithContext(withSession(request.Context(), session))
	recorder := httptest.NewRecorder()
	service.HandleStepUp(recorder, request)
	return recorder
}

func withSession(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, session)
}

func sessionCookie(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == SessionCookieName && cookie.Value != "" {
			return cookie.Value
		}
	}
	t.Fatal("the response carried no session cookie")
	return ""
}

func auditCount(t *testing.T, pool *pgxpool.Pool, operatorID, action string) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM operator.operator_audit WHERE operator_id = $1::uuid AND action = $2`,
		operatorID, action).Scan(&count); err != nil {
		t.Fatalf("count the audit rows: %v", err)
	}
	return count
}
