/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * What the organisation legally is, and what the person using it prefers.
 *
 * Both used to live in the organisation app, and neither belonged there. An app
 * is installed per tenant and an administrator can remove it; these two are
 * read by things that have no opinion about which apps a tenant has:
 *
 *	the control plane names an organisation when it creates or suspends one;
 *	the XYP rail proves a legal entity against its registration number;
 *	an SSO consent screen shows whose account the caller is about to hand over;
 *	every document and invoice prints the registered name and address.
 *
 * A tenant whose only installed app has been removed still has a legal name, and
 * a person with no apps at all still has a language. So the legal profile is a
 * property of the tenant and the preference is a property of the person, and
 * both are served here — on the platform path, behind authentication and
 * nothing else. What stays an app is the part that really is one: departments
 * and the people in them (see internal/apps/organisation).
 */

package profile

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/jackc/pgx/v5"
)

// TenantProfile is what the tenant is, as opposed to what it is called.
//
// The name on the sidebar is in `tenants`; everything here is the version a
// document, an invoice or a government request has to carry — a legal name, a
// registration number, an address somebody could deliver to.
type TenantProfile struct {
	TenantID           string `json:"tenant_id"`
	Slug               string `json:"slug"`
	Name               string `json:"name"`
	LegalName          string `json:"legal_name"`
	RegistrationNumber string `json:"registration_number"`
	TaxNumber          string `json:"tax_number"`
	CountryCode        string `json:"country_code"`
	Province           string `json:"province"`
	District           string `json:"district"`
	Khoroo             string `json:"khoroo"`
	AddressLine        string `json:"address_line"`
	PostalCode         string `json:"postal_code"`
	Phone              string `json:"phone"`
	Email              string `json:"email"`
	Website            string `json:"website"`
	LogoURL            string `json:"logo_url"`
	Timezone           string `json:"timezone"`
	Locale             string `json:"locale"`
	Currency           string `json:"currency"`
	// The organisation this one is a subsidiary of, if any. It is a statement
	// about the world and not a grant: see migration 00036. A branch or an
	// office is a department, not this.
	ParentTenantID string `json:"parent_tenant_id,omitempty"`
	ParentName     string `json:"parent_name,omitempty"`
}

const tenantProfileColumns = `SELECT t.id::text, t.slug, t.name,
	p.legal_name, p.registration_number, p.tax_number, p.country_code,
	p.province, p.district, p.khoroo, p.address_line, p.postal_code,
	p.phone, p.email, p.website, p.logo_url, p.timezone, p.locale, p.currency,
	COALESCE(p.parent_tenant_id::text, ''), COALESCE(parent.name, '')
	FROM registry.tenants t
	JOIN workspace.tenant_profiles p ON p.tenant_id = t.id
	LEFT JOIN registry.tenants parent ON parent.id = p.parent_tenant_id
	WHERE t.id = $1`

// HandleGetTenantProfile answers for any member of the organisation. It used to
// require the organisation app's read permission, which every role held anyway
// — and the name it returns is already on the sidebar of every screen.
func (h *Handlers) HandleGetTenantProfile(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := nexus.RequireWorkspace(w, r)
	if !ok {
		return
	}

	var o TenantProfile
	err := h.db.QueryRow(r.Context(), tenantProfileColumns, tenantID).Scan(
		&o.TenantID, &o.Slug, &o.Name, &o.LegalName, &o.RegistrationNumber, &o.TaxNumber,
		&o.CountryCode, &o.Province, &o.District, &o.Khoroo, &o.AddressLine, &o.PostalCode,
		&o.Phone, &o.Email, &o.Website, &o.LogoURL, &o.Timezone, &o.Locale, &o.Currency,
		&o.ParentTenantID, &o.ParentName)
	if err != nil {
		// The profile row is created with the tenant and by the migration, so
		// its absence is a broken invariant rather than a missing page.
		slog.Error("tenant: could not read the profile", "error", err, "tenant_id", tenantID)
		httpx.Error(w, http.StatusInternalServerError, "could not load the organisation")
		return
	}
	httpx.JSON(w, http.StatusOK, o)
}

// HandleUpdateTenantProfile is mounted behind requireAdmin.
//
// It was behind the organisation app's manage permission, which the manager role
// also held. Tightening it to the tenant administrator is deliberate: the fields
// here are the organisation's legal identity, they print on documents and they
// are what the XYP rail checks a registration number against, and this is now a
// platform setting rather than a screen inside an app somebody chose to install.
func (h *Handlers) HandleUpdateTenantProfile(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := nexus.RequireWorkspace(w, r)
	if !ok {
		return
	}

	var body struct {
		Name               *string `json:"name"`
		LegalName          *string `json:"legal_name"`
		RegistrationNumber *string `json:"registration_number"`
		TaxNumber          *string `json:"tax_number"`
		CountryCode        *string `json:"country_code"`
		Province           *string `json:"province"`
		District           *string `json:"district"`
		Khoroo             *string `json:"khoroo"`
		AddressLine        *string `json:"address_line"`
		PostalCode         *string `json:"postal_code"`
		Phone              *string `json:"phone"`
		Email              *string `json:"email"`
		Website            *string `json:"website"`
		LogoURL            *string `json:"logo_url"`
		Timezone           *string `json:"timezone"`
		Locale             *string `json:"locale"`
		Currency           *string `json:"currency"`
		ParentTenantID     *string `json:"parent_tenant_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "malformed request body")
		return
	}

	// Pointers rather than values, so a form that sends three fields changes
	// three fields. A struct of plain strings would blank everything the caller
	// happened not to mention — which is how a registration number disappears
	// because somebody edited a phone number.
	// The parent is settled before anything is written, because two of its
	// three refusals are about other tenants and one of them has to look
	// outside this one — which is not something to do half way through a
	// transaction that has already changed the profile.
	var parent *string
	if body.ParentTenantID != nil {
		resolved, ok := h.resolveParentTenant(w, r, tenantID, strings.TrimSpace(*body.ParentTenantID))
		if !ok {
			return
		}
		parent = resolved
	}

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "could not save the organisation")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	if body.Name != nil {
		if strings.TrimSpace(*body.Name) == "" {
			httpx.Error(w, http.StatusBadRequest, "an organisation needs a name")
			return
		}
		if _, err := tx.Exec(r.Context(),
			`UPDATE registry.tenants SET name = $1 WHERE id = $2`, strings.TrimSpace(*body.Name), tenantID); err != nil {
			slog.Error("tenant: could not rename the organisation", "error", err, "tenant_id", tenantID)
			httpx.Error(w, http.StatusInternalServerError, "could not save the organisation")
			return
		}
	}

	if body.ParentTenantID != nil {
		if _, err := tx.Exec(r.Context(),
			`UPDATE workspace.tenant_profiles SET parent_tenant_id = $2::uuid, updated_at = NOW()
			 WHERE tenant_id = $1`, tenantID, parent); err != nil {
			slog.Error("tenant: could not set the parent organisation", "error", err, "tenant_id", tenantID)
			httpx.Error(w, http.StatusInternalServerError, "could not save the organisation")
			return
		}
	}

	if _, err := tx.Exec(r.Context(),
		`UPDATE workspace.tenant_profiles SET
		     legal_name          = COALESCE($2, legal_name),
		     registration_number = COALESCE($3, registration_number),
		     tax_number          = COALESCE($4, tax_number),
		     country_code        = COALESCE($5, country_code),
		     province            = COALESCE($6, province),
		     district            = COALESCE($7, district),
		     khoroo              = COALESCE($8, khoroo),
		     address_line        = COALESCE($9, address_line),
		     postal_code         = COALESCE($10, postal_code),
		     phone               = COALESCE($11, phone),
		     email               = COALESCE($12, email),
		     website             = COALESCE($13, website),
		     logo_url            = COALESCE($14, logo_url),
		     timezone            = COALESCE($15, timezone),
		     locale              = COALESCE($16, locale),
		     currency            = COALESCE($17, currency),
		     updated_at          = NOW()
		 WHERE tenant_id = $1`,
		tenantID, body.LegalName, body.RegistrationNumber, body.TaxNumber, body.CountryCode,
		body.Province, body.District, body.Khoroo, body.AddressLine, body.PostalCode,
		body.Phone, body.Email, body.Website, body.LogoURL, body.Timezone, body.Locale,
		body.Currency); err != nil {
		slog.Error("tenant: could not save the profile", "error", err, "tenant_id", tenantID)
		httpx.Error(w, http.StatusInternalServerError, "could not save the organisation")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "could not save the organisation")
		return
	}
	h.HandleGetTenantProfile(w, r)
}

// resolveParentTenant decides whether this organisation may be recorded as a
// subsidiary of the one named, and answers the caller itself when it may not.
//
// Three refusals, and each is a different kind of wrong:
//
//	itself — the schema refuses that one too, but saying so in words beats a
//	  constraint violation surfacing as "could not save the organisation";
//
//	a chain that comes back round — A under B under A. The schema cannot see
//	  this; a CHECK constrains one row and this is a walk. Left in, every
//	  reader that follows the chain hangs;
//
//	an organisation the caller has nothing to do with. Anyone could otherwise
//	  declare their company a subsidiary of a ministry, and the claim would
//	  print on documents. Membership of the parent is the smallest honest
//	  proof of a relationship this platform can check, and it is the same
//	  proof the tenant switcher already trusts.
func (h *Handlers) resolveParentTenant(w http.ResponseWriter, r *http.Request, tenantID, parentID string) (*string, bool) {
	if parentID == "" {
		return nil, true // "no parent" is a value, not an omission
	}
	if parentID == tenantID {
		httpx.Error(w, http.StatusBadRequest, "an organisation cannot be a subsidiary of itself")
		return nil, false
	}
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return nil, false
	}

	// Crossing tenants is the point here, so the tenant binding comes off for
	// these two reads — the same thing the tenant switcher does, and for the
	// same reason: under this tenant's policies, another tenant's rows are not
	// visible at all, and the answer would always be "no such organisation".
	ctx := nexus.WithoutWorkspace(r.Context())

	var member bool
	if err := h.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM workspace.memberships
		                 WHERE tenant_id = $1::uuid AND user_id = $2 AND active)`,
		parentID, claims.UserID).Scan(&member); err != nil {
		slog.Error("tenant: could not check the parent organisation", "error", err, "parent", parentID)
		httpx.Error(w, http.StatusInternalServerError, "could not check that organisation")
		return nil, false
	}
	if !member {
		// Deliberately the same answer whether the organisation does not exist
		// or the caller simply does not belong to it. Which of the two it is
		// would itself be an answer about somebody else's organisation.
		httpx.Error(w, http.StatusBadRequest,
			"you can only record an organisation you belong to as the parent")
		return nil, false
	}

	// Walk up from the proposed parent. Meeting ourselves means the link would
	// close a loop. The bound is a guard against a cycle that already exists in
	// the data rather than a limit on how deep a group may be.
	seen, cursor := 0, parentID
	for cursor != "" && seen < 32 {
		if cursor == tenantID {
			httpx.Error(w, http.StatusConflict,
				"that organisation is already below this one; the two would report to each other")
			return nil, false
		}
		var next string
		if err := h.db.QueryRow(ctx,
			`SELECT COALESCE(parent_tenant_id::text, '') FROM workspace.tenant_profiles WHERE tenant_id = $1::uuid`,
			cursor).Scan(&next); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				break
			}
			slog.Error("tenant: could not walk the organisation chain", "error", err, "at", cursor)
			httpx.Error(w, http.StatusInternalServerError, "could not check that organisation")
			return nil, false
		}
		cursor = next
		seen++
	}

	return &parentID, true
}

// Preferences are the person's own, and follow them between organisations.
type Preferences struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Locale   string `json:"locale"`
	Timezone string `json:"timezone"`
	// What the organisation would use if the person expresses no preference.
	// Sent alongside so a screen can say "Mongolian (organisation default)"
	// rather than showing an empty selector.
	OrganisationLocale   string `json:"organisation_locale"`
	OrganisationTimezone string `json:"organisation_timezone"`
}

func (h *Handlers) HandleGetPreferences(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := nexus.RequireWorkspace(w, r)
	if !ok {
		return
	}
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var p Preferences
	if err := h.db.QueryRow(r.Context(),
		`SELECT u.name, u.email, u.phone, u.locale, u.timezone, tp.locale, tp.timezone
		   FROM registry.users u, workspace.tenant_profiles tp
		  WHERE u.id = $1 AND tp.tenant_id = $2`, claims.UserID, tenantID).
		Scan(&p.Name, &p.Email, &p.Phone, &p.Locale, &p.Timezone,
			&p.OrganisationLocale, &p.OrganisationTimezone); err != nil {
		slog.Error("tenant: could not read preferences", "error", err, "user_id", claims.UserID)
		httpx.Error(w, http.StatusInternalServerError, "could not load your preferences")
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func (h *Handlers) HandleUpdatePreferences(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		Name     *string `json:"name"`
		Phone    *string `json:"phone"`
		Locale   *string `json:"locale"`
		Timezone *string `json:"timezone"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "malformed request body")
		return
	}
	if body.Name != nil && strings.TrimSpace(*body.Name) == "" {
		httpx.Error(w, http.StatusBadRequest, "a name cannot be empty")
		return
	}
	// The email is deliberately not editable here. It is the login and the
	// address a verification link goes to, so changing it is a proof-of-address
	// flow rather than a text field — see emailverify.
	if _, err := h.db.Exec(r.Context(),
		`UPDATE registry.users SET
		     name     = COALESCE($2, name),
		     phone    = COALESCE($3, phone),
		     locale   = COALESCE($4, locale),
		     timezone = COALESCE($5, timezone)
		 WHERE id = $1`,
		claims.UserID, body.Name, body.Phone, body.Locale, body.Timezone); err != nil {
		slog.Error("tenant: could not save preferences", "error", err, "user_id", claims.UserID)
		httpx.Error(w, http.StatusInternalServerError, "could not save your preferences")
		return
	}
	h.HandleGetPreferences(w, r)
}
