/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package tenants

import "context"

// Installer puts an app into an organisation, the way the store does.
//
// An interface rather than the concrete installer because the console must not
// be able to reach the rest of it: this is the whole of what CP-2 asks the
// platform to do on its behalf, and a narrower dependency is a narrower blast
// radius when somebody adds a handler here in a hurry.
type Installer interface {
	InstallAppForTenant(ctx context.Context, tenantID, appSlug, userID string) error
}
