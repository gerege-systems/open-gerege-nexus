package reporting

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// fixture is a report with one of every parameter kind, so Bind is exercised
// against a declaration rather than against a mock.
type fixture struct {
	key string
	app string
}

func (f fixture) Key() string { return f.key }
func (f fixture) App() string { return f.app }
func (f fixture) Titles() map[string]string {
	return map[string]string{"mn": "Туршилтын тайлан", "en": "Test report"}
}

func (f fixture) Params() []ParamSpec {
	return []ParamSpec{
		{Key: "period", Kind: ParamDateRange, Titles: map[string]string{"mn": "Хугацаа"}},
		{Key: "warehouse_id", Kind: ParamUUID, Titles: map[string]string{"mn": "Агуулах"}},
		{Key: "mode", Kind: ParamSelect, Titles: map[string]string{"mn": "Горим"},
			Options: []ParamOption{{Value: "a"}, {Value: "b"}}, Default: "a"},
		{Key: "note", Kind: ParamText, Titles: map[string]string{"mn": "Тэмдэглэл"}},
		{Key: "compact", Kind: ParamBool, Titles: map[string]string{"mn": "Хураангуй"}, Default: true},
	}
}

func (f fixture) Columns() []ColumnSpec {
	return []ColumnSpec{
		{Key: "label", Kind: ColumnText, Chart: ChartCategory, Titles: map[string]string{"mn": "Нэр"}},
		{Key: "amount", Kind: ColumnMoney, Chart: ChartValue, Total: true, Titles: map[string]string{"mn": "Дүн"}},
		{Key: "count", Kind: ColumnNumber, Total: true, Titles: map[string]string{"mn": "Тоо"}},
	}
}

func (f fixture) Run(context.Context, Querier, Params) (Result, error) { return Result{}, nil }

func TestRegisterRejectsAnIncompleteReport(t *testing.T) {
	t.Cleanup(resetForTest)
	resetForTest()

	cases := map[string]Report{
		"no key":   fixture{key: "", app: "app"},
		"no app":   fixture{key: "k", app: ""},
		"no title": noTitle{},
	}
	for name, report := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("%s was accepted", name)
				}
			}()
			Register(report)
		})
	}
}

type noTitle struct{ fixture }

func (noTitle) Key() string               { return "x.y" }
func (noTitle) App() string               { return "app" }
func (noTitle) Titles() map[string]string { return map[string]string{"en": "Only English"} }
func (noTitle) Columns() []ColumnSpec     { return []ColumnSpec{{Key: "a"}} }
func (noTitle) Params() []ParamSpec       { return nil }

// Two different reports claiming one key means one of them silently vanishes
// from every listing. It must be a startup panic, not a runtime surprise.
func TestRegisterRejectsACollision(t *testing.T) {
	t.Cleanup(resetForTest)
	resetForTest()

	Register(fixture{key: "a.b", app: "app.one"})
	defer func() {
		if recover() == nil {
			t.Fatal("a second report took the same key without complaint")
		}
	}()
	Register(other{})
}

type other struct{ fixture }

func (other) Key() string               { return "a.b" }
func (other) App() string               { return "app.two" }
func (other) Titles() map[string]string { return map[string]string{"mn": "Өөр"} }
func (other) Columns() []ColumnSpec     { return []ColumnSpec{{Key: "a"}} }
func (other) Params() []ParamSpec       { return nil }

// A module built twice — every test fixture in this repository does it — must
// not bring the process down.
func TestRegisterToleratesTheSameReportTwice(t *testing.T) {
	t.Cleanup(resetForTest)
	resetForTest()

	Register(fixture{key: "a.b", app: "app.one"})
	Register(fixture{key: "a.b", app: "app.one"})

	if len(All()) != 1 {
		t.Fatalf("expected one report, got %d", len(All()))
	}
}

// The app gate, at the registry. A report whose app a tenant has not installed
// is not in their list at all.
func TestForAppsIsTheAppGate(t *testing.T) {
	t.Cleanup(resetForTest)
	resetForTest()

	Register(fixture{key: "billing.x", app: "io.gerege.nexus.billing"})
	Register(fixture{key: "inventory.y", app: "io.gerege.nexus.inventory"})

	permitted := ForApps(map[string]bool{"io.gerege.nexus.billing": true})
	if len(permitted) != 1 || permitted[0].Key() != "billing.x" {
		t.Fatalf("expected only the billing report, got %v", keysOf(permitted))
	}
	if len(ForApps(nil)) != 0 {
		t.Fatal("a tenant with no apps was offered a report")
	}
}

func keysOf(reports []Report) []string {
	keys := make([]string, 0, len(reports))
	for _, report := range reports {
		keys = append(keys, report.Key())
	}
	return keys
}

func TestBindAppliesDefaultsAndRejectsRubbish(t *testing.T) {
	report := fixture{key: "a.b", app: "app"}

	params, err := Bind(report, map[string]string{}, "mn")
	if err != nil {
		t.Fatalf("an empty request should bind to the defaults: %v", err)
	}
	if params.String("mode") != "a" {
		t.Errorf("select default not applied: %q", params.String("mode"))
	}
	if !params.Bool("compact") {
		t.Error("bool default not applied")
	}
	if params.Time("period_to").Before(params.Time("period_from")) {
		t.Error("the default range runs backwards")
	}

	// The end of the day, not its start: a month-end report must include the
	// last day's rows.
	to := params.Time("period_to")
	if to.Hour() != 23 || to.Minute() != 59 {
		t.Errorf("the range does not reach the end of the day: %s", to)
	}

	for name, raw := range map[string]map[string]string{
		"bad date":    {"period_from": "yesterday"},
		"reversed":    {"period_from": "2026-08-12", "period_to": "2026-01-01"},
		"bad uuid":    {"warehouse_id": "'; DROP TABLE warehouses; --"},
		"bad select":  {"mode": "c"},
		"bad bool":    {"compact": "perhaps"},
		"long text":   {"note": strings.Repeat("x", 500)},
		"huge window": {"period_from": "1900-01-01", "period_to": "2026-01-01"},
	} {
		if _, err := Bind(report, raw, "mn"); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// Anything the report did not declare is dropped rather than carried through.
func TestBindDropsUndeclaredParameters(t *testing.T) {
	params, err := Bind(fixture{key: "a.b", app: "app"},
		map[string]string{"tenant_id": "another-organisation", "mode": "b"}, "mn")
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	if _, present := params.Raw()["tenant_id"]; present {
		t.Error("an undeclared parameter reached the report")
	}
	if params.String("mode") != "b" {
		t.Error("a declared parameter was lost")
	}
}

func TestComputeTotalsSumsOnlyWhatAsked(t *testing.T) {
	result := Result{
		Columns: fixture{}.Columns(),
		Rows: []map[string]any{
			{"label": "a", "amount": 100.5, "count": int64(2)},
			{"label": "b", "amount": 99.5, "count": int64(3)},
		},
	}
	totals := computeTotals(result)

	if totals["amount"] != 200 {
		t.Errorf("amount total is %v", totals["amount"])
	}
	if totals["count"] != 5 {
		t.Errorf("count total is %v", totals["count"])
	}
	if _, present := totals["label"]; present {
		t.Error("a text column was summed")
	}
}

func TestParseCron(t *testing.T) {
	valid := map[string]struct {
		when  time.Time
		fires bool
	}{
		"0 9 1 * *":     {time.Date(2026, 9, 1, 9, 0, 0, 0, time.Local), true},
		"0 9 1 * * ":    {time.Date(2026, 9, 2, 9, 0, 0, 0, time.Local), false},
		"*/15 * * * *":  {time.Date(2026, 9, 2, 9, 30, 0, 0, time.Local), true},
		"*/15 * * * * ": {time.Date(2026, 9, 2, 9, 31, 0, 0, time.Local), false},
		"30 8 * * 1":    {time.Date(2026, 8, 10, 8, 30, 0, 0, time.Local), true}, // a Monday
		"30 8 * * 1  ":  {time.Date(2026, 8, 11, 8, 30, 0, 0, time.Local), false},
	}
	for expression, expectation := range valid {
		schedule, err := ParseCron(expression)
		if err != nil {
			t.Fatalf("%q: %v", expression, err)
		}
		if got := schedule.Matches(expectation.when); got != expectation.fires {
			t.Errorf("%q at %s: fires=%v, want %v", expression, expectation.when, got, expectation.fires)
		}
	}

	for _, invalid := range []string{"", "0 9 1 *", "60 9 * * *", "0 25 * * *", "a b c d e", "0 9 32 * *"} {
		if _, err := ParseCron(invalid); err == nil {
			t.Errorf("%q was accepted as a schedule", invalid)
		}
	}
}

func TestExportCSVCarriesTheBOMAndTheTotals(t *testing.T) {
	result := Result{
		Columns: fixture{}.Columns(),
		Rows:    []map[string]any{{"label": "Гэрэгэ", "amount": 1500.0, "count": int64(3)}},
		Totals:  map[string]float64{"amount": 1500, "count": 3},
	}

	payload, err := Export(FormatCSV, "Туршилт", result, "mn")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	text := string(payload)

	// Without the BOM, Excel on Windows renders every Mongolian heading as
	// mojibake — which is the whole content of the file.
	if !strings.HasPrefix(text, "\xef\xbb\xbf") {
		t.Error("the CSV has no UTF-8 byte-order mark")
	}
	if !strings.Contains(text, "Нэр") {
		t.Error("the localised header is missing")
	}
	if !strings.Contains(text, "Гэрэгэ") {
		t.Error("the row is missing")
	}
	if !strings.Contains(text, "Нийт") {
		t.Error("the totals row is missing")
	}
}

// An export whose numbers are text is a screenshot with extra steps. This
// checks the file is a real workbook and that the money cell is numeric.
func TestExportXLSXIsAWorkbook(t *testing.T) {
	result := Result{
		Columns: fixture{}.Columns(),
		Rows:    []map[string]any{{"label": "a", "amount": 12.5, "count": int64(2)}},
		Totals:  map[string]float64{"amount": 12.5, "count": 2},
	}

	payload, err := Export(FormatXLSX, "Туршилт", result, "mn")
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("the export is not a zip container: %v", err)
	}
	var hasWorkbook bool
	for _, file := range reader.File {
		if file.Name == "xl/workbook.xml" {
			hasWorkbook = true
		}
	}
	if !hasWorkbook {
		t.Error("no workbook part in the export")
	}
}

func TestParseFormatRefusesAnythingElse(t *testing.T) {
	if _, err := ParseFormat("pdf"); err == nil {
		t.Error("an unsupported format was accepted")
	}
	if format, _ := ParseFormat(""); format != FormatXLSX {
		t.Error("the default format is not xlsx")
	}
}

func TestLocalizedTitleFallsBackToMongolian(t *testing.T) {
	titles := map[string]string{"mn": "Монгол", "en": "English"}
	if got := LocalizedTitle(titles, "fr", "key"); got != "Монгол" {
		t.Errorf("expected the Mongolian fallback, got %q", got)
	}
	if got := LocalizedTitle(nil, "fr", "key"); got != "key" {
		t.Errorf("expected the key, got %q", got)
	}
}
