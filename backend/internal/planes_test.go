package internal_test

import (
	"errors"
	"go/build"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The line between the two planes, written before the code moves across it.
//
// internal/apps/boundaries_test.go stops an app reaching into another app, and
// it was written the same way round: the property first, the tree afterwards.
// The reason to do it in that order is that a move is cheap and a rule is not.
// While the packages are still where they are, the rule below costs one file;
// once internal/workspace and internal/operator both exist and the compiler is
// happy with whatever imports they happen to have, the same rule costs an
// argument about which of the imports were meant.
//
// So this test is green today and green the day the tree appears. What changes
// is what it is measuring: nothing yet, then the boundary, without anybody
// having to remember to switch it on. What it does say today is how far there
// is to go — TestCountTodaysCrossPlaneImports prints the number Үе C has to
// bring to zero.
//
// One rule, from docs/TWO_PLANES_PROPOSAL.md §2.3 and §2.8, and it does not
// count planes:
//
//	a plane must not import another plane
//	internal/kernel must import none of them; all may import it, and pkg/…
//
// The second is what keeps kernel a floor rather than one more plane. A kernel
// package that imports a plane has picked a side, and everything under it
// inherits the choice — which is how internal/operator came to mean "the rest
// of the code" in the first place.
//
// Naming the rule this way rather than as a pair of directed statements is what
// made the third plane cost thirty lines instead of an argument: see `planes`.

const modulePrefix = "github.com/gerege-systems/open-gerege-nexus/backend"

// The planes. Each answers for a different subject, and each runs as its own
// PostgreSQL role — which is the layer that actually decides what a query may
// read, and therefore the layer that decides what is a plane:
//
//	workspace  gerege_nexus_tenant    one organisation's rows
//	operator   gerege_nexus_operator  every organisation, named columns only
//	person     gerege_nexus_person    every organisation, only the rows about me
//
// person is in this list before internal/person exists, for the reason the
// other two were: a rule written after the code is an argument, a rule written
// before it is a fact. planePackages reports a missing tree rather than failing
// on it, so every rule below is green today and starts measuring on the day the
// directory appears — with nobody having to remember to switch it on.
var planes = []string{"workspace", "operator", "person"}

// crossPlaneExceptions are the imports across the planes that are meant.
//
// There are none, and there should not be one. Where the planes genuinely have
// to meet, the meeting is a contract rather than an import: five tables the
// platform writes and a tenant reads (ownership_test.go), and a small interface
// in kernel where a token minted by one plane is verified by the other
// (§2.5). Both of those are things a reviewer can look at.
//
// The map stays for the same reason internal/apps has one: the next argument
// of that kind should have to be written down, and adding an entry here should
// feel like making a decision rather than fixing a test.
var crossPlaneExceptions = map[string]map[string]string{}

// Where each file still sitting in a plane's root package is going.
//
// internal/operator's 81 packages and files have been dealt with: what is left
// there is one service.go, which is the plane composing its own subpackages
// rather than a place to put a handler. The workspace plane is one step behind —
// its 43 handler files arrived together and are being taken apart by domain —
// so the same list now describes that, and the same test refuses a file with
// nowhere to go.
//
// It is a map and not prose because a file added to a plane's root while the
// split is in progress lands in no list, and TestEveryRootFileHasAPlannedHome
// says so on the run after it appears. A move that quietly gains files is how
// the last root directory ended up with 46 of them.
//
// The values are destinations. The plane is the first segment; the ones with no
// slash are the composition root itself.
// Where each file still sitting in a plane's root package is going.
//
// Four left, and all four are the composition root doing its job rather than
// work waiting to be done: service.go builds the plane, and the three tests
// build the whole of it because what they assert is the assembly — which
// capabilities a built plane publishes, that an extra module is constructed,
// and that a module's routes are behind the gate. A handler would not belong
// here, and this is what says so.
var plannedWorkspacePackages = map[string]string{
	"capabilities_test.go":  "workspace (asserts what building the plane publishes)",
	"extra_modules_test.go": "workspace (asserts a distribution's module is constructed)",
	"gate_e2e_test.go":      "workspace (asserts a module's routes are behind the gate, through the assembled router)",
}

// The root's test builds the whole operator route composition, so it belongs
// beside service.go rather than in one screen's subpackage.
var plannedOperatorPackages = map[string]string{
	"service_test.go": "operator (asserts the versioned route and its guarded compatibility address)",
}

// The person plane's root, which is empty because the plane is.
//
// It is declared now so that the first file put in internal/person has a list
// to be missing from: TestEveryRootFileHasAPlannedHome names it on the run
// after it appears, which is the whole mechanism that stopped the last root
// directory reaching 46 files. A handler does not belong here either — the
// root composes subpackages and nothing else.
var plannedPersonPackages = map[string]string{}

// The floor. These own no table and answer to no plane, which is what
// makes them safe for both to import — and why the third rule below matters
// more than it looks: internal/kernel/security already imports
// internal/workspace/auth, and auth is the workspace plane's.
var plannedKernelPackages = map[string]string{}

// service.go, which did not move whole.
//
// server.go was the seam: it built both planes, mounted both route tables and
// owned the router. It is three files now — internal/workspace/service.go,
// internal/operator/service.go and pkg/host/server.go — and the last of
// those is where the two planes become one process. The route tests and the
// golden route table went with it, because the surface they describe is the
// assembled one.
var plannedSplitOrRemoved = map[string]string{
	"service.go": "already three files: internal/workspace, internal/operator and pkg/host, which is the only one that names both planes",
}

func TestWorkspaceDoesNotImportOperator(t *testing.T) {
	assertNoImportsAcross(t, "workspace", "operator",
		"A workspace plane that imports the operator plane's packages cannot be reasoned about "+
			"separately, deployed separately or reviewed separately, which is the whole "+
			"of what the split buys. Where the planes must meet, they meet through the "+
			"five boundary tables or a contract in kernel — see ownership_test.go.")
}

func TestOperatorDoesNotImportWorkspace(t *testing.T) {
	assertNoImportsAcross(t, "operator", "workspace",
		"This is the direction that rots quietly: the console needs one thing a tenant "+
			"handler already does, imports it, and now the operator's code runs a query "+
			"written for somebody acting inside one organisation. If the operator plane needs "+
			"the answer, it asks the database for it.")
}

// The person plane crosses organisations; a workspace package is inside one.
//
// This is the import that would be easiest to justify and worst to have. The
// person plane needs a session resolved, and workspace/auth already resolves
// sessions — so the import writes itself. What arrives with it is every query
// in that package, each one written for somebody acting inside an organisation,
// now reachable from a request that belongs to somebody who is in none of them.
//
// What the person plane actually needs is an answer to one question — who is
// this token — and a question is a port. It declares the interface; pkg/host,
// the only place allowed to name more than one plane, passes the implementation
// in. docs/PERSON_PLANE_PROMPTS.md P0 is where that port is written.
func TestPersonDoesNotImportAnotherPlane(t *testing.T) {
	assertNoImportsAcross(t, "person", "workspace",
		"The person plane answers for one human across every organisation; a workspace "+
			"package answers for one organisation. Importing the second into the first is "+
			"how a query written for somebody acting inside an organisation comes to run "+
			"for somebody who is not in it. What this plane needs from a session is a "+
			"question — who is this token — and a question is a port, not an import.")
	assertNoImportsAcross(t, "person", "operator",
		"A citizen's screen has no business holding the console's statements. The operator "+
			"plane reads every organisation on a reason and an audit row; the person plane "+
			"reads its own rows on neither, because it needs neither. An import here would "+
			"put the first set of queries one typo away from the second.")
}

// And the other direction, which rots more quietly.
func TestNoPlaneImportsPerson(t *testing.T) {
	assertNoImportsAcross(t, "workspace", "person",
		"An organisation's handler reaching for a person-scoped read is how a statement "+
			"bound to gerege_nexus_person comes to run inside a request bound to "+
			"gerege_nexus_tenant. The roles are the boundary; an import that crosses them "+
			"makes the roles decorative. If the workspace plane needs to know something "+
			"about a person, it asks the database, the same way the operator plane does.")
	assertNoImportsAcross(t, "operator", "person",
		"The console looking at what a citizen sees is impersonation — a reason, a clock "+
			"and two audit rows — not an import.")
}

// The kernel is a floor, not one more plane.
//
// It is the rule with something already against it: internal/kernel/security
// imports internal/workspace/auth today, and auth is the workspace plane's. That
// import is what this test exists to make visible before the directories are
// named — after the move it would be a kernel package that quietly belongs to
// one plane, and everything built on it would inherit the choice.
func TestKernelImportsNeitherPlane(t *testing.T) {
	pkgs := planePackages(t, "kernel")
	if len(pkgs) == 0 {
		return
	}
	for _, pkg := range pkgs {
		for _, imported := range directImports(t, pkg) {
			for _, plane := range planes {
				if !strings.HasPrefix(imported, modulePrefix+"/internal/"+plane+"/") {
					continue
				}
				t.Errorf("%s imports %s.\n"+
					"kernel is what every plane stands on; a kernel package that imports one "+
					"of them has picked a side, and every package built on it inherits the "+
					"choice without being asked. If the plane needs this, it belongs in the "+
					"plane; if both do, what they share is smaller than this import.",
					short(pkg), short(imported))
			}
		}
	}
}

// Every directory and file under internal/operator has somewhere to go.
//
// The lists above are Үе C's work order, and this is what keeps them being
// that. A file added to internal/operator while the move is under way is a
// file nobody decided about, and the failure names it on the first run after
// it appears rather than on the day the directory is supposed to be empty.
func TestEveryRootFileHasAPlannedHome(t *testing.T) {
	var entries []os.DirEntry
	for _, plane := range planes {
		found, err := os.ReadDir(plane)
		if errors.Is(err, os.ErrNotExist) {
			// A plane named in the rule before its directory exists. Reporting
			// it beats failing on it: the alternative is that adding a name to
			// `planes` breaks a test about files, which teaches the next person
			// to add the name last — after the code, when the rule is an
			// argument again.
			t.Logf("internal/%s does not exist yet; nothing to place", plane)
			continue
		}
		if err != nil {
			t.Fatalf("read internal/%s: %v", plane, err)
		}
		entries = append(entries, found...)
	}

	planned := map[string]string{}
	for _, list := range []map[string]string{
		plannedWorkspacePackages, plannedOperatorPackages,
		plannedPersonPackages, plannedKernelPackages, plannedSplitOrRemoved,
	} {
		for name, home := range list {
			if other, dup := planned[name]; dup {
				t.Errorf("%s is planned twice: %q and %q", name, other, home)
			}
			planned[name] = home
		}
	}

	var unplanned, gone []string
	present := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if name == "testdata" {
			continue
		}
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue // a subpackage is where a file is going, not a thing to place
		}
		present[name] = true
		if _, ok := planned[name]; !ok {
			unplanned = append(unplanned, name)
		}
	}
	for name := range planned {
		if !present[name] {
			gone = append(gone, name)
		}
	}

	sort.Strings(unplanned)
	if len(unplanned) > 0 {
		t.Errorf(`a plane's root package holds %d file(s) with no planned home:

	%s

A plane's root package composes its subpackages; it is not where a handler
lives. Say which subpackage this one belongs in — or add it to
plannedSplitOrRemoved, if the honest answer is that it does not go anywhere
whole.`, len(unplanned), strings.Join(unplanned, "\n\t"))
	}

	// The other direction is the progress bar: a name that has already moved
	// is a name to strike off, and a list that keeps naming things which are
	// not there stops being a work order.
	sort.Strings(gone)
	if len(gone) > 0 {
		t.Logf("already in a subpackage: %s", strings.Join(gone, ", "))
	}
}

// How far there is to go, printed rather than asserted.
//
// Everything here is one package tree today, so none of these imports is
// wrong yet — they are wrong the moment the directories are named, which is
// exactly the work Үе C is. The number is the size of that work, and it should
// read zero on the day the move lands.
//
// It counts at package granularity. The 46 files in internal/operator's root
// are one Go package spanning both planes, so nothing can be attributed to a
// file until server.go splits; those imports are logged separately.
func TestCountTodaysCrossPlaneImports(t *testing.T) {
	plane := map[string]string{}
	for name := range plannedWorkspacePackages {
		plane[name] = "workspace"
	}
	for name := range plannedOperatorPackages {
		plane[name] = "operator"
	}
	for name := range plannedKernelPackages {
		plane[name] = "kernel"
	}

	var crossings []string
	for name, from := range plane {
		if strings.HasSuffix(name, ".go") {
			continue // a file in the root package, counted below
		}
		if !holdsGoFiles(filepath.Join("operator", name)) {
			continue // already moved out
		}
		seen := map[string]bool{}
		for _, imported := range directImports(t, modulePrefix+"/internal/operator/"+name) {
			dep, ok := strings.CutPrefix(imported, modulePrefix+"/internal/operator/")
			if !ok {
				continue
			}
			dep = strings.SplitN(dep, "/", 2)[0]
			to, known := plane[dep]
			if !known || to == from || to == "kernel" && from != "kernel" || seen[dep] {
				continue // kernel is what both planes are allowed to import
			}
			seen[dep] = true
			crossings = append(crossings, from+"/"+name+" imports "+to+"/"+dep)
		}
	}

	sort.Strings(crossings)
	for _, crossing := range crossings {
		t.Log(crossing)
	}
	t.Logf("cross-plane imports between internal/operator's subpackages: %d", len(crossings))

	// The root package is both planes at once until server.go is split, so its
	// imports are the work rather than a violation of it.
	root := map[string]int{}
	for _, imported := range directImports(t, modulePrefix+"/internal/operator") {
		if dep, ok := strings.CutPrefix(imported, modulePrefix+"/internal/operator/"); ok {
			root[plane[strings.SplitN(dep, "/", 2)[0]]]++
		}
	}
	t.Logf("internal/operator (the root package, both planes) imports: %d workspace, %d operator, %d kernel",
		root["workspace"], root["operator"], root["kernel"])
}

// assertNoImportsAcross is both directions of the first two rules.
func assertNoImportsAcross(t *testing.T, from, to, why string) {
	t.Helper()
	pkgs := planePackages(t, from)
	if len(pkgs) == 0 || len(planePackages(t, to)) == 0 {
		return
	}
	for _, pkg := range pkgs {
		for _, imported := range directImports(t, pkg) {
			if !strings.HasPrefix(imported, modulePrefix+"/internal/"+to+"/") {
				continue
			}
			fromPkg, toPkg := short(pkg), short(imported)
			if reason, allowed := crossPlaneExceptions[fromPkg][toPkg]; allowed {
				t.Logf("%s imports %s — %s", fromPkg, toPkg, reason)
				continue
			}
			t.Errorf("%s imports %s.\n%s", fromPkg, toPkg, why)
		}
	}
}

// planePackages walks internal/<plane> and every package under it, the same
// way internal/apps/boundaries_test.go walks internal/operator.
//
// The tree does not exist yet, and that is a state to report rather than to
// skip: a skipped test reads as "not applicable here" long after it has become
// applicable, and this one becomes applicable on a day nobody will think to
// come back and check.
func planePackages(t *testing.T, plane string) []string {
	t.Helper()
	root := filepath.Join(plane)
	if _, err := os.Stat(root); err != nil {
		t.Logf("internal/%s does not exist yet; this rule starts measuring the day Үе C creates it", plane)
		return nil
	}
	var pkgs []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err //nolint:wrapcheck // walk errors are reported as they are
		}
		if !entry.IsDir() || strings.Contains(filepath.ToSlash(path), "/testdata") || !holdsGoFiles(path) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err //nolint:wrapcheck
		}
		importPath := modulePrefix + "/internal/" + plane
		if rel != "." {
			importPath += "/" + filepath.ToSlash(rel)
		}
		pkgs = append(pkgs, importPath)
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal/%s: %v", plane, err)
	}
	return pkgs
}

// directImports is what a package imports itself, not what it reaches
// transitively — the same choice, and for the same reason, as
// internal/apps/boundaries_test.go makes: the error should name the package
// that made the decision, and a transitive walk names every package above it.
//
// Test files are included. A test that reaches across the boundary is the same
// coupling; the code compiles without it and the move still breaks on it.
func directImports(t *testing.T, importPath string) []string {
	t.Helper()
	pkg, err := build.Import(importPath, ".", 0)
	if err != nil {
		var noGo *build.NoGoError
		if errors.As(err, &noGo) {
			return nil
		}
		t.Fatalf("resolve %s: %v", importPath, err)
	}
	return append(append([]string{}, pkg.Imports...), append(pkg.TestImports, pkg.XTestImports...)...)
}

// holdsGoFiles reports whether a directory is a package at all. A directory
// that has been emptied by the move — or one that only ever held others — is
// not something to resolve an import path for.
func holdsGoFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return true
		}
	}
	return false
}

func short(importPath string) string { return strings.TrimPrefix(importPath, modulePrefix+"/") }
