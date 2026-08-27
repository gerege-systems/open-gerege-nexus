/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The old name for pkg/host, kept so a distribution builds without an edit.
 *
 * `platform` had come to mean four things at once — the operator plane, the
 * registry schema, this host package and the product itself — so the plane
 * became internal/operator and this became pkg/host. The word left here is a
 * host: it builds both planes into one process and runs it.
 *
 * A distribution's main.go is the one place outside this repository that names
 * it, and there are eleven of those. Deleting the name would have turned a
 * package rename into a breaking change for every one of them, which
 * RELEASING.md's semver promise does not allow at a minor. So the name stays,
 * delegating, until the next major removes it.
 *
 * Nothing else belongs in this file. Anything added to pkg/host is reached
 * through pkg/host; this is a forwarding address, not a second surface.
 */

package platform

import "github.com/gerege-systems/open-gerege-nexus/backend/pkg/host"

// Options is [host.Options].
//
// An alias and not a wrapper struct on purpose: a wrapper would have to copy
// every field, and the field added to host.Options next month would compile
// here while silently never reaching the server.
type Options = host.Options

// Run is [host.Run].
//
// Deprecated: use [host.Run]. This forwarding name is removed in the next major.
func Run(opts Options) error { return host.Run(opts) }
