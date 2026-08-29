-- +goose Up

-- The people screen answers "who is this account" with every way it can be
-- signed into: eID, and whatever federated providers it has been linked to.
-- 00099 gave the console the first; this gives it the second.
--
-- SELECT only, for the same reason: a link between an account and a provider
-- is written by the sign-in that proved it. The console reads who is linked to
-- what — which is how an operator answers "why can this person get in" and
-- "what happens if that provider goes away" — and writes nothing.
GRANT SELECT ON registry.user_sso_identities TO gerege_nexus_operator;

-- +goose Down

REVOKE SELECT ON registry.user_sso_identities FROM gerege_nexus_operator;
