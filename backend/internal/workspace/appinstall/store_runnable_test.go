package appinstall

import (
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// The catalogue outlives the split, and the store keeps advertising what left.
//
// This is not a hypothetical. State Services moved to its own repository on
// 2026-08-15 and nexus.gerege.mn went on carrying `io.gerege.nexus.gov_services`
// in its apps table the same afternoon, because that row comes from a signed
// catalogue served to every deployment in the field — not from this
// repository's manifests. Republishing the catalogue is a deliberate act with
// its own consequences, so a deployment has to be able to tell the truth about
// itself in the meantime.
func TestTheStoreDoesNotOfferAnAppThisBinaryCannotRun(t *testing.T) {
	withModules(t, gatedModule{})

	compiled := catalog.CatalogApp{ID: "io.gerege.test.gated"}
	if !runnableHere(compiled) {
		t.Error("an app with a compiled module must be offered")
	}

	// The app that left. Its manifest is still in the catalogue, there is no
	// module behind it, and the installer would refuse with a message about
	// binary registries that means nothing to whoever pressed Install.
	departed := catalog.CatalogApp{ID: "io.gerege.nexus.gov_services"}
	if runnableHere(departed) {
		t.Error("an app with no module in this binary must not be offered")
	}
}

// External apps have no Go module by definition — they are somebody else's
// running service, reached over OIDC. The rule above would hide the entire
// category if it did not except them, which would be a worse bug than the one
// it fixes: third-party apps are the whole point of a public catalogue.
func TestAnExternalAppIsOfferedWithoutAModule(t *testing.T) {
	withModules(t)

	external := catalog.CatalogApp{ID: "com.example.hrms"}
	external.Manifest.Type = "external"
	if !external.Manifest.IsExternal() {
		t.Fatalf("fixture is wrong: the manifest does not read as external")
	}
	if !runnableHere(external) {
		t.Error("an external app must be offered even though nothing compiled it")
	}
}

// The same rule, applied to what a tenant already has rather than to what the
// store offers.
//
// The screen that lists installed apps read straight from workspace.app_installations,
// so on nexus.gerege.mn it showed nine rows under a banner saying the
// catalogue has five: State Services, Products, Inventory and Billing were all
// listed as installed and active months after their code left for other
// repositories. Nothing about them worked — no routes, no menu entry — and the
// row offered a button to disable an app that was not running.
func TestTheInstalledListDropsAnAppThisBinaryCannotRun(t *testing.T) {
	withModules(t, gatedModule{})

	server := New(Deps{Installer: NewAppInstaller(nil, []catalog.CatalogApp{
		{ID: "io.gerege.test.gated", Slug: "gated", Version: "1.0.0"},
		// Advertised by the signed catalogue, built from another repository.
		{ID: "io.gerege.nexus.gov_services", Slug: "gov-services", Version: "1.1.0"},
	}, "1.0.0")})

	if !server.presentableInstallation("io.gerege.test.gated") {
		t.Error("an app with a compiled module must stay on the list")
	}
	if server.presentableInstallation("io.gerege.nexus.gov_services") {
		t.Error("an app that left this binary must not be listed as installed")
	}
	// Never heard of by the catalogue and not compiled either: the four rows
	// this test is named after, after their manifests were dropped from
	// catalog/apps.json.
	if server.presentableInstallation("io.gerege.nexus.billing") {
		t.Error("an app in neither the catalogue nor the binary must not be listed")
	}
}

// A distribution's own module is real from the moment the binary starts, and
// the catalogue may learn about it minutes later or never — a deployment can
// run from a file-mode catalogue that names nothing it did not write itself.
// Hiding it until a document catches up would hide a working app.
func TestAnInstalledModuleTheCatalogueHasNotHeardOfIsKept(t *testing.T) {
	withModules(t, gatedModule{})

	server := New(Deps{Installer: NewAppInstaller(nil, nil, "1.0.0")})
	if !server.presentableInstallation("io.gerege.test.gated") {
		t.Error("a compiled module must be listed even when the catalogue is silent about it")
	}
}

var _ nexus.Module = gatedModule{}
