package signing

import (
	"strings"
	"testing"
)

func TestValidatePDFRejectsNonPDFAndTruncation(t *testing.T) {
	good := []byte("%PDF-1.4\nbody\ntrailer\n%%EOF\n")
	if err := validatePDF(good); err != nil {
		t.Errorf("a well-formed PDF was rejected: %v", err)
	}

	cases := map[string][]byte{
		"an image renamed .pdf":      []byte("\x89PNG\r\n\x1a\n rest of a png"),
		"an empty file":              {},
		"a header shorter than 5 by": []byte("%PD"),
		// The dangerous one: a valid header on a truncated body. The signing
		// service accepts it and returns something that will not open.
		"a truncated PDF": append([]byte("%PDF-1.4\n"), make([]byte, 4096)...),
	}
	for name, body := range cases {
		if err := validatePDF(body); err == nil {
			t.Errorf("validatePDF accepted %s", name)
		}
	}
}

func TestSanitizeFileNameStripsTraversalAndKeepsCyrillic(t *testing.T) {
	cases := map[string]string{
		"Гэрээ.pdf":              "Гэрээ.pdf",
		"../../etc/passwd":       "passwd.pdf",
		"../../../../secret.pdf": "secret.pdf",
		`quote".pdf`:             "quote.pdf",
		"":                       "document.pdf",
		"   ":                    "document.pdf",
		"no-extension":           "no-extension.pdf",
		"UPPER.PDF":              "UPPER.PDF",
	}
	for in, want := range cases {
		if got := sanitizeFileName(in); got != want {
			t.Errorf("sanitizeFileName(%q) = %q, want %q", in, got, want)
		}
	}

	// A name long enough to break a header is truncated but stays a PDF.
	long := sanitizeFileName(strings.Repeat("а", 400) + ".pdf")
	if n := len([]rune(long)); n > 120 {
		t.Errorf("sanitizeFileName kept %d runes, want it capped at 120", n)
	}
	if !strings.HasSuffix(long, ".pdf") {
		t.Errorf("truncation dropped the extension: %q", long)
	}
}

func TestPlacementValidateKeepsTheStampOnThePage(t *testing.T) {
	if err := DefaultPlacement().validate(); err != nil {
		t.Fatalf("the default placement is invalid: %v", err)
	}

	// Each of these would put the stamp somewhere invisible while still
	// marking the document signed.
	cases := map[string]Placement{
		"past the right edge":  {X: 500, Y: 200, Width: 200, Height: 56, Text: "x"},
		"past the bottom edge": {X: 80, Y: 800, Width: 200, Height: 56, Text: "x"},
		"absurdly wide":        {X: 0, Y: 0, Width: 5000, Height: 56, Text: "x"},
		"too small to see":     {X: 80, Y: 200, Width: 10, Height: 56, Text: "x"},
		"caption far too long": {X: 80, Y: 200, Width: 200, Height: 56, Text: strings.Repeat("б", 200)},
	}
	for name, p := range cases {
		if err := p.validate(); err == nil {
			t.Errorf("validate accepted a placement %s", name)
		}
	}
}

func TestPlacementNormalizeFillsFromDefaults(t *testing.T) {
	got := Placement{}.normalize()
	if got != DefaultPlacement() {
		t.Errorf("normalize() = %+v, want the default placement", got)
	}

	// An explicit value survives normalisation.
	got = Placement{X: 120, Text: "Баталгаажив"}.normalize()
	if got.X != 120 || got.Text != "Баталгаажив" {
		t.Errorf("normalize overwrote explicit fields: %+v", got)
	}
	if got.Width != defaultSigWidth {
		t.Errorf("width = %d, want the default %d", got.Width, defaultSigWidth)
	}
}

func TestPlacementMergeOverLetsARequestOverrideOneField(t *testing.T) {
	base := Placement{X: 100, Y: 300, Width: 220, Height: 60, PageNumber: 2, Text: "base"}
	got := Placement{Y: 400}.mergeOver(base)
	if got.Y != 400 {
		t.Errorf("Y = %d, want the override 400", got.Y)
	}
	if got.X != 100 || got.Width != 220 || got.Text != "base" {
		t.Errorf("merge lost the base placement: %+v", got)
	}
}

func TestPolicyNormalizeResolvesAContradiction(t *testing.T) {
	// A policy that demands eID but names the HSM as its default would refuse
	// every signature it started.
	got := Policy{RequireEID: true, DefaultProvider: ProviderHSM}.normalize()
	if got.DefaultProvider != ProviderEID {
		t.Errorf("default provider = %q, want EID when eID is required", got.DefaultProvider)
	}
}

func TestPolicyNormalizeRejectsNonsense(t *testing.T) {
	got := Policy{
		DefaultProvider:     "CARRIER_PIGEON",
		MinCertificateLevel: "NONE",
		RetentionDays:       -5,
		MaxUploadMB:         10000,
	}.normalize()

	d := DefaultPolicy()
	if got.DefaultProvider != d.DefaultProvider {
		t.Errorf("provider = %q, want it reset to %q", got.DefaultProvider, d.DefaultProvider)
	}
	if got.MinCertificateLevel != d.MinCertificateLevel {
		t.Errorf("certificate level = %q, want it reset to %q", got.MinCertificateLevel, d.MinCertificateLevel)
	}
	if got.RetentionDays != 0 {
		t.Errorf("retention = %d, want a negative value clamped to 0", got.RetentionDays)
	}
	if got.MaxUploadMB != d.MaxUploadMB {
		t.Errorf("upload cap = %d, want it clamped to the platform ceiling %d", got.MaxUploadMB, d.MaxUploadMB)
	}
}

func TestSessionIDPatternMatchesTheGenerator(t *testing.T) {
	// Session ids are now issued by the shared signing library (its randID:
	// 16 random bytes, hex-encoded). This guard has to keep accepting that
	// shape, because the browser refuses to poll an id that fails it.
	if !sessionIDPattern.MatchString("8d3f2a86ab09dfbf73bf96b1badfc404") {
		t.Error("the guard rejects the id format the signing library issues")
	}

	for _, bad := range []string{
		"", "short",
		"ABCDEF0123456789ABCDEF0123456789",   // upper case
		"g123456789012345678901234567890a",   // non-hex
		"0123456789012345678901234567890123", // too long
		"../../etc/passwd",                   // traversal
		"0123456789012345678901234567890",    // 31 chars
	} {
		if sessionIDPattern.MatchString(bad) {
			t.Errorf("the guard accepted a malformed session id %q", bad)
		}
	}
}

func TestValidateEtsiGuardsThePathSegment(t *testing.T) {
	// The Cyrillic cases are the ones that matter: a Mongolian registration
	// number is УА00112233, and an ASCII-only character class rejected every
	// real one — including the example the signing screen offers as a
	// placeholder. It reached production and returned INVALID_SIGNER for
	// anybody who typed their own number.
	for _, good := range []string{
		"PNOMN-111949212017", "NTRMN-1234567",
		"PNOMN-УА00112233", "PNOMN-МА74101813", "NTRMN-УБ1234567",
	} {
		if err := validateEtsi(good); err != nil {
			t.Errorf("validateEtsi(%q) = %v, want nil", good, err)
		}
	}
	// A signer identifier goes straight into an upstream URL path, so anything
	// that could escape it has to be refused before the request is built.
	for _, bad := range []string{
		"", "PNOMN-", "111949212017", "PNOMN-../../admin",
		"PNOMN-abc/def", "OTHER-123", "PNOMN-" + strings.Repeat("9", 40),
		// Still refused: these are what the guard is actually for.
		"PNOMN-УА/../admin", "PNOMN-УА 00112233", "PNOMN-УА.00112233",
		"PNOMN-" + strings.Repeat("У", 40),
	} {
		if err := validateEtsi(bad); err == nil {
			t.Errorf("validateEtsi accepted %q", bad)
		}
	}
}

func TestCivilIDFromEtsi(t *testing.T) {
	cases := map[string]string{
		"PNOMN-111949212017": "111949212017",
		"NTRMN-1234567":      "1234567",
		"bare":               "bare",
		"":                   "",
	}
	for in, want := range cases {
		if got := civilIDFromEtsi(in); got != want {
			t.Errorf("civilIDFromEtsi(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDedupePreservesOrder(t *testing.T) {
	got := dedupe([]string{"b", "a", "b", "", "  ", "c", "a"})
	want := []string{"b", "a", "c"}
	if len(got) != len(want) {
		t.Fatalf("dedupe = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dedupe = %v, want %v", got, want)
		}
	}
}

func TestSignedNameKeepsOneExtension(t *testing.T) {
	cases := map[string]string{
		"Гэрээ.pdf":    "Гэрээ-signed.pdf",
		"contract.PDF": "contract-signed.pdf",
		"no-ext":       "no-ext-signed.pdf",
	}
	for in, want := range cases {
		if got := signedName(in); got != want {
			t.Errorf("signedName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestActorCanGrantsAdminEverything(t *testing.T) {
	admin := Actor{IsAdmin: true}
	for _, perm := range []string{PermRead, PermSign, PermManage} {
		if !admin.can(perm) {
			t.Errorf("an admin was denied %s", perm)
		}
	}

	// The point of this module's own guard: a user holding read must not be
	// able to sign, which the platform's blanket app gate cannot express.
	reader := Actor{Perms: map[string]bool{PermRead: true}}
	if !reader.can(PermRead) {
		t.Error("a reader was denied esign.read")
	}
	if reader.can(PermSign) || reader.can(PermManage) {
		t.Error("a reader was granted signing or management rights")
	}
}

func TestIsReachableRejectionSeparatesRefusalFromOutage(t *testing.T) {
	// A refusal means the endpoint is configured correctly and answered.
	for _, msg := range []string{
		"digital signature certificate is invalid or not registered",
		"certificate UID does not match the supplied civil ID",
	} {
		if !isReachableRejection(errString(msg)) {
			t.Errorf("a service refusal was reported as unreachable: %q", msg)
		}
	}
	for _, msg := range []string{
		"dial tcp: lookup hsm.gerege.mn: no such host",
		"dial tcp 1.2.3.4:443: connection refused",
		"context deadline exceeded",
		"net/http: TLS handshake timeout",
	} {
		if isReachableRejection(errString(msg)) {
			t.Errorf("a transport failure was reported as reachable: %q", msg)
		}
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestNullableTurnsBlanksIntoNULL(t *testing.T) {
	// Storing "" in a nullable column defeats every COALESCE downstream.
	for _, blank := range []string{"", "   ", "\t\n"} {
		if nullable(blank) != nil {
			t.Errorf("nullable(%q) should be NULL", blank)
		}
	}
	if got := nullable("value"); got == nil || *got != "value" {
		t.Error("nullable dropped a real value")
	}
}
