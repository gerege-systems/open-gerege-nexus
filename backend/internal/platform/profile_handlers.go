/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * A person's own record of themselves.
 *
 * Everything here answers for the caller and only the caller. There is no id
 * parameter anywhere in this file on purpose: the session decides whose record
 * is read, so there is no version of these queries that can be pointed at
 * somebody else. An administrator looking at another person belongs behind the
 * access-control screens, which is a different question with a different
 * answer.
 *
 * It is a platform screen rather than an installed app. Apps are installed per
 * organisation and an administrator can remove one; a person's own record of
 * which identities are linked to their account is not something their employer
 * should be able to take away. And somebody who belongs to several
 * organisations has one profile, not one per membership.
 */

package platform

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/tenant"
)

// linkedIdentity is one way the person can prove who they are.
type linkedIdentity struct {
	// Kind is "sso" or "eid" — which of the two tables it came from, and so
	// which kind of thing it proves.
	Kind string `json:"kind"`
	// Issuer is the provider's own identifier. Provider is what to call it on
	// screen; they differ because a URL is not a name.
	Issuer   string    `json:"issuer"`
	Provider string    `json:"provider"`
	Subject  string    `json:"subject"`
	Email    string    `json:"email,omitempty"`
	Name     string    `json:"name,omitempty"`
	Surname  string    `json:"surname,omitempty"`
	LinkedAt time.Time `json:"linked_at"`
	LastSeen time.Time `json:"last_seen_at"`
	// Claims is what that provider actually said. It is the person's own, and
	// this is the screen it exists for.
	Claims map[string]any `json:"claims,omitempty"`
	// Removable says whether this one may be unlinked right now. The screen
	// could work this out itself — it has the whole list — but then the rule
	// would live in two places and only one of them would be enforced. The
	// server decides; the button merely reflects the decision.
	Removable bool `json:"removable"`
}

// handleProfile answers with the caller's own record.
func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	ctx := r.Context()

	var name, email string
	var createdAt time.Time
	if err := s.db.QueryRow(ctx,
		`SELECT name, email, created_at FROM users WHERE id = $1`, claims.UserID).
		Scan(&name, &email, &createdAt); err != nil {
		slog.Error("could not read a profile", "error", err, "user_id", claims.UserID)
		httpx.Error(w, http.StatusInternalServerError, "could not load the profile")
		return
	}

	// The organisations this person belongs to. Crosses tenants by definition,
	// so it runs on the platform path — under the caller's own policies a
	// membership elsewhere is not visible, and the list would be one long.
	memberships, err := s.sessions.TenantsForUser(tenant.Without(ctx), claims.UserID)
	if err != nil {
		slog.Warn("could not list a person's organisations", "error", err)
		memberships = nil
	}

	identities := s.linkedIdentities(ctx, claims.UserID)

	// How many other places this account is signed in. Not the tokens — those
	// are never readable — only that they exist, which is what somebody needs
	// in order to decide whether to end them.
	var activeSessions int
	_ = s.db.QueryRow(ctx,
		`SELECT count(*) FROM sessions
		  WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > NOW()`,
		claims.UserID).Scan(&activeSessions)

	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":              claims.UserID,
		"name":            name,
		"email":           email,
		"created_at":      createdAt,
		"is_admin":        claims.IsAdmin,
		"current_tenant":  claims.TenantID,
		"organisations":   memberships,
		"identities":      identities,
		"active_sessions": activeSessions,
	})
}

// linkedIdentities gathers both kinds into one list, newest link first.
//
// They live in two tables because they are two different things — one is a
// national identity, the other an account at a provider — but to the person
// they are one list of ways in, so that is how they arrive.
func (s *Server) linkedIdentities(ctx context.Context, userID string) []linkedIdentity {
	identities := make([]linkedIdentity, 0, 2)

	rows, err := s.db.Query(ctx,
		`SELECT issuer, subject, COALESCE(email,''), COALESCE(name,''), claims, linked_at, last_seen_at
		   FROM user_sso_identities WHERE user_id = $1`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var it linkedIdentity
			var raw []byte
			if err := rows.Scan(&it.Issuer, &it.Subject, &it.Email, &it.Name,
				&raw, &it.LinkedAt, &it.LastSeen); err != nil {
				continue
			}
			it.Kind = "sso"
			it.Provider = s.bindingProviderName(it.Issuer)
			_ = json.Unmarshal(raw, &it.Claims)
			identities = append(identities, it)
		}
	} else {
		slog.Warn("could not read linked provider identities", "error", err)
	}

	var eid linkedIdentity
	var raw []byte
	if err := s.db.QueryRow(ctx,
		`SELECT person_etsi, COALESCE(given_name,''), COALESCE(surname,''), claims, linked_at, last_seen_at
		   FROM user_eid_identities WHERE user_id = $1`, userID).
		Scan(&eid.Subject, &eid.Name, &eid.Surname, &raw, &eid.LinkedAt, &eid.LastSeen); err == nil {
		eid.Kind = "eid"
		eid.Provider = "eID Mongolia"
		_ = json.Unmarshal(raw, &eid.Claims)
		identities = append(identities, eid)
	}

	// Nobody may remove their last way in. A person whose only identity is the
	// one they are about to detach would be locked out of their own account by
	// a single click — and the account they lose access to may be the one
	// holding their memberships. Two or more, and any single one is expendable.
	removable := len(identities) > 1
	for i := range identities {
		identities[i].Removable = removable
	}

	return identities
}

// unlinkRequest names one identity to detach. Deliberately not an id: the
// caller says which provider and which account at that provider, and the query
// is scoped to their own user row, so there is no way to phrase this request
// that reaches somebody else's identity.
type unlinkRequest struct {
	Kind    string `json:"kind"`
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
}

// handleUnlinkIdentity detaches one linked identity from the caller's account.
//
// Deleting the row is the whole of it. There is no "disabled" state to fall
// back to, because a way in that is remembered but refused is still a record
// of somebody's national identity or their account elsewhere, kept after they
// asked for it to be forgotten.
//
// Signing in with that provider afterwards behaves exactly as it did the first
// time: unrecognised, so Google is parked and eID is asked for again. That is
// not a special case — it falls out of the row being gone.
func (s *Server) handleUnlinkIdentity(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req unlinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "could not read the request")
		return
	}

	ctx := r.Context()

	// Counted here rather than trusted from the screen. The list the browser
	// holds may be minutes old, and the one identity it does not know about is
	// exactly the one that makes this safe or unsafe.
	current := s.linkedIdentities(ctx, claims.UserID)
	if len(current) <= 1 {
		httpx.Error(w, http.StatusConflict, "the last way of signing in cannot be removed")
		return
	}

	var tag int64
	switch req.Kind {
	case "eid":
		ct, err := s.db.Exec(ctx,
			`DELETE FROM user_eid_identities WHERE user_id = $1 AND person_etsi = $2`,
			claims.UserID, req.Subject)
		if err != nil {
			slog.Error("could not unlink an eID identity", "error", err, "user_id", claims.UserID)
			httpx.Error(w, http.StatusInternalServerError, "could not unlink the identity")
			return
		}
		tag = ct.RowsAffected()
	case "sso":
		ct, err := s.db.Exec(ctx,
			`DELETE FROM user_sso_identities WHERE user_id = $1 AND issuer = $2 AND subject = $3`,
			claims.UserID, req.Issuer, req.Subject)
		if err != nil {
			slog.Error("could not unlink a provider identity", "error", err, "user_id", claims.UserID)
			httpx.Error(w, http.StatusInternalServerError, "could not unlink the identity")
			return
		}
		tag = ct.RowsAffected()
	default:
		httpx.Error(w, http.StatusBadRequest, "unknown identity kind")
		return
	}

	if tag == 0 {
		// Either it was never linked or somebody unlinked it already. Both are
		// the state the caller asked for, but saying so distinguishes a stale
		// screen from a successful removal.
		httpx.Error(w, http.StatusNotFound, "no such linked identity")
		return
	}

	slog.Info("a person unlinked one of their identities",
		"user_id", claims.UserID, "kind", req.Kind, "issuer", req.Issuer)
	httpx.JSON(w, http.StatusOK, map[string]any{"identities": s.linkedIdentities(ctx, claims.UserID)})
}
