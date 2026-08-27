package appinstall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/appid"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultApps are installed for every tenant that has never had them.
//
// A distribution's decision, set through platform.Options.DefaultApps. It used
// to be a literal naming this repository's own apps, which stopped being
// possible the moment the last of them moved out: a deployment could carry an
// app every tenant should have and no way to say so.
//
// Empty until a distribution says otherwise, and empty is an ordinary answer —
// a platform with no apps of its own installs none. Note what empty also
// switches off: the catalogue-staleness refusal in internal/operator's
// VerifyCatalogVersions is written against this list, so a deployment that
// declares nothing here is a deployment where a catalogue older than the
// binary is accepted whole.
//
// A default app should be one this binary carries, but nothing checks that at
// startup: VerifyCatalogVersions asks only whether the catalogue names the id,
// and it skips any id with no compiled module by design. An id with a
// catalogue entry and no module boots clean and then fails once per tenant,
// for ever, inside EnsureDefaultApps.
//
// Uninstalling one is a gate, not a deletion: DisableApp leaves the
// installation row in place, which is also what keeps EnsureDefaultApps from
// putting back what somebody has just removed.
//
// It is a list rather than a flag in the manifest, because a third party
// publishing an app that installs itself everywhere is not a thing this store
// should be able to express.
var DefaultApps []string

// ErrAppNotFound is returned when a slug names no app in the catalogue.
//
// It is a sentinel because it is the one installation failure that is the
// caller's mistake rather than this deployment's: everything else — a missing
// module, an unresolvable dependency, a database that will not take the row —
// is an operator's problem, and telling a browser about it in detail only
// describes the inside of the server to whoever asked.
var ErrAppNotFound = errors.New("app not found in the store catalog")

// ErrNotInstalled is returned when an operation addresses an installation this
// tenant does not have.
var ErrNotInstalled = errors.New("app is not installed for this tenant")

// ErrAlreadyCurrent is returned when an upgrade is asked for and there is
// nothing to move to. It is a refusal rather than a silent success: a store
// that answered "upgraded" here would keep offering the button.
var ErrAlreadyCurrent = errors.New("the installed version is already the catalog version")

type AppInstaller struct {
	db              *pgxpool.Pool
	platformVersion string

	// The catalogue is no longer fixed for the life of the process: with a
	// registry configured, a background sync replaces it while requests are
	// being served. Every read goes through the lock — a slice header swapped
	// under a ranging goroutine is a data race, and this one would be reading
	// the list that decides what a tenant may install.
	mu      sync.RWMutex
	catalog []catalog.CatalogApp
}

func NewAppInstaller(db *pgxpool.Pool, apps []catalog.CatalogApp, platformVersion string) *AppInstaller {
	return &AppInstaller{
		db:              db,
		catalog:         apps,
		platformVersion: platformVersion,
	}
}

// SetCatalog replaces the catalogue this installer works from.
//
// Installations already written are untouched: what a tenant has installed is a
// database row, and this only changes what the store offers and what an upgrade
// would move to.
func (ai *AppInstaller) SetCatalog(apps []catalog.CatalogApp) {
	ai.mu.Lock()
	ai.catalog = apps
	ai.mu.Unlock()
}

func (ai *AppInstaller) GetCatalog() []catalog.CatalogApp {
	ai.mu.RLock()
	defer ai.mu.RUnlock()
	return ai.catalog
}

// GetAppBySlug also answers to the slug an app used to have.
//
// The slug is what a store URL carries — /store/apps/{slug}/install — so a
// script or a bookmark written before a rename would otherwise get a 404 that
// reads as "this deployment does not have that app".
//
// DEPRECATED: the fallback, not the method — remove in vNEXT with appcatalog's
// rename table.
func (ai *AppInstaller) GetAppBySlug(slug string) (catalog.CatalogApp, bool) {
	current := appid.ResolveAppSlug(slug)
	for _, app := range ai.GetCatalog() {
		if app.Slug == current {
			return app, true
		}
	}
	return catalog.CatalogApp{}, false
}

// GetAppByID also answers to the id an app used to have — an installation row
// written before the migration ran, or a manifest from a registry that has not
// republished.
//
// DEPRECATED: the fallback, not the method — remove in vNEXT.
func (ai *AppInstaller) GetAppByID(id string) (catalog.CatalogApp, bool) {
	current := appid.ResolveAppID(id)
	for _, app := range ai.GetCatalog() {
		if app.ID == current {
			return app, true
		}
	}
	return catalog.CatalogApp{}, false
}

// InstallApp handles recursive dependency resolution and tenant installation.
func (ai *AppInstaller) InstallApp(ctx context.Context, tenantID, appSlug, userID string) error {
	targetApp, ok := ai.GetAppBySlug(appSlug)
	if !ok {
		return fmt.Errorf("%w: %s", ErrAppNotFound, appSlug)
	}

	if err := ai.installOrUpgrade(ctx, tenantID, appSlug, userID); err != nil {
		return err
	}

	audit.Record(ctx, tenantID, userID, "app.install", targetApp.ID, map[string]any{
		"app_slug": targetApp.Slug,
		"version":  targetApp.Version,
	})

	return nil
}

// installOrUpgrade is the transaction both installing and upgrading run.
//
// They are the same work — resolve the dependency order, write or move every
// installation row in it, grant the permissions, record what happened — and
// differ only in what they refuse beforehand and what they write to the audit
// trail afterwards.
func (ai *AppInstaller) installOrUpgrade(ctx context.Context, tenantID, appSlug, userID string) error {
	targetApp, ok := ai.GetAppBySlug(appSlug)
	if !ok {
		return fmt.Errorf("%w: %s", ErrAppNotFound, appSlug)
	}

	// Build dependency graph from catalog
	available := ai.GetCatalog()
	manifests := make([]catalog.Manifest, 0, len(available))
	for _, app := range available {
		manifests = append(manifests, app.Manifest)
	}
	graph := NewDependencyGraph(manifests)

	installOrderIDs, err := graph.ResolveInstallOrder(targetApp.ID)
	if err != nil {
		return fmt.Errorf("dependency resolution failed: %w", err)
	}

	// Verify all modules in install order are compiled into binary. An external
	// app has no Go module by definition — it is somebody else's running service
	// — so requiring one would make the whole category uninstallable.
	for _, appID := range installOrderIDs {
		app, ok := ai.GetAppByID(appID)
		if ok && app.Manifest.IsExternal() {
			continue
		}
		if err := nexus.VerifyModuleExists(appID); err != nil {
			return fmt.Errorf("compile-time module missing for %s: %w", appID, err)
		}
	}

	// A module's own schema, before anything claims the app is installed. See
	// runModuleMigrations for why this is outside the transaction below.
	for _, appID := range installOrderIDs {
		if err := ai.runModuleMigrations(ctx, appID); err != nil {
			return err
		}
	}

	tx, err := ai.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// The tenant's administrator role is the same row for every permission of
	// every app in this order, so it is resolved once. It used to be queried
	// again inside the permission loop — eight apps' worth of permissions meant
	// eight times as many round trips for an answer that cannot change inside a
	// transaction. A tenant with no admin role is not an error: the grant below
	// is skipped and the permission still exists for the role editor to hand out.
	var adminRoleID string
	if err := tx.QueryRow(ctx,
		`SELECT id FROM workspace.roles WHERE tenant_id = $1 AND code = 'admin'`, tenantID).Scan(&adminRoleID); err != nil {
		adminRoleID = ""
	}

	// Install apps in topological order (dependencies first)
	for _, appID := range installOrderIDs {
		app, _ := ai.GetAppByID(appID)

		// Check if already installed, and on which version: an installation
		// that stays behind the catalogue is what "update" has to move, so the
		// version it was on is read before it is overwritten.
		var existingID, previousVersion string
		err := tx.QueryRow(ctx,
			`SELECT id, installed_version FROM workspace.app_installations WHERE tenant_id = $1 AND app_id = $2`,
			tenantID, app.ID).Scan(&existingID, &previousVersion)

		now := time.Now()
		var installID string
		upgradedFrom := ""

		if err != nil {
			// Not installed yet — insert installation
			installID = uuid.New().String()
			_, err = tx.Exec(ctx,
				`INSERT INTO workspace.app_installations (id, tenant_id, app_id, installed_version, status, enabled, installed_at, updated_at)
				 VALUES ($1, $2, $3, $4, 'installed', TRUE, $5, $6)`,
				installID, tenantID, app.ID, app.Version, now, now)
			if err != nil {
				return fmt.Errorf("insert app installation for %s: %w", app.ID, err)
			}
		} else {
			installID = existingID
			// installed_version moves with the catalogue. It used to be left
			// alone here, so a tenant that reinstalled an app the catalogue had
			// carried to 1.1.0 was still recorded as running 1.0.0 for ever —
			// and nothing could tell an out-of-date installation from a current
			// one.
			_, err = tx.Exec(ctx,
				`UPDATE workspace.app_installations SET status = 'installed', enabled = TRUE,
				     installed_version = $1, updated_at = $2
				 WHERE id = $3`, app.Version, now, installID)
			if err != nil {
				return fmt.Errorf("update app installation for %s: %w", app.ID, err)
			}
			if previousVersion != app.Version {
				upgradedFrom = previousVersion
			}
		}

		if err := ai.grantAppPermissions(ctx, tx, tenantID, adminRoleID, app); err != nil {
			return err
		}

		// Log installation event. A version that moved is recorded as an
		// upgrade rather than as another install, so the trail answers "when did
		// this tenant leave 1.0.0" without diffing timestamps.
		eventType, details := "installed", map[string]string{"version": app.Version, "user_id": userID}
		if upgradedFrom != "" {
			eventType = "upgraded"
			details = map[string]string{"from": upgradedFrom, "to": app.Version, "user_id": userID}
		}
		if err := recordInstallationEvent(ctx, tx, installID, eventType, details, now); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit install transaction: %w", err)
	}

	return nil
}

// grantAppPermissions registers an app's permissions and hands them to the
// tenant's default roles.
//
// Every statement here is now checked. They used to be written `_, _ =`, so a
// tenant could finish an installation with none of the app's permissions
// granted and be told it had succeeded — the app then appeared installed and
// refused every request behind it.
func (ai *AppInstaller) grantAppPermissions(ctx context.Context, tx pgx.Tx, tenantID, adminRoleID string, app catalog.CatalogApp) error {
	// Where the permissions come from follows what the app is. A module's are
	// read from the compiled module, which is the code that will enforce them;
	// an external app has no code here, so its manifest is the only statement of
	// what it asks for — and the manifest arrived signed by the registry.
	permissions := app.Manifest.Permissions
	if !app.Manifest.IsExternal() {
		// Register app permissions for tenant. Get returning !ok used to fall
		// through to a nil-interface method call and panic the request.
		mod, ok := nexus.Get(app.ID)
		if !ok {
			return fmt.Errorf("compile-time module missing for %s", app.ID)
		}
		permissions = mod.Permissions()
	}

	for _, perm := range permissions {
		// Refused before anything is written. A definition that is both
		// AdminOnly and asks for default roles, or that names a role that does
		// not exist, is a statement its author believes and the platform cannot
		// keep — and the failure it would otherwise produce is a permission
		// quietly reaching more people or fewer than intended.
		if err := perm.Validate(); err != nil {
			return fmt.Errorf("app %s: %w", app.ID, err)
		}

		permID := uuid.New().String()
		if _, err := tx.Exec(ctx,
			`INSERT INTO registry.permissions (id, code, name, description)
			 VALUES ($1, $2, $3, $4) ON CONFLICT (code) DO NOTHING`,
			permID, perm.Code, perm.Name, perm.Description); err != nil {
			return fmt.Errorf("register permission %s for %s: %w", perm.Code, app.ID, err)
		}

		// Grant to tenant admin role
		if adminRoleID != "" {
			if _, err := tx.Exec(ctx,
				`INSERT INTO workspace.role_permissions (role_id, permission_id)
				 SELECT $1, p.id FROM registry.permissions p WHERE p.code = $2
				 ON CONFLICT DO NOTHING`, adminRoleID, perm.Code); err != nil {
				return fmt.Errorf("grant %s to the admin role: %w", perm.Code, err)
			}
		}

		// A permission the module marks administrative stops here. The admin
		// role has it from the grant above; the default manager and user roles
		// do not get it, and an administrator who wants somebody to have it
		// says so in Access control.
		if perm.AdminOnly {
			continue
		}

		for _, roleCode := range defaultRolesFor(perm) {
			if _, err := tx.Exec(ctx, `INSERT INTO workspace.role_permissions(role_id,permission_id)
				SELECT r.id,p.id FROM workspace.roles r JOIN registry.permissions p ON p.code=$3
				WHERE r.tenant_id=$1 AND r.code=$2 AND r.active ON CONFLICT DO NOTHING`,
				tenantID, roleCode, perm.Code); err != nil {
				return fmt.Errorf("grant %s to the %s role: %w", perm.Code, roleCode, err)
			}
		}
	}
	return nil
}

// defaultRolesFor is who gets this permission when the app is installed.
//
// The app's own answer, when it gave one. When it did not, the suffix rule the
// platform has always used stands in: `.read` to everybody, `.manage` to
// managers.
//
// Deprecated: the suffix fallback. A naming convention should not be a
// contract — it cannot express "apply for a service", it cannot be checked, and
// it silently decides who may do something for every module that has not heard
// of it. A permission with no DefaultRoles will reach no default role in v2;
// see docs/RELEASING.md. Until then a module that says nothing keeps exactly
// the grants it had.
func defaultRolesFor(perm nexus.PermissionDefinition) []string {
	if len(perm.DefaultRoles) > 0 {
		return perm.DefaultRoles
	}
	switch {
	case strings.HasSuffix(perm.Code, ".read"):
		return []string{nexus.DefaultRoleManager, nexus.DefaultRoleUser}
	case strings.HasSuffix(perm.Code, ".manage"):
		return []string{nexus.DefaultRoleManager}
	default:
		return nil
	}
}

// recordInstallationEvent appends one line to an installation's history.
func recordInstallationEvent(ctx context.Context, tx pgx.Tx, installID, eventType string, details map[string]string, at time.Time) error {
	encoded, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("encode %s event details: %w", eventType, err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO workspace.installation_events (id, installation_id, event_type, details, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		uuid.New().String(), installID, eventType, encoded, at); err != nil {
		return fmt.Errorf("record %s event: %w", eventType, err)
	}
	return nil
}

// UpgradeApp moves one tenant's installation to the version the catalogue now
// carries, and reports which version it came from.
//
// The work is InstallApp's: dependencies are resolved again (a new version may
// have gained one), every version in the resolved order moves, and each app
// whose version actually changed records an 'upgraded' event. What is added
// here is the two refusals — an app this tenant never installed, and an
// installation that is already on the catalogue version — because "upgrade"
// has to be able to say no rather than quietly reinstall.
func (ai *AppInstaller) UpgradeApp(ctx context.Context, tenantID, appSlug, userID string) (from string, to string, err error) {
	targetApp, ok := ai.GetAppBySlug(appSlug)
	if !ok {
		return "", "", fmt.Errorf("%w: %s", ErrAppNotFound, appSlug)
	}

	var installed, pinned string
	err = ai.db.QueryRow(ctx,
		`SELECT installed_version, COALESCE(pinned_version, '')
		   FROM workspace.app_installations WHERE tenant_id = $1 AND app_id = $2`,
		tenantID, targetApp.ID).Scan(&installed, &pinned)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", fmt.Errorf("%w: %s", ErrNotInstalled, appSlug)
	}
	if err != nil {
		return "", "", fmt.Errorf("read installation of %s: %w", targetApp.ID, err)
	}

	if !catalog.IsNewerVersion(targetApp.Version, installed) {
		return installed, targetApp.Version, ErrAlreadyCurrent
	}

	if err := ai.installOrUpgrade(ctx, tenantID, appSlug, userID); err != nil {
		return installed, targetApp.Version, err
	}

	// A pinned installation that is upgraded on purpose stays pinned — to the
	// version the administrator has just approved. Clearing the pin instead
	// would turn one deliberate upgrade into consent for every future one,
	// which is the opposite of what pinning was asked for.
	if pinned != "" {
		if _, err := ai.db.Exec(ctx,
			`UPDATE workspace.app_installations SET pinned_version = $1, updated_at = $2
			   WHERE tenant_id = $3 AND app_id = $4`,
			targetApp.Version, time.Now(), tenantID, targetApp.ID); err != nil {
			return installed, targetApp.Version, fmt.Errorf("move the version pin of %s: %w", targetApp.ID, err)
		}
	}

	audit.Record(ctx, tenantID, userID, "app.upgrade", targetApp.ID, map[string]any{
		"app_slug": targetApp.Slug,
		"from":     installed,
		"to":       targetApp.Version,
	})

	return installed, targetApp.Version, nil
}

func (ai *AppInstaller) DisableApp(ctx context.Context, tenantID, appSlug, userID string) error {
	targetApp, ok := ai.GetAppBySlug(appSlug)
	if !ok {
		return fmt.Errorf("%w: %s", ErrAppNotFound, appSlug)
	}

	now := time.Now()
	res, err := ai.db.Exec(ctx,
		`UPDATE workspace.app_installations SET enabled = FALSE, status = 'disabled', updated_at = $1
		 WHERE tenant_id = $2 AND app_id = $3`,
		now, tenantID, targetApp.ID)
	if err != nil || res.RowsAffected() == 0 {
		return fmt.Errorf("app %s is not installed for tenant", targetApp.Slug)
	}

	audit.Record(ctx, tenantID, userID, "app.disable", targetApp.ID, map[string]any{
		"app_slug": targetApp.Slug,
	})

	return nil
}

func (ai *AppInstaller) EnableApp(ctx context.Context, tenantID, appSlug, userID string) error {
	targetApp, ok := ai.GetAppBySlug(appSlug)
	if !ok {
		return fmt.Errorf("%w: %s", ErrAppNotFound, appSlug)
	}

	now := time.Now()
	res, err := ai.db.Exec(ctx,
		`UPDATE workspace.app_installations SET enabled = TRUE, status = 'installed', updated_at = $1
		 WHERE tenant_id = $2 AND app_id = $3`,
		now, tenantID, targetApp.ID)
	if err != nil || res.RowsAffected() == 0 {
		return fmt.Errorf("app %s is not installed for tenant", targetApp.Slug)
	}

	audit.Record(ctx, tenantID, userID, "app.enable", targetApp.ID, map[string]any{
		"app_slug": targetApp.Slug,
	})

	return nil
}

// SyncCatalog mirrors catalog/apps.json into the `apps` table.
//
// app_installations.app_id has a foreign key to apps(id), and the rows used to
// come from a hand-written INSERT in the demo seeder that only listed three of
// the six shipped apps. Installing any of the others — or any app added to the
// catalog later — failed with a foreign-key violation. The catalog file is the
// single source of truth; the table is now derived from it on every boot.
//
// One entry that cannot be written no longer takes the other eight with it.
// It used to return on the first failure, and a single stale row was enough to
// stop the whole catalogue reaching the database: a `slug` is unique across
// apps, so a catalogue naming an app under an id the platform had since
// renamed collided on the slug, the sync aborted before the rest, and every
// app the store went on offering failed to install on the foreign key it never
// got. The failures are still reported — an aggregate reaches the caller and
// each one is logged with the app it belongs to — but what can be written is.
func (ai *AppInstaller) SyncCatalog(ctx context.Context) error {
	var failed []string
	for _, app := range ai.GetCatalog() {
		_, err := ai.db.Exec(ctx,
			`INSERT INTO registry.apps (id, slug, name, description, icon_url, category, visibility)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (id) DO UPDATE SET
			     slug        = EXCLUDED.slug,
			     name        = EXCLUDED.name,
			     description = EXCLUDED.description,
			     icon_url    = EXCLUDED.icon_url,
			     category    = EXCLUDED.category,
			     visibility  = EXCLUDED.visibility`,
			app.ID, app.Slug, app.Name, app.Description, app.IconURL, app.Category, app.Visibility)
		if err != nil {
			slog.Error("could not sync a catalogue app", "error", err, "app_id", app.ID, "slug", app.Slug)
			failed = append(failed, app.ID)
			continue
		}

		// The version the catalogue currently carries is kept as history.
		// app_versions has existed since migration 00002 and nothing ever wrote
		// to it, so "which versions has this instance seen, and what did their
		// manifest say" had no answer at all — which is the question every
		// upgrade and every rollback starts from.
		//
		// DO NOTHING rather than an update: a published version is immutable.
		// A manifest that changes under a version number it has already used is
		// a publisher error, and overwriting the row here would hide it.
		manifest, err := json.Marshal(app.Manifest)
		if err != nil {
			slog.Error("could not encode a manifest", "error", err, "app_id", app.ID)
			failed = append(failed, app.ID)
			continue
		}
		platformConstraint := app.Manifest.Platform
		if platformConstraint == "" {
			platformConstraint = ">=0.1.0"
		}
		_, err = ai.db.Exec(ctx,
			`INSERT INTO registry.app_versions (app_id, version, platform_constraint, manifest)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (app_id, version) DO NOTHING`,
			app.ID, app.Version, platformConstraint, manifest)
		if err != nil {
			slog.Error("could not record a catalogue version",
				"error", err, "app_id", app.ID, "version", app.Version)
			failed = append(failed, app.ID)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("could not sync %d catalogue app(s): %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}

// Installation is what one tenant has of one app.
type Installation struct {
	Version    string
	Enabled    bool
	AutoUpdate bool
	// PinnedVersion is empty unless the tenant has been held at a version on
	// purpose. See migration 00033.
	PinnedVersion string
}

// GetInstallationsForTenant returns every installed app for the tenant.
// Presence in the map means "installed".
//
// It used to answer with enabled/not-enabled alone, which was enough for a
// store that could only install and disable. The store now has to say whether
// an installation is behind the catalogue, and that is a question about the
// version it is on.
func (ai *AppInstaller) GetInstallationsForTenant(ctx context.Context, tenantID string) (map[string]Installation, error) {
	rows, err := ai.db.Query(ctx,
		`SELECT app_id, installed_version, enabled, auto_update, COALESCE(pinned_version, '')
		   FROM workspace.app_installations WHERE tenant_id = $1`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make(map[string]Installation)
	for rows.Next() {
		var id string
		var held Installation
		if err := rows.Scan(&id, &held.Version, &held.Enabled, &held.AutoUpdate, &held.PinnedVersion); err != nil {
			return nil, err
		}
		states[id] = held
	}
	return states, rows.Err()
}

// EnsureDefaultApps installs the platform's default apps for every tenant that
// has no record of them.
//
// A tenant created by a path that installs nothing — the seeder, an eID sign-in
// that makes a membership on the spot — would otherwise start with an empty
// sidebar.
//
// The NOT EXISTS is doing two jobs, and the second one is the important one: a
// tenant that installed the app and then removed it still has its row, with
// enabled = FALSE, so this sweep passes it over. Without that, "uninstall"
// would last until the next catalogue refresh.
func (ai *AppInstaller) EnsureDefaultApps(ctx context.Context) error {
	for _, appID := range DefaultApps {
		app, known := ai.GetAppByID(appID)
		if !known {
			// A catalogue that does not carry a default app is a deployment
			// fault, not something to install around.
			slog.Warn("default app is missing from the catalogue", "app_id", appID)
			continue
		}

		rows, err := ai.db.Query(ctx,
			`SELECT t.id::text FROM registry.tenants t
			  WHERE NOT EXISTS (SELECT 1 FROM workspace.app_installations ai
			                     WHERE ai.tenant_id = t.id AND ai.app_id = $1)`, appID)
		if err != nil {
			return err
		}
		tenants := make([]string, 0)
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			tenants = append(tenants, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		for _, tenantID := range tenants {
			if err := ai.installOrUpgrade(ctx, tenantID, app.Slug, SystemActor); err != nil {
				// One tenant's failure is not the sweep's.
				slog.Error("could not install a default app for a tenant",
					"error", err, "app_id", appID, "tenant_id", tenantID)
				continue
			}
			slog.Info("installed a default app", "app_id", appID, "tenant_id", tenantID)
		}
	}
	return nil
}

func (ai *AppInstaller) GetEnabledAppIDsForTenant(ctx context.Context, tenantID string) ([]string, error) {
	rows, err := ai.db.Query(ctx,
		`SELECT app_id FROM workspace.app_installations WHERE tenant_id = $1 AND enabled = TRUE`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Both errors are reported rather than skipped. This list is what the menu
	// is built from, so a row that failed to scan — or a stream that broke
	// partway, which leaves rows.Next() returning false exactly as a clean end
	// does — used to reach the caller as a short list with a nil error, and the
	// apps that fell off it read as ones the tenant had never installed.
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
