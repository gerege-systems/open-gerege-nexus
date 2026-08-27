package appinstall

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/menu"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// The whole external-app path, from the manifest that ships in catalog/ to the
// sign-in the platform will or will not perform for it.
//
// It reads the real example manifest rather than a fixture: what is being
// checked is that a third party can describe their running platform in a file
// and have this one accept it, so a copy of that file written to suit the test
// would be checking the test.
const exampleExternalManifest = "../../../../catalog/manifests/example-external.json"

// externalCatalogApp is the shipped example as the store would carry it.
func externalCatalogApp(t *testing.T) catalog.CatalogApp {
	t.Helper()
	manifest, err := catalog.LoadManifest(filepath.FromSlash(exampleExternalManifest), config.PlatformVersion)
	if err != nil {
		t.Fatalf("load the example external manifest: %v", err)
	}
	return catalog.CatalogApp{
		ID: manifest.ID, Slug: "example-external", Name: manifest.Name,
		Category: "Third party", Visibility: "public", Version: manifest.Version,
		Manifest: manifest,
	}
}

func TestTheShippedExternalManifestIsAcceptedAsOne(t *testing.T) {
	app := externalCatalogApp(t)

	if !app.Manifest.IsExternal() {
		t.Fatalf("expected the example manifest to declare type external, got %q", app.Manifest.Type)
	}
	if app.Manifest.External.SSOClientID == "" {
		t.Fatal("an external app without an sso_client_id cannot be gated by installation")
	}
	// A catalogue carrying it has to pass the same validation every source goes
	// through — including the one place that would refuse a Go module for it.
	if err := catalog.ValidateCatalog([]catalog.CatalogApp{app}, config.PlatformVersion); err != nil {
		t.Fatalf("expected the example external app to be a valid catalog entry: %v", err)
	}
	// And no compiled module is expected of it. This is the check that made
	// every external app uninstallable until it learned about them.
	if err := VerifyCatalogVersions(withDefaultApp(app)); err != nil {
		t.Fatalf("expected an external app to need no compiled module: %v", err)
	}
}

func TestAnExternalAppMustLaunchOverHTTPS(t *testing.T) {
	app := externalCatalogApp(t)

	cases := map[string]string{
		"plain http":  "http://external.example.mn/sso/gerege",
		"relative":    "/sso/gerege",
		"scheme only": "https://",
	}
	for name, launchURL := range cases {
		t.Run(name, func(t *testing.T) {
			manifest := app.Manifest
			spec := *manifest.External
			spec.LaunchURL = launchURL
			manifest.External = &spec

			err := catalog.ValidateManifest(manifest, config.PlatformVersion)
			if err == nil {
				t.Fatalf("expected launch_url %q to be refused", launchURL)
			}
			if !strings.Contains(err.Error(), "launch_url") {
				t.Fatalf("the error should name the field; got %v", err)
			}
		})
	}
}

// fakeInstalledApps stands in for the installer's view of a tenant.
type fakeInstalledApps struct {
	enabled []string
	catalog []catalog.CatalogApp
}

func (f fakeInstalledApps) GetEnabledAppIDsForTenant(context.Context, string) ([]string, error) {
	return f.enabled, nil
}
func (f fakeInstalledApps) GetCatalog() []catalog.CatalogApp { return f.catalog }

func TestAnInstalledExternalAppContributesAnExternalMenuEntry(t *testing.T) {
	app := externalCatalogApp(t)
	store := fakeInstalledApps{enabled: []string{app.ID}, catalog: []catalog.CatalogApp{app}}

	menus, err := menu.GetTenantMenus(context.Background(), store, "tenant", "mn")
	if err != nil {
		t.Fatalf("menus: %v", err)
	}

	var entry *nexus.MenuDefinition
	for i := range menus {
		if menus[i].ExternalURL != "" {
			entry = &menus[i]
		}
	}
	if entry == nil {
		t.Fatalf("expected an external menu entry; got %+v", menus)
	}
	if entry.Path != "" {
		t.Fatalf("an external entry must carry no path — it is not a route here; got %q", entry.Path)
	}
	if entry.ExternalURL != app.Manifest.External.LaunchURL {
		t.Fatalf("expected the manifest launch_url, got %q", entry.ExternalURL)
	}
	if entry.AppID != app.ID || entry.ParentID == "" {
		t.Fatalf("expected the entry to be grouped under its app; got %+v", entry)
	}
}

func TestAnUninstalledExternalAppContributesNothing(t *testing.T) {
	app := externalCatalogApp(t)
	store := fakeInstalledApps{catalog: []catalog.CatalogApp{app}}

	menus, err := menu.GetTenantMenus(context.Background(), store, "tenant", "mn")
	if err != nil {
		t.Fatalf("menus: %v", err)
	}
	if len(menus) != 0 {
		t.Fatalf("expected no menus for a tenant that installed nothing; got %+v", menus)
	}
}

func TestTheInstallGateAnswersForExternalClientsOnly(t *testing.T) {
	app := externalCatalogApp(t)
	clientID := app.Manifest.External.SSOClientID

	installed := map[string]bool{}
	gate := externalAppGate{
		catalog: func() []catalog.CatalogApp { return []catalog.CatalogApp{app} },
		installed: func(_ context.Context, _, appID string) (bool, error) {
			return installed[appID], nil
		},
	}

	// A client that belongs to no external app is none of the gate's business:
	// the developer portal's own client, and everything a tenant registered for
	// itself, must keep working exactly as before.
	if allowed, err := gate.AllowClient(context.Background(), "tenant", "gerege-dev-portal"); err != nil || !allowed {
		t.Fatalf("expected an unrelated client to be allowed; got %v %v", allowed, err)
	}

	// The tenant has not installed the app: this is the sign-in that used to
	// succeed for anyone in any organisation.
	if allowed, err := gate.AllowClient(context.Background(), "tenant", clientID); err != nil || allowed {
		t.Fatalf("expected an uninstalled external app to be refused; got %v %v", allowed, err)
	}

	installed[app.ID] = true
	if allowed, err := gate.AllowClient(context.Background(), "tenant", clientID); err != nil || !allowed {
		t.Fatalf("expected an installed external app to be allowed; got %v %v", allowed, err)
	}
}

func TestTheInstallGateReportsAFailedLookupRatherThanAllowing(t *testing.T) {
	app := externalCatalogApp(t)
	gate := externalAppGate{
		catalog: func() []catalog.CatalogApp { return []catalog.CatalogApp{app} },
		installed: func(context.Context, string, string) (bool, error) {
			return false, errors.New("database unreachable")
		},
	}

	// The provider turns an error into a refusal. What must not happen is this
	// layer deciding the answer is yes because it could not find out.
	allowed, err := gate.AllowClient(context.Background(), "tenant", app.Manifest.External.SSOClientID)
	if err == nil {
		t.Fatal("expected the lookup failure to be reported")
	}
	if allowed {
		t.Fatal("a lookup that failed must not answer allowed")
	}
}
