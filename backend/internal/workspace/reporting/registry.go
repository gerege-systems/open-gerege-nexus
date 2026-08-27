/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Where every module's reports meet.
 */

package reporting

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
)

// registry holds every report compiled into this binary.
//
// A package-level registry rather than a dependency passed around, for the same
// reason appregistry is one: a module registers its reports from its own
// constructor, and the reports app — which knows nothing about billing or
// inventory — serves whatever is there. The alternative is the reports module
// importing every other module, which is the coupling this architecture spent
// its effort avoiding.
var registry = struct {
	mu      sync.RWMutex
	reports map[string]Report
}{reports: make(map[string]Report)}

// Register adds a report. It panics on a duplicate key or an incomplete
// declaration, at startup, deliberately: a report whose key collides with
// another silently shadows it, and a report with no Mongolian title is one
// nobody can find in the list. Both are programming errors that must not reach
// a running deployment, and both are impossible to notice at runtime.
func Register(report Report) {
	if err := validate(report); err != nil {
		panic("reporting: " + err.Error())
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()
	if existing, taken := registry.reports[report.Key()]; taken {
		// The same report registering again is a module constructed twice —
		// which a test fixture does routinely, and which the module registry
		// itself tolerates by overwriting. Same key, same app and same Go type
		// is the same report; anything else is a collision, and a collision
		// means one of two reports silently disappears from every listing.
		if sameReport(existing, report) {
			return
		}
		panic(fmt.Sprintf("reporting: two different reports claim the key %q (%s and %s)",
			report.Key(), existing.App(), report.App()))
	}
	registry.reports[report.Key()] = report
}

func sameReport(a, b Report) bool {
	return a.App() == b.App() && reflect.TypeOf(a) == reflect.TypeOf(b)
}

func validate(report Report) error {
	if report.Key() == "" {
		return fmt.Errorf("a report was registered with no key")
	}
	if report.App() == "" {
		return fmt.Errorf("report %q names no app; a report nobody's app gates is a report every tenant sees", report.Key())
	}
	if report.Titles()["mn"] == "" {
		return fmt.Errorf("report %q has no Mongolian title", report.Key())
	}
	if len(report.Columns()) == 0 {
		return fmt.Errorf("report %q declares no columns", report.Key())
	}
	seen := make(map[string]bool, len(report.Columns()))
	for _, column := range report.Columns() {
		if column.Key == "" {
			return fmt.Errorf("report %q has a column with no key", report.Key())
		}
		if seen[column.Key] {
			return fmt.Errorf("report %q declares the column %q twice", report.Key(), column.Key)
		}
		seen[column.Key] = true
	}
	for _, param := range report.Params() {
		if param.Key == "" {
			return fmt.Errorf("report %q has a parameter with no key", report.Key())
		}
		if param.Kind == ParamSelect && len(param.Options) == 0 {
			return fmt.Errorf("report %q declares select parameter %q with no options",
				report.Key(), param.Key)
		}
	}
	return nil
}

// Get returns one report by key.
func Get(key string) (Report, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	report, ok := registry.reports[key]
	return report, ok
}

// All returns every registered report, ordered by key so a listing is stable
// between requests and between replicas.
func All() []Report {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	reports := make([]Report, 0, len(registry.reports))
	for _, report := range registry.reports {
		reports = append(reports, report)
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].Key() < reports[j].Key() })
	return reports
}

// ForApps returns the reports belonging to the given apps — the ones a tenant
// that has installed those apps may see.
//
// This is the app gate, applied to reports. A tenant that never installed
// billing does not get billing's revenue report in its list, cannot fetch its
// metadata, and cannot run it; the handler checks the same set again before
// running, so a key typed directly into the API is refused the same way.
func ForApps(appIDs map[string]bool) []Report {
	all := All()
	permitted := make([]Report, 0, len(all))
	for _, report := range all {
		if appIDs[report.App()] {
			permitted = append(permitted, report)
		}
	}
	return permitted
}

// resetForTest empties the registry. Only tests call it, and they call it to
// keep one test's fixture out of another's listing.
func resetForTest() {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.reports = make(map[string]Report)
}
