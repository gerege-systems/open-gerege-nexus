/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import "context"

// Starter is implemented by a module that has periodic work of its own.
//
// Optional, like AccessPolicy. The platform calls Start once, after the server
// is assembled and alongside its own housekeeping, with a context it cancels on
// graceful shutdown — so a module's ticker stops with the process rather than
// being cut off mid-statement when the pool closes under it.
//
// Work started from a constructor instead has no such context to watch: the
// constructor runs before the platform has one, and a test that builds the
// module by hand would start the work too.
type Starter interface {
	Start(ctx context.Context)
}
