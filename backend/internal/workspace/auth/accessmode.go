/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Whether strangers may become users of this platform, and what a platform in
 * Maintenance still allows.
 */

package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
	"github.com/jackc/pgx/v5"
)

// Access mode (§C of the plan; CP-3 item 0).
//
// Until now, whether a first-time visitor became a user of this platform was
// decided by which environment variables happened to be set: EID_JIT_TENANT_SLUG
// made eID provision accounts, SSO_CLIENT_TENANT made a federated provider do
// the same. Two variables in two files, read by two packages, and no single
// place that answered "can a stranger get in".
//
// This is that place. One check, at the one thing all of those paths do —
// create an account — and it is closed by default.
//
// What it does NOT close:
//
//   - Signing in as somebody who already has an account. The mode is about
//     provisioning, not authentication; a person who was invited last week
//     signs in exactly as before.
//   - The invitation flow. An operator or a tenant administrator inviting
//     somebody *is* the pre-registration, so an account created that way is
//     not just-in-time provisioning and is unaffected.
//   - The console. Operators are a separate table and a separate world.

// ErrProvisioningClosed is what the account-creating paths get in private mode.
var ErrProvisioningClosed = errors.New("this platform is closed: only people who have already been registered may sign in")

// AccessMode returns the current mode.
func AccessMode() string { return settings.Get(settings.AccessMode) }

// PlatformIsPublic reports whether strangers may be provisioned. Exported for
// the handful of screens that ask before offering to sign somebody up.
func PlatformIsPublic() bool { return AccessMode() == settings.AccessPublic }

// MayProvisionAccount is the one check.
//
// It returns a SignInError, so every caller that already knows how to show a
// reason to somebody signing in shows this one — in their own language, on the
// sign-in screen, rather than as a 500 they cannot act on.
func MayProvisionAccount(method string) error {
	if PlatformIsPublic() {
		return nil
	}
	// Logged at info rather than warn: in private mode this is the platform
	// working correctly, and it will happen every time somebody outside the
	// organisation tries the front door. What makes it worth a line at all is
	// the support call that follows — "it says I have no access" — which is
	// answered by this line naming the method they used.
	slog.Info("refused to provision an account: the platform is private", "method", method)
	return SignInError{"Энэ платформ хаалттай горимд байна. Танд эрх нээгдээгүй байна — " +
		"байгууллагынхаа админаас урилга хүсэн үү."}
}

// Maintenance.
//
// Two levels, and they mean the same thing to the person using the platform:
// you may look, you may not change. The platform-wide one is a setting; a
// single organisation's is a column on its row, set by the console.
//
// Reads are deliberately still allowed. A Maintenance mode that refuses
// everything is an outage with a nicer message, and the reason to have one at
// all is to keep people able to see what they need while something underneath
// is being moved.

// maintenanceNotice is what a request is refused with, and what the shell shows
// in its banner.
type maintenanceNotice struct {
	Active  bool   `json:"active"`
	Message string `json:"message"`
	// Scope is "platform" or "tenant", so the banner can say whether this is
	// everybody's afternoon or only theirs.
	Scope string `json:"scope,omitempty"`
}

// Maintenance reports whether writing is closed for this organisation.
func (h *Handlers) Maintenance(ctx context.Context, tenantID string) maintenanceNotice {
	if settings.Bool(settings.Maintenance) {
		return maintenanceNotice{
			Active:  true,
			Scope:   "platform",
			Message: settings.Get(settings.MaintenanceMessage),
		}
	}
	if tenantID == "" {
		return maintenanceNotice{}
	}

	var at *time.Time
	var message string
	if err := h.db.QueryRow(ctx,
		`SELECT maintenance_at, maintenance_message FROM registry.tenants WHERE id = $1::uuid`,
		tenantID).Scan(&at, &message); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("could not check whether the organisation is in Maintenance",
				"tenant_id", tenantID, "error", err)
		}
		return maintenanceNotice{}
	}
	if at == nil {
		return maintenanceNotice{}
	}
	return maintenanceNotice{Active: true, Scope: "tenant", Message: message}
}

// RefuseIfReadOnly answers 503 to a write while Maintenance is on.
//
// Safe methods pass, and so does signing out: somebody who wants to leave
// should always be able to, and a Maintenance mode that traps people in a
// session is one nobody will turn on again.
func (h *Handlers) RefuseIfReadOnly(w http.ResponseWriter, r *http.Request, tenantID string) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	if r.URL.Path == "/api/v1/auth/logout" {
		return false
	}

	notice := h.Maintenance(r.Context(), tenantID)
	if !notice.Active {
		return false
	}

	message := notice.Message
	if message == "" {
		message = "Платформ засварын горимд байна. Түр хугацаанд зөвхөн унших боломжтой."
	}
	// 503 with Retry-After, because that is what this is: a temporary
	// unavailability of writing, and a client that retries later is behaving
	// correctly.
	w.Header().Set("Retry-After", "600")
	httpx.JSON(w, http.StatusServiceUnavailable, map[string]any{
		"error":       message,
		"Maintenance": notice,
	})
	return true
}

// Notice is one thing the platform wants to tell somebody: a Maintenance
// window, or an announcement an operator broadcast.
type Notice struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Notices assembles what the shell should show this person.
//
// One query and one settings read, on /me, which the shell asks for on every
// page load anyway — rather than a second endpoint the shell would have to
// remember to poll.
func (h *Handlers) Notices(ctx context.Context, tenantID string) []Notice {
	Notices := make([]Notice, 0, 2)

	if notice := h.Maintenance(ctx, tenantID); notice.Active {
		message := notice.Message
		if message == "" {
			message = "Платформ засварын горимд байна: одоогоор зөвхөн унших боломжтой."
		}
		Notices = append(Notices, Notice{Kind: "Maintenance", Title: message})
	}

	rows, err := h.db.Query(ctx,
		`SELECT kind, title, body FROM registry.announcements
		  WHERE starts_at <= NOW() AND (ends_at IS NULL OR ends_at > NOW())
		  ORDER BY starts_at DESC
		  LIMIT 5`)
	if err != nil {
		slog.Warn("could not read the announcements", "error", err)
		return Notices
	}
	defer rows.Close()
	for rows.Next() {
		var notice Notice
		if err := rows.Scan(&notice.Kind, &notice.Title, &notice.Body); err != nil {
			slog.Warn("could not read an announcement", "error", err)
			return Notices
		}
		Notices = append(Notices, notice)
	}
	return Notices
}

// WarnAboutConflictingConfiguration says so when the demo seeder and a private
// platform are both asked for.
//
// The seeder creates accounts, which is precisely what private mode exists to
// prevent, so a deployment with both is one where somebody's intent is not
// what the configuration says. Neither is overridden — guessing which one was
// meant is worse than saying so — and the console's home screen shows the same
// warning, because a line in a boot log is seen by nobody after the first
// morning.
func WarnAboutConflictingConfiguration() {
	if !config.SeedingEnabled() {
		return
	}
	if PlatformIsPublic() {
		return
	}
	slog.Warn("the demo seeder is enabled while the platform is private; "+
		"the seeded accounts exist but nobody else will be provisioned",
		"access_mode", AccessMode(), "remedy",
		"switch the platform to public in the console, or unset SEED_DEMO_DATA")
}

// ConfigurationWarnings is what the console's home screen shows. A slice
// rather than a boolean so CP-4 can add to it without changing the shape.
func ConfigurationWarnings() []string {
	warnings := make([]string, 0, 1)
	if config.SeedingEnabled() && !PlatformIsPublic() {
		warnings = append(warnings,
			"Demo seeder асаалттай атлаа платформ хаалттай горимд байна: "+
				"seed хийсэн бүртгэлүүд ажиллах ч өөр хэн ч бүртгүүлж чадахгүй.")
	}
	return warnings
}
