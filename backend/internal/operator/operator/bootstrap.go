package operator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The first operator cannot be created through the console, because there is
// nobody to sign in and create them. Every platform solves this the same way
// and most solve it badly: a default password, a first-run web page, an
// environment variable that stays in the deployment for ever. Each of those is
// a way in that exists whether or not anybody uses it.
//
// This is the other answer: an account is created by somebody who already holds
// the database credentials, from a command, on the host. It grants no authority
// that the person running it did not already have — DATABASE_URL is the keys to
// the building — and it leaves nothing behind that could be used again.
//
// Two calls rather than one, because enrolment has to be confirmed. Between
// them the account exists and cannot sign in: sign-in refuses an account whose
// second factor was never confirmed (login.go), so an interrupted bootstrap
// leaves a locked door rather than a password-only one.

// ErrOperatorExists is returned when the address is already taken.
var ErrOperatorExists = errors.New("an operator with that address already exists")

// MinPasswordLength is what a console account is held to. Longer than the
// tenant side's, because there are five of these accounts and they are the
// platform.
//
// Exported so the command that asks for the password can say the rule before
// asking rather than after refusing.
const MinPasswordLength = 12

// NewOperator is what the bootstrap command was asked to create.
type NewOperator struct {
	Email    string
	Name     string
	Role     Role
	Password string
	// BreakGlass marks this as the emergency account (§2.4): the one whose
	// password lives in a safe and whose use pages everybody. It grants
	// nothing extra — see migration 00054.
	BreakGlass bool
}

// Enrolment is what the person has to put into their authenticator. It is
// returned once, never stored anywhere but the account's own row, and never
// logged.
type Enrolment struct {
	Secret string
	URI    string
}

// CreateOperator writes the account and its unconfirmed second factor.
//
// It runs on the platform path rather than as the operator role: INSERT on
// operator_accounts is a privilege migration 00049 deliberately withholds from
// the console, so that a flaw in a console handler cannot mint an operator.
// This command is not a console handler — it is the database owner, acting
// deliberately, from a shell.
func CreateOperator(ctx context.Context, db *pgxpool.Pool, params NewOperator) (Operator, Enrolment, error) {
	email := strings.ToLower(strings.TrimSpace(params.Email))
	name := strings.TrimSpace(params.Name)
	switch {
	case email == "" || !strings.Contains(email, "@"):
		return Operator{}, Enrolment{}, errors.New("an e-mail address is required")
	case name == "":
		return Operator{}, Enrolment{}, errors.New("a name is required")
	case !params.Role.Valid():
		return Operator{}, Enrolment{}, fmt.Errorf("%q is not one of the four operator roles", params.Role)
	case len(params.Password) < MinPasswordLength:
		return Operator{}, Enrolment{}, fmt.Errorf("the password must be at least %d characters", MinPasswordLength)
	}

	hash, err := security.HashPassword(params.Password)
	if err != nil {
		return Operator{}, Enrolment{}, fmt.Errorf("hash the password: %w", err)
	}
	secret, uri, err := NewTOTPSecret(email)
	if err != nil {
		return Operator{}, Enrolment{}, err
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return Operator{}, Enrolment{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var taken bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM operator.operator_accounts WHERE lower(email) = $1)`, email).
		Scan(&taken); err != nil {
		return Operator{}, Enrolment{}, fmt.Errorf("check for an existing operator: %w", err)
	}
	if taken {
		return Operator{}, Enrolment{}, ErrOperatorExists
	}

	operator := Operator{Email: email, Name: name, Role: params.Role}
	if err := tx.QueryRow(ctx,
		`INSERT INTO operator.operator_accounts (email, name, role, password_hash, totp_secret, break_glass)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id::text`,
		email, name, string(params.Role), hash, secret, params.BreakGlass).Scan(&operator.ID); err != nil {
		return Operator{}, Enrolment{}, fmt.Errorf("create the operator: %w", err)
	}

	// The trail starts with the account's own creation, attributed to itself.
	// There is no other operator to attribute it to, and leaving it unrecorded
	// would make the first row of the audit table an action by somebody who
	// appeared from nowhere.
	if err := recordAudit(ctx, tx, Session{Operator: operator}, Change{
		Action:     "operator.create",
		TargetType: "operator",
		TargetID:   operator.ID,
		Reason:     "bootstrapped from the command line",
		After:      map[string]any{"email": email, "name": name, "role": string(params.Role)},
	}, "cli"); err != nil {
		return Operator{}, Enrolment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Operator{}, Enrolment{}, fmt.Errorf("commit: %w", err)
	}
	return operator, Enrolment{Secret: secret, URI: uri}, nil
}

// PendingEnrolment finds an account whose authenticator was never confirmed.
//
// It exists because the bootstrap command's own error message promises it: an
// interrupted enrolment leaves an account that cannot sign in and whose
// address is taken, and "run the command again" has to mean something.
func PendingEnrolment(ctx context.Context, db *pgxpool.Pool, email string) (string, error) {
	var id string
	err := db.QueryRow(ctx,
		`SELECT id::text FROM operator.operator_accounts
		  WHERE lower(email) = lower($1) AND totp_confirmed_at IS NULL`, email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errors.New("no such operator is waiting to confirm an authenticator")
	}
	if err != nil {
		return "", fmt.Errorf("look for the operator: %w", err)
	}
	return id, nil
}

// ConfirmSecondFactor completes enrolment by checking a code the authenticator
// has just produced.
//
// Asking for the code rather than trusting that the QR was scanned is the whole
// value of this step: it proves the secret arrived intact, on a device the
// person is holding, before the account becomes usable. An enrolment that was
// mistyped is discovered here rather than the first time somebody needs to sign
// in urgently.
func ConfirmSecondFactor(ctx context.Context, db *pgxpool.Pool, operatorID, code string) error {
	var secret string
	var confirmed bool
	err := db.QueryRow(ctx,
		`SELECT totp_secret, totp_confirmed_at IS NOT NULL FROM operator.operator_accounts WHERE id = $1::uuid`,
		operatorID).Scan(&secret, &confirmed)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("no such operator")
	}
	if err != nil {
		return fmt.Errorf("read the operator: %w", err)
	}
	if confirmed {
		return errors.New("this operator's authenticator is already confirmed")
	}

	step, ok := verifyTOTP(secret, code, time.Now())
	if !ok {
		return errors.New("that code was not right")
	}
	if _, err := db.Exec(ctx,
		`UPDATE operator.operator_accounts
		    SET totp_confirmed_at = NOW(), totp_last_step = $2, updated_at = NOW()
		  WHERE id = $1::uuid`, operatorID, step); err != nil {
		return fmt.Errorf("confirm the second factor: %w", err)
	}
	return nil
}
