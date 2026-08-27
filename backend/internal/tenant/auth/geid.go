/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package auth

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/identity/eid"
)

// The citizen's Gerege number, and the address it gives them.
//
// An account opened by eID has no address of its own: the citizen proved who
// they are with a certificate, not with a mailbox. This platform therefore
// invented one — `eid+<32 hex>@identity.invalid` — which was unique, stable and
// unreadable, and which every relying party that federates here received as the
// `email` claim of a real person.
//
// eID now returns `person.geID`, the citizen's number in the Gerege register.
// It is the identifier the rest of the ecosystem already uses, so it is both a
// better address and a claim worth handing on in its own right — which is why
// both go downstream (see internal/tenant/ssoprovider) rather than one standing
// in for the other.

// GeIDDomain is the domain an eID account's address is formed under.
//
// Not a mailbox and not routable, exactly as `identity.invalid` was not. What
// changed is that a person reading it can tell whose account it is.
const GeIDDomain = "gemail.com"

// GeIDEmail is the address of the citizen with this Gerege number.
func GeIDEmail(geID int64) string {
	return strconv.FormatInt(geID, 10) + "@" + GeIDDomain
}

// isInventedAddress reports whether an address is one this platform made up for
// an account that never had one — either shape.
func isInventedAddress(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	return strings.HasSuffix(email, "@identity.invalid") || strings.HasSuffix(email, "@"+GeIDDomain)
}

// rememberGeID writes the number onto an account that did not have it, and
// upgrades the address that was invented before the number was known.
//
// Only an invented address is rewritten. A citizen who signed up with a real
// mailbox and later linked their eID keeps the address they chose: this is
// their account, and a sign-in method is not a reason to rename them.
//
// A failure is logged and swallowed. The sign-in has already succeeded by the
// time this runs, and refusing it now would mean a citizen eID approved being
// turned away because a bookkeeping column would not take a value.
func (h *Handlers) rememberGeID(ctx context.Context, userID string, identity *eid.EIDIdentity) {
	if identity == nil || identity.GeID == 0 || userID == "" {
		return
	}
	if _, err := h.db.Exec(ctx,
		`UPDATE registry.users
		    SET ge_id = $2,
		        email = CASE WHEN $3 THEN $4 ELSE email END
		  WHERE id = $1::uuid
		    AND (ge_id IS DISTINCT FROM $2 OR ($3 AND email <> $4))`,
		userID, identity.GeID, isInventedAddressOf(ctx, h, userID), GeIDEmail(identity.GeID)); err != nil {
		slog.Warn("could not record the citizen's Gerege number",
			"user_id", userID, "error", err)
	}
}

// isInventedAddressOf answers the CASE above: may this account's address be
// rewritten? Read rather than inferred, because the answer depends on what is
// in the row and not on how this sign-in arrived.
func isInventedAddressOf(ctx context.Context, h *Handlers, userID string) bool {
	var email string
	if err := h.db.QueryRow(ctx,
		`SELECT email FROM registry.users WHERE id = $1::uuid`, userID).Scan(&email); err != nil {
		return false
	}
	return isInventedAddress(email)
}
