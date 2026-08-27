package tenants

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/support"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/async"
	"github.com/jackc/pgx/v5"
)

// An organisation's life, from the console.
//
// The shape is §3.A of the plan and the one thing it insists on is that
// nothing is sudden:
//
//	active → suspended        (reversible, one operator, reason recorded)
//	       → deletion pending (two superadmins, then thirty days, reversible)
//	       → deleted          (a sweep, not a button)
//
// There is no hard delete anywhere in this package, and the console's database
// role holds no DELETE privilege on anything — the sweep runs on the platform
// path, after the grace period, and it is the only thing that removes a row.
// An operator who wants an organisation gone today cannot have it, and that is
// the feature: the mistake this protects against is not malice, it is the
// wrong row selected in a list.

// slugPattern is what an organisation's slug may look like. It ends up in URLs
// and in the OAuth issuer's audience list, so it is deliberately narrow.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)

var (
	// ErrSlugTaken is a slug another organisation already has.
	ErrSlugTaken = errors.New("that slug is already in use")
	// ErrInvalidSlug is one that could not be.
	ErrInvalidSlug = errors.New("a slug is 3-64 characters of lowercase letters, digits and hyphens")
	// ErrNotSuspended is asking to resume an organisation that is running.
	ErrNotSuspended = errors.New("that organisation is not suspended")
	// ErrNotScheduled is asking to cancel a deletion nobody asked for.
	ErrNotScheduled = errors.New("that organisation is not scheduled for deletion")
)

// NewTenant is what the console was asked to create.
type NewTenant struct {
	Name               string `json:"name"`
	Slug               string `json:"slug"`
	LegalName          string `json:"legal_name"`
	RegistrationNumber string `json:"registration_number"`
	// Apps are catalogue slugs, installed in the order given. A bundle — the
	// "trade" or "government office" template of §3.A — is a list the console
	// sends, not a concept the backend needs to know.
	Apps []string `json:"apps"`
	// AdminEmail is the person who will run it. They receive an invitation
	// rather than a password: a password an operator chose is a password an
	// operator knows.
	AdminEmail string `json:"admin_email"`
	AdminName  string `json:"admin_name"`
	Reason     string `json:"reason"`
}

// CreatedTenant reports what happened, including the parts that did not.
type CreatedTenant struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
	// Installed and Failed are separate because installing five apps is five
	// independent acts. An organisation created with four of the five apps its
	// operator asked for is a fact they need on the screen, not a failure that
	// throws the organisation away.
	Installed []string `json:"installed"`
	Failed    []string `json:"failed"`
	// Invited says whether the first administrator was actually sent
	// something. False on a deployment with no mail configured — the account
	// exists and the operator has to reach them another way.
	Invited bool `json:"invited"`
	// InviteError explains a false Invited to the operator looking at the
	// screen, rather than only to the log.
	InviteError string `json:"invite_error,omitempty"`
}

// CreateTenant opens an organisation.
//
// The organisation, its profile, its administrator's account and their
// membership are one transaction with the audit row. The apps are installed
// after it, through the platform's own installer — the same code path the
// store endpoint uses, with its dependency resolution and its compiled-module
// check — because writing app_installations rows here would let the console
// create an organisation whose apps have no Go module behind them.
func (s *Service) CreateTenant(ctx context.Context, sess operator.Session, params NewTenant) (CreatedTenant, error) {
	name := strings.TrimSpace(params.Name)
	slug := strings.ToLower(strings.TrimSpace(params.Slug))
	adminEmail := strings.ToLower(strings.TrimSpace(params.AdminEmail))

	switch {
	case name == "":
		return CreatedTenant{}, errors.New("a name is required")
	case !slugPattern.MatchString(slug):
		return CreatedTenant{}, ErrInvalidSlug
	case adminEmail == "" || !strings.Contains(adminEmail, "@"):
		return CreatedTenant{}, errors.New("the first administrator's e-mail address is required")
	}

	created := CreatedTenant{Slug: slug, Name: name}
	var adminUserID string
	err := s.op.Do(ctx, sess, operator.Change{
		Action:     "tenant.create",
		TargetType: "tenant",
		Reason:     params.Reason,
		After: map[string]any{
			"name": name, "slug": slug, "admin_email": adminEmail, "apps": params.Apps,
		},
	}, func(ctx context.Context, tx pgx.Tx) error {
		var taken bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM registry.tenants WHERE slug = $1)`, slug).
			Scan(&taken); err != nil {
			return fmt.Errorf("check the slug: %w", err)
		}
		if taken {
			return ErrSlugTaken
		}

		if err := tx.QueryRow(ctx,
			`INSERT INTO registry.tenants (slug, name) VALUES ($1, $2) RETURNING id::text`,
			slug, name).Scan(&created.ID); err != nil {
			return fmt.Errorf("create the organisation: %w", err)
		}
		if params.LegalName != "" || params.RegistrationNumber != "" {
			if _, err := tx.Exec(ctx,
				`INSERT INTO workspace.tenant_profiles (tenant_id, legal_name, registration_number)
				 VALUES ($1::uuid, $2, $3)
				 ON CONFLICT (tenant_id) DO UPDATE
				    SET legal_name = EXCLUDED.legal_name,
				        registration_number = EXCLUDED.registration_number`,
				created.ID, strings.TrimSpace(params.LegalName),
				strings.TrimSpace(params.RegistrationNumber)); err != nil {
				return fmt.Errorf("write the organisation's details: %w", err)
			}
		}

		var err error
		adminUserID, err = ensureAdmin(ctx, tx, created.ID, adminEmail, strings.TrimSpace(params.AdminName), "")
		return err
	})
	if err != nil {
		return CreatedTenant{}, err
	}

	// Everything below is outside the transaction, and each part reports
	// itself. The organisation exists either way; what varies is how complete
	// it is, and the operator is told which parts landed.
	created.Installed, created.Failed = s.installApps(ctx, created.ID, adminUserID, params.Apps)

	if err := s.support.Invite(ctx, created.ID, adminUserID, adminEmail, name, sess); err != nil {
		created.InviteError = err.Error()
		slog.Warn("control plane: the first administrator could not be invited",
			"tenant_id", created.ID, "error", err)
	} else {
		created.Invited = true
	}

	return created, nil
}

// ensureAdmin gives the organisation its first person and makes them its
// administrator.
//
// The account is created without a usable password — a random one nobody
// keeps — because the way in is the invitation. An operator who could set the
// first password would be an operator who could sign in as the customer, which
// is exactly what impersonation exists to make deliberate and visible.
//
// passwordHash is empty for exactly that reason on every call from the console.
// The one caller that passes one is the first-boot bootstrap, where there is no
// operator to distrust and no mail to invite anybody with: see bootstrap.go.
//
// An address that already belongs to somebody is reused rather than refused:
// one person administering two organisations is ordinary, and their existing
// password is not touched.
func ensureAdmin(ctx context.Context, tx pgx.Tx, tenantID, email, name, passwordHash string) (string, error) {
	if name == "" {
		name = email
	}
	if passwordHash == "" {
		var err error
		if passwordHash, err = support.UnusablePassword(); err != nil {
			return "", err
		}
	}

	// Insert-or-select rather than an upsert, in all three of the statements
	// below, and the reason is the database's rather than Go's: the console's
	// role holds INSERT on these tables and UPDATE on almost none of their
	// columns (migrations 00050 and 00051). An `ON CONFLICT DO UPDATE` asks
	// for UPDATE even when the conflict never happens, so the natural way to
	// write this is refused by PostgreSQL — which is the grant working, not a
	// problem with it. The conflict is a real case here: 00008's trigger
	// creates an `admin` role the moment the organisation row lands.
	userID, err := insertOrSelect(ctx, tx,
		`INSERT INTO registry.users (email, password_hash, name, is_admin)
		 VALUES ($1, $2, $3, FALSE) ON CONFLICT (email) DO NOTHING
		 RETURNING id::text`,
		`SELECT id::text FROM registry.users WHERE email = $1`,
		[]any{email, passwordHash, name}, []any{email})
	if err != nil {
		return "", fmt.Errorf("create the administrator's account: %w", err)
	}

	membershipID, err := insertOrSelect(ctx, tx,
		`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1::uuid, $2::uuid)
		 ON CONFLICT (tenant_id, user_id) DO NOTHING RETURNING id::text`,
		`SELECT id::text FROM workspace.memberships WHERE tenant_id = $1::uuid AND user_id = $2::uuid`,
		[]any{tenantID, userID}, []any{tenantID, userID})
	if err != nil {
		return "", fmt.Errorf("add the administrator to the organisation: %w", err)
	}

	roleID, err := insertOrSelect(ctx, tx,
		`INSERT INTO workspace.roles (tenant_id, code, name) VALUES ($1::uuid, 'admin', 'Tenant Admin')
		 ON CONFLICT (tenant_id, code) DO NOTHING RETURNING id::text`,
		`SELECT id::text FROM workspace.roles WHERE tenant_id = $1::uuid AND code = 'admin'`,
		[]any{tenantID}, []any{tenantID})
	if err != nil {
		return "", fmt.Errorf("create the administrator role: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO workspace.membership_roles (membership_id, role_id) VALUES ($1::uuid, $2::uuid)
		 ON CONFLICT DO NOTHING`, membershipID, roleID); err != nil {
		return "", fmt.Errorf("grant the administrator role: %w", err)
	}
	return userID, nil
}

// insertOrSelect runs an insert that may find the row already there, and
// returns the id either way.
//
// Two statements in one transaction rather than an upsert. See the note in
// ensureAdmin for why the upsert is not available to this role, and note that
// the pair is safe here for the ordinary reason: they are in the caller's
// transaction, and the unique constraint is what decides the race.
func insertOrSelect(ctx context.Context, tx pgx.Tx, insert, selectExisting string,
	insertArgs, selectArgs []any,
) (string, error) {
	var id string
	err := tx.QueryRow(ctx, insert, insertArgs...).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	// No rows from the insert means the conflict clause swallowed it, so the
	// row belongs to somebody else's earlier statement — or to a trigger.
	if err := tx.QueryRow(ctx, selectExisting, selectArgs...).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

// installApps asks the platform to install each app and reports both lists.
//
// It runs on the platform path rather than as the operator role, and the
// distinction is worth being precise about: this is not the console writing to
// a tenant's tables, it is the console asking the platform to do the same
// thing a tenant's own administrator does from the store. The installer opens
// its own transaction, resolves dependencies and records an installation event
// — none of which the console should be reimplementing with its own grants.
func (s *Service) installApps(ctx context.Context, tenantID, userID string, apps []string) (installed, failed []string) {
	installed, failed = []string{}, []string{}
	if s.installer == nil {
		return installed, apps
	}
	for _, slug := range apps {
		slug = strings.TrimSpace(slug)
		if slug == "" {
			continue
		}
		// context.WithoutCancel: the browser hanging up half way through must
		// not leave an organisation with three of its five apps.
		if err := s.installer.InstallAppForTenant(context.WithoutCancel(ctx), tenantID, slug, userID); err != nil {
			slog.Warn("control plane: could not install an app for a new organisation",
				"tenant_id", tenantID, "app", slug, "error", err)
			failed = append(failed, slug)
			continue
		}
		installed = append(installed, slug)
	}
	return installed, failed
}

// Suspend closes an organisation. Signing in stops working, every API call is
// refused, and the data is untouched.
func (s *Service) Suspend(ctx context.Context, sess operator.Session, tenantID, reason string) error {
	before, err := s.op.StateOf(ctx, tenantID)
	if err != nil {
		return err
	}
	// After the transaction either way: a rollback leaves the platform's cache
	// holding what is still true, and a needless invalidation costs one query.
	defer s.changed(tenantID)
	return s.op.Do(ctx, sess, operator.Change{
		Action:     "tenant.suspend",
		TargetType: "tenant",
		TargetID:   tenantID,
		Reason:     reason,
		Before:     before,
		After:      map[string]any{"suspended": true, "reason": reason},
	}, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`UPDATE registry.tenants SET suspended_at = NOW(), suspension_reason = $2 WHERE id = $1::uuid`,
			tenantID, reason); err != nil {
			return fmt.Errorf("suspend the organisation: %w", err)
		}
		// The sessions people are holding right now. Without this, suspension
		// would only stop the next sign-in, and everybody already signed in
		// would keep working until their session expired hours later.
		if _, err := tx.Exec(ctx,
			`UPDATE workspace.sessions SET revoked_at = NOW()
			  WHERE tenant_id = $1::uuid AND revoked_at IS NULL AND expires_at > NOW()`,
			tenantID); err != nil {
			return fmt.Errorf("end the organisation's sessions: %w", err)
		}
		return nil
	})
}

// Resume reopens a suspended organisation.
func (s *Service) Resume(ctx context.Context, sess operator.Session, tenantID, reason string) error {
	before, err := s.op.StateOf(ctx, tenantID)
	if err != nil {
		return err
	}
	if before.SuspendedAt == nil {
		return ErrNotSuspended
	}
	defer s.changed(tenantID)
	return s.op.Do(ctx, sess, operator.Change{
		Action:     "tenant.resume",
		TargetType: "tenant",
		TargetID:   tenantID,
		Reason:     reason,
		Before:     before,
		After:      map[string]any{"suspended": false},
	}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`UPDATE registry.tenants SET suspended_at = NULL, suspension_reason = '' WHERE id = $1::uuid`,
			tenantID)
		return err
	})
}

// CancelDeletion takes an organisation back off the deletion list. It is the
// one-button recovery the grace period exists for.
func (s *Service) CancelDeletion(ctx context.Context, sess operator.Session, tenantID, reason string) error {
	before, err := s.op.StateOf(ctx, tenantID)
	if err != nil {
		return err
	}
	if before.DeletionScheduledAt == nil {
		return ErrNotScheduled
	}
	defer s.changed(tenantID)
	return s.op.Do(ctx, sess, operator.Change{
		Action:     "tenant.deletion.cancel",
		TargetType: "tenant",
		TargetID:   tenantID,
		Reason:     reason,
		Before:     before,
	}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`UPDATE registry.tenants SET deletion_scheduled_at = NULL WHERE id = $1::uuid`, tenantID)
		return err
	})
}

// StartDeletionSweep removes organisations whose grace period has run out.
//
// On the platform path, not the console's role, and that is the design rather
// than a convenience: the console holds no DELETE privilege on any table, so
// there is no sequence of console requests — mistaken, malicious or
// mis-authorised — that removes an organisation. What removes it is time
// having passed, checked by the process itself.
func (s *Service) StartDeletionSweep(ctx context.Context) {
	async.Go("control-plane-deletion-sweep", func() {
		ticker := time.NewTicker(deletionSweepInterval)
		defer ticker.Stop()
		for {
			s.SweepDeletions(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
}

// deletionSweepInterval is hourly. Nothing needs an organisation gone to the
// minute, and a sweep that runs while somebody is cancelling a deletion should
// be rare enough that the row check below is never the thing that saves them.
const deletionSweepInterval = time.Hour

func (s *Service) SweepDeletions(ctx context.Context) {
	sweepCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	rows, err := s.db.Query(sweepCtx,
		`SELECT id::text, slug FROM registry.tenants
		  WHERE deletion_scheduled_at IS NOT NULL AND deletion_scheduled_at <= NOW()`)
	if err != nil {
		slog.Warn("control plane: could not look for organisations to delete", "error", err)
		return
	}
	type doomed struct{ id, slug string }
	pending := make([]doomed, 0, 4)
	for rows.Next() {
		var row doomed
		if err := rows.Scan(&row.id, &row.slug); err != nil {
			rows.Close()
			slog.Warn("control plane: could not read an organisation to delete", "error", err)
			return
		}
		pending = append(pending, row)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		slog.Warn("control plane: could not list the organisations to delete", "error", err)
		return
	}

	for _, row := range pending {
		// The condition is repeated in the DELETE rather than trusted from the
		// SELECT above: a cancellation that landed in between must win, and it
		// would not if this deleted by id alone.
		tag, err := s.db.Exec(sweepCtx,
			`DELETE FROM registry.tenants
			  WHERE id = $1::uuid AND deletion_scheduled_at IS NOT NULL
			    AND deletion_scheduled_at <= NOW()`, row.id)
		if err != nil {
			slog.Error("control plane: could not delete an organisation whose grace period ended",
				"tenant_id", row.id, "slug", row.slug, "error", err)
			continue
		}
		if tag.RowsAffected() == 0 {
			slog.Info("control plane: a scheduled deletion was cancelled before it ran",
				"tenant_id", row.id, "slug", row.slug)
			continue
		}
		// Loudly. Everything belonging to the organisation went with it, by
		// the cascades every table has carried since 00001, and this line is
		// the last trace of when — the operator_audit rows survive it, because
		// they belong to the platform rather than to the organisation.
		slog.Warn("control plane: an organisation's grace period ended and it was deleted",
			"tenant_id", row.id, "slug", row.slug)
	}
}

// TenantsAwaitingDeletion is what the console's home screen shows, and it is
// the other half of a grace period being useful: a countdown nobody can see is
// a countdown nobody stops.
func (s *Service) TenantsAwaitingDeletion(ctx context.Context) ([]operator.TenantState, error) {
	rows, err := s.db.Query(operator.Scoped(ctx),
		`SELECT id::text, slug, name, suspended_at, suspension_reason, deletion_scheduled_at
		   FROM registry.tenants WHERE deletion_scheduled_at IS NOT NULL
		  ORDER BY deletion_scheduled_at`)
	if err != nil {
		return nil, fmt.Errorf("control plane: list the organisations awaiting deletion: %w", err)
	}
	defer rows.Close()

	states := make([]operator.TenantState, 0, 4)
	for rows.Next() {
		var state operator.TenantState
		if err := rows.Scan(&state.ID, &state.Slug, &state.Name, &state.SuspendedAt,
			&state.SuspensionReason, &state.DeletionScheduledAt); err != nil {
			return nil, fmt.Errorf("control plane: read an organisation awaiting deletion: %w", err)
		}
		states = append(states, state)
	}
	return states, rows.Err()
}
