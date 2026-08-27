/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package profile

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/credentials"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/geregecore"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// Refreshing an organisation's own details from the register that holds them.
//
// The fields on this screen are the organisation's legal identity: they print
// on documents and the XYP rail checks a registration number against them. They
// were also, until now, typed in — once, by whoever installed the software, and
// never again. An organisation that changes its address changes it at the
// register and nowhere else, so the copy here goes stale silently, and the day
// it matters is the day somebody is looking at a document that disagrees with
// the state's record of them.
//
// This asks the register instead. It is deliberately a button rather than a
// nightly job: what comes back overwrites what an administrator can see on the
// screen in front of them, and doing that unattended would mean an edit
// somebody made on Tuesday quietly disappearing on Wednesday.

// core is the directory client, built per request so a token an operator sets
// in the console is the token the next refresh presents.
func core() *geregecore.Client {
	return geregecore.New(os.Getenv("GEREGE_CORE_URL"),
		func() string { return credentials.Get(credentials.CoreAPIToken) })
}

// HandleSyncTenantProfileFromCore is mounted behind requireAdmin, like the
// update it performs.
func (h *Handlers) HandleSyncTenantProfileFromCore(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := nexus.RequireWorkspace(w, r)
	if !ok {
		return
	}

	var registration string
	if err := h.db.QueryRow(r.Context(),
		`SELECT COALESCE(registration_number, '') FROM workspace.tenant_profiles WHERE tenant_id = $1`,
		tenantID).Scan(&registration); err != nil {
		slog.Error("tenant: could not read the registration number", "error", err, "tenant_id", tenantID)
		httpx.Error(w, http.StatusInternalServerError, "could not load the organisation")
		return
	}
	if strings.TrimSpace(registration) == "" {
		httpx.Error(w, http.StatusBadRequest,
			"this organisation has no registration number, so there is nothing to look up")
		return
	}

	found, err := core().FindOrganisation(r.Context(), registration)
	switch {
	case errors.Is(err, geregecore.ErrNotConfigured):
		httpx.Error(w, http.StatusServiceUnavailable, err.Error())
		return
	case errors.Is(err, geregecore.ErrNotFound):
		httpx.Error(w, http.StatusNotFound,
			"the Gerege Core register has no organisation with that number")
		return
	case err != nil:
		httpx.Error(w, http.StatusBadGateway, err.Error())
		return
	}

	// COALESCE on the register's side rather than the column's: a field the
	// directory does not hold leaves what is here alone. A refresh that blanked
	// a phone number because the register has none would be worse than one that
	// never ran.
	if _, err := h.db.Exec(r.Context(),
		`UPDATE workspace.tenant_profiles
		    SET legal_name   = COALESCE(NULLIF($2, ''), legal_name),
		        phone        = COALESCE(NULLIF($3, ''), phone),
		        email        = COALESCE(NULLIF($4, ''), email),
		        address_line = COALESCE(NULLIF($5, ''), address_line),
		        logo_url     = COALESCE(NULLIF($6, ''), logo_url)
		  WHERE tenant_id = $1`,
		tenantID, found.Name, found.PhoneNo, found.Email, found.AddressDetail, found.LogoImageURL,
	); err != nil {
		slog.Error("tenant: could not write the refreshed profile", "error", err, "tenant_id", tenantID)
		httpx.Error(w, http.StatusInternalServerError, "could not save the organisation")
		return
	}

	slog.Info("tenant: the organisation was refreshed from the Gerege Core register",
		"tenant_id", tenantID, "registration_number", found.RegNo, "core_id", found.ID)
	h.HandleGetTenantProfile(w, r)
}
