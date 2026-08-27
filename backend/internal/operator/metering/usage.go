package metering

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/tenants"

	usagemetric "github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/usage"
)

// What each organisation used (§G of the plan).
//
// The console reads it and does not write it: usage_events is granted to the
// operator role for SELECT only (migration 00053), so there is no console
// request that can change a number a bill might one day be based on. That is
// the first question anybody asks of a metering system in a dispute, and the
// answer should be a grant rather than a promise.

// usageWindow is how far back the chart goes. Ninety days is three billing
// months, which is what somebody looking at a trend actually wants, and it
// keeps the response small enough to draw without paging.
const usageWindow = 90

// UsagePoint is one organisation's one metric on one day.
type UsagePoint struct {
	Day   string `json:"day"`
	Value int64  `json:"value"`
}

// UsageSeries is one metric over the window, with what it is measured against.
type UsageSeries struct {
	Metric string       `json:"metric"`
	Points []UsagePoint `json:"points"`
	// Total is the sum over the window for a counted metric, and the latest
	// value for a measured one (storage). The console labels them differently
	// and the distinction is real: adding up daily storage would be nonsense.
	Total int64 `json:"total"`
	// MonthToDate is what a monthly limit is checked against.
	MonthToDate int64 `json:"month_to_date"`
	// Limit is the organisation's quota for this metric, or nil.
	Limit *int `json:"limit"`
	// Enforced says whether crossing the limit actually stops anything today.
	Enforced bool `json:"enforced"`
}

// Usage is the organisation's whole usage screen.
type Usage struct {
	TenantID string        `json:"tenant_id"`
	Series   []UsageSeries `json:"series"`
	// Collected is when the metering job last wrote anything for this
	// organisation. An empty chart with a recent collection means they used
	// nothing; an empty chart with no collection means nobody has counted yet,
	// and the screen says which.
	Collected *time.Time `json:"collected"`
}

// UsageFor assembles the chart.
func (s *Service) UsageFor(ctx context.Context, tenantID string) (Usage, error) {
	ctx = operator.Scoped(ctx)
	usage := Usage{TenantID: tenantID, Series: []UsageSeries{}}

	quota, err := s.tenants.GetQuota(ctx, tenantID)
	if err != nil {
		return Usage{}, err
	}

	rows, err := s.db.Query(ctx,
		`SELECT metric, to_char(day, 'YYYY-MM-DD'), value
		   FROM platform.usage_events
		  WHERE tenant_id = $1::uuid AND day >= CURRENT_DATE - $2::int
		  ORDER BY day`, tenantID, usageWindow)
	if err != nil {
		return Usage{}, fmt.Errorf("control plane: read the usage: %w", err)
	}
	defer rows.Close()

	points := map[string][]UsagePoint{}
	for rows.Next() {
		var metric string
		var point UsagePoint
		if err := rows.Scan(&metric, &point.Day, &point.Value); err != nil {
			return Usage{}, fmt.Errorf("control plane: read a usage row: %w", err)
		}
		points[metric] = append(points[metric], point)
	}
	if err := rows.Err(); err != nil {
		return Usage{}, err
	}

	if err := s.db.QueryRow(ctx,
		`SELECT max(recorded_at) FROM platform.usage_events WHERE tenant_id = $1::uuid`,
		tenantID).Scan(&usage.Collected); err != nil {
		return Usage{}, fmt.Errorf("control plane: read the collection time: %w", err)
	}

	for _, metric := range Metrics() {
		series := UsageSeries{Metric: metric, Points: points[metric]}
		if series.Points == nil {
			series.Points = []UsagePoint{}
		}
		switch metric {
		case usagemetric.StorageMB:
			// Measured rather than accumulated: the total is the last reading.
			if len(series.Points) > 0 {
				series.Total = series.Points[len(series.Points)-1].Value
			}
			series.MonthToDate = series.Total
			series.Limit = quota.MaxStorageMB
			// Recorded and shown; nothing refuses an upload for it yet. The
			// screen says so rather than implying an enforcement that is not
			// there.
			series.Enforced = false
		case usagemetric.AICalls:
			for _, point := range series.Points {
				series.Total += point.Value
			}
			series.MonthToDate = monthToDate(series.Points)
			series.Limit = quota.MaxAICallsMonthly
			series.Enforced = quota.Enforcement == tenants.EnforcementHard
		case usagemetric.ActiveUsers:
			// The peak rather than the sum: "how many people did they have"
			// is a maximum, and adding daily active users together would
			// count the same person thirty times.
			for _, point := range series.Points {
				if point.Value > series.Total {
					series.Total = point.Value
				}
			}
			series.MonthToDate = series.Total
			series.Limit = quota.MaxUsers
			series.Enforced = quota.Enforcement == tenants.EnforcementHard
		default:
			for _, point := range series.Points {
				series.Total += point.Value
			}
			series.MonthToDate = monthToDate(series.Points)
		}
		usage.Series = append(usage.Series, series)
	}
	return usage, nil
}

// monthToDate sums the points that fall in the current month.
func monthToDate(points []UsagePoint) int64 {
	prefix := time.Now().Format("2006-01")
	var total int64
	for _, point := range points {
		if len(point.Day) >= 7 && point.Day[:7] == prefix {
			total += point.Value
		}
	}
	return total
}

// WriteUsageCSV streams the window as a spreadsheet.
//
// CSV rather than xlsx: this is one wide table of numbers going into somebody
// else's spreadsheet or invoice, and the reporting module's xlsx machinery
// would add a dependency to the console for a file with no formatting in it.
func (s *Service) WriteUsageCSV(ctx context.Context, w http.ResponseWriter, tenantID string) error {
	usage, err := s.UsageFor(ctx, tenantID)
	if err != nil {
		return err
	}

	// One row per day, one column per metric — the shape somebody pastes into
	// a spreadsheet, rather than the long form the database holds.
	days := map[string]map[string]int64{}
	for _, series := range usage.Series {
		for _, point := range series.Points {
			if days[point.Day] == nil {
				days[point.Day] = map[string]int64{}
			}
			days[point.Day][series.Metric] = point.Value
		}
	}

	writer := csv.NewWriter(w)
	header := append([]string{"day"}, Metrics()...)
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, day := range sortedKeys(days) {
		row := []string{day}
		for _, metric := range Metrics() {
			row = append(row, strconv.FormatInt(days[day][metric], 10))
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func sortedKeys(days map[string]map[string]int64) []string {
	keys := make([]string, 0, len(days))
	for day := range days {
		keys = append(keys, day)
	}
	// The days are ISO dates, so lexical order is chronological order.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
