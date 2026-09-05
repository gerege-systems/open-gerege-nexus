package identity

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/eid"
)

func TestEIDStartAcceptsOptionalBody(t *testing.T) {
	t.Setenv("EID_MOCK_MODE", "true")
	h := &Handlers{eidSvc: eid.NewEIDService()}
	for _, body := range []string{"", "{}", "{} \n"} {
		r := httptest.NewRequest(http.MethodPost, "/auth/eid/start", strings.NewReader(body))
		w := httptest.NewRecorder()
		h.HandleEIDStart(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("body %q: got status %d, want 200", body, w.Code)
		}
	}
}

func TestEIDStartRejectsInvalidBodyBeforeStartingSession(t *testing.T) {
	for _, body := range []string{
		`{"callbackUrl":`,
		`{"callbackUrl":123}`,
		`{} {}`,
		`{}` + strings.Repeat(" ", 8<<10),
	} {
		r := httptest.NewRequest(http.MethodPost, "/auth/eid/start", strings.NewReader(body))
		w := httptest.NewRecorder()
		// No service is needed: invalid input must never initiate a session.
		(&Handlers{}).HandleEIDStart(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got status %d, want 400", w.Code)
		}
	}
}
