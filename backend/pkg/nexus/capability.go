/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import (
	"fmt"
	"log/slog"
	"reflect"
	"sync"
)

// What this deployment can do, asked for by type.
//
// Before this, adding a capability meant writing the same pair of functions
// again — a package-level variable, a mutex, a Use* that sets it and a getter
// that returns it or a sentinel error. It is written four times in this
// package already: Link, DocumentFiler, AuditSink, ReportSink. Nothing about
// any of the four is specific to what it carries.
//
// That would only be repetition. What made it worth replacing is the two ways
// the pattern failed:
//
//   - Bootstrap. Every dependency the platform lends its modules is a
//     parameter on internal/apps.Bootstrap, so lending one more changes the
//     signature. Between 2026-08-09 and 2026-08-20 — eleven days — it went
//     from four parameters to nine, in seven separate changes. It is under
//     internal/, so no distribution's build ever said so.
//
//   - MeetingBooker. The contract was declared in meetings.go on 2026-08-15,
//     the adapter that satisfies it was written the same day in
//     internal/workspace/integration, and no way to *get* one was ever added.
//     Six days later AsMeetingBooker still had no callers, because there was
//     nothing a module could call. Half a capability is none.
//
// So capabilities live in one registry, keyed by their own type. A
// distribution can now publish one this repository has never heard of:
//
//	nexus.Provide[mydist.Pricing](myPricing{db})   // in main()
//	p, err := nexus.Capability[mydist.Pricing]()   // in a module
//
// Like the module registry beside it in registry.go, this is package-level
// state on purpose, and for the same reason given there: "what this program
// can do" is a property of the program, and threading a registry value through
// every constructor would only let two of them disagree.
var (
	capabilityMu sync.RWMutex
	capabilities = map[reflect.Type]any{}
)

// Provide installs the capability of type T for this deployment.
//
// The platform calls it while it boots and a distribution's main() calls it for
// anything of its own; a module never does — a module asks.
//
// Last writer wins, the same rule Register uses for modules, but unlike a
// module id there is nothing later that will surface a disagreement, so an
// overwrite is logged. Two distributions layering one capability can be
// deliberate; it can equally be two constructors that both thought they owned
// it, and the difference is not visible from here.
//
// Providing a nil interface withdraws the capability rather than storing an
// empty one, so Capability keeps answering with an error instead of handing
// back something that panics on first use. Tests rely on this to undo a
// Provide: see the t.Cleanup calls in internal/apps/urtuu.
func Provide[T any](impl T) {
	key := reflect.TypeFor[T]()
	boxed := any(impl)

	capabilityMu.Lock()
	if boxed == nil {
		delete(capabilities, key)
	} else {
		if _, replaced := capabilities[key]; replaced {
			slog.Warn("a capability was provided twice and the later one wins",
				"capability", key.String())
		}
		capabilities[key] = boxed
	}
	capabilityMu.Unlock()

	if hook, ok := provideHooks[key]; ok && boxed != nil {
		hook(boxed)
	}
}

// Capability returns what this deployment provides for T, or an error naming
// the type it does not.
//
// Asked for per use rather than captured at construction. A module may be built
// before the platform has provided what it needs — that is the ordinary case
// for anything a distribution's main() wires up after its modules — so the
// answer is not cached and not read once into a field.
//
// An error rather than a zero value: a module that forgets to check nil gets a
// panic at the first call, and a module that forgets to check an error gets a
// vet warning. The failure should be loud where it is written, not where it is
// used.
func Capability[T any]() (T, error) {
	var zero T
	key := reflect.TypeFor[T]()

	capabilityMu.RLock()
	stored, ok := capabilities[key]
	capabilityMu.RUnlock()

	if !ok {
		return zero, fmt.Errorf("nexus: this deployment provides no %s", key)
	}
	impl, ok := stored.(T)
	if !ok {
		// Unreachable through Provide, which keys by the same type it boxes.
		return zero, fmt.Errorf("nexus: this deployment provides no %s", key)
	}
	return impl, nil
}

// provideHooks run after a capability of their type has been stored.
//
// One entry, and the table exists rather than a branch inside Provide because
// Provide must not know what any particular capability is: the moment it does,
// a distribution's own capability is a second-class one. The behaviour lives in
// report.go beside the buffer it drains; only the wiring is here.
var provideHooks = map[reflect.Type]func(any){
	reflect.TypeFor[ReportSink](): func(sink any) { deliverBufferedReports(sink.(ReportSink)) },
}
