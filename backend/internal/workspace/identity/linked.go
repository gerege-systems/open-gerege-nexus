/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The ways into one account, as one list.
 *
 * Read by the profile screen and asserted by the binding tests, which is why it
 * is here rather than beside either: both are asking this package what it knows
 * about a person, and neither owns the answer.
 */

package identity

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

// LinkedIdentity is one way the person can prove who they are.
// LinkedIdentity is one way into an account.
type LinkedIdentity struct {
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

// LinkedIdentities gathers both kinds into one list, newest link first.
//
// They live in two tables because they are two different things — one is a
// national identity, the other an account at a provider — but to the person
// they are one list of ways in, so that is how they arrive.
func (h *Handlers) LinkedIdentities(ctx context.Context, userID string) []LinkedIdentity {
	identities := make([]LinkedIdentity, 0, 2)

	rows, err := h.db.Query(ctx,
		`SELECT issuer, subject, COALESCE(email,''), COALESCE(name,''), claims, linked_at, last_seen_at
		   FROM registry.user_sso_identities WHERE user_id = $1`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var it LinkedIdentity
			var raw []byte
			if err := rows.Scan(&it.Issuer, &it.Subject, &it.Email, &it.Name,
				&raw, &it.LinkedAt, &it.LastSeen); err != nil {
				continue
			}
			it.Kind = "sso"
			it.Provider = h.BindingProviderName(it.Issuer)
			_ = json.Unmarshal(raw, &it.Claims)
			identities = append(identities, it)
		}
		// Тасарсан уншилтын дараа жагсаалт нь дутуу боловч бүрэн мэт
		// харагдана: хүн өөрийн нэвтрэх аргаа алга болсон гэж үзээд
		// дахин холбохыг оролдоно.
		if err := rows.Err(); err != nil {
			slog.Warn("could not read every linked provider identity", "error", err)
		}
	} else {
		slog.Warn("could not read linked provider identities", "error", err)
	}

	var eid LinkedIdentity
	var raw []byte
	if err := h.db.QueryRow(ctx,
		`SELECT person_etsi, COALESCE(given_name,''), COALESCE(surname,''), claims, linked_at, last_seen_at
		   FROM registry.user_eid_identities WHERE user_id = $1`, userID).
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
