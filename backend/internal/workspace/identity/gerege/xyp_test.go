package gerege_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/gerege"
)

func TestGeregeMockCitizenQuery(t *testing.T) {
	svc := gerege.NewGeregeService()

	info, err := svc.GetCitizenInfo(context.Background(), "AA90010111")
	if err != nil {
		t.Fatalf("unexpected error during mock Gerege citizen query: %v", err)
	}

	if info.RegNumber != "AA90010111" {
		t.Errorf("expected RegNumber AA90010111, got %s", info.RegNumber)
	}
	if !info.Verified {
		t.Errorf("expected verified = true")
	}
}

func TestGeregeMockCompanyQuery(t *testing.T) {
	svc := gerege.NewGeregeService()

	company, err := svc.GetCompanyInfo(context.Background(), "5589412")
	if err != nil {
		t.Fatalf("unexpected error during mock Gerege company query: %v", err)
	}

	if company.Name != "Гэрэгэ Системс ХХК" {
		t.Errorf("unexpected company name: %s", company.Name)
	}
	if !company.VatPayer {
		t.Errorf("expected vat_payer = true")
	}
}

// The live path, against a stub standing in for the gateway.
//
// What is worth pinning is the contract this had to be written against and
// cannot infer: the path is /v1/org/lookup on an origin, the credential is
// HTTP Basic, and `found: false` is a 200 rather than an error status.
func TestTheCompanyLookupSpeaksTheGatewaysContract(t *testing.T) {
	var gotPath, gotUser, gotSecret, gotBody string
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotUser, gotSecret, _ = r.BasicAuth()
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		_, _ = w.Write([]byte(`{"found":true,"organization":{"reg_no":"6235972",
			"name":"Гэрэгэ системс","type":"Хязгаарлагдмал хариуцлагатай компани",
			"ceo":"Нацагдорж Энхжаргал","address":"Улаанбаатар, Сүхбаатар"}}`))
	}))
	defer registry.Close()

	t.Setenv("XYP_ENDPOINT", registry.URL)
	t.Setenv("XYP_MOCK_MODE", "false")
	t.Setenv("XYP_CLIENT_ID", "vfy_test")
	t.Setenv("XYP_CLIENT_SECRET", "shhh")

	service := gerege.NewGeregeService()
	if mode := service.Mode(); mode != "live" {
		t.Fatalf("the mode is %q, want live", mode)
	}

	company, err := service.GetCompanyInfo(context.Background(), "6235972")
	if err != nil {
		t.Fatalf("the lookup failed: %v", err)
	}
	if gotPath != "/v1/org/lookup" {
		t.Errorf("it asked %q, want /v1/org/lookup", gotPath)
	}
	if gotUser != "vfy_test" || gotSecret != "shhh" {
		t.Errorf("the credential did not arrive: %q / %q", gotUser, gotSecret)
	}
	if !strings.Contains(gotBody, `"6235972"`) {
		t.Errorf("the registration number was not sent: %s", gotBody)
	}
	if company.Name != "Гэрэгэ системс" || company.Executive != "Нацагдорж Энхжаргал" {
		t.Errorf("the reply was mapped wrong: %+v", company)
	}
	// Not carried by the gateway, so it must stay empty rather than read as
	// "this company is not registered for VAT".
	if company.VatPayer {
		t.Errorf("VatPayer was invented")
	}
}

func TestAnUnregisteredCompanyIsAnError(t *testing.T) {
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"found":false}`))
	}))
	defer registry.Close()

	t.Setenv("XYP_ENDPOINT", registry.URL)
	t.Setenv("XYP_MOCK_MODE", "false")
	t.Setenv("XYP_CLIENT_ID", "vfy_test")
	t.Setenv("XYP_CLIENT_SECRET", "shhh")

	if _, err := gerege.NewGeregeService().GetCompanyInfo(context.Background(), "0000000"); err == nil {
		t.Fatal("an unregistered company came back without an error")
	}
}

func TestWithoutACredentialTheLookupSaysSo(t *testing.T) {
	t.Setenv("XYP_MOCK_MODE", "false")
	t.Setenv("XYP_CLIENT_ID", "")
	t.Setenv("XYP_CLIENT_SECRET", "")

	service := gerege.NewGeregeService()
	if mode := service.Mode(); mode != "unconfigured" {
		t.Fatalf("the mode is %q, want unconfigured", mode)
	}
	_, err := service.GetCompanyInfo(context.Background(), "6235972")
	if err == nil || !strings.Contains(err.Error(), "XYP_CLIENT_ID") {
		t.Fatalf("the error does not name what is missing: %v", err)
	}
}
