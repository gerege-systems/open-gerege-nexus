package observability

import (
	"encoding/json"
	"strings"
	"testing"
)

// Every list on this screen is a list on the wire.
//
// The console renders each of them with .map, and a nil slice in Go marshals as
// `null`. That is not a smaller list — it is a different type, and the page it
// reaches answers "This page couldn't load" with nothing else on it. A
// deployment whose warnings callback was never wired up hit exactly that, and
// the console's front page was blank until it was found.
func TestNoListOnTheFrontPageIsEverNull(t *testing.T) {
	// The zero value on purpose: no Prometheus, no Alertmanager, no callbacks.
	// It is the shape a deployment has before anybody configures anything, and
	// it is the shape that broke. Health() ends with this same call.
	overview := Overview{}.withLists()

	encoded, err := json.Marshal(overview)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, field := range []string{"external", "infra", "alerts", "background", "tenant_trouble", "warnings"} {
		raw, present := wire[field]
		if !present {
			t.Errorf("%s is missing from the answer", field)
			continue
		}
		if string(raw) == "null" {
			t.Errorf("%s is null; the console reads it as a list", field)
		}
	}

	// And the catalogue's own list, one level down.
	if strings.Contains(string(encoded), `"apps":null`) {
		t.Error("catalog.apps is null; the console reads it as a list")
	}
}

// The callback the console's front page is built with. A Service without one is
// a deployment that forgot to wire it, and the answer is an empty list rather
// than a broken page.
func TestWarningsWithoutACallbackIsEmptyNotNil(t *testing.T) {
	if warnings := (&Service{}).warnings(); warnings == nil {
		t.Fatal("warnings() returned nil")
	}
	// And a callback that itself answers nil is treated the same way.
	service := &Service{warningsFrom: func() []string { return nil }}
	if warnings := service.warnings(); warnings == nil {
		t.Fatal("a nil answer from the callback became a nil list")
	}
}
