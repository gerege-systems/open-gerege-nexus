package identity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/eid"
)

// eID is offered on the sign-in screens only when this deployment holds
// relying-party credentials. Without them every start answers 502, so the card
// would be an offer the server cannot keep.
func TestSSOConfigOffersEIDOnlyWithRelyingPartyCredentials(t *testing.T) {
	cases := []struct {
		name         string
		uuid, secret string
		want         bool
	}{
		{"none", "", "", false},
		{"uuid only", "rp-uuid", "", false},
		{"secret only", "", "rp-secret", false},
		{"both", "rp-uuid", "rp-secret", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("EID_MOCK_MODE", "false")
			t.Setenv("EID_RP_UUID", c.uuid)
			t.Setenv("EID_RP_SECRET", c.secret)
			server := New(Deps{EID: eid.NewEIDService()})

			rec := httptest.NewRecorder()
			server.HandleSSOConfig(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/config", nil))
			var body struct {
				EID struct {
					Enabled bool `json:"enabled"`
				} `json:"eid"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.EID.Enabled != c.want {
				t.Errorf("eid.enabled = %v, want %v", body.EID.Enabled, c.want)
			}
		})
	}
}

// A deployment built without an eID service at all never offers it.
func TestSSOConfigWithoutAnEIDService(t *testing.T) {
	rec := httptest.NewRecorder()
	New(Deps{}).HandleSSOConfig(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/config", nil))
	var body struct {
		EID struct {
			Enabled bool `json:"enabled"`
		} `json:"eid"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.EID.Enabled {
		t.Error("eID offered with no eID service")
	}
}
