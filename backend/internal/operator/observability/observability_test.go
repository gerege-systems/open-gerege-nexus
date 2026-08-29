package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The console's front page has to survive its sources being down, because the
// moment somebody opens it is the moment something is wrong. These check the
// two halves of that: a Prometheus that answers nonsense does not become a
// number, and a monitoring stack that is not configured at all produces a page
// that says so rather than a page of zeroes.

func TestAnUnconfiguredMonitoringStackIsSaidRatherThanShownAsZero(t *testing.T) {
	t.Setenv("PROMETHEUS_URL", "")
	t.Setenv("ALERTMANAGER_URL", "")

	if _, err := instantQuery(context.Background(), rpsQuery); err == nil {
		t.Fatal("a query against no Prometheus answered without an error")
	}

	service := &Service{}
	// Only the parts that need no database: Health itself would query one.
	if prometheusURL() != "" {
		t.Fatal("the URL is not empty")
	}
	if got := service.firingAlerts(context.Background()); len(got) != 0 {
		t.Fatalf("alerts were read with no Alertmanager: %v", got)
	}
}

func TestAnInstantQueryReadsTheValue(t *testing.T) {
	prometheus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") == "" {
			t.Error("the query was not sent")
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector",
		    "result":[{"metric":{},"value":[1700000000,"12.5"]}]}}`))
	}))
	defer prometheus.Close()
	t.Setenv("PROMETHEUS_URL", prometheus.URL)

	value, err := instantQuery(context.Background(), rpsQuery)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if value != 12.5 {
		t.Fatalf("read %v, want 12.5", value)
	}
}

// An expression that matched nothing is not an error: a metric nobody has
// recorded — logins on a deployment where nobody has signed in — is a real and
// ordinary state, and it means zero.
func TestAnEmptyResultIsZeroRatherThanAFailure(t *testing.T) {
	prometheus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer prometheus.Close()
	t.Setenv("PROMETHEUS_URL", prometheus.URL)

	value, err := instantQuery(context.Background(), errorRateQuery)
	if err != nil {
		t.Fatalf("an empty result was an error: %v", err)
	}
	if value != 0 {
		t.Fatalf("an empty result read %v", value)
	}
}

// NaN is what dividing by nothing produces in PromQL, and it must not reach a
// screen as "NaN%" or as JSON that a browser cannot parse.
func TestNaNIsReadAsZero(t *testing.T) {
	prometheus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"result":[{"value":[1,"NaN"]}]}}`))
	}))
	defer prometheus.Close()
	t.Setenv("PROMETHEUS_URL", prometheus.URL)

	value, err := instantQuery(context.Background(), errorRateQuery)
	if err != nil || value != 0 {
		t.Fatalf("NaN read as %v (%v)", value, err)
	}
}

func TestAlertsAreReadFromAlertmanager(t *testing.T) {
	alertmanager := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"labels":{"alertname":"NexusDiskFillingUp","severity":"ticket"},
		    "annotations":{"summary":"Дискний зай багасч байна","runbook":"docs/RUNBOOKS.md#x"},
		    "startsAt":"2026-08-13T10:00:00Z","status":{"state":"active","silencedBy":["abc"]}}]`))
	}))
	defer alertmanager.Close()
	t.Setenv("ALERTMANAGER_URL", alertmanager.URL)

	alerts := (&Service{}).firingAlerts(context.Background())
	if len(alerts) != 1 {
		t.Fatalf("read %d alerts", len(alerts))
	}
	if alerts[0].Name != "NexusDiskFillingUp" || alerts[0].Severity != "ticket" {
		t.Fatalf("the alert came back as %+v", alerts[0])
	}
	// Silenced is shown rather than filtered: an alert somebody muted during a
	// migration is a different thing from one nobody has seen, and the console
	// should show both.
	if !alerts[0].Silenced {
		t.Fatal("a silenced alert was reported as unsilenced")
	}
}

func TestToneMatchesTheThresholds(t *testing.T) {
	cases := []struct {
		value float64
		want  string
	}{{0, "green"}, {0.04, "green"}, {0.05, "amber"}, {0.19, "amber"}, {0.2, "red"}, {1, "red"}}
	for _, c := range cases {
		if got := tone(c.value, 0.05, 0.2); got != c.want {
			t.Errorf("tone(%v) = %q, want %q", c.value, got, c.want)
		}
	}
}

// A system nobody has called is not a system that is well.
//
// Prometheus holds no series for it, every query answers with an empty vector,
// and the screen used to render that as 0.00% errors and a green light — the
// most reassuring possible way to say "nothing is watching this". Six external
// systems sat green on nexus.gerege.mn for as long as the panel had existed.
func TestAnUnmeasuredSystemIsNotGreen(t *testing.T) {
	prometheus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer prometheus.Close()
	t.Setenv("PROMETHEUS_URL", prometheus.URL)

	service := New(nil, Deps{})
	for _, dot := range service.externalSystems(context.Background()) {
		if dot.State != "unknown" || dot.Measured {
			t.Fatalf("%s reads %+v with nothing measured", dot.System, dot)
		}
	}
	for _, gauge := range service.infrastructure(context.Background()) {
		if gauge.State != "unknown" || gauge.Measured {
			t.Fatalf("%s reads %+v with nothing measured", gauge.Name, gauge)
		}
	}
}

// And a system that is measured keeps its colour: the fix must not turn a
// working panel into a screen of question marks.
func TestAMeasuredSystemKeepsItsColour(t *testing.T) {
	prometheus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"result":[{"value":[1,"0.01"]}]}}`))
	}))
	defer prometheus.Close()
	t.Setenv("PROMETHEUS_URL", prometheus.URL)

	service := New(nil, Deps{})
	dots := service.externalSystems(context.Background())
	if len(dots) == 0 {
		t.Fatal("no external systems came back")
	}
	for _, dot := range dots {
		if !dot.Measured || dot.State != "green" {
			t.Fatalf("%s reads %+v with a one-percent error rate", dot.System, dot)
		}
	}
}
