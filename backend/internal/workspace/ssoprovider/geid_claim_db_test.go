package ssoprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// A citizen who signed in with eID reaches a relying party with two things:
// the address this platform gave them, and the Gerege number it was derived
// from. Both, because a relying party that had only the address would be
// parsing an identifier out of a string, and would go on doing it after the
// address form changes.

func TestBothTheAddressAndTheGeregeNumberReachTheRelyingParty(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	const geID = int64(910000042)
	if _, err := f.pool.Exec(ctx,
		`UPDATE registry.users SET ge_id = $2 WHERE id = $1::uuid`, f.userID, geID); err != nil {
		t.Fatalf("give the account a Gerege number: %v", err)
	}

	accessToken, idToken := exchangeForTokens(t, f)

	claims := verifyIDToken(t, f, idToken)
	if claims["email"] != f.email {
		t.Errorf("email claim is %v, want %s", claims["email"], f.email)
	}
	// JSON has one number type, so the claim arrives as a float64. What is
	// being checked is the value, not the encoding.
	if got, ok := claims["ge_id"].(float64); !ok || int64(got) != geID {
		t.Errorf("ge_id claim in the id_token is %v, want %d", claims["ge_id"], geID)
	}

	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	f.provider.HandleUserInfo(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("userinfo returned %d: %s", rec.Code, rec.Body.String())
	}
	var info map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &info)
	if info["email"] != f.email {
		t.Errorf("userinfo email is %v", info["email"])
	}
	if got, ok := info["ge_id"].(float64); !ok || int64(got) != geID {
		t.Errorf("ge_id claim at userinfo is %v, want %d", info["ge_id"], geID)
	}
}

// An account with no Gerege number — everybody who signed in with a password —
// carries no claim at all. Nought is a number, and a relying party must not
// have to tell it apart from "none".
func TestAnAccountWithoutAGeregeNumberCarriesNoClaim(t *testing.T) {
	f := newFixture(t)
	_, idToken := exchangeForTokens(t, f)
	claims := verifyIDToken(t, f, idToken)
	if _, present := claims["ge_id"]; present {
		t.Errorf("ge_id was sent for an account that has none: %v", claims["ge_id"])
	}
}

// exchangeForTokens runs the flow up to a pair of tokens. The end-to-end test
// beside this one does the same thing inline; this is the two lines of it that
// a claim test needs.
func exchangeForTokens(t *testing.T, f *fixture, scope ...string) (accessToken, idToken string) {
	t.Helper()
	status, body := f.token(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {f.codeFromRedirect(t, scope...)},
		"redirect_uri":  {f.client.RedirectURIs[0]},
		"code_verifier": {f.verifier},
	})
	if status != http.StatusOK {
		t.Fatalf("token exchange failed with %d: %v", status, body)
	}
	accessToken, _ = body["access_token"].(string)
	idToken, _ = body["id_token"].(string)
	if accessToken == "" || idToken == "" {
		t.Fatalf("the exchange returned no tokens: %v", body)
	}
	return accessToken, idToken
}
