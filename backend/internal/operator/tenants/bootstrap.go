package tenants

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A deployment's first way in.
//
// The console creates organisations, and using it takes an operator account,
// its own vhost and a confirmed authenticator. A deployment that wants one
// organisation and no console therefore had no supported way to make it: the
// two products that shipped before this were provisioned by hand-written SQL,
// which has to get three non-obvious things right — the `admin` role is made by
// a trigger on the tenant row, `platform.users.is_admin` is not where
// administration lives, and the membership needs its role row — and got them
// wrong on the first attempt.
//
// The answer is the one the console's own first account already gets
// (internal/operator/operator): a command, run by somebody who holds
// DATABASE_URL, granting no authority they did not already have. It refuses
// once the deployment has an organisation, so it is a way in exactly once —
// not a default password, not an environment variable that stays in the
// deployment for ever, and not a first-run web page, which is a door standing
// open from boot until somebody happens to walk through it.

// MinAdminPasswordLength is the rule somebody choosing their own password is
// held to. It repeats internal/tenant/access's rather than sharing it: the two
// planes do not import each other (internal/operator/service.go).
//
// Exported so the command that asks for the password can say the rule before
// asking rather than after refusing.
const MinAdminPasswordLength = 10

// ErrAlreadyProvisioned is a deployment that has an organisation already. The
// command is not a way to reset a password: whoever holds the database can do
// that, and doing it through here would mean the door stayed open.
var ErrAlreadyProvisioned = errors.New("this deployment already has an organisation")

// FirstTenant is the organisation a fresh deployment is being given, and the
// person who will run it.
type FirstTenant struct {
	Slug string
	Name string
	// LegalName and RegistrationNumber are the organisation as a register
	// holds it, filled from the Gerege Core directory when the wizard was used
	// and empty when somebody typed the name at a terminal. Both are optional:
	// the profile row is only written when there is something to put in it.
	LegalName          string
	RegistrationNumber string
	AdminEmail         string
	AdminName          string
	// Password is the administrator's, chosen at the terminal. Unlike
	// CreateTenant's invitation there is nobody to send mail as yet, and no
	// operator whose knowing it would matter — the person running this holds
	// the database.
	Password string
}

// Bootstrap gives a deployment with no organisation its first one.
//
// Apps are not installed: that needs the running server's installer, and the
// administrator this creates can do it from the store on their first sign-in.
func Bootstrap(ctx context.Context, db *pgxpool.Pool, p FirstTenant) (tenantID, userID string, err error) {
	name := strings.TrimSpace(p.Name)
	slug := strings.ToLower(strings.TrimSpace(p.Slug))
	email := strings.ToLower(strings.TrimSpace(p.AdminEmail))
	switch {
	case name == "":
		return "", "", errors.New("the organisation needs a name")
	case !slugPattern.MatchString(slug):
		return "", "", ErrInvalidSlug
	case email == "" || !strings.Contains(email, "@"):
		return "", "", errors.New("the administrator's e-mail address is required")
	case len([]rune(p.Password)) < MinAdminPasswordLength:
		return "", "", fmt.Errorf("the password must be at least %d characters", MinAdminPasswordLength)
	}

	hash, err := security.HashPassword(p.Password)
	if err != nil {
		return "", "", fmt.Errorf("hash the password: %w", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return "", "", fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Two people running this at the same moment on a fresh deployment would
	// otherwise both read an empty table and both create an organisation. The
	// lock is held for the transaction and released with it, and needs no
	// privilege the connection does not already have.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(4207341)`); err != nil {
		return "", "", fmt.Errorf("take the bootstrap lock: %w", err)
	}
	var provisioned bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM platform.tenants)`).Scan(&provisioned); err != nil {
		return "", "", fmt.Errorf("look for an existing organisation: %w", err)
	}
	if provisioned {
		return "", "", ErrAlreadyProvisioned
	}

	// ensureAdmin reuses an account that already has this address and leaves its
	// password alone — right for the console, where one person administers two
	// organisations, and silently wrong here: the command would report success
	// and the password just chosen would not be the one that signs in.
	var taken bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM platform.users WHERE email = $1)`, email).
		Scan(&taken); err != nil {
		return "", "", fmt.Errorf("look for an existing account: %w", err)
	}
	if taken {
		return "", "", fmt.Errorf("an account already exists for %s", email)
	}

	tenantID, userID, err = bootstrapTx(ctx, tx, first{
		slug: slug, name: name, legalName: strings.TrimSpace(p.LegalName),
		registrationNumber: strings.TrimSpace(p.RegistrationNumber),
		email:              email, adminName: strings.TrimSpace(p.AdminName), passwordHash: hash,
	})
	if err != nil {
		return "", "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", "", fmt.Errorf("commit: %w", err)
	}
	return tenantID, userID, nil
}

// bootstrapTx is the writing half, without the check that there is nothing
// there yet. Separate so a test can run the real statements — the trigger that
// makes the role, the membership, the grant — inside a transaction it rolls
// back, on a database that already has other organisations in it.
// first is bootstrapTx's arguments. A struct rather than seven strings in a
// row, because six of them are strings and a transposed pair would compile.
type first struct {
	slug, name, legalName, registrationNumber string
	email, adminName, passwordHash            string
}

func bootstrapTx(ctx context.Context, tx pgx.Tx, p first) (string, string, error) {
	var tenantID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO platform.tenants (slug, name) VALUES ($1, $2) RETURNING id::text`,
		p.slug, p.name).Scan(&tenantID); err != nil {
		return "", "", fmt.Errorf("create the organisation: %w", err)
	}
	// An upsert, not an insert: migration 00034 puts an AFTER INSERT trigger on
	// tenants that creates the profile row already, with the organisation's
	// name as its legal name. A plain insert here collides with the row the
	// database made a microsecond earlier.
	if p.legalName != "" || p.registrationNumber != "" {
		if _, err := tx.Exec(ctx,
			`INSERT INTO tenant.tenant_profiles (tenant_id, legal_name, registration_number)
			 VALUES ($1::uuid, $2, $3)
			 ON CONFLICT (tenant_id) DO UPDATE
			    SET legal_name = EXCLUDED.legal_name,
			        registration_number = EXCLUDED.registration_number`,
			tenantID, p.legalName, p.registrationNumber); err != nil {
			return "", "", fmt.Errorf("write the organisation's details: %w", err)
		}
	}
	userID, err := ensureAdmin(ctx, tx, tenantID, p.email, p.adminName, p.passwordHash)
	if err != nil {
		return "", "", err
	}
	return tenantID, userID, nil
}

// Unprovisioned reports whether this deployment has no organisation at all,
// which is the state in which nobody can sign in to it.
func Unprovisioned(ctx context.Context, db *pgxpool.Pool) (bool, error) {
	var exists bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM platform.tenants)`).Scan(&exists); err != nil {
		return false, err
	}
	return !exists, nil
}
