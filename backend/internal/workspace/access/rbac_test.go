package access

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
)

type fakePermissionStore map[string]bool

func (f fakePermissionStore) GetUserPermissions(context.Context, string, string) (map[string]bool, error) {
	return f, nil
}

func TestRequirePermission(t *testing.T) {
	tests := []struct {
		name   string
		claims *auth.UserClaims
		perms  fakePermissionStore
		want   int
	}{
		{"unauthenticated", nil, nil, http.StatusUnauthorized},
		{"missing permission", &auth.UserClaims{UserID: "u", WorkspaceID: "t"}, fakePermissionStore{}, http.StatusForbidden},
		{"granted through role", &auth.UserClaims{UserID: "u", WorkspaceID: "t"}, fakePermissionStore{"contacts.read": true}, http.StatusNoContent},
		{"tenant administrator bypass", &auth.UserClaims{UserID: "u", WorkspaceID: "t", IsAdmin: true}, fakePermissionStore{}, http.StatusNoContent},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.claims != nil {
				req = req.WithContext(auth.WithUserContext(req.Context(), *tc.claims))
			}
			rr := httptest.NewRecorder()
			RequirePermission(tc.perms, "contacts.read")(next).ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("status=%d want=%d", rr.Code, tc.want)
			}
		})
	}
}
