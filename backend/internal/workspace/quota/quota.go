/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package quota answers "may this organisation have any more".
//
// It is small on purpose. The limits are set in the operator console (CP-2),
// the numbers are counted nightly by the metering job (CP-5), and this is the
// third piece: the check itself, in one place that every enforcement point
// calls rather than each of them reading the two tables their own way.
//
// It lives below both the platform and the app modules, like httpx, because
// the enforcement points are in both: the AI middleware is platform code and
// the upload path is an app module's, and neither should have to import the
// other to ask a question about a number.
package quota

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/jackc/pgx/v5"
)

// ErrExceeded is a hard limit refusing.
//
// Soft limits do not produce it: they log and allow, which is the difference
// between measuring a boundary and enforcing one, and every organisation
// crosses the first before anybody decides on the second.
var ErrExceeded = errors.New("this organisation has reached its limit")

// Limit is one organisation's ceiling and where it stands against it.
type Limit struct {
	Max       int
	Used      int64
	Hard      bool
	Unlimited bool
}

// Exceeded reports whether one more would cross the line.
func (l Limit) Exceeded(adding int64) bool {
	return !l.Unlimited && l.Used+adding > int64(l.Max)
}

// Storage decides whether an organisation may keep another `adding` bytes.
//
// The number it checks against is the last measurement rather than a live sum,
// because summing every blob on the platform on every upload would make the
// check cost more than the upload. An organisation can therefore cross its
// storage limit by whatever it uploads between two nightly measurements, and
// that is the right trade: a storage quota is a commercial boundary, not a
// safety one — the disk alert is what protects the platform.
func Storage(ctx context.Context, db nexus.DB, tenantID string, adding int64) error {
	limit, err := storageLimit(ctx, db, tenantID)
	if err != nil {
		// A check that cannot run allows. Refusing every upload because a
		// quota query failed would turn a slow database into an outage for
		// organisations nowhere near their limit.
		slog.Warn("quota: could not check the storage limit", "tenant_id", tenantID, "error", err)
		return nil
	}
	// Bytes in, megabytes stored: the limit is written in megabytes because
	// that is what an operator types, and rounding up means a 1 MB limit
	// admits one file of 1 MB rather than one of 1 048 577 bytes.
	addingMB := (adding + 1048575) / 1048576
	if !limit.Exceeded(addingMB) {
		return nil
	}
	if !limit.Hard {
		slog.Warn("quota: an organisation is over its storage limit",
			"tenant_id", tenantID, "limit_mb", limit.Max, "used_mb", limit.Used)
		return nil
	}
	return fmt.Errorf("%w: %d MB of %d MB", ErrExceeded, limit.Used, limit.Max)
}

// storageLimit reads the ceiling and the latest measurement in one statement.
func storageLimit(ctx context.Context, db nexus.DB, tenantID string) (Limit, error) {
	var maximum int
	var enforcement string
	var used int64
	err := db.QueryRow(ctx,
		`SELECT COALESCE(q.max_storage_mb, -1), COALESCE(q.enforcement, 'soft'),
		        COALESCE((SELECT value FROM registry.usage_events u
		                   WHERE u.tenant_id = q.tenant_id AND u.metric = 'storage_mb'
		                   ORDER BY u.day DESC LIMIT 1), 0)
		   FROM registry.tenant_quotas q
		  WHERE q.tenant_id = $1::uuid`, tenantID).Scan(&maximum, &enforcement, &used)
	if errors.Is(err, pgx.ErrNoRows) {
		// No row is the ordinary case: an organisation nobody has set a limit
		// for has none.
		return Limit{Unlimited: true}, nil
	}
	if err != nil {
		return Limit{}, err
	}
	if maximum < 0 {
		return Limit{Unlimited: true}, nil
	}
	return Limit{Max: maximum, Used: used, Hard: enforcement == "hard"}, nil
}
