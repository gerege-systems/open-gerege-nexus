package signing

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

// The export endpoint reads an absent body and a body naming one connector as
// two different requests: no body means every automatic destination, a body
// means that destination alone. Discarding a malformed body — which is what
// `_ = decodeLargeJSON(...)` did — collapses the second into the first, so a
// request to file a signed contract in the archive files it in every connected
// Drive and Dropbox the tenant has. That is a disclosure nobody asked for, and
// it happens silently, on a 200.
func TestDecodeOptionalJSONSeparatesAnAbsentBodyFromABrokenOne(t *testing.T) {
	type body struct {
		IntegrationID string `json:"integration_id"`
	}

	accepted := []struct {
		name string
		raw  string
		want string
	}{
		{"no body at all", "", ""},
		{"whitespace only", "  \n\t ", ""},
		{"an empty object", "{}", ""},
		{"a named destination", `{"integration_id":"c0ffee"}`, "c0ffee"},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			var out body
			req := httptest.NewRequest(http.MethodPost, "/export", strings.NewReader(tc.raw))
			if err := decodeOptionalJSON(req, &out); err != nil {
				t.Fatalf("decodeOptionalJSON rejected %q: %v", tc.raw, err)
			}
			if out.IntegrationID != tc.want {
				t.Fatalf("integration_id is %q, want %q", out.IntegrationID, tc.want)
			}
		})
	}

	rejected := []struct {
		name string
		raw  string
	}{
		{"truncated JSON", `{"integration_id":`},
		{"not JSON", "integration_id=c0ffee"},
		{"a JSON array where an object belongs", `["c0ffee"]`},
		// The one that motivates this: a client sent a destination and got the
		// quoting wrong. Read as "no body", it becomes a copy to everywhere.
		{"an unquoted value", `{integration_id: c0ffee}`},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			var out body
			req := httptest.NewRequest(http.MethodPost, "/export", strings.NewReader(tc.raw))
			err := decodeOptionalJSON(req, &out)
			if err == nil {
				t.Fatalf("decodeOptionalJSON accepted %q as an absent body", tc.raw)
			}
			var domainErr *Error
			if !errors.As(err, &domainErr) {
				t.Fatalf("error is %T, want *Error so the handler answers with a code", err)
			}
			if domainErr.Code != "INVALID_BODY" {
				t.Fatalf("error code is %q, want INVALID_BODY", domainErr.Code)
			}
			if domainErr.Status != http.StatusBadRequest {
				t.Fatalf("status is %d, want %d", domainErr.Status, http.StatusBadRequest)
			}
			if out.IntegrationID != "" {
				t.Fatalf("a rejected body still populated integration_id as %q", out.IntegrationID)
			}
		})
	}
}

// The filename a document is filed under is built from a title a user typed, so
// it has to survive being one. This covers the export path specifically: the
// name reaches Drive's metadata and Dropbox's path.
func TestExportFilenameIsAlwaysAName(t *testing.T) {
	const fallback = "7f3a1c8e-0000-4000-8000-000000000000"

	cases := map[string]string{
		"Гэрээ":                     "Гэрээ.pdf",
		"Contract":                  "Contract.pdf",
		"Contract.pdf":              "Contract.pdf",
		"Contract.PDF":              "Contract.PDF",
		"../../etc/passwd":          "passwd.pdf",
		`C:\Windows\System32\hosts`: "C Windows System32 hosts.pdf",
		"report: Q1 <draft>":        "report Q1 draft.pdf",
		"  spaced   out  ":          "spaced out.pdf",
		"":                          fallback + ".pdf",
	}
	for title, want := range cases {
		got := exportFilename(title, fallback)
		if got != want {
			t.Errorf("exportFilename(%q) = %q, want %q", title, got, want)
		}
	}

	// A title that is only separators has nothing left to file under, so it
	// falls back to the id rather than to an empty name.
	if got := exportFilename("///", fallback); got != fallback+".pdf" {
		t.Errorf("exportFilename(%q) = %q, want the document id", "///", got)
	}
}

// A Mongolian title is two bytes per letter, so the length cap lands inside a
// character roughly half the time. name[:200] left the broken half in place,
// and that filename then travels through JSON to Drive and through a header to
// Dropbox — neither of which is expecting a byte that is not a character.
func TestExportFilenameCutsLongTitlesOnACharacterBoundary(t *testing.T) {
	const fallback = "7f3a1c8e-0000-4000-8000-000000000000"

	long := strings.Repeat("Гэрээ ", 60) // ~660 bytes, well past the cap
	got := exportFilename(long, fallback)

	if !utf8.ValidString(got) {
		t.Fatalf("the filename is not valid UTF-8: %q", []byte(got))
	}
	if len(got) > 255 {
		t.Fatalf("the filename is %d bytes, past what these filesystems accept", len(got))
	}
	if !strings.HasSuffix(got, ".pdf") {
		t.Fatalf("the extension was lost: %q", got)
	}
	if !strings.HasPrefix(long, strings.TrimSuffix(got, ".pdf")) {
		t.Fatalf("the kept part is not a prefix of the title: %q", got)
	}
}
