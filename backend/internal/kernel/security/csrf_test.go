package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFMiddleware(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://nexus.example")
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, tc := range []struct {
		name, origin, fetchSite string
		cookie                  bool
		want                    int
	}{
		{"allowed cookie request", "https://nexus.example", "same-origin", true, http.StatusNoContent},
		{"foreign cookie request", "https://evil.example", "cross-site", true, http.StatusForbidden},
		{"bearer client", "https://evil.example", "cross-site", false, http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Sec-Fetch-Site", tc.fetchSite)
			if tc.cookie {
				r.AddCookie(&http.Cookie{Name: TenantSessionCookie, Value: "token"})
			}
			w := httptest.NewRecorder()
			CSRFMiddleware(next).ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d", w.Code, tc.want)
			}
		})
	}
}

// The two gaps the earlier version left open, and the path the desktop clients
// are meant to use.
func TestCookieWritesNeedEvidenceThatAPageOfOursMadeThem(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://open.gerege.mn")
	handler := CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	withCookie := func(headers map[string]string, cookie bool) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contacts/", nil)
		if cookie {
			req.AddCookie(&http.Cookie{Name: TenantSessionCookie, Value: "t"})
		}
		for name, value := range headers {
			req.Header.Set(name, value)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	for _, tc := range []struct {
		name    string
		headers map[string]string
		cookie  bool
		want    int
	}{
		{"same-origin is our own page", map[string]string{"Sec-Fetch-Site": "same-origin"}, true, http.StatusNoContent},
		{"none is the person's own address bar", map[string]string{"Sec-Fetch-Site": "none"}, true, http.StatusNoContent},
		{"cross-site is the attack", map[string]string{"Sec-Fetch-Site": "cross-site"}, true, http.StatusForbidden},
		{"cross-site from a foreign origin", map[string]string{
			"Sec-Fetch-Site": "cross-site", "Origin": "https://phishing.example"}, true, http.StatusForbidden},

		// same-site says only "not this origin", so the allowlist decides. A
		// sibling product under gerege.mn carries an Origin that is not on it.
		{"same-site with no origin", map[string]string{"Sec-Fetch-Site": "same-site"}, true, http.StatusForbidden},
		{"same-site sibling product", map[string]string{
			"Sec-Fetch-Site": "same-site", "Origin": "https://sso.gerege.mn"}, true, http.StatusForbidden},

		// The same value is what a browser sends in development, where the
		// frontend and the API are one host on two ports. There the Origin is
		// on the list, and blocking it would break every local sign-in.
		{"same-site in development", map[string]string{
			"Sec-Fetch-Site": "same-site", "Origin": "https://open.gerege.mn"}, true, http.StatusNoContent},

		// Older clients that still send Origin.
		{"allowed origin", map[string]string{"Origin": "https://open.gerege.mn"}, true, http.StatusNoContent},
		{"foreign origin", map[string]string{"Origin": "https://phishing.example"}, true, http.StatusForbidden},

		// The hole: participate in neither signal and the check used to pass.
		{"neither header", nil, true, http.StatusForbidden},

		// A bearer client never reaches the check, because nothing about it is
		// ambient. This is how the macOS and Windows clients are meant to call.
		{"no cookie at all", map[string]string{"Authorization": "Bearer t"}, false, http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := withCookie(tc.headers, tc.cookie); got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

// Reads are never blocked: the defence is about writes made with somebody
// else's ambient authority.
func TestSafeMethodsAreNeverBlocked(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		req := httptest.NewRequest(method, "/api/v1/contacts/", nil)
		req.AddCookie(&http.Cookie{Name: TenantSessionCookie, Value: "t"})
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s was blocked: %d", method, rec.Code)
		}
	}
}
