-- +goose Up

-- CP-2 has arrived: the console adds operators.
--
-- Migration 00049 withheld INSERT on this table on purpose and said why —
-- "INSERT нь ЗӨВХӨН CP-2-т (оператор нэмэх) хэрэгтэй болох тул одоохондоо
-- олгогдоогүй". Until now the only way to a second operator was
-- cmd/operator-bootstrap on the server, as the database owner, which is a
-- fine control for the first account and a poor one for the fourth: it means
-- shell access on the production host every time somebody joins.
--
-- What replaces it is not "trust the handler". The console's screen mints
-- through THIS role, so the grant below is the control: it is one line to
-- revoke, and `\dp operator.operator_accounts` says who may. Above it sit
-- three more — the capability table (operator.write, superadmin only), a
-- second factor on the request, and an audit row written in the same
-- transaction as the account.
--
-- Deliberately still withheld: DELETE. An operator who should not be here is
-- disabled, and the audit trail keeps pointing at a row that still exists.
GRANT INSERT ON operator.operator_accounts TO gerege_nexus_operator;

-- +goose Down

REVOKE INSERT ON operator.operator_accounts FROM gerege_nexus_operator;
