package menu

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// An icon named in Go that the frontend cannot draw renders a fallback glyph.
//
// Not a broken image and no longer an empty square — but still the wrong icon,
// on a screen nobody looks at twice. That is how three of these shipped before,
// and how core's `building-2` and `network` would have shipped: the menu is
// declared in Go and drawn in TypeScript, and no compiler sees both ends.
//
// What changed is where the answer comes from. The check used to parse a
// hand-written map out of frontend/components/Layout.tsx — sixty icons imported
// by name, which was also the reason an app outside this repository could not
// use an icon the core had not thought of. The map is gone; lucide resolves
// names at runtime, so the drawable set is lucide's own and is exported to
// testdata/lucide-icons.txt by frontend/scripts/export-lucide-icons.mjs.
//
// So this test no longer holds a frontend file to a Go file. It holds Go to the
// icon library, which is the dependency that actually decides.
var goMenuIcons = regexp.MustCompile(`Icon:\s*"([a-z0-9-]+)"`)

func TestEveryMenuIconIsDrawnByTheFrontend(t *testing.T) {
	root := repoRoot(t)

	drawn := lucideIcons(t, root)
	if len(drawn) == 0 {
		t.Fatal("read no icons out of the lucide export; the check is not measuring anything")
	}

	// One source since 2026-08-23: every menu entry is declared by the module
	// that owns it. There was a second — a blueprint of screens still to be
	// built, kept in this package — and reading Go source for `Icon: "..."` is
	// what covers both shapes without knowing either.
	named := map[string]string{}
	appsDir := filepath.Join(root, "backend", "internal", "apps")
	err := filepath.WalkDir(appsDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return err //nolint:wrapcheck // walk errors are reported as they are
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err //nolint:wrapcheck
		}
		for _, match := range goMenuIcons.FindAllStringSubmatch(string(source), -1) {
			rel, _ := filepath.Rel(root, path)
			named[match[1]] = rel
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan the modules: %v", err)
	}

	for icon, where := range named {
		if icon == "" {
			continue // an entry with no icon is a choice, not a gap
		}
		if !drawn[icon] {
			t.Errorf("%s names the icon %q, which lucide does not have;"+
				" use a name from backend/internal/workspace/menu/testdata/lucide-icons.txt."+
				" If lucide has it and the list does not, the list is stale:"+
				" run npm run icons:export from frontend/", where, icon)
		}
	}
}

// lucideIcons reads the generated list of names the icon library can draw.
func lucideIcons(t *testing.T, root string) map[string]bool {
	t.Helper()
	path := filepath.Join(root, "backend", "internal", "workspace", "menu", "testdata", "lucide-icons.txt")
	file, err := os.Open(path) // #nosec G304 -- a fixed path under the repository
	if err != nil {
		t.Fatalf("read the lucide icon list: %v (run `npm run icons:export` from frontend/)", err)
	}
	defer file.Close() //nolint:errcheck // read-only

	names := map[string]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		names[line] = true
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan the lucide icon list: %v", err)
	}
	return names
}

// repoRoot is the directory holding both halves of the repository.
//
// It lived in blueprints_test.go until that file went with the blueprint table
// on 2026-08-23; this is the only test left in this package that reaches across
// the two halves, so it lives here now.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cwd: %v", err)
	}
	// Walk up until the directory holding both halves of the repository.
	for range 8 {
		if _, err := os.Stat(filepath.Join(dir, "frontend", "app")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skip("frontend tree not found next to the backend module; skipping the icon check")
	return ""
}
