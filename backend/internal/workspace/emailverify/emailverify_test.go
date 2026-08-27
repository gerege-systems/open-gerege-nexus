package emailverify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// The address is handed to a sending service as a recipient. Anything with
// structure in it — a display name, a comma, a newline — is a way to append a
// second recipient, so it is refused rather than cleaned up.
func TestNormalizeEmailRefusesAnythingButAPlainAddress(t *testing.T) {
	valid := map[string]string{
		"user@example.com":       "user@example.com",
		"  User@Example.COM  ":   "user@example.com",
		"first.last+tag@mail.mn": "first.last+tag@mail.mn",
	}
	for input, want := range valid {
		got, err := NormalizeEmail(input)
		if err != nil {
			t.Errorf("NormalizeEmail(%q) returned %v, want it accepted", input, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeEmail(%q) = %q, want %q", input, got, want)
		}
	}

	rejected := []string{
		"",
		"   ",
		"not-an-address",
		"Ops <ops@example.com>",
		"a@example.com, b@example.com",
		"a@example.com\r\nBcc: victim@example.com",
		"a@example.com\nSubject: forged",
		"a@b@example.com",
		strings.Repeat("a", 315) + "@example.com",
	}
	for _, input := range rejected {
		if got, err := NormalizeEmail(input); err == nil {
			t.Errorf("NormalizeEmail(%q) accepted it as %q, want a refusal", input, got)
		}
	}
}

// The landing endpoint forwards people onward from this platform's own domain.
// An unchecked destination is what turns Gerege Nexus into the open redirector
// a phishing link wants to borrow.
func TestValidateRedirectRefusesWhatWouldMakeUsAnOpenRedirector(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	// This test is about the shape of a URL — scheme, credentials, CRLF, whether
	// it is absolute at all. Which hosts are permitted is a separate question,
	// answered in redirect_test.go, so the host used here is simply listed.
	t.Setenv("EMAIL_VERIFY_REDIRECT_HOSTS", "theirapp.com")

	accepted := []string{
		"",
		"https://theirapp.com/verified",
		"https://theirapp.com/verified?next=%2Fhome",
		"http://localhost:3000/verified",
		"http://127.0.0.1:3000/verified",
	}
	for _, input := range accepted {
		if _, err := ValidateRedirect(input); err != nil {
			t.Errorf("ValidateRedirect(%q) returned %v, want it accepted", input, err)
		}
	}

	rejected := []string{
		"http://theirapp.com/verified",
		"/verified",
		"theirapp.com/verified",
		"javascript:alert(1)",
		"https://user:pass@theirapp.com/verified",
		"https://theirapp.com/\r\nSet-Cookie: a=b",
		"data:text/html,<script>alert(1)</script>",
	}
	for _, input := range rejected {
		if _, err := ValidateRedirect(input); err == nil {
			t.Errorf("ValidateRedirect(%q) accepted it, want a refusal", input)
		}
	}
}

// Outside development even localhost has to be HTTPS: a production deployment
// resolving "localhost" is resolving something on its own host.
func TestValidateRedirectRefusesPlainHTTPInProduction(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("EMAIL_VERIFY_REDIRECT_HOSTS", "theirapp.com")
	if _, err := ValidateRedirect("http://localhost:3000/verified"); err == nil {
		t.Fatal("plain HTTP to localhost was accepted in production")
	}
	if _, err := ValidateRedirect("https://theirapp.com/verified"); err != nil {
		t.Fatalf("HTTPS was refused in production: %v", err)
	}
}

// The reference is a bearer credential for exactly one act, and the database
// holds only its hash — so the table cannot be read back into a working link.
func TestReferencesAreUniqueAndStoredOnlyAsHashes(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 64; i++ {
		ref, err := randomRef()
		if err != nil {
			t.Fatalf("randomRef failed: %v", err)
		}
		if _, duplicate := seen[ref]; duplicate {
			t.Fatalf("randomRef repeated itself: %q", ref)
		}
		seen[ref] = struct{}{}

		hash := hashSecret(ref)
		if hash == ref {
			t.Fatal("the stored hash is the reference itself")
		}
		if len(hash) != 64 {
			t.Fatalf("hash is %d characters, the column holds 64", len(hash))
		}
		if hashSecret(ref) != hash {
			t.Fatal("hashing the same reference twice gave two answers")
		}
	}
}

// Where the service is asked to send people back to. It comes from
// PUBLIC_ORIGIN, never from a request: it is handed to another service and
// outlives the call, so a forged Host header must not be able to point it.
func TestReturnURLIsBuiltFromPublicOrigin(t *testing.T) {
	t.Setenv("PUBLIC_ORIGIN", "https://nexus.gerege.mn/")
	if got := ReturnURL(); got != "https://nexus.gerege.mn/api/v1/verify/landed" {
		t.Errorf("ReturnURL() = %q", got)
	}

	t.Setenv("PUBLIC_ORIGIN", "")
	if got := ReturnURL(); !strings.HasPrefix(got, "http://localhost:8080/") {
		t.Errorf("unset PUBLIC_ORIGIN gave %q, want the local development default", got)
	}
}

func TestProviderURLIsOverridable(t *testing.T) {
	t.Setenv("EMAIL_VERIFY_BASE_URL", "")
	if got := ProviderURL(); got != DefaultProviderURL {
		t.Errorf("ProviderURL() = %q, want the default %q", got, DefaultProviderURL)
	}
	t.Setenv("EMAIL_VERIFY_BASE_URL", "https://verify.internal/api/verify/")
	if got := ProviderURL(); got != "https://verify.internal/api/verify" {
		t.Errorf("ProviderURL() = %q, want the trailing slash trimmed", got)
	}
}

// Without a key nothing can be sent, and that is an operator's problem rather
// than the caller's — worth its own error so the screen can say which.
func TestConfiguredFollowsTheKey(t *testing.T) {
	t.Setenv("EMAIL_VERIFY_API_KEY", "")
	if Configured() {
		t.Error("Configured() is true with no key set")
	}
	t.Setenv("EMAIL_VERIFY_API_KEY", "   ")
	if Configured() {
		t.Error("Configured() is true for a key of spaces")
	}
	t.Setenv("EMAIL_VERIFY_API_KEY", "evk_test")
	if !Configured() {
		t.Error("Configured() is false with a key set")
	}
}

// stubProvider stands in for the hosted service and records what it was asked,
// so a test can assert on the request as well as on the answer.
type stubProvider struct {
	mu         sync.Mutex
	status     int
	body       string
	retryAfter string
	payload    map[string]string
	calls      int
	server     *httptest.Server
}

func newStubProvider(t *testing.T, status int, body string) *stubProvider {
	t.Helper()
	stub := &stubProvider{status: status, body: body}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		var payload map[string]string
		_ = json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&payload)

		stub.mu.Lock()
		stub.calls++
		stub.payload = payload
		status, body, retryAfter := stub.status, stub.body, stub.retryAfter
		stub.mu.Unlock()

		if r.Header.Get("Authorization") != "Bearer evk_test" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(stub.server.Close)
	t.Setenv("EMAIL_VERIFY_BASE_URL", stub.server.URL)
	t.Setenv("EMAIL_VERIFY_API_KEY", "evk_test")
	return stub
}

func (s *stubProvider) seen() (int, map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls, s.payload
}

// requestSend is where somebody else's status codes become this package's
// errors, and that mapping is the contract every caller reads: a bad address is
// final, a rejected key is the operator's problem, a rate limit and a failure
// upstream are worth retrying.
func TestUpstreamStatusCodesBecomeErrorsCallersCanActOn(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		retryAfter string
		assert     func(t *testing.T, err error)
	}{
		{
			name: "a malformed address is the caller's mistake", status: http.StatusBadRequest,
			body: `{"error":"invalid_email"}`,
			assert: func(t *testing.T, err error) {
				var invalidErr *InvalidError
				if !errors.As(err, &invalidErr) {
					t.Fatalf("got %v, want an InvalidError", err)
				}
			},
		},
		{
			// The return address is ours, not the caller's, so this is a
			// deployment fault arriving dressed as a caller fault.
			name: "a refused return address is our deployment", status: http.StatusBadRequest,
			body: `{"error":"redirect_url_must_be_https"}`,
			assert: func(t *testing.T, err error) {
				if !errors.Is(err, ErrOriginNotHTTPS) {
					t.Fatalf("got %v, want ErrOriginNotHTTPS", err)
				}
			},
		},
		{
			name: "a rejected key is not the caller's fault", status: http.StatusUnauthorized,
			body: `{"error":"unauthorized"}`,
			assert: func(t *testing.T, err error) {
				if !errors.Is(err, ErrUnauthorizedKey) {
					t.Fatalf("got %v, want ErrUnauthorizedKey", err)
				}
			},
		},
		{
			name: "a rate limit carries the provider's own wait", status: http.StatusTooManyRequests,
			body: `{"error":"rate_limited"}`, retryAfter: "90",
			assert: func(t *testing.T, err error) {
				var limited *RateLimitedError
				if !errors.As(err, &limited) {
					t.Fatalf("got %v, want a RateLimitedError", err)
				}
				if limited.RetryAfter != 90*time.Second {
					t.Fatalf("Retry-After is %v, want the provider's 90s", limited.RetryAfter)
				}
			},
		},
		{
			name: "a failure to send is retryable", status: http.StatusBadGateway,
			body: `{"error":"send_failed"}`,
			assert: func(t *testing.T, err error) {
				if !errors.Is(err, ErrUpstream) {
					t.Fatalf("got %v, want ErrUpstream", err)
				}
			},
		},
		{
			// An answer nobody documented must not be read as success: that is
			// how a verification nobody was asked for gets recorded.
			name: "an unrecognised answer is not success", status: http.StatusTeapot, body: `nope`,
			assert: func(t *testing.T, err error) {
				if !errors.Is(err, ErrUpstream) {
					t.Fatalf("got %v, want ErrUpstream", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := newStubProvider(t, tc.status, tc.body)
			stub.retryAfter = tc.retryAfter
			svc := &Service{http: &http.Client{Timeout: upstreamTimeout}}
			_, err := svc.requestSend(context.Background(), "user@example.com", "https://nexus.test/api/v1/verify/landed?ref=abc")
			tc.assert(t, err)
		})
	}
}

// The happy path: the request carries the address and our return URL, and the
// provider's own deadline is what comes back.
func TestASuccessfulSendCarriesTheReturnAddressAndTheProvidersExpiry(t *testing.T) {
	stub := newStubProvider(t, http.StatusOK,
		`{"ok":true,"email":"user@example.com","expires_at":"2026-08-09T12:00:00Z"}`)
	svc := &Service{http: &http.Client{Timeout: upstreamTimeout}}

	expiresAt, err := svc.requestSend(context.Background(), "user@example.com",
		"https://nexus.test/api/v1/verify/landed?ref=abc")
	if err != nil {
		t.Fatalf("requestSend: %v", err)
	}
	if !expiresAt.Equal(time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("expiry came back as %v", expiresAt)
	}

	calls, payload := stub.seen()
	if calls != 1 {
		t.Fatalf("the provider was called %d times", calls)
	}
	if payload["email"] != "user@example.com" {
		t.Errorf("the provider was asked to write to %q", payload["email"])
	}
	if payload["redirect_url"] != "https://nexus.test/api/v1/verify/landed?ref=abc" {
		t.Errorf("the return address sent was %q", payload["redirect_url"])
	}
}

// An expiry we cannot read is not worth failing a send that already happened —
// the local placeholder deadline stands.
func TestAnUnreadableExpiryDoesNotFailASendThatHappened(t *testing.T) {
	newStubProvider(t, http.StatusOK, `{"ok":true,"expires_at":"next tuesday"}`)
	svc := &Service{http: &http.Client{Timeout: upstreamTimeout}}

	expiresAt, err := svc.requestSend(context.Background(), "user@example.com", "https://nexus.test/return")
	if err != nil {
		t.Fatalf("requestSend: %v", err)
	}
	if !expiresAt.IsZero() {
		t.Fatalf("expiry came back as %v, want the zero time so the placeholder stands", expiresAt)
	}
}

// A provider that cannot be reached at all is the same class of failure as one
// that answers 502: retryable, and never silently a success.
func TestAnUnreachableProviderIsUpstream(t *testing.T) {
	t.Setenv("EMAIL_VERIFY_API_KEY", "evk_test")
	// Reserved for documentation use and therefore not routable.
	t.Setenv("EMAIL_VERIFY_BASE_URL", "http://192.0.2.1:9")
	svc := &Service{http: &http.Client{Timeout: 250 * time.Millisecond}}

	if _, err := svc.requestSend(context.Background(), "user@example.com", "https://nexus.test/return"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("got %v, want ErrUpstream", err)
	}
}

func TestHealthReportsWhatTheProviderSaid(t *testing.T) {
	stub := newStubProvider(t, http.StatusOK, `{"ok":true}`)
	svc := &Service{http: &http.Client{Timeout: upstreamTimeout}}
	if err := svc.Health(context.Background()); err != nil {
		t.Fatalf("a healthy provider reported %v", err)
	}

	stub.server.Close()
	if err := svc.Health(context.Background()); !errors.Is(err, ErrUpstream) {
		t.Fatalf("a provider that is down reported %v, want ErrUpstream", err)
	}
}

// Retry-After has to be a number the caller can obey. Zero, or a negative
// duration from a window that has already passed, is an instruction to retry
// immediately — which is what the limit exists to prevent.
func TestRetryAfterIsAlwaysWorthWaiting(t *testing.T) {
	if got := retryAfter(nil); got < time.Minute {
		t.Errorf("retryAfter(nil) = %v, want at least a minute", got)
	}
	longPast := time.Now().Add(-3 * time.Hour)
	if got := retryAfter(&longPast); got < time.Minute {
		t.Errorf("retryAfter(long past) = %v, want at least a minute", got)
	}
	justNow := time.Now()
	if got := retryAfter(&justNow); got < 50*time.Minute || got > time.Hour {
		t.Errorf("retryAfter(now) = %v, want close to the full hour", got)
	}
}

func TestResultPageSpeaksEveryPlatformLanguage(t *testing.T) {
	for _, locale := range []string{"mn", "ar", "zh", "en", "fr", "ru", "es", "ko"} {
		for _, confirmed := range []bool{true, false} {
			title, body := ResultPage(locale, confirmed)
			if title == "" || body == "" {
				t.Errorf("ResultPage(%q, %v) returned an empty page", locale, confirmed)
			}
		}
	}
	confirmedTitle, _ := ResultPage("en", true)
	spentTitle, _ := ResultPage("en", false)
	if confirmedTitle == spentTitle {
		t.Error("a confirmed link and a spent one produce the same page")
	}
}
