package appcatalog_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/appcatalog"
)

// The disk cache is the one source that can be older than the binary reading
// it: it was written by a previous build, from a registry that has since moved
// on, and it survives a deployment. It was also the one source accepted without
// being held against that binary.
//
// That combination took the store down in production. A cache written before
// the platform's apps were renamed was served whole; the platform's own core
// app was absent from it; no tenant received the screens; and every app the
// store went on offering failed to install on a foreign key, because the ids
// the cache named no longer existed in the apps table.
func TestACacheThatDoesNotMatchThisBuildIsNotUsed(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "catalog.cache.json")

	// A cached document from before a rename: well-formed, correctly signed as
	// far as this provider is concerned (it is read from disk, not verified
	// again), and naming an app this build has never heard of.
	stale := `{"generated_at":"2026-08-01T00:00:00Z","key_id":"k","apps":[` +
		`{"id":"io.old.contacts","slug":"contacts","name":"Contacts","version":"1.0.0",` +
		`"visibility":"public","manifest":{"id":"io.old.contacts","name":"Contacts","version":"1.0.0"}}]}`
	if err := os.WriteFile(cachePath, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}

	// What the platform passes as Verify: this build's own opinion of what a
	// catalogue must contain.
	//
	// It used to be "carries one of this repository's apps", and it named a
	// different one every time one left — the organisation, then reports, then
	// sso_clients. This repository ships none at all since 2026-08-25, so the
	// same opinion is stated the other way round: a catalogue naming an id from
	// before the renames is older than this binary whatever else it carries.
	verify := func(apps []catalog.CatalogApp) error {
		for _, app := range apps {
			if strings.HasPrefix(app.ID, "io.old.") {
				return errors.New("catalog names an app this build has never heard of: " + app.ID)
			}
		}
		return nil
	}

	provider := appcatalog.NewProvider(appcatalog.Config{
		// A registry that cannot be reached, which is the state that makes the
		// cache load in the first place.
		URL:             "http://127.0.0.1:1",
		FilePath:        filepath.FromSlash("../../../../catalog/apps.json"),
		CachePath:       cachePath,
		PlatformVersion: "1.0.0",
		Verify:          verify,
	})

	apps, err := provider.Load(context.Background())
	if err != nil {
		t.Fatalf("the bundled file should have carried the boot: %v", err)
	}
	for _, app := range apps {
		if strings.HasPrefix(app.ID, "io.old.") {
			t.Fatalf("the stale cache was served: %s", app.ID)
		}
	}
	if err := verify(apps); err != nil {
		t.Fatalf("what was loaded does not satisfy this build: %v", err)
	}
}
