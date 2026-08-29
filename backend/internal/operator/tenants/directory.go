/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package tenants

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/geregecore"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Where a new organisation's details come from, and who its first
// administrator is.
//
// Both used to be typed. A name typed from a form is a name that is nearly
// right — a missing ХХК, a different transliteration — and an administrator
// typed as an e-mail address is an invitation sent to whatever was typed. The
// register answers the first properly, and the second is chosen from the
// people this platform has already seen prove who they are with eID.

// DirectoryOrganisation is what the register says about a registration number.
type DirectoryOrganisation struct {
	CoreID             int64  `json:"core_id"`
	Name               string `json:"name"`
	LegalName          string `json:"legal_name"`
	RegistrationNumber string `json:"registration_number"`
	SuggestedSlug      string `json:"suggested_slug"`
	Email              string `json:"email"`
	Phone              string `json:"phone"`
	Address            string `json:"address"`
}

// FindOrganisation asks the Gerege Core directory.
func (s *Service) FindOrganisation(ctx context.Context, regNo string) (DirectoryOrganisation, error) {
	org, err := s.core.FindOrganisation(ctx, strings.TrimSpace(regNo))
	if err != nil {
		return DirectoryOrganisation{}, err
	}
	name := org.Name
	if name == "" {
		name = org.ShortName
	}
	return DirectoryOrganisation{
		CoreID: org.ID, Name: name, LegalName: org.Name,
		RegistrationNumber: org.RegNo,
		// The register's own number is the slug worth suggesting: it is unique,
		// it is stable, and it is what every other system on this platform
		// already calls this organisation.
		SuggestedSlug: org.RegNo,
		Email:         org.Email, Phone: org.PhoneNo, Address: org.AddressDetail,
	}, nil
}

// DirectoryPerson is what the register says about a person's registration
// number. It is not an account on this platform and may never become one: the
// operator screen uses it to fill in a name and an address that are spelled
// the way the register spells them.
type DirectoryPerson struct {
	CoreID             int64  `json:"core_id"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	Phone              string `json:"phone"`
	RegistrationNumber string `json:"registration_number"`
}

// FindPerson asks the Gerege Core directory about one registration number.
func (s *Service) FindPerson(ctx context.Context, regNo string) (DirectoryPerson, error) {
	// The country code is required by the endpoint and would otherwise come
	// back as a complaint about a field the caller never saw.
	person, err := s.core.FindPerson(ctx, strings.TrimSpace(regNo), "MN")
	if err != nil {
		return DirectoryPerson{}, err
	}
	return DirectoryPerson{
		CoreID: person.ID, Name: person.FullName(), Email: person.Email,
		Phone: person.PhoneNo, RegistrationNumber: person.RegNo,
	}, nil
}

// VerifiedPerson is somebody who has signed in with eID on this deployment.
type VerifiedPerson struct {
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	RegNumber string    `json:"reg_number"`
	LinkedAt  time.Time `json:"linked_at"`
	LastSeen  time.Time `json:"last_seen_at"`
	// Organisations is how many they already belong to. An operator choosing
	// an administrator wants to know they are picking a person who is already
	// somewhere rather than a stranger with the same name.
	Organisations int `json:"organisations"`
}

// VerifiedPeople lists them, most recently seen first.
//
// Only people with an eID identity: the point of choosing rather than typing
// is that the platform has watched this person prove who they are.
func (s *Service) VerifiedPeople(ctx context.Context, search string) ([]VerifiedPerson, error) {
	search = strings.TrimSpace(search)
	rows, err := s.db.Query(operator.Scoped(ctx), `
		SELECT u.id::text, u.name, u.email, COALESCE(e.reg_number, ''),
		       e.linked_at, e.last_seen_at,
		       (SELECT count(*) FROM workspace.memberships m WHERE m.user_id = u.id)
		  FROM registry.user_eid_identities e
		  JOIN registry.users u ON u.id = e.user_id
		 WHERE $1 = '' OR u.name ILIKE '%' || $1 || '%'
		    OR u.email ILIKE '%' || $1 || '%'
		    OR COALESCE(e.reg_number, '') ILIKE '%' || $1 || '%'
		 ORDER BY e.last_seen_at DESC
		 LIMIT 50`, search)
	if err != nil {
		return nil, fmt.Errorf("control plane: list the verified people: %w", err)
	}
	defer rows.Close()

	people := make([]VerifiedPerson, 0, 16)
	for rows.Next() {
		var person VerifiedPerson
		if err := rows.Scan(&person.UserID, &person.Name, &person.Email, &person.RegNumber,
			&person.LinkedAt, &person.LastSeen, &person.Organisations); err != nil {
			return nil, fmt.Errorf("control plane: read a verified person: %w", err)
		}
		people = append(people, person)
	}
	return people, rows.Err()
}

func (s *Service) handleFindOrganisation(w http.ResponseWriter, r *http.Request) {
	org, err := s.FindOrganisation(r.Context(), r.URL.Query().Get("reg_no"))
	if err != nil {
		// The directory's three answers are kept apart, because the operator
		// does something different about each: a misspelled number is retyped,
		// a missing token is configured, and a directory that cannot be
		// reached is waited for.
		switch {
		case errors.Is(err, geregecore.ErrNotFound):
			httpx.Error(w, http.StatusNotFound, "the Gerege Core directory has no organisation with that number")
		case errors.Is(err, geregecore.ErrNotConfigured):
			httpx.Error(w, http.StatusServiceUnavailable, err.Error())
		default:
			httpx.Error(w, http.StatusBadGateway, err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusOK, org)
}

func (s *Service) handleFindPerson(w http.ResponseWriter, r *http.Request) {
	person, err := s.FindPerson(r.Context(), r.URL.Query().Get("reg_no"))
	if err != nil {
		switch {
		case errors.Is(err, geregecore.ErrNotFound):
			httpx.Error(w, http.StatusNotFound, "the Gerege Core directory has no person with that number")
		case errors.Is(err, geregecore.ErrNotConfigured):
			httpx.Error(w, http.StatusServiceUnavailable, err.Error())
		default:
			httpx.Error(w, http.StatusBadGateway, err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusOK, person)
}

func (s *Service) handleVerifiedPeople(w http.ResponseWriter, r *http.Request) {
	people, err := s.VerifiedPeople(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		fail(w, err, "could not read the verified people")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"people": people, "directory": s.core.Configured()})
}

// verifiedPerson reads one chosen administrator, and refuses an account that
// has never proved who it is.
func (s *Service) verifiedPerson(ctx context.Context, userID string) (VerifiedPerson, error) {
	var person VerifiedPerson
	err := s.db.QueryRow(operator.Scoped(ctx), `
		SELECT u.id::text, u.name, u.email
		  FROM registry.user_eid_identities e
		  JOIN registry.users u ON u.id = e.user_id
		 WHERE u.id = $1::uuid`, userID).
		Scan(&person.UserID, &person.Name, &person.Email)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return VerifiedPerson{}, errors.New("that person has not signed in with eID on this deployment")
	case err != nil:
		if operator.IsInvalidUUID(err) {
			return VerifiedPerson{}, errors.New("that is not a person on this deployment")
		}
		return VerifiedPerson{}, fmt.Errorf("control plane: read the chosen administrator: %w", err)
	}
	return person, nil
}

// AddMember puts somebody into an organisation with the least the platform
// offers.
//
// No role is granted here and that is deliberate: migration 00008's trigger
// gives every new membership the `user` role — read access and self-service —
// so the smallest thing this can do is insert the membership and let the
// database decide what that means. A console that chose the role would be a
// second answer to a question the schema already answers, and the day they
// disagreed the console's would win silently.
//
// Anything above `user` is granted by the organisation's own administrator, in
// their own access screen, where the person who has to live with the decision
// can see it.
func (s *Service) AddMember(ctx context.Context, sess operator.Session, tenantID, userID, reason string) error {
	person, err := s.verifiedPerson(ctx, userID)
	if err != nil {
		return err
	}

	// The console shows a limit on one screen; it must not be the way past it
	// on another. Soft enforcement warns rather than refuses, exactly as it
	// does everywhere else — the mode is the platform's decision, not this
	// screen's.
	quota, err := s.GetQuota(ctx, tenantID)
	if err != nil {
		return err
	}
	if quota.MaxUsers != nil && quota.Enforcement == EnforcementHard && quota.Users >= *quota.MaxUsers {
		return fmt.Errorf("this organisation is at its limit of %d people", *quota.MaxUsers)
	}

	return s.op.Do(ctx, sess, operator.Change{
		Action:     "tenant.member.add",
		TargetType: "tenant",
		TargetID:   tenantID,
		Reason:     reason,
		After:      map[string]any{"user_id": userID, "email": person.Email},
	}, func(ctx context.Context, tx pgx.Tx) error {
		var membershipID string
		err := tx.QueryRow(ctx,
			`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1::uuid, $2::uuid)
			 ON CONFLICT (tenant_id, user_id) DO NOTHING RETURNING id::text`,
			tenantID, userID).Scan(&membershipID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("that person is already in this organisation")
		}
		if err != nil {
			if operator.IsInvalidUUID(err) {
				return operator.ErrTenantNotFound
			}
			return fmt.Errorf("add the person to the organisation: %w", err)
		}
		return nil
	})
}

func (s *Service) handleAddMember(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		UserID string `json:"user_id"`
		Reason string `json:"reason"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.AddMember(r.Context(), sess, chi.URLParam(r, "id"), body.UserID, body.Reason); err != nil {
		fail(w, err, "could not add the person")
		return
	}
	s.changed(chi.URLParam(r, "id"))
	httpx.JSON(w, http.StatusCreated, map[string]string{"status": "added"})
}
