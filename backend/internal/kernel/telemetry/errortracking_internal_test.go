package telemetry

import (
	"testing"

	"github.com/getsentry/sentry-go"
)

// An error tracker receives a copy of whatever was in flight when something
// broke. On this platform that is national identifiers, session cookies and
// bearer tokens, so what the scrubber removes is a security control rather than
// tidiness — hence a test rather than a comment.
func TestScrubEventRemovesCredentialsAndPII(t *testing.T) {
	event := sentry.NewEvent()
	event.Request = &sentry.Request{
		URL:         "https://open.gerege.mn/api/v1/verify/landed",
		QueryString: "ref=single-use-secret-reference",
		Cookies:     "gerege_session=abc123",
		Data:        `{"password":"hunter2","reg_number":"AA90010111"}`,
		Headers: map[string]string{
			"Authorization":   "Bearer a-real-token",
			"Cookie":          "gerege_session=abc123",
			"X-Request-Id":    "req-9",
			"User-Agent":      "Mozilla/5.0",
			"Accept-Language": "mn",
			"X-Forwarded-For": "203.0.113.9",
		},
	}
	event.User = sentry.User{
		ID:        "11111111-1111-1111-1111-111111111111",
		Email:     "bat@example.mn",
		Username:  "bat",
		IPAddress: "203.0.113.9",
	}

	scrubbed := scrubEvent(event, nil)

	if scrubbed.Request.QueryString != "" {
		t.Error("the query string survived; it is where single-use references live")
	}
	if scrubbed.Request.Cookies != "" {
		t.Error("cookies survived; that is a live session")
	}
	if scrubbed.Request.Data != "" {
		t.Error("the request body survived")
	}
	for _, forbidden := range []string{"Authorization", "Cookie", "X-Forwarded-For"} {
		if _, present := scrubbed.Request.Headers[forbidden]; present {
			t.Errorf("%s survived the header allow-list", forbidden)
		}
	}
	for _, kept := range []string{"X-Request-Id", "User-Agent", "Accept-Language"} {
		if _, present := scrubbed.Request.Headers[kept]; !present {
			t.Errorf("%s was dropped; it identifies the request without identifying a person", kept)
		}
	}
	if scrubbed.User.Email != "" || scrubbed.User.Username != "" || scrubbed.User.IPAddress != "" {
		t.Errorf("the person survived on the event: %+v", scrubbed.User)
	}
	// The tenant stays: it is what turns a list of events into "this affected
	// four organisations" without naming anybody.
	if scrubbed.User.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("the tenant id was lost: %q", scrubbed.User.ID)
	}
}

// The allow-list has to be closed: a header added by a future proxy must not
// arrive at a third-party service because nobody thought to exclude it.
func TestAllowedHeadersIsAnAllowList(t *testing.T) {
	kept := allowedHeaders(map[string]string{
		"X-Some-Header-Invented-Later": "value",
		"x-request-id":                 "req-1",
	})
	if len(kept) != 1 {
		t.Fatalf("expected only the request id, got %v", kept)
	}
	// Canonicalised, so a lower-cased header name is not a way past the list.
	if kept["X-Request-Id"] != "req-1" {
		t.Errorf("a lower-cased header name was not matched: %v", kept)
	}
}
