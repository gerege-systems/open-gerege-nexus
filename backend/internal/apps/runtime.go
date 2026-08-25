// Package apps assembles the business-module runtime. The platform owns HTTP,
// sessions and infrastructure; it does not need to import every module or know
// its route table.
package apps

import (
	"context"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

type BackgroundModule interface {
	StartHousekeeping(context.Context)
}

type Runtime struct {
	Background []BackgroundModule
}

// InstalledApps is how the platform tells a module which apps a tenant has.
type InstalledApps = nexus.InstalledApps

// Bootstrap builds every module this binary carries, in the order they need.
//
// It carries none, and that is the finished state rather than a gap: the
// criterion ECOSYSTEM_GIT_STRATEGY set for the split was a platform that boots
// with no business app at all and takes every one of them from a catalogue.
// The function stays because it is the seam a distribution's own Bootstrap
// replaces — see pkg/platform.Options.Modules.
//
// Where they went, and what each one left behind:
//
//	appstore_registry, publisher_studio, store_review  appstore-gerege-nexus
//	egov, documents, organisation, urtuu, reports      client-gerege-nexus (2026-08-23)
//	sso_clients                                        appstore-gerege-nexus (2026-08-25)
//
// None of them took a rail with it. Өртөө's channel is still the platform's —
// nexus.Link to send, nexus.PeerDirectory to read — so a link an administrator
// established keeps carrying what is in flight over it whatever apps come and
// go. Reports left the screens and kept the engine: the SQL, the export, the
// sweep that mails a schedule at three in the morning, the check that lets one
// organisation's report read another's rows, published as nexus.ReportEngine,
// ReportSchedules and ReportGrants. sso_clients left the screens that register
// OAuth2 clients and kept the authorization server, published as
// nexus.SSOClientRegistry.
func Bootstrap(p nexus.Platform) Runtime {
	return Runtime{}
}
