package eidrp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// What the person block means, asked of the decoder rather than of eID.
//
// The three names are the part worth pinning: firstName is the citizen's own
// name and lastName is the patronymic, and a decoder that swapped them would
// produce something that reads correctly in English and is visibly wrong to
// anybody Mongolian.

func serve(t *testing.T, path string, answer any) Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("asked for %s, expected %s", r.URL.Path, path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer rp_sk_test" {
			t.Errorf("authorization header is %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(answer)
	}))
	t.Cleanup(server.Close)
	return NewClient(server.URL, "rp-uuid", "Gerege Nexus", "rp_sk_test", "")
}

func TestACompletedSessionCarriesTheWholePersonBlock(t *testing.T) {
	client := serve(t, "/session/s-1", map[string]any{
		"state":  "COMPLETE",
		"result": map[string]any{"endResult": "OK", "documentNumber": "DOC-1"},
		"cert":   map[string]any{"certificateLevel": "ADVANCED"},
		"person": map[string]any{
			"firstName": "Эрдэнэбат", "lastName": "Цэнддорж", "familyName": "Харчин",
			"firstNameEn": "ERDENEBAT", "lastNameEn": "TSENDDORJ",
			"civilId": "111949212017", "regNo": "МА74101813",
			"birthDate": "1974-10-18", "gender": "M", "geID": 10000263,
		},
	})

	result, err := client.Session(context.Background(), "s-1", 25000)
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	if result.State != StateComplete || result.Identity == nil {
		t.Fatalf("state is %q with identity %v", result.State, result.Identity)
	}
	id := result.Identity
	switch {
	case id.FirstName != "Эрдэнэбат":
		t.Errorf("FirstName is %q — the citizen's own name is firstName", id.FirstName)
	case id.LastName != "Цэнддорж":
		t.Errorf("LastName is %q — the patronymic is lastName", id.LastName)
	case id.FamilyName != "Харчин":
		t.Errorf("FamilyName is %q — the clan name is familyName", id.FamilyName)
	case id.FirstNameEn != "ERDENEBAT" || id.LastNameEn != "TSENDDORJ":
		t.Errorf("the Latin names are %q / %q", id.FirstNameEn, id.LastNameEn)
	case id.BirthDate != "1974-10-18":
		t.Errorf("BirthDate is %q — a date, with no clock", id.BirthDate)
	case id.Gender != "M":
		t.Errorf("Gender is %q", id.Gender)
	case id.GeID != 10000263:
		t.Errorf("GeID is %d", id.GeID)
	case id.CivilID != "111949212017" || id.NationalID != "МА74101813":
		t.Errorf("the identifiers are %q / %q", id.CivilID, id.NationalID)
	case id.DocumentNumber != "DOC-1" || id.KYCLevel != "ADVANCED":
		t.Errorf("document %q, level %q", id.DocumentNumber, id.KYCLevel)
	}
}

// A deployment pointed at an eID that has not been upgraded yet still knows
// the citizen's name. Losing every name on the day one side moves would be a
// worse failure than the naming confusion this replaced.
func TestTheOlderPersonBlockStillYieldsAName(t *testing.T) {
	client := serve(t, "/session/s-2", map[string]any{
		"state":  "COMPLETE",
		"result": map[string]any{"endResult": "OK"},
		"person": map[string]any{
			"givenName": "Эрдэнэбат", "surname": "Цэнддорж",
			"civilId": "111949212017", "regNo": "МА74101813",
		},
	})
	result, err := client.Session(context.Background(), "s-2", 25000)
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	if result.Identity.FirstName != "Эрдэнэбат" || result.Identity.LastName != "Цэнддорж" {
		t.Fatalf("the older block was not read: %+v", result.Identity)
	}
	// And nothing is invented for the fields it does not carry.
	if result.Identity.GeID != 0 || result.Identity.BirthDate != "" {
		t.Errorf("fields the older block does not carry were filled: %+v", result.Identity)
	}
}

// COMPLETE says the session ended. Whether the citizen approved is endResult's
// to say, and reading only the state would sign in somebody who refused.
func TestTheTerminalResultDecidesTheState(t *testing.T) {
	for endResult, want := range map[string]string{
		"OK":                     StateComplete,
		"TIMEOUT":                StateExpired,
		"USER_REFUSED":           StateRefused,
		"USER_REFUSED_VC_CHOICE": StateRefused,
		"WRONG_VC":               StateRefused,
		"DOCUMENT_UNUSABLE":      StateRefused,
	} {
		client := serve(t, "/session/s-3", map[string]any{
			"state":  "COMPLETE",
			"result": map[string]any{"endResult": endResult},
			"person": map[string]any{"firstName": "Иргэн", "regNo": "АА00112233"},
		})
		result, err := client.Session(context.Background(), "s-3", 1000)
		if err != nil {
			t.Fatalf("%s: %v", endResult, err)
		}
		if result.State != want {
			t.Errorf("endResult %s became %s, want %s", endResult, result.State, want)
		}
	}
}

// A session eID no longer holds is over, not broken. Reported as a failure it
// became a 502 on the citizen's sign-in card, once per check.
func TestSessionTreatsNotFoundAsExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	client := NewClient(server.URL, "rp-uuid", "", "rp_sk_test", "")

	result, err := client.Session(context.Background(), "s-gone", 1000)
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	if result.State != StateExpired {
		t.Fatalf("state is %s, want %s", result.State, StateExpired)
	}
}

// A citizen who represents nobody is an answer, not a failure.
func TestRepresentationsTreatsNotFoundAsNone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	client := NewClient(server.URL, "rp-uuid", "", "rp_sk_test", "")

	reps, err := client.Representations(context.Background(), "PNOMN-111949212017")
	if err != nil {
		t.Fatalf("representations: %v", err)
	}
	if len(reps) != 0 {
		t.Fatalf("expected none, got %d", len(reps))
	}
}

// The initiate body, which is the part that fails quietly.
//
// Authentication's challenge field is `rpChallenge`. Sending signing's
// `digest`/`hashType` instead leaves the server with an empty challenge, and
// what the citizen then sees is their approval failing inside the eID app with
// a message about processing — nowhere near the mistake.
func TestTheInitiateBodyCarriesTheAuthenticationChallenge(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		if r.URL.Path != "/authentication/notification/etsi/PNOMN-111949212017" {
			t.Errorf("initiate went to %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sessionID": "s-9",
			"vc":        map[string]any{"type": "alphaNumeric4", "value": "0489"},
		})
	}))
	t.Cleanup(server.Close)
	client := NewClient(server.URL, "rp-uuid", "Gerege Nexus", "rp_sk_test", "")

	started, err := client.Initiate(context.Background(), "111949212017", "Нэвтрэх", "gerege-nexus://auth")
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if started.SessionID != "s-9" || started.VerificationCode != "0489" {
		t.Fatalf("the answer was not read: %+v", started)
	}
	if challenge, _ := body["rpChallenge"].(string); challenge == "" {
		t.Error("rpChallenge is empty; the citizen's approval would fail inside the app")
	}
	if _, wrong := body["digest"]; wrong {
		t.Error("the signing challenge was sent to an authentication endpoint")
	}
	if body["signatureProtocol"] != "ACSP_V2" || body["certificateLevel"] != "ADVANCED" {
		t.Errorf("protocol %v, level %v", body["signatureProtocol"], body["certificateLevel"])
	}
	if body["initialCallbackUrl"] != "gerege-nexus://auth" {
		t.Errorf("native callback was not passed to eID: %v", body["initialCallbackUrl"])
	}
	// The text the eID app shows, capped at sixty characters by the protocol.
	interactions, _ := body["interactions"].([]any)
	if len(interactions) != 1 {
		t.Fatalf("interactions: %v", body["interactions"])
	}
	first, _ := interactions[0].(map[string]any)
	if first["type"] != "displayTextAndPIN" || first["displayText60"] != "Нэвтрэх" {
		t.Errorf("the approval screen would show %v", first)
	}
}
