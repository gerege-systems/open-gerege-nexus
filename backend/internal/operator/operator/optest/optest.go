/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package optest builds what a console screen's database test needs: a pool, an
// operator with a confirmed second factor, and the code that second factor
// would produce.
//
// It exists because the console is many packages now and every one of them
// needs the same three things. A test helper cannot be imported across
// packages, and four copies of "make an operator" would drift — which for this
// helper means tests that are quietly signing in as somebody with different
// capabilities from the one they name.
package optest

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/dbguard"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func Pool(t *testing.T) *pgxpool.Pool {
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
func Code(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	code, err := totp.GenerateCodeCustom(secret, at, totp.ValidateOpts{
		Period: operator.TOTPPeriod, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
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
func Account(t *testing.T, pool *pgxpool.Pool, role operator.Role) (operator.Operator, string) {
	t.Helper()
	ctx := context.Background()

	email := fmt.Sprintf("%s+%d@controlplane.test", strings.ToLower(t.Name()), time.Now().UnixNano())
	account, enrolment, err := operator.CreateOperator(ctx, pool, operator.NewOperator{
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
		_, _ = pool.Exec(context.Background(), `DELETE FROM operator.operator_accounts WHERE id = $1::uuid`, account.ID)
	})

	// Confirmed with the previous step's code, which the skew window accepts.
	// It leaves the account's high-water mark one step behind now, so the tests
	// below can sign in with the current code and step up with the next one —
	// without waiting thirty seconds between each, and without pretending a
	// code can be reused, which is the property being protected.
	if err := operator.ConfirmSecondFactor(ctx, pool, account.ID,
		Code(t, enrolment.Secret, time.Now().Add(-operator.TOTPPeriod*time.Second))); err != nil {
		t.Fatalf("confirm the second factor: %v", err)
	}
	return account, enrolment.Secret
}

func SessionCookie(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == operator.SessionCookieName && cookie.Value != "" {
			return cookie.Value
		}
	}
	t.Fatal("the response carried no session cookie")
	return ""
}

func AuditCount(t *testing.T, pool *pgxpool.Pool, operatorID, action string) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM operator.operator_audit WHERE operator_id = $1::uuid AND action = $2`,
		operatorID, action).Scan(&count); err != nil {
		t.Fatalf("count the audit rows: %v", err)
	}
	return count
}

// sessionFor is an operator session value, as the middleware would have built
// it. The handlers and the service take it as a parameter, so a test does not
// have to sign in to exercise what they do.
// Session is an operator session value, as the middleware would have built
// it. The screens take one as a parameter, so a test does not have to sign in
// to exercise what they do.
func Session(account operator.Operator) operator.Session {
	return operator.Session{Operator: account, SteppedUpAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
}

// newTenant makes an organisation for one test and takes it away afterwards.
// Tenant creates an organisation for a screen's test to act on, and removes
// it afterwards.
func Tenant(t *testing.T, pool *pgxpool.Pool) (id, slug string) {
	t.Helper()
	slug = fmt.Sprintf("cp-test-%d", time.Now().UnixNano())
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO registry.tenants (slug, name) VALUES ($1, $1) RETURNING id::text`, slug).
		Scan(&id); err != nil {
		t.Fatalf("create a test organisation: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1::uuid`, id)
	})
	return id, slug
}

// newPerson adds somebody to an organisation.
// Person creates somebody in an organisation, and removes them afterwards.
func Person(t *testing.T, pool *pgxpool.Pool, tenantID string) (userID, email string) {
	t.Helper()
	email = fmt.Sprintf("person-%d@controlplane.test", time.Now().UnixNano())
	ctx := context.Background()
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name) VALUES ($1, 'x', 'Test Person')
		 RETURNING id::text`, email).Scan(&userID); err != nil {
		t.Fatalf("create a test person: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1::uuid, $2::uuid)`,
		tenantID, userID); err != nil {
		t.Fatalf("add the person to the organisation: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id = $1::uuid`, userID)
	})
	return userID, email
}
