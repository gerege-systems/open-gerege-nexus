/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package operators

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp/totp"
)

// The account the console mints is unusable until somebody proves the
// authenticator works. That is the property that makes minting from a browser
// acceptable at all: a stolen console session can create an account, and the
// account it creates cannot sign in.
func TestAConsoleCreatedOperatorCannotSignInUntilEnrolled(t *testing.T) {
	pool := optest.Pool(t)
	service, sess := screen(t, pool)
	ctx := context.Background()

	created, err := service.Create(ctx, sess, NewOperator{
		Email: fmt.Sprintf("second+%d@controlplane.test", time.Now().UnixNano()),
		Name:  "Second Admin",
		Role:  string(operator.RoleSuperadmin),
	}, "prove the console can add an operator")
	if err != nil {
		t.Fatalf("create the operator: %v", err)
	}
	forget(t, pool, created.ID)

	if created.Password == "" || created.Secret == "" || created.URI == "" {
		t.Fatalf("the screen showed nothing to hand over: %+v", created)
	}

	var confirmed bool
	if err := pool.QueryRow(ctx,
		`SELECT totp_confirmed_at IS NOT NULL FROM operator.operator_accounts WHERE id = $1::uuid`,
		created.ID).Scan(&confirmed); err != nil {
		t.Fatalf("read the new account: %v", err)
	}
	if confirmed {
		t.Fatal("the console confirmed an authenticator nobody has scanned")
	}

	if err := service.Confirm(ctx, sess, created.ID, "000000", "a wrong code"); err == nil {
		t.Fatal("a wrong code confirmed the enrolment")
	}

	code, err := totp.GenerateCode(created.Secret, time.Now())
	if err != nil {
		t.Fatalf("produce a code: %v", err)
	}
	if err := service.Confirm(ctx, sess, created.ID, code, "prove the enrolment"); err != nil {
		t.Fatalf("confirm the enrolment: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT totp_confirmed_at IS NOT NULL FROM operator.operator_accounts WHERE id = $1::uuid`,
		created.ID).Scan(&confirmed); err != nil {
		t.Fatalf("read the new account: %v", err)
	}
	if !confirmed {
		t.Fatal("the enrolment did not take")
	}

	if optest.AuditCount(t, pool, sess.ID, "operator.create") == 0 {
		t.Error("adding an operator left no audit row")
	}
}

// The address is the identity, so a second account on it would be a second
// answer to "who signed in".
func TestAnAddressIsTakenOnlyOnce(t *testing.T) {
	pool := optest.Pool(t)
	service, sess := screen(t, pool)
	ctx := context.Background()

	email := fmt.Sprintf("twice+%d@controlplane.test", time.Now().UnixNano())
	created, err := service.Create(ctx, sess, NewOperator{Email: email, Name: "First", Role: string(operator.RoleOperator)}, "first")
	if err != nil {
		t.Fatalf("create the operator: %v", err)
	}
	forget(t, pool, created.ID)

	if _, err := service.Create(ctx, sess, NewOperator{Email: email, Name: "Second", Role: string(operator.RoleOperator)}, "second"); err == nil {
		t.Fatal("the same address was taken twice")
	}
}

// Nobody can take themselves out of the console.
//
// Both halves are refused for the same reason: the session doing it keeps
// working until it expires, so the operator appears to have succeeded and
// finds out at the worst moment.
func TestAnOperatorCannotTakeThemselvesOut(t *testing.T) {
	pool := optest.Pool(t)
	service, sess := screen(t, pool)
	ctx := context.Background()

	if err := service.SetEnabled(ctx, sess, sess.ID, false, "lock myself out"); err == nil {
		t.Error("an operator disabled their own account")
	}
	if err := service.SetRole(ctx, sess, sess.ID, operator.RoleAuditor, "demote myself"); err == nil {
		t.Error("an operator demoted their own account")
	}
}

// The platform must not be left without a superadmin who can sign in.
//
// Exercised in a transaction that is rolled back, rather than by disabling
// every other superadmin for real: this database is shared with every other
// package's tests, and a test that empties the roster to prove a point takes
// their accounts with it. It did — the operators package failed on a branch
// that had not touched it.
func TestTheLastSuperadminIsKept(t *testing.T) {
	pool := optest.Pool(t)
	_, sess := screen(t, pool)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	// A second superadmin who never finished enrolling cannot sign in, so the
	// guard must not count them: a deployment whose only other superadmin is a
	// half-finished enrolment is locked out just as thoroughly.
	if _, err := tx.Exec(ctx,
		`INSERT INTO operator.operator_accounts (email, name, role, password_hash, totp_secret)
		 VALUES ($1, 'Half Enrolled', 'superadmin', 'x', 'x')`,
		fmt.Sprintf("half+%d@controlplane.test", time.Now().UnixNano())); err != nil {
		t.Fatalf("write a half-enrolled superadmin: %v", err)
	}
	// Everybody else, out of the way — inside this transaction only.
	if _, err := tx.Exec(ctx,
		`UPDATE operator.operator_accounts SET disabled_at = NOW()
		  WHERE role = 'superadmin' AND id <> $1::uuid`, sess.ID); err != nil {
		t.Fatalf("clear the field: %v", err)
	}

	if err := lastSuperadminGuard(ctx, tx, sess.ID); err == nil {
		t.Fatal("the last superadmin who can sign in was allowed to go")
	}

	// And with one more who can sign in, the same change is allowed.
	var other string
	if err := tx.QueryRow(ctx,
		`INSERT INTO operator.operator_accounts (email, name, role, password_hash, totp_secret, totp_confirmed_at)
		 VALUES ($1, 'Second Superadmin', 'superadmin', 'x', 'x', NOW()) RETURNING id::text`,
		fmt.Sprintf("second+%d@controlplane.test", time.Now().UnixNano())).Scan(&other); err != nil {
		t.Fatalf("write a second superadmin: %v", err)
	}
	if err := lastSuperadminGuard(ctx, tx, sess.ID); err != nil {
		t.Fatalf("with two superadmins the guard still refused: %v", err)
	}
}

// A console left open is a console somebody else can lock the owner out of, so
// the current password is asked for.
func TestChangingAPasswordNeedsTheCurrentOne(t *testing.T) {
	pool := optest.Pool(t)
	service, sess := screen(t, pool)
	ctx := context.Background()

	if err := service.ChangePassword(ctx, sess, "not the password", "a much longer replacement"); err == nil {
		t.Fatal("the password changed without the current one")
	}
	if err := service.ChangePassword(ctx, sess, "correct horse battery", "short"); err == nil {
		t.Fatal("a short password was accepted")
	}
	if err := service.ChangePassword(ctx, sess, "correct horse battery", "a much longer replacement"); err != nil {
		t.Fatalf("change the password: %v", err)
	}
	if optest.AuditCount(t, pool, sess.ID, "operator.password") == 0 {
		t.Error("a password change left no audit row")
	}
}

// screen builds the service and a superadmin's session for it. The password is
// the one optest.Account uses, which the password test needs to know.
func screen(t *testing.T, pool *pgxpool.Pool) (*Service, operator.Session) {
	t.Helper()
	op := operator.New(pool)
	account, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	return New(op, Deps{DB: pool}), optest.Session(account)
}

// forget removes an account this test made, so a database shared by every
// package does not fill up with them.
func forget(t *testing.T, pool *pgxpool.Pool, id string) {
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operator.operator_accounts WHERE id = $1::uuid`, id)
	})
}
