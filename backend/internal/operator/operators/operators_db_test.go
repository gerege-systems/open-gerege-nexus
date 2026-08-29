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

// A console that can lock itself out is a console somebody eventually locks
// themselves out of. The refusal has nothing to do with how many other
// superadmins there are: the session doing it would keep working until it
// expired, so the operator would appear to have succeeded and would find out at
// the worst moment.
func TestAnOperatorCannotLockThemselvesOut(t *testing.T) {
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

// One of two may go. This is the other side of the guard below, and the one
// that would be missed by a rule written as "a superadmin cannot be disabled".
func TestOneOfTwoSuperadminsMayBeDisabled(t *testing.T) {
	pool := optest.Pool(t)
	service, sess := screen(t, pool)
	ctx := context.Background()

	other, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	if err := service.SetEnabled(ctx, sess, other.ID, false, "one of several"); err != nil {
		t.Fatalf("disabling one of several superadmins was refused: %v", err)
	}
}

// The last superadmin who can sign in is kept, whether the change is a
// disable or a demotion.
//
// Asked of the guard inside a transaction that is rolled back, rather than of
// the screen. The rule counts every superadmin on the deployment, so a test
// that drove the screen could only make the count reach one by disabling the
// accounts already there — which on a developer's own database means the
// operator they signed in with. It reported green on an empty database and red
// on a real one, which is the wrong way round for a test of a lockout.
//
// Counted as "enabled and enrolled": an account whose authenticator was never
// confirmed cannot sign in either, and a console that mints accounts creates
// exactly that state.
func TestTheLastSuperadminIsKept(t *testing.T) {
	pool := optest.Pool(t)
	ctx := context.Background()
	keeper, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	spare, _ := optest.Account(t, pool, operator.RoleSuperadmin)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Nothing here is committed: the accounts this disables include whichever
	// ones the deployment already had.
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`UPDATE operator.operator_accounts SET disabled_at = NOW()
		  WHERE role = 'superadmin' AND disabled_at IS NULL
		    AND totp_confirmed_at IS NOT NULL AND id <> $1::uuid`, keeper.ID); err != nil {
		t.Fatalf("leave one superadmin standing: %v", err)
	}

	if err := lastSuperadminGuard(ctx, tx, keeper.ID); err == nil {
		t.Error("the last superadmin who can sign in was allowed to go")
	}

	// A second one who can sign in makes the same change fine.
	if _, err := tx.Exec(ctx,
		`UPDATE operator.operator_accounts SET disabled_at = NULL WHERE id = $1::uuid`, spare.ID); err != nil {
		t.Fatalf("bring the spare back: %v", err)
	}
	if err := lastSuperadminGuard(ctx, tx, keeper.ID); err != nil {
		t.Errorf("one of two superadmins was refused: %v", err)
	}

	// An enrolment nobody finished does not count: that account cannot sign in
	// either, and it is the state the console's own screen leaves behind.
	if _, err := tx.Exec(ctx,
		`UPDATE operator.operator_accounts SET totp_confirmed_at = NULL WHERE id = $1::uuid`, spare.ID); err != nil {
		t.Fatalf("un-enrol the spare: %v", err)
	}
	if err := lastSuperadminGuard(ctx, tx, keeper.ID); err == nil {
		t.Error("a half-finished enrolment was counted as a superadmin who can sign in")
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
