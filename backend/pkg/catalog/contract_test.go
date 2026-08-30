/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * An external test package, so it sees only what a registry or a publisher sees.
 */

package catalog_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// What this package promises to whoever publishes against it.
//
// The rules it enforces were covered from internal/kernel/appcatalog, which
// reaches them through its own wrappers: the semver, the platform constraint,
// the visibility set and the chronicle's shape all have tests there. What had
// none were the halves a publisher meets and the platform does not — the
// external-app rules, the provenance fields, the slug guard, and the loader
// that turns a directory into a catalogue. Those are here, in the package's
// own tests, because the day pkg/catalog becomes a repository of its own is
// the day the tests in internal/ stop travelling with it.

// aManifest is the smallest manifest that validates. Each test copies it and
// breaks one thing, so what is being asserted is that one thing.
func aManifest() catalog.Manifest {
	return catalog.Manifest{
		ID:      "io.gerege.nexus.contract",
		Name:    "Contract",
		Version: "1.2.3",
	}
}

func TestAnExternalAppIsHeldToItsLaunchURL(t *testing.T) {
	external := func(spec *catalog.ExternalSpec) catalog.Manifest {
		m := aManifest()
		m.Type = catalog.TypeExternal
		m.External = spec
		return m
	}

	for _, refused := range []struct {
		why      string
		manifest catalog.Manifest
	}{
		{"no external section at all", external(nil)},
		{"an empty launch_url", external(&catalog.ExternalSpec{})},
		{"a relative launch_url, which would resolve against this platform's own origin",
			external(&catalog.ExternalSpec{LaunchURL: "/apps/somewhere"})},
		{"a plain-http launch_url, which carries a signed-in person out of TLS",
			external(&catalog.ExternalSpec{LaunchURL: "http://elsewhere.test/start"})},
		{"a launch_url with no host",
			external(&catalog.ExternalSpec{LaunchURL: "https:///start"})},
		{"an unknown embed mode",
			external(&catalog.ExternalSpec{LaunchURL: "https://elsewhere.test/start", Embed: "popup"})},
	} {
		if err := catalog.ValidateManifest(refused.manifest, ""); err == nil {
			t.Errorf("an external app with %s was accepted", refused.why)
		}
	}

	for _, accepted := range []string{"", "new_tab", "iframe"} {
		m := external(&catalog.ExternalSpec{LaunchURL: "https://elsewhere.test/start", Embed: accepted})
		if err := catalog.ValidateManifest(m, ""); err != nil {
			t.Errorf("embed %q should be accepted: %v", accepted, err)
		}
	}
}

// The two halves of "what kind of app is this" have to agree, and an app that
// runs somewhere else may not present itself as part of this shell.
func TestAnAppIsOneKindOrTheOther(t *testing.T) {
	module := aManifest()
	module.External = &catalog.ExternalSpec{LaunchURL: "https://elsewhere.test/start"}
	if err := catalog.ValidateManifest(module, ""); err == nil {
		t.Error("a module carrying an external section was accepted")
	}

	chrome := aManifest()
	chrome.Type = catalog.TypeExternal
	chrome.Chrome = true
	chrome.External = &catalog.ExternalSpec{LaunchURL: "https://elsewhere.test/start"}
	if err := catalog.ValidateManifest(chrome, ""); err == nil {
		t.Error("an external app claiming to be shell chrome was accepted")
	}

	unknown := aManifest()
	unknown.Type = "widget"
	if err := catalog.ValidateManifest(unknown, ""); err == nil {
		t.Error("an app of an unknown type was accepted")
	}

	// A module may say nothing about its type, because every manifest written
	// before external apps existed says nothing.
	if err := catalog.ValidateManifest(aManifest(), ""); err != nil {
		t.Errorf("a manifest with no type is a module: %v", err)
	}
}

// Provenance is optional and, when present, has to mean something: a name
// nobody typed is an empty line in a storefront credit list, and a link this
// platform vouches for is absolute and https or it is not shown.
func TestProvenanceMeansSomethingOrIsAbsent(t *testing.T) {
	blank := aManifest()
	blank.Authors = []catalog.Person{{Name: "  "}}
	if err := catalog.ValidateManifest(blank, ""); err == nil {
		t.Error("an author with a blank name was accepted")
	}
	maintainer := aManifest()
	maintainer.Maintainers = []catalog.Person{{Name: ""}}
	if err := catalog.ValidateManifest(maintainer, ""); err == nil {
		t.Error("a maintainer with no name was accepted")
	}

	for _, link := range []string{"github.com/example/app", "http://example.test", "https:///app", "::"} {
		repo := aManifest()
		repo.Repository = link
		if err := catalog.ValidateManifest(repo, ""); err == nil {
			t.Errorf("repository %q was accepted; it is a link this platform vouches for", link)
		}
		home := aManifest()
		home.Homepage = link
		if err := catalog.ValidateManifest(home, ""); err == nil {
			t.Errorf("homepage %q was accepted", link)
		}
	}

	for _, spdx := range []string{"Apache-2.0", "MIT", "GPL-3.0-or-later WITH Classpath-exception-2.0"} {
		m := aManifest()
		m.License = spdx
		if err := catalog.ValidateManifest(m, ""); err != nil {
			t.Errorf("license %q should be accepted: %v", spdx, err)
		}
	}
	sentence := aManifest()
	sentence.License = "do whatever you like with it"
	if err := catalog.ValidateManifest(sentence, ""); err == nil {
		t.Error("a sentence typed into the license field was accepted")
	}

	negative := aManifest()
	negative.Order = -1
	if err := catalog.ValidateManifest(negative, ""); err == nil {
		t.Error("a negative rail order was accepted")
	}
}

// A dependency the installer cannot resolve is refused where the manifest is
// read, not where the graph is walked.
func TestADependencyNamesAnAppAndAVersionItCanParse(t *testing.T) {
	nameless := aManifest()
	nameless.Dependencies = []nexus.Dependency{{VersionConstraint: ">=1.0.0"}}
	if err := catalog.ValidateManifest(nameless, ""); err == nil {
		t.Error("a dependency on nothing was accepted")
	}

	unparseable := aManifest()
	unparseable.Dependencies = []nexus.Dependency{{ID: "io.b", VersionConstraint: "the latest one"}}
	if err := catalog.ValidateManifest(unparseable, ""); err == nil {
		t.Error("a dependency constraint that is not semver was accepted")
	}

	// A dependency may name no version: "this app, whichever release you have"
	// is a claim the installer can act on.
	open := aManifest()
	open.Dependencies = []nexus.Dependency{{ID: "io.b"}}
	if err := catalog.ValidateManifest(open, ""); err != nil {
		t.Errorf("a dependency with no constraint was refused: %v", err)
	}
}

// A permission a manifest declares is held to the same rule as a compiled
// module's: the manifest arrives from a registry, which is not a reason to
// check it less.
func TestAManifestsPermissionsAreCheckedLikeAModules(t *testing.T) {
	contradictory := aManifest()
	contradictory.Permissions = []nexus.PermissionDefinition{{
		Code: "payroll.approve", Name: "Approve", AdminOnly: true,
		DefaultRoles: []string{nexus.DefaultRoleUser},
	}}
	if err := catalog.ValidateManifest(contradictory, ""); err == nil {
		t.Error("a permission that is admin-only and also given to everybody was accepted")
	}

	invented := aManifest()
	invented.Permissions = []nexus.PermissionDefinition{{
		Code: "payroll.approve", Name: "Approve", DefaultRoles: []string{"accountant"},
	}}
	if err := catalog.ValidateManifest(invented, ""); err == nil {
		t.Error("a permission asking for a role that does not exist yet was accepted")
	}
}

// A release note travelling inside a manifest may not claim a different
// version from the manifest carrying it.
func TestAReleaseNoteCannotDisagreeWithItsManifest(t *testing.T) {
	note := catalog.ReleaseNote{
		Kind:    catalog.KindFix,
		Summary: map[string]string{"mn": "Засвар", "en": "A fix"},
	}

	carried := aManifest()
	carried.ReleaseNotes = &note
	if err := catalog.ValidateManifest(carried, ""); err != nil {
		t.Fatalf("a note with no version of its own travels: %v", err)
	}

	agreeing := note
	agreeing.Version = carried.Version
	withAgreement := aManifest()
	withAgreement.ReleaseNotes = &agreeing
	if err := catalog.ValidateManifest(withAgreement, ""); err != nil {
		t.Errorf("a note repeating its manifest's version travels: %v", err)
	}

	disagreeing := note
	disagreeing.Version = "9.9.9"
	withDisagreement := aManifest()
	withDisagreement.ReleaseNotes = &disagreeing
	if err := catalog.ValidateManifest(withDisagreement, ""); err == nil {
		t.Error("a note claiming a version its manifest does not was accepted")
	}

	// The note's own rules apply wherever it is: a kind outside the set, or a
	// summary in neither source language, is refused here too.
	unroutable := aManifest()
	unroutable.ReleaseNotes = &catalog.ReleaseNote{Kind: catalog.KindFix,
		Summary: map[string]string{"mn": "Засвар"}}
	if err := catalog.ValidateManifest(unroutable, ""); err == nil {
		t.Error("a note with no English summary travelled inside a manifest")
	}
}

// A slug is a URL path segment and a manifest filename, so this is a
// path-traversal guard before it is a naming rule.
func TestASlugCannotLeaveItsDirectory(t *testing.T) {
	for _, refused := range []string{
		"", "../secrets", "a/b", "..", "a.b", "Payroll", "тооцоо", "app name", "app\x00",
		strings.Repeat("a", 65),
	} {
		if catalog.IsValidSlug(refused) {
			t.Errorf("slug %q was accepted", refused)
		}
	}
	for _, accepted := range []string{"a", "payroll", "point-of-sale", "old_app", "app2",
		strings.Repeat("a", 64)} {
		if !catalog.IsValidSlug(accepted) {
			t.Errorf("slug %q was refused", accepted)
		}
	}

	// The guard is the loader's too, and there it is what stands between a
	// slug and the filesystem.
	if _, _, err := catalog.LoadChronicleFile(t.TempDir(), "../../etc"); err == nil {
		t.Error("a chronicle was read for a slug that leaves the directory")
	}
}

// Two apps claiming one slug is not a cosmetic conflict: whichever is listed
// first answers for both.
func TestACatalogueCannotContradictItself(t *testing.T) {
	entry := func(id, slug, version string) catalog.CatalogApp {
		m := aManifest()
		m.ID, m.Version = id, version
		return catalog.CatalogApp{ID: id, Slug: slug, Version: version, Manifest: m}
	}

	if err := catalog.ValidateCatalog([]catalog.CatalogApp{
		entry("io.a", "payroll", "1.0.0"), entry("io.b", "payroll", "1.0.0"),
	}, ""); err == nil {
		t.Error("two apps claiming one slug were accepted")
	}

	if err := catalog.ValidateCatalog([]catalog.CatalogApp{
		entry("io.a", "one", "1.0.0"), entry("io.a", "two", "1.0.0"),
	}, ""); err == nil {
		t.Error("one app listed twice was accepted")
	}

	mismatched := entry("io.a", "one", "1.0.0")
	mismatched.Manifest.ID = "io.b"
	if err := catalog.ValidateCatalog([]catalog.CatalogApp{mismatched}, ""); err == nil {
		t.Error("an entry whose manifest declares another id was accepted")
	}

	stale := entry("io.a", "one", "1.0.0")
	stale.Manifest.Version = "2.0.0"
	if err := catalog.ValidateCatalog([]catalog.CatalogApp{stale}, ""); err == nil {
		t.Error("an entry whose version disagrees with its manifest was accepted")
	}

	nameless := entry("", "one", "1.0.0")
	if err := catalog.ValidateCatalog([]catalog.CatalogApp{nameless}, ""); err == nil {
		t.Error("an entry with no id was accepted")
	}

	if err := catalog.ValidateCatalog([]catalog.CatalogApp{
		entry("io.a", "one", "1.0.0"), entry("io.b", "two", "2.0.0"),
	}, ""); err != nil {
		t.Errorf("a catalogue that agrees with itself was refused: %v", err)
	}
}

// A translation overrides what it carries and nothing else, and it does not
// reach back into the entry it was read from.
func TestATranslationOverridesOnlyWhatItCarries(t *testing.T) {
	app := catalog.CatalogApp{
		Name: "Payroll", Description: "Pay people", Category: "finance",
		Translations: map[string]catalog.CatalogAppText{
			"mn": {Name: "Цалин", Category: "санхүү"},
		},
	}

	untranslated := app.Localized("de")
	if untranslated.Name != "Payroll" || untranslated.Description != "Pay people" {
		t.Errorf("a locale with no translation changed the entry: %+v", untranslated)
	}

	mn := app.Localized("mn")
	if mn.Name != "Цалин" || mn.Category != "санхүү" {
		t.Errorf("the translation was not applied: %+v", mn)
	}
	if mn.Description != "Pay people" {
		t.Errorf("a field the translation omits should keep its default, got %q", mn.Description)
	}
	if app.Name != "Payroll" {
		t.Error("Localized changed the entry it was called on")
	}
}

func TestAChronicleIsSearchedByVersion(t *testing.T) {
	chronicle := catalog.Chronicle{AppID: "io.a", Entries: []catalog.ReleaseNote{
		{Version: "2.0.0", Kind: catalog.KindBreaking},
		{Version: "1.0.0", Kind: catalog.KindFeature},
	}}
	found, ok := chronicle.Find("1.0.0")
	if !ok || found.Kind != catalog.KindFeature {
		t.Errorf("Find(1.0.0) = %+v, %v", found, ok)
	}
	if _, ok := chronicle.Find("1.0.1"); ok {
		t.Error("a version that was never released was found")
	}
}

// The loader, end to end: apps.json, the manifest beside it, and the chronicle
// entry for the version being shipped folded into the manifest.
func TestACatalogueDirectoryLoads(t *testing.T) {
	dir := writeCatalogue(t, catalogueFixture{
		entries: []catalog.CatalogApp{{
			ID: "io.gerege.nexus.payroll", Slug: "payroll", Name: "Payroll", Version: "1.2.3",
		}},
		manifests: map[string]catalog.Manifest{"payroll": {
			ID: "io.gerege.nexus.payroll", Name: "Payroll", Version: "1.2.3",
			Platform: ">=1.0.0",
		}},
		chronicles: map[string]catalog.Chronicle{"payroll": {
			AppID: "io.gerege.nexus.payroll",
			Entries: []catalog.ReleaseNote{
				{Version: "1.2.3", Kind: catalog.KindFeature, ReleasedAt: "2026-08-30",
					Summary: map[string]string{"mn": "Шинэ", "en": "New"}},
				{Version: "1.0.0", Kind: catalog.KindFeature,
					Summary: map[string]string{"mn": "Эхлэл", "en": "First"}},
			},
		}},
	})

	apps, err := catalog.LoadFile(filepath.Join(dir, "apps.json"), "1.4.0")
	if err != nil {
		t.Fatalf("the catalogue did not load: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("loaded %d apps, want 1", len(apps))
	}
	notes := apps[0].Manifest.ReleaseNotes
	if notes == nil {
		t.Fatal("the chronicle entry for the shipped version did not travel in the manifest")
	}
	if notes.Summary["en"] != "New" {
		t.Errorf("the wrong entry travelled: %+v", notes)
	}
	// Dropped on the way in: the manifest already declares the version, and
	// validateProvenance refuses the two disagreeing.
	if notes.Version != "" {
		t.Errorf("the embedded note repeats its version %q", notes.Version)
	}
	// An entry that says nothing about visibility is public once assembled, so
	// nothing downstream has to read an empty string and decide.
	if apps[0].Visibility != catalog.VisibilityPublic {
		t.Errorf("an entry with no visibility assembled as %q", apps[0].Visibility)
	}
}

// A private manifest makes the entry private, and both halves are written so
// the API, the apps table and the store card cannot disagree.
func TestAPrivateManifestMakesTheEntryPrivate(t *testing.T) {
	dir := writeCatalogue(t, catalogueFixture{
		entries: []catalog.CatalogApp{{ID: "io.a", Slug: "hidden", Name: "Hidden", Version: "1.0.0"}},
		manifests: map[string]catalog.Manifest{"hidden": {
			ID: "io.a", Name: "Hidden", Version: "1.0.0", Visibility: catalog.VisibilityPrivate,
		}},
	})

	apps, err := catalog.LoadFile(filepath.Join(dir, "apps.json"), "")
	if err != nil {
		t.Fatalf("the catalogue did not load: %v", err)
	}
	if !apps[0].IsPrivate() {
		t.Fatal("a privately published app assembled as public")
	}
	if apps[0].Visibility != catalog.VisibilityPrivate ||
		apps[0].Manifest.Visibility != catalog.VisibilityPrivate {
		t.Errorf("the two halves disagree: entry %q, manifest %q",
			apps[0].Visibility, apps[0].Manifest.Visibility)
	}
}

// A manifest that fails to load is an error rather than a stub. Three shipped
// manifests were once malformed and nobody noticed, because the apps installed
// with an empty dependency graph and contributed no menu.
func TestABrokenCatalogueIsAnErrorRatherThanAnEmptyApp(t *testing.T) {
	base := catalogueFixture{
		entries:   []catalog.CatalogApp{{ID: "io.a", Slug: "payroll", Name: "Payroll", Version: "1.0.0"}},
		manifests: map[string]catalog.Manifest{"payroll": {ID: "io.a", Name: "Payroll", Version: "1.0.0"}},
	}

	missing := writeCatalogue(t, catalogueFixture{entries: base.entries})
	if _, err := catalog.LoadFile(filepath.Join(missing, "apps.json"), ""); err == nil {
		t.Error("a catalogue whose manifest file is absent loaded")
	}

	malformed := writeCatalogue(t, base)
	if err := os.WriteFile(filepath.Join(malformed, "manifests", "payroll.json"),
		[]byte(`{"id":"io.a","permissions":"all of them"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.LoadFile(filepath.Join(malformed, "apps.json"), ""); err == nil {
		t.Error("a manifest with a string where an array belongs loaded")
	}

	badSlug := writeCatalogue(t, catalogueFixture{
		entries:   []catalog.CatalogApp{{ID: "io.a", Slug: "../escape", Name: "Escape", Version: "1.0.0"}},
		manifests: base.manifests,
	})
	if _, err := catalog.LoadFile(filepath.Join(badSlug, "apps.json"), ""); err == nil {
		t.Error("an entry whose slug leaves the directory loaded")
	}

	absent := filepath.Join(t.TempDir(), "apps.json")
	if _, err := catalog.LoadFile(absent, ""); err == nil {
		t.Error("a catalogue file that does not exist loaded")
	}

	notJSON := t.TempDir()
	if err := os.WriteFile(filepath.Join(notJSON, "apps.json"), []byte("apps: none"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.LoadFile(filepath.Join(notJSON, "apps.json"), ""); err == nil {
		t.Error("a catalogue file that is not JSON loaded")
	}
}

// An app that keeps no chronicle is not an error, and neither is a chronicle
// with no entry for the version being shipped: a deployment whose bundled file
// predates the chronicle still boots. Whether an entry had to be written is a
// question for a CI guard, which is where it is asked.
func TestAnAbsentChronicleIsNotAnError(t *testing.T) {
	none := writeCatalogue(t, catalogueFixture{
		entries:   []catalog.CatalogApp{{ID: "io.a", Slug: "payroll", Name: "Payroll", Version: "1.0.0"}},
		manifests: map[string]catalog.Manifest{"payroll": {ID: "io.a", Name: "Payroll", Version: "1.0.0"}},
	})
	apps, err := catalog.LoadFile(filepath.Join(none, "apps.json"), "")
	if err != nil {
		t.Fatalf("an app that keeps no chronicle did not load: %v", err)
	}
	if apps[0].Manifest.ReleaseNotes != nil {
		t.Error("release notes appeared from nowhere")
	}

	older := writeCatalogue(t, catalogueFixture{
		entries:   []catalog.CatalogApp{{ID: "io.a", Slug: "payroll", Name: "Payroll", Version: "2.0.0"}},
		manifests: map[string]catalog.Manifest{"payroll": {ID: "io.a", Name: "Payroll", Version: "2.0.0"}},
		chronicles: map[string]catalog.Chronicle{"payroll": {AppID: "io.a", Entries: []catalog.ReleaseNote{
			{Version: "1.0.0", Kind: catalog.KindFeature, Summary: map[string]string{"mn": "Эхлэл", "en": "First"}},
		}}},
	})
	apps, err = catalog.LoadFile(filepath.Join(older, "apps.json"), "")
	if err != nil {
		t.Fatalf("a chronicle without this version did not load: %v", err)
	}
	if apps[0].Manifest.ReleaseNotes != nil {
		t.Error("a note for another version travelled")
	}

	// An invalid chronicle is a different matter: it is read, so it is checked.
	broken := writeCatalogue(t, catalogueFixture{
		entries:   []catalog.CatalogApp{{ID: "io.a", Slug: "payroll", Name: "Payroll", Version: "1.0.0"}},
		manifests: map[string]catalog.Manifest{"payroll": {ID: "io.a", Name: "Payroll", Version: "1.0.0"}},
		chronicles: map[string]catalog.Chronicle{"payroll": {AppID: "io.a", Entries: []catalog.ReleaseNote{
			{Version: "1.0.0", Kind: "rewrite", Summary: map[string]string{"mn": "?", "en": "?"}},
		}}},
	})
	if _, err := catalog.LoadFile(filepath.Join(broken, "apps.json"), ""); err == nil {
		t.Error("a chronicle with a kind outside the set loaded")
	}
}

type catalogueFixture struct {
	entries    []catalog.CatalogApp
	manifests  map[string]catalog.Manifest
	chronicles map[string]catalog.Chronicle
}

// writeCatalogue lays out apps.json, manifests/ and chronicle/ the way a
// catalogue directory is laid out, and returns the directory.
func writeCatalogue(t *testing.T, fixture catalogueFixture) string {
	t.Helper()
	dir := t.TempDir()
	write := func(path string, value any) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "apps.json"), fixture.entries)
	for slug, manifest := range fixture.manifests {
		write(filepath.Join(dir, "manifests", slug+".json"), manifest)
	}
	for slug, chronicle := range fixture.chronicles {
		write(filepath.Join(dir, "chronicle", slug+".json"), chronicle)
	}
	return dir
}
