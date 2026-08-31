/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Opening many organisations in one act.
 *
 * # Why this exists
 *
 * `CreateTenant` is the right shape for the console's form: one organisation,
 * one operator, one decision. It is the wrong shape for the day a deployment
 * starts — a fuel regulator onboards two hundred licensed companies, a
 * ministry onboards its agencies — because two hundred form submissions is not
 * a workflow, it is an afternoon nobody will spend and a list somebody will
 * paste into SQL instead. `tenant-bootstrap` does not help either: it refuses
 * once an organisation exists, by design, because it is the first-run tool.
 *
 * So the gap this fills is narrow and real: the console can already open one,
 * and nothing could open many.
 *
 * # Three properties, in the order they matter
 *
 * **Not a transaction.** Row 137 failing must not undo the 136 organisations
 * that opened. Each row is its own act with its own audit row, exactly as if
 * an operator had submitted the form 200 times. A bulk endpoint that rolled
 * back would turn one bad slug into an afternoon of nothing.
 *
 * **Every row reports.** The response is a list of outcomes, not a count and
 * not the first error. An operator who sends 200 rows needs to know which
 * three did not land and why — and a caller who gets `400 invalid slug` learns
 * nothing about the other 199.
 *
 * **Re-running is safe.** A slug that already exists is `exists`, not a
 * failure. Onboarding is iterative: a file is corrected and sent again, and
 * the second run must not be a wall of errors that hides the two rows that are
 * genuinely new. This is what makes the endpoint usable more than once.
 *
 * # What it deliberately does not do
 *
 * No CSV, no Excel, no file upload. The console speaks JSON and whoever holds
 * the company list can convert it; a parser here would be a second place for
 * an encoding bug to live and would still need this endpoint underneath.
 */

package tenants

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
)

// MaxBulkTenants bounds one request.
//
// Not a performance limit — each row is a small transaction — but a blast
// radius. A runaway loop on the caller's side should be a refused request, not
// four thousand organisations that then need two superadmins each to remove.
const MaxBulkTenants = 200

// ErrTooManyTenants is a batch larger than the platform will take at once.
var ErrTooManyTenants = fmt.Errorf("a batch is at most %d organisations", MaxBulkTenants)

// BulkOutcome is what happened to one row.
//
// `Status` is the word the screen shows and the field a script branches on;
// `Created` carries the full per-organisation report (which apps installed,
// whether the administrator was actually invited) for the rows that opened,
// because a bulk run hides those details otherwise and they are the details
// that turn into support calls.
type BulkOutcome struct {
	Slug    string         `json:"slug"`
	Status  string         `json:"status"` // created | exists | failed
	Created *CreatedTenant `json:"created,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// The three words a row can end on, named once.
const (
	BulkCreated = "created"
	BulkExists  = "exists"
	BulkFailed  = "failed"
)

// CreateTenants opens each organisation in turn.
//
// The error return is for the batch being unacceptable as a whole — too large,
// empty. Anything that goes wrong with a single organisation is reported in
// that row's outcome and does not stop the rest.
func (s *Service) CreateTenants(ctx context.Context, sess operator.Session, list []NewTenant) ([]BulkOutcome, error) {
	if len(list) == 0 {
		return nil, errors.New("no organisations were given")
	}
	if len(list) > MaxBulkTenants {
		return nil, ErrTooManyTenants
	}

	outcomes := make([]BulkOutcome, 0, len(list))
	created, existed, failed := 0, 0, 0
	for _, params := range list {
		row := BulkOutcome{Slug: params.Slug}
		switch tenant, err := s.CreateTenant(ctx, sess, params); {
		case err == nil:
			row.Status, row.Created = BulkCreated, &tenant
			created++
		case errors.Is(err, ErrSlugTaken):
			// Not a failure. See the file comment: a corrected file sent again
			// must report the rows that are already in place as settled, or
			// nobody will send it again.
			row.Status = BulkExists
			existed++
		default:
			row.Status, row.Error = BulkFailed, err.Error()
			failed++
		}
		outcomes = append(outcomes, row)
	}

	// One line for the batch, beside the per-organisation audit rows
	// CreateTenant already wrote. The counts are what an operator asks about
	// afterwards, and reconstructing them from 200 audit rows is work.
	slog.Info("the console opened organisations in bulk",
		"operator", sess.Email, "created", created, "existed", existed, "failed", failed)

	return outcomes, nil
}

func (s *Service) handleCreateTenants(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		Organisations []NewTenant `json:"organisations"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}

	outcomes, err := s.CreateTenants(r.Context(), sess, body.Organisations)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// 200 rather than 207 or 400, even when rows failed. The request was
	// carried out; what each row did is in the body. A status code that
	// summarised 200 outcomes into one number would be a summary the caller
	// has to distrust and read the body anyway.
	httpx.JSON(w, http.StatusOK, map[string]any{"results": outcomes})
}
