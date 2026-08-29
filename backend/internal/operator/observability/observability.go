package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/backup"
)

// The console's front page (§E of the plan).
//
// It is a summary and it is not Grafana. The distinction matters, because the
// temptation with a screen like this is to grow it into a dashboard nobody
// maintains — so what it shows is only what somebody would want at the moment
// they open the console: is the platform serving requests, are the government
// systems answering, is anything alerting, is the disk filling up. Every panel
// carries a link into Grafana, where the actual investigation happens.
//
// Nothing here is required. A deployment with no monitoring stack — which is
// every development machine, and any production one that has not run the
// monitoring compose — gets a page that says so, rather than a page of zeroes
// that reads as "everything is fine".

// prometheusURL and alertmanagerURL are where the summary is read from.
//
// Both empty by default. The monitoring stack publishes on the host's loopback
// (deploy/docker-compose.monitoring.yml), so a containerised API reaches it
// through the host gateway — see docs/CONTROL_PLANE.md §4з for the two lines
// that arrange it.
func prometheusURL() string   { return strings.TrimRight(settings.Get(settings.PrometheusURL), "/") }
func alertmanagerURL() string { return strings.TrimRight(settings.Get(settings.AlertmanagerURL), "/") }

// GrafanaURL is where the "look closer" links point. Public, because the
// operator's browser follows them.
func GrafanaURL() string { return strings.TrimRight(settings.Get(settings.GrafanaURL), "/") }

// monitoringTimeout bounds one query. The console must not hang because
// Prometheus is busy; a panel that says "could not read" is a better page than
// one that never arrives.
const monitoringTimeout = 5 * time.Second

// Overview is the front page.
type Overview struct {
	// Monitoring is false when this deployment has no Prometheus configured,
	// which is what the screen says instead of showing zeroes.
	Monitoring bool `json:"monitoring"`
	// GrafanaURL is empty when there is nothing to link to.
	GrafanaURL string `json:"grafana_url"`

	API      APIHealth   `json:"api"`
	External []SystemDot `json:"external"`
	Infra    []Gauge     `json:"infra"`
	Alerts   []Alert     `json:"alerts"`

	// Everything below is read from this platform's own database, so it is
	// there whether or not the monitoring stack is.
	Background []BackgroundJob `json:"background"`
	Tenants    []TenantTrouble `json:"tenant_trouble"`
	Backups    backup.Status   `json:"backups"`
	Catalog    CatalogStatus   `json:"catalog"`
	Version    VersionInfo     `json:"version"`
	Warnings   []string        `json:"warnings"`
}

// APIHealth is the RED summary: how much, how badly, how slowly.
type APIHealth struct {
	RequestsPerSecond float64 `json:"requests_per_second"`
	ErrorRate         float64 `json:"error_rate"`
	P95Seconds        float64 `json:"p95_seconds"`
	// Read is false when the query failed, so the screen can distinguish "zero
	// requests" from "we do not know".
	Read bool `json:"read"`
}

// SystemDot is one external system's light.
type SystemDot struct {
	System     string  `json:"system"`
	ErrorRate  float64 `json:"error_rate"`
	P95Seconds float64 `json:"p95_seconds"`
	// State is green, amber, red — or unknown, when nothing has measured this
	// system at all. Decided here rather than in the browser, so the console
	// and any future alert agree about what "degraded" means.
	State string `json:"state"`
	// Measured is whether Prometheus holds any sample for this system. An
	// unmeasured system read as a green zero for as long as this screen has
	// existed: no series means no error rate, and "no error rate" is not the
	// same claim as "no errors".
	Measured bool `json:"measured"`
}

// Gauge is one infrastructure number with the level it is judged against.
type Gauge struct {
	Name    string  `json:"name"`
	Value   float64 `json:"value"`
	Unit    string  `json:"unit"`
	Warning float64 `json:"warning"`
	// State is green, amber, red — or unknown, when no exporter is answering
	// for this number.
	State string `json:"state"`
	// Measured is whether there was a sample at all. Without it a stopped
	// exporter reads as 0% and green.
	Measured bool `json:"measured"`
}

// Alert is one firing alert, from Alertmanager.
type Alert struct {
	Name     string    `json:"name"`
	Severity string    `json:"severity"`
	Summary  string    `json:"summary"`
	StartsAt time.Time `json:"starts_at"`
	Runbook  string    `json:"runbook"`
	Silenced bool      `json:"silenced"`
}

// Health assembles the front page.
//
// Each part is fetched independently and each failure is local: a Prometheus
// that is down leaves the alert list and the database-backed panels intact.
// The alternative — one error for the whole page — would make the console
// useless in exactly the situation it exists for.
func (s *Service) Health(ctx context.Context) Overview {
	overview := Overview{
		Monitoring: prometheusURL() != "",
		GrafanaURL: GrafanaURL(),
		External:   []SystemDot{},
		Infra:      []Gauge{},
		Alerts:     []Alert{},
		Warnings:   s.warnings(),
	}

	if overview.Monitoring {
		overview.API = s.apiHealth(ctx)
		overview.External = s.externalSystems(ctx)
		overview.Infra = s.infrastructure(ctx)
	}
	overview.Alerts = s.firingAlerts(ctx)

	overview.Background = s.backgroundJobs(ctx)
	overview.Tenants = s.tenantTrouble(ctx)
	overview.Backups = s.backup.StatusOf(ctx)
	overview.Catalog = s.CatalogStatus(ctx)
	overview.Version = s.Version(ctx)
	return overview.withLists()
}

// withLists makes every list on this screen a list on the wire.
//
// A nil slice in Go marshals as `null`, and the console renders each of these
// with .map. `null.map` is not a smaller table, it is a blank page saying "This
// page couldn't load" — which is what a deployment saw when one callback above
// was never wired up and `warnings` came back nil.
//
// Done once, here, rather than by asking every builder to remember: they are
// eight, they are edited separately, and the one that forgets takes the whole
// screen with it.
func (o Overview) withLists() Overview {
	if o.External == nil {
		o.External = []SystemDot{}
	}
	if o.Infra == nil {
		o.Infra = []Gauge{}
	}
	if o.Alerts == nil {
		o.Alerts = []Alert{}
	}
	if o.Background == nil {
		o.Background = []BackgroundJob{}
	}
	if o.Tenants == nil {
		o.Tenants = []TenantTrouble{}
	}
	if o.Warnings == nil {
		o.Warnings = []string{}
	}
	if o.Catalog.Apps == nil {
		o.Catalog.Apps = []AppInstalled{}
	}
	return o
}

// The queries. Written here rather than in the dashboards' JSON because these
// are the four numbers a person wants in the first five seconds, and they have
// to mean the same thing on this screen as they do in Grafana.
const (
	rpsQuery       = `sum(rate(http_server_request_duration_seconds_count[5m]))`
	errorRateQuery = `sum(rate(http_server_request_duration_seconds_count{http_response_status_code=~"5.."}[5m])) / clamp_min(sum(rate(http_server_request_duration_seconds_count[5m])), 0.001)`
	p95Query       = `histogram_quantile(0.95, sum by (le) (rate(http_server_request_duration_seconds_bucket[5m])))`
)

func (s *Service) apiHealth(ctx context.Context) APIHealth {
	health := APIHealth{Read: true}
	for query, target := range map[string]*float64{
		rpsQuery:       &health.RequestsPerSecond,
		errorRateQuery: &health.ErrorRate,
		p95Query:       &health.P95Seconds,
	} {
		value, err := instantQuery(ctx, query)
		if err != nil {
			slog.Warn("control plane: could not read a metric", "error", err)
			health.Read = false
			continue
		}
		*target = value
	}
	return health
}

// externalSystems is the light per government or third-party system.
//
// The label set is the closed list from the instrumentation (§Үе шат 1), so a
// system that stops being called simply disappears from the screen rather than
// showing a stale green.
func (s *Service) externalSystems(ctx context.Context) []SystemDot {
	systems := []string{"xyp", "eid", "dan", "esign", "gemini", "emailverify"}
	dots := make([]SystemDot, 0, len(systems))
	for _, system := range systems {
		errorRate, sawErrors, errErr := instantSample(ctx, fmt.Sprintf(
			`sum(rate(external_request_duration_seconds_count{system=%q,status="error"}[15m])) / `+
				`clamp_min(sum(rate(external_request_duration_seconds_count{system=%q}[15m])), 0.001)`,
			system, system))
		p95, sawLatency, p95Err := instantSample(ctx, fmt.Sprintf(
			`histogram_quantile(0.95, sum by (le) (rate(external_request_duration_seconds_bucket{system=%q}[15m])))`,
			system))
		if errErr != nil && p95Err != nil {
			continue
		}
		// Nothing has called this system since Prometheus last kept a sample,
		// so there is no error rate to judge. Listed rather than dropped: an
		// operator wants to know that eID is unmeasured, which is different
		// from eID being absent and very different from eID being well.
		if !sawErrors && !sawLatency {
			dots = append(dots, SystemDot{System: system, State: "unknown"})
			continue
		}
		dots = append(dots, SystemDot{
			System: system, ErrorRate: errorRate, P95Seconds: p95,
			State: tone(errorRate, 0.05, 0.2), Measured: true,
		})
	}
	return dots
}

func (s *Service) infrastructure(ctx context.Context) []Gauge {
	type spec struct {
		name, unit, query string
		warning, critical float64
	}
	specs := []spec{
		{"disk", "%", `100 - (min(node_filesystem_avail_bytes{fstype!~"tmpfs|overlay"}) / min(node_filesystem_size_bytes{fstype!~"tmpfs|overlay"}) * 100)`, 80, 90},
		{"memory", "%", `100 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes * 100)`, 85, 95},
		{"postgres_connections", "%", `sum(pg_stat_activity_count) / clamp_min(max(pg_settings_max_connections), 1) * 100`, 70, 85},
		{"redis_memory", "%", `redis_memory_used_bytes / clamp_min(redis_memory_max_bytes, 1) * 100`, 80, 90},
	}

	gauges := make([]Gauge, 0, len(specs))
	for _, item := range specs {
		value, measured, err := instantSample(ctx, item.query)
		if err != nil {
			continue
		}
		// A gauge with no sample is an exporter that is not running, and it
		// used to read as 0% and green — the most reassuring possible way to
		// display "nobody is watching this disk".
		if !measured {
			gauges = append(gauges, Gauge{
				Name: item.name, Unit: item.unit, Warning: item.warning, State: "unknown",
			})
			continue
		}
		gauges = append(gauges, Gauge{
			Name: item.name, Value: value, Unit: item.unit, Warning: item.warning,
			State: tone(value, item.warning, item.critical), Measured: true,
		})
	}
	return gauges
}

// tone turns a number into a colour. One function so that "amber" means the
// same thing in every panel.
func tone(value, warning, critical float64) string {
	switch {
	case value >= critical:
		return "red"
	case value >= warning:
		return "amber"
	default:
		return "green"
	}
}

// firingAlerts reads Alertmanager.
//
// Alertmanager rather than Prometheus, because what matters here is what is
// *being notified about* — an alert that is silenced during a planned
// migration should look different from one nobody has seen.
func (s *Service) firingAlerts(ctx context.Context) []Alert {
	base := alertmanagerURL()
	if base == "" {
		return []Alert{}
	}

	var payload []struct {
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
		StartsAt    time.Time         `json:"startsAt"`
		Status      struct {
			State      string   `json:"state"`
			SilencedBy []string `json:"silencedBy"`
		} `json:"status"`
	}
	if err := getJSON(ctx, base+"/api/v2/alerts?active=true&silenced=true", &payload); err != nil {
		slog.Warn("control plane: could not read the alerts", "error", err)
		return []Alert{}
	}

	alerts := make([]Alert, 0, len(payload))
	for _, item := range payload {
		alerts = append(alerts, Alert{
			Name:     item.Labels["alertname"],
			Severity: item.Labels["severity"],
			Summary:  item.Annotations["summary"],
			Runbook:  item.Annotations["runbook"],
			StartsAt: item.StartsAt,
			Silenced: len(item.Status.SilencedBy) > 0,
		})
	}
	return alerts
}

// instantQuery asks Prometheus one question and reads one number.
//
// A vector with no samples is not an error: it is what a metric that has never
// been recorded looks like — a deployment where nobody has signed in yet has
// no logins_total — and it answers zero.
// instantQuery answers with zero when Prometheus holds nothing, which is the
// right reading for a counter nobody has incremented — no sign-ins on a
// deployment where nobody has signed in is a real zero.
func instantQuery(ctx context.Context, query string) (float64, error) {
	value, _, err := instantSample(ctx, query)
	return value, err
}

// instantSample is the same read, and says whether there was a sample at all.
//
// The difference matters wherever a missing series means "nobody has measured
// this" rather than "this is zero": an external system with no traffic since
// the last restart, a gauge whose exporter is not running. Both used to reach
// the screen as a confident green zero.
func instantSample(ctx context.Context, query string) (float64, bool, error) {
	base := prometheusURL()
	if base == "" {
		return 0, false, fmt.Errorf("no Prometheus is configured")
	}

	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Value []any `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	address := base + "/api/v1/query?query=" + url.QueryEscape(query)
	if err := getJSON(ctx, address, &payload); err != nil {
		return 0, false, err
	}
	if payload.Status != "success" {
		return 0, false, fmt.Errorf("prometheus answered %q", payload.Status)
	}
	if len(payload.Data.Result) == 0 || len(payload.Data.Result[0].Value) < 2 {
		return 0, false, nil
	}

	// [ <unix time>, "<value>" ] — the value is a string, and it may be NaN
	// when the expression divided by nothing.
	text, ok := payload.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, false, nil
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		// A value Prometheus sent that Go cannot read is a version skew, not a
		// number: reported rather than shown, so the panel says "could not
		// read" instead of "zero".
		return 0, false, fmt.Errorf("prometheus sent %q, which is not a number: %w", text, err)
	}
	// NaN is what dividing by nothing produces, and it is an honest zero here:
	// an error rate over no requests is not an error rate, and it must not
	// reach a screen as "NaN%" or a JSON body a browser refuses to parse.
	if math.IsNaN(value) || math.IsInf(value, 0) {
		// The sample existed; it is simply not a number worth showing.
		return 0, true, nil
	}
	return value, true, nil
}

// getJSON is one GET with a short deadline.
//
// No shared client with connection reuse, because these run a few times a
// minute at most and a client held across a monitoring stack being restarted is
// a client holding a connection to something that no longer exists.
func getJSON(ctx context.Context, address string, into any) error {
	ctx, cancel := context.WithTimeout(ctx, monitoringTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answered %d", address, response.StatusCode)
	}
	return json.NewDecoder(response.Body).Decode(into)
}
