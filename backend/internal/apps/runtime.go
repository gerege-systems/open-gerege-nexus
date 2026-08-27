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
// replaces — see pkg/host.Options.Modules.
//
// Where they went, and what each one left behind:
//
//	appstore_registry, publisher_studio, store_review  appstore-gerege-nexus
//	egov, documents, organisation, urtuu, reports      client-gerege-nexus (2026-08-23)
//	sso_clients                                        appstore-gerege-nexus (2026-08-25)
//	urtuu's channel                                    client-gerege-nexus (2026-08-27)
//
// Most of them left a rail behind. Reports left the screens and kept the
// engine: the SQL, the export, the sweep that mails a schedule at three in the
// morning, the check that lets one organisation's report read another's rows,
// published as nexus.ReportEngine, ReportSchedules and ReportGrants.
// sso_clients left the screens that register OAuth2 clients and kept the
// authorization server, published as nexus.SSOClientRegistry.
//
// Өртөө is the one that did not, in the end. Its channel stayed here for four
// days on the rule that a link an administrator established keeps carrying what
// is in flight whatever apps come and go — which is true, and was not the
// question. The question a rail has to answer is whether more than one thing
// needs it, and in three months nexus.Link and nexus.PeerDirectory had exactly
// one caller between them. So the transport, the wire contract and both
// interfaces followed the app, and the core keeps nothing of Өртөө: no tables
// (00087), no routes, no environment variables.
func Bootstrap(p nexus.Platform) Runtime {
	return Runtime{}
}
