/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package setup is the first-run wizard: the screens a deployment is
// configured from before anybody can sign in to it.
//
// It exists alongside cmd/tenant-bootstrap rather than instead of it. The
// command is the answer for somebody at a terminal with the database
// credentials; this is the answer for somebody standing in front of the
// browser, which is where the organisation's own details are — and where the
// Gerege Core lookup can fill them in rather than being typed twice.
//
// What makes a first-run web page safe is the part most implementations leave
// out. An open /setup on a public HTTPS host is owned by whoever reaches it
// first, so this one is armed with a token: 256 bits, minted in memory at boot
// only while the deployment has no organisation, written once to the log the
// operator is already reading, never stored, and dropped the moment an
// organisation exists. That is the same bargain internal/operator/operator
// makes — authority that comes from already holding the host, and that leaves
// nothing behind.
//
// The same authority is what lets the wizard open the operator console by
// creating its first account. The console has no sign-up screen and this is not
// one: it answers only behind that token, only while this deployment has no
// operator at all, and only where there is a CONTROL_PLANE_HOST for the console
// to answer on. Every account after the first is the console's own to make.
package setup

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/credentials"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/geregecore"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/tenants"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service is the wizard.
type Service struct {
	db   *pgxpool.Pool
	core *geregecore.Client

	// mu guards token, which is the whole of this service's state. It is a
	// string rather than a row because it must not survive the process: a
	// restart mints a new one and says so, and nothing on disk can be replayed.
	mu    sync.Mutex
	token string
}

// New builds the wizard. It performs no I/O and arms nothing: see Arm.
func New(db *pgxpool.Pool) *Service {
	return &Service{db: db, core: geregecore.New(os.Getenv("GEREGE_CORE_URL"),
		func() string { return credentials.Get(credentials.CoreAPIToken) })}
}

// Arm mints the setup token when the deployment has no organisation, and says
// so in the log. It is a no-op on a deployment that has one, which is what
// makes it safe to call on every boot.
//
// The token is logged rather than mailed or written to a file because the
// person who can read this log is the person who deployed this: the same
// authority the bootstrap command asks for, spelled differently.
func (s *Service) Arm(ctx context.Context) {
	empty, err := tenants.Unprovisioned(ctx, s.db)
	if err != nil {
		slog.Warn("could not tell whether this deployment has an organisation", "error", err)
		return
	}
	if !empty {
		return
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		slog.Error("could not mint the setup token; use the tenant-bootstrap command instead", "error", err)
		return
	}
	token := hex.EncodeToString(buf)
	s.mu.Lock()
	s.token = token
	s.mu.Unlock()

	origin := strings.TrimRight(os.Getenv("PUBLIC_ORIGIN"), "/")
	if origin == "" {
		origin = "http://localhost:3000"
	}
	slog.Warn("this deployment has no organisation, so nobody can sign in yet; "+
		"open the setup wizard with the address below, or run the tenant-bootstrap command",
		"setup_url", origin+"/setup?token="+token,
		"command", `docker exec -it <backend container> /app/tenant-bootstrap `+
			`-org "Your Organisation" -slug your-org -email you@example.mn -name "Your Name"`)
}

// disarm forgets the token. Called when an organisation exists, so the wizard
// stops being a door the moment it has been walked through.
func (s *Service) disarm() {
	s.mu.Lock()
	s.token = ""
	s.mu.Unlock()
}

// armed reports whether the wizard would accept a token at all.
func (s *Service) armed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token != ""
}

// authorised compares the presented token with the minted one in constant time.
func (s *Service) authorised(r *http.Request) bool {
	s.mu.Lock()
	want := s.token
	s.mu.Unlock()
	if want == "" {
		return false
	}
	got := strings.TrimSpace(r.Header.Get("X-Setup-Token"))
	if got == "" {
		got = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// Routes mounts the wizard. Every path is public in the sense the route policy
// means — no session can exist yet — and every one but the status carries the
// token instead. See pkg/host/route_policy_test.go.
func (s *Service) Routes(r chi.Router) {
	r.Route("/api/v1/setup", func(r chi.Router) {
		r.Get("/status", s.handleStatus)
		r.Group(func(r chi.Router) {
			r.Use(s.requireToken)
			r.Post("/organisation", s.handleFindOrganisation)
			r.Post("/person", s.handleFindPerson)
			// The console's first operator, and the code that proves its
			// authenticator. Both before /complete, never after: completing
			// disarms the token these two are gated by.
			r.Post("/operator", s.handleCreateOperator)
			r.Post("/operator/confirm", s.handleConfirmOperator)
			r.Post("/complete", s.handleComplete)
		})
	})
}

// requireToken is the gate. It answers 404 rather than 401 on a deployment that
// is not being set up: an endpoint that says "wrong token" to a stranger is an
// endpoint that tells them there is a token to guess.
func (s *Service) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authorised(r) {
			httpx.Error(w, http.StatusNotFound, "not found")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleStatus is what the browser asks before it renders anything.
//
// It is the one route here without the token, because the sign-in screen has to
// know whether to send somebody to the wizard and it has nothing to present. It
// discloses one bit — that this deployment has no organisation — which is
// already visible to anybody who tries to sign in.
func (s *Service) handleStatus(w http.ResponseWriter, r *http.Request) {
	empty, err := tenants.Unprovisioned(r.Context(), s.db)
	if err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "the database is not reachable")
		return
	}
	if !empty {
		s.disarm()
	}
	// Whether a console can be opened here, and whether one is still unclaimed.
	// A deployment with no CONTROL_PLANE_HOST has nowhere to serve one, and one
	// that already has an operator is past this door — in both cases the wizard
	// leaves the step out rather than offering something that will refuse.
	consoleHost := operator.ConfiguredHost()
	consoleEmpty := false
	if consoleHost != "" {
		if none, err := operator.None(r.Context(), s.db); err == nil {
			consoleEmpty = none
		} else {
			slog.Warn("could not tell whether this deployment has an operator", "error", err)
		}
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"required": empty,
		"console": map[string]any{
			"host":  consoleHost,
			"empty": consoleEmpty,
		},
		// Whether the wizard can actually be used, as opposed to whether it is
		// needed. False on a deployment that came up before this release, or
		// one whose token was lost to a restart: the screen then says to look
		// in the log rather than offering a form that will refuse.
		"armed": empty && s.armed(),
		"core":  s.core.Configured(),
	})
}

// Completion is the wizard's last step.
type Completion struct {
	Organisation struct {
		Name               string `json:"name"`
		Slug               string `json:"slug"`
		LegalName          string `json:"legal_name"`
		RegistrationNumber string `json:"registration_number"`
	} `json:"organisation"`
	Admin struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"admin"`
	Password string `json:"password"`
}

func (s *Service) handleComplete(w http.ResponseWriter, r *http.Request) {
	var in Completion
	if err := httpx.DecodeLimited(r, &in, 1<<16); err != nil {
		httpx.Error(w, http.StatusBadRequest, "the request could not be read")
		return
	}

	tenantID, _, err := tenants.Bootstrap(r.Context(), s.db, tenants.FirstTenant{
		Slug:               in.Organisation.Slug,
		Name:               in.Organisation.Name,
		LegalName:          in.Organisation.LegalName,
		RegistrationNumber: in.Organisation.RegistrationNumber,
		AdminEmail:         in.Admin.Email,
		AdminName:          in.Admin.Name,
		Password:           in.Password,
	})
	if errors.Is(err, tenants.ErrAlreadyProvisioned) {
		s.disarm()
		httpx.Error(w, http.StatusConflict, "this deployment already has an organisation")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// The door closes behind the first person through it, before the response
	// is written: a token that stayed valid for the second request would be a
	// way to create a second organisation nobody asked for.
	s.disarm()
	slog.Info("the deployment was set up from the wizard", "tenant_id", tenantID, "admin", in.Admin.Email)

	httpx.JSON(w, http.StatusCreated, map[string]any{
		"tenant_id": tenantID,
		"slug":      in.Organisation.Slug,
	})
}

// handleCreateOperator opens the control plane by giving it its first account.
//
// The console has no sign-up screen and this is not one: it answers only behind
// the wizard's token — minted in memory at boot, written once to the log, gone
// the moment an organisation exists — and only while this deployment has no
// operator at all. That is the same authority operator-bootstrap requires,
// which is "already holds this deployment", rather than a new one; what changes
// is that the person exercising it does not need a shell on the box.
//
// Two further conditions, both refusals rather than warnings. Without
// CONTROL_PLANE_HOST there is no address the console would answer on, so an
// account made here could never sign in. With an operator already present this
// route would be a way to mint a second, which is the console's own privilege
// and stays there: migration 00049 withholds INSERT on operator_accounts from
// the console role precisely so that a flaw in one handler cannot make an
// operator, and a wizard that could do it twice would be that flaw.
func (s *Service) handleCreateOperator(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := httpx.DecodeLimited(r, &in, 1<<14); err != nil {
		httpx.Error(w, http.StatusBadRequest, "the request could not be read")
		return
	}

	if operator.ConfiguredHost() == "" {
		httpx.Error(w, http.StatusConflict, "this deployment has no console address (CONTROL_PLANE_HOST)")
		return
	}
	none, err := operator.None(r.Context(), s.db)
	if err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "the database is not reachable")
		return
	}
	if !none {
		httpx.Error(w, http.StatusConflict, "this deployment already has an operator")
		return
	}

	// superadmin, and not a choice: it is the first account, so anything less
	// would leave a console nobody can grant a role from.
	account, enrolment, err := operator.CreateOperator(r.Context(), s.db, operator.NewOperator{
		Email:    in.Email,
		Name:     in.Name,
		Role:     operator.RoleSuperadmin,
		Password: in.Password,
	})
	if errors.Is(err, operator.ErrOperatorExists) {
		httpx.Error(w, http.StatusConflict, "an operator with that address already exists")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// The address, but never the secret: the enrolment is what the person is
	// about to photograph, and a log line is where it would outlive them.
	slog.Info("the console's first operator was created from the wizard", "operator", account.Email)

	httpx.JSON(w, http.StatusCreated, map[string]any{
		"secret": enrolment.Secret,
		"uri":    enrolment.URI,
		"host":   operator.ConfiguredHost(),
	})
}

// handleConfirmOperator finishes the enrolment with a code from the authenticator.
//
// The account is found by its address rather than by an id held between the two
// requests: an unconfirmed enrolment is already a state this platform knows how
// to find (operator.PendingEnrolment, which the bootstrap command's -confirm
// uses), and a wizard that carried the id in memory would lose it to the
// restart that a half-finished enrolment tends to be followed by.
func (s *Service) handleConfirmOperator(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := httpx.DecodeLimited(r, &in, 1<<12); err != nil {
		httpx.Error(w, http.StatusBadRequest, "the request could not be read")
		return
	}

	id, err := operator.PendingEnrolment(r.Context(), s.db, in.Email)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := operator.ConfirmSecondFactor(r.Context(), s.db, id, strings.TrimSpace(in.Code)); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	slog.Info("the console's first operator confirmed their authenticator", "operator", in.Email)
	w.WriteHeader(http.StatusNoContent)
}

// handleFindOrganisation fills the organisation step from the directory.
//
// The slug is suggested rather than derived from the name: an organisation's
// name is Cyrillic here, and a slug transliterated from it would be a guess
// that ends up in every URL. The registration number is already unique, already
// stable, and already valid against slugPattern.
func (s *Service) handleFindOrganisation(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RegistrationNumber string `json:"registration_number"`
	}
	if err := httpx.DecodeLimited(r, &in, 1<<14); err != nil {
		httpx.Error(w, http.StatusBadRequest, "the request could not be read")
		return
	}

	org, err := s.core.FindOrganisation(r.Context(), in.RegistrationNumber)
	if err != nil {
		s.failLookup(w, err)
		return
	}
	name := org.Name
	if name == "" {
		name = org.ShortName
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"core_id":             org.ID,
		"name":                name,
		"legal_name":          org.Name,
		"registration_number": org.RegNo,
		"suggested_slug":      org.RegNo,
		"email":               org.Email,
		"phone":               org.PhoneNo,
		"address":             org.AddressDetail,
	})
}

// handleFindPerson fills the administrator step from the directory.
func (s *Service) handleFindPerson(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RegistrationNumber string `json:"registration_number"`
		CountryCode        string `json:"country_code"`
	}
	if err := httpx.DecodeLimited(r, &in, 1<<14); err != nil {
		httpx.Error(w, http.StatusBadRequest, "the request could not be read")
		return
	}

	person, err := s.core.FindPerson(r.Context(), in.RegistrationNumber, in.CountryCode)
	if err != nil {
		s.failLookup(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"core_id":             person.ID,
		"name":                person.FullName(),
		"email":               person.Email,
		"phone":               person.PhoneNo,
		"registration_number": person.RegNo,
	})
}

// failLookup keeps the directory's three answers apart, because the operator
// standing in front of the wizard does something different about each: a
// misspelled number is retyped, a missing token is configured, and a directory
// that cannot be reached is waited for.
func (s *Service) failLookup(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, geregecore.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "the Gerege Core directory has no record with that number")
	case errors.Is(err, geregecore.ErrNotConfigured):
		httpx.Error(w, http.StatusServiceUnavailable, err.Error())
	default:
		httpx.Error(w, http.StatusBadGateway, err.Error())
	}
}
