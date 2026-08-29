-- +goose Up

-- The console chooses an organisation's first administrator from the people
-- this platform has already watched prove who they are with eID, rather than
-- from an address somebody types. To offer that list it has to read the eID
-- identities, and it had no grant on them at all.
--
-- SELECT and nothing else. The console shows a name, an address and a
-- registration number so an operator can tell two people apart; it never
-- writes an identity, and it must not be able to — a linkage between a citizen
-- and an account is written by the sign-in that proved it, and by nothing
-- else.
GRANT SELECT ON registry.user_eid_identities TO gerege_nexus_operator;

-- The rows are the platform's rather than any one organisation's, so there is
-- no policy to add: this table carries no tenant_id and no row-level security.

-- +goose Down

REVOKE SELECT ON registry.user_eid_identities FROM gerege_nexus_operator;
