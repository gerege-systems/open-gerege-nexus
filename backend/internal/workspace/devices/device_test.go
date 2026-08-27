package devices

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnrollmentCodeIsUsableAndStoredAsDigest(t *testing.T) {
	code, err := enrollmentCode()
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 19 || strings.Count(code, "-") != 3 {
		t.Fatalf("unexpected display code %q", code)
	}
	if secretHash(code) != secretHash(strings.ReplaceAll(strings.ToLower(code), "-", "")) {
		t.Fatal("display formatting changed the digest")
	}
	if strings.Contains(secretHash(code), code) || len(secretHash(code)) != 64 {
		t.Fatal("code was not SHA-256 digested")
	}
}

func TestDeviceKindsAreClosedAllowlists(t *testing.T) {
	for _, test := range []struct {
		platform, factor string
		want             bool
	}{
		{"windows", "kiosk", true}, {"android", "pos", true}, {"ios", "tablet", true},
		{"linux", "desktop", false}, {"android", "terminal", false}, {"", "pos", false},
	} {
		if got := validDeviceKind(test.platform, test.factor); got != test.want {
			t.Errorf("validDeviceKind(%q,%q)=%v", test.platform, test.factor, got)
		}
	}
}

func TestDeviceAuthorizationScheme(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/devices/me", nil)
	r.Header.Set("Authorization", "Device secret-token")
	if got := deviceTokenFromRequest(r); got != "secret-token" {
		t.Fatalf("got %q", got)
	}
	r.Header.Set("Authorization", "Bearer secret-token")
	if got := deviceTokenFromRequest(r); got != "" {
		t.Fatalf("accepted bearer token as device token: %q", got)
	}
}
