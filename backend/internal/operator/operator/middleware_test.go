package operator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// reached is a handler that answers 200 and records that it ran, so a test can
// tell "the gate refused" from "the gate let it through and the handler said
// no".
func reached(hit *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hit = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestHostGateAnswersOnTheConsolesHostnameOnly(t *testing.T) {
	service := &Console{host: "cp.nexus.gerege.mn"}

	cases := []struct {
		name   string
		host   string
		status int
	}{
		{"the console's hostname", "cp.nexus.gerege.mn", http.StatusOK},
		{"with a port", "cp.nexus.gerege.mn:443", http.StatusOK},
		{"in capitals", "CP.Nexus.Gerege.MN", http.StatusOK},
		// The one that matters. The console and the platform are the same
		// process listening on the same socket, so without this gate every
		// /api/platform/v1 route would be served to anybody who found it on the public
		// hostname.
		{"the platform's hostname", "nexus.gerege.mn", http.StatusNotFound},
		{"a look-alike", "cp.nexus.gerege.mn.attacker.example", http.StatusNotFound},
		{"nothing at all", "", http.StatusNotFound},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var hit bool
			request := httptest.NewRequest(http.MethodGet, "/api/platform/v1/tenants", nil)
			request.Host = c.host
			recorder := httptest.NewRecorder()

			service.HostGate(reached(&hit)).ServeHTTP(recorder, request)

			if recorder.Code != c.status {
				t.Fatalf("host %q answered %d, want %d", c.host, recorder.Code, c.status)
			}
			if hit != (c.status == http.StatusOK) {
				t.Fatalf("host %q reached the handler: %v", c.host, hit)
			}
		})
	}
}

// A deployment that never set CONTROL_PLANE_HOST has no console. The
// alternative reading of an unset value — "any hostname will do" — would put
// the console on the hostname every tenant already uses, which is the one place
// it must never be.
func TestHostGateIsClosedInProductionWithoutAHostname(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	service := &Console{host: ""}

	var hit bool
	request := httptest.NewRequest(http.MethodGet, "/api/platform/v1/tenants", nil)
	request.Host = "nexus.gerege.mn"
	recorder := httptest.NewRecorder()

	service.HostGate(reached(&hit)).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound || hit {
		t.Fatalf("an unconfigured production deployment served the console: %d", recorder.Code)
	}
	if service.Enabled() {
		t.Fatal("Enabled reported a console that cannot be reached")
	}
}

func TestHostGateIsOpenInDevelopmentWithoutAHostname(t *testing.T) {
	t.Setenv("ENVIRONMENT", "")
	service := &Console{host: ""}

	var hit bool
	request := httptest.NewRequest(http.MethodGet, "/api/platform/v1/tenants", nil)
	request.Host = "localhost:3000"
	recorder := httptest.NewRecorder()

	service.HostGate(reached(&hit)).ServeHTTP(recorder, request)

	if !hit {
		t.Fatalf("the console is unreachable on a development machine: %d", recorder.Code)
	}
}

func TestRequireCapabilityFollowsTheRoleTable(t *testing.T) {
	service := &Console{}
	guarded := service.RequireCapability(CapOperatorRead)

	cases := []struct {
		role   Role
		status int
	}{
		{RoleSuperadmin, http.StatusOK},
		{RoleOperator, http.StatusOK},
		{RoleAuditor, http.StatusOK},
		{RoleSupport, http.StatusForbidden},
		// A role nobody defined holds nothing. The failure mode being guarded
		// against is a `switch` with a default that lets an unknown value
		// through as though it were the most privileged one.
		{Role("root"), http.StatusForbidden},
	}

	for _, c := range cases {
		var hit bool
		request := httptest.NewRequest(http.MethodGet, "/api/platform/v1/operators", nil)
		request = request.WithContext(context.WithValue(request.Context(), sessionKey{},
			Session{Operator: Operator{ID: "1", Role: c.role}}))
		recorder := httptest.NewRecorder()

		guarded(reached(&hit)).ServeHTTP(recorder, request)

		if recorder.Code != c.status {
			t.Errorf("role %q answered %d, want %d", c.role, recorder.Code, c.status)
		}
	}
}

// Without a session at all — a route mounted outside RequireOperator by
// mistake — the capability check refuses rather than reading a zero Session as
// a role with no name and falling through.
func TestRequireCapabilityRefusesAnUnauthenticatedRequest(t *testing.T) {
	var hit bool
	recorder := httptest.NewRecorder()
	(&Console{}).RequireCapability(CapTenantRead)(reached(&hit)).
		ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/platform/v1/tenants", nil))

	if recorder.Code != http.StatusUnauthorized || hit {
		t.Fatalf("an unauthenticated request answered %d", recorder.Code)
	}
}

func TestRequireStepUpExpires(t *testing.T) {
	service := &Console{}

	cases := []struct {
		name        string
		steppedUpAt time.Time
		status      int
	}{
		{"just now", time.Now(), http.StatusOK},
		{"within the window", time.Now().Add(-StepUpWindow + time.Minute), http.StatusOK},
		{"after the window", time.Now().Add(-StepUpWindow - time.Second), http.StatusForbidden},
		{"never", time.Time{}, http.StatusForbidden},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var hit bool
			request := httptest.NewRequest(http.MethodPost, "/api/platform/v1/tenants/x/suspend", nil)
			request = request.WithContext(context.WithValue(request.Context(), sessionKey{},
				Session{Operator: Operator{ID: "1", Role: RoleSuperadmin}, SteppedUpAt: c.steppedUpAt}))
			recorder := httptest.NewRecorder()

			service.RequireStepUp(reached(&hit)).ServeHTTP(recorder, request)

			if recorder.Code != c.status {
				t.Fatalf("answered %d, want %d", recorder.Code, c.status)
			}
			if c.status == http.StatusForbidden &&
				!contains(recorder.Body.String(), StepUpRequiredCode) {
				t.Fatalf("the refusal did not carry %q, so the UI cannot ask for a code: %s",
					StepUpRequiredCode, recorder.Body.String())
			}
		})
	}
}

// The rule from §2.5, exercised: a write that answered successfully without an
// audit row does not reach the caller as a success.
func TestRequireAuditWithholdsAnUnrecordedWrite(t *testing.T) {
	service := &Console{}

	unrecorded := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "cp_session=leaked")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"done"}`))
	})

	recorder := httptest.NewRecorder()
	service.RequireAudit(unrecorded).ServeHTTP(recorder,
		httptest.NewRequest(http.MethodPost, "/api/platform/v1/tenants/x/suspend", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("an unrecorded write answered %d, want 500", recorder.Code)
	}
	if body := recorder.Body.String(); contains(body, "done") {
		t.Fatalf("the handler's body reached the caller: %s", body)
	}
	// The buffer is discarded whole, headers included — otherwise a handler
	// that set a session cookie would have handed one out for an action the
	// caller was told had failed.
	if recorder.Header().Get("Set-Cookie") != "" {
		t.Fatal("a header from the withheld response reached the caller")
	}
}

func TestRequireAuditReleasesARecordedWrite(t *testing.T) {
	service := &Console{}

	recorded := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// What Do stamps after it commits.
		if ticket := ticketFrom(r.Context()); ticket != nil {
			ticket.recorded = true
		}
		w.Header().Set("Set-Cookie", "cp_session=fresh")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"done"}`))
	})

	recorder := httptest.NewRecorder()
	service.RequireAudit(recorded).ServeHTTP(recorder,
		httptest.NewRequest(http.MethodPost, "/api/platform/v1/session", nil))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("a recorded write answered %d, want 201", recorder.Code)
	}
	if !contains(recorder.Body.String(), "done") {
		t.Fatalf("the handler's body was lost: %s", recorder.Body.String())
	}
	// Sign-in depends on this: its Set-Cookie is written into the buffer.
	if recorder.Header().Get("Set-Cookie") != "cp_session=fresh" {
		t.Fatal("the response's cookie was lost in the buffer")
	}
}

// A refusal changed nothing, so there is nothing to record and nothing to
// withhold. Without this, every 400 and 401 from a console write would be
// rewritten into a 500 and the reason the caller was refused would be lost.
func TestRequireAuditLetsRefusalsThrough(t *testing.T) {
	service := &Console{}

	refusing := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"nope"}`))
	})

	recorder := httptest.NewRecorder()
	service.RequireAudit(refusing).ServeHTTP(recorder,
		httptest.NewRequest(http.MethodPost, "/api/platform/v1/session", nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("a refusal answered %d, want 401", recorder.Code)
	}
	if !contains(recorder.Body.String(), "nope") {
		t.Fatalf("the refusal's reason was lost: %s", recorder.Body.String())
	}
}

func TestRequireAuditDoesNotBufferReads(t *testing.T) {
	service := &Console{}

	var hit bool
	recorder := httptest.NewRecorder()
	service.RequireAudit(reached(&hit)).ServeHTTP(recorder,
		httptest.NewRequest(http.MethodGet, "/api/platform/v1/tenants", nil))

	if !hit || recorder.Code != http.StatusOK {
		t.Fatalf("a read was interfered with: %d", recorder.Code)
	}
}

// Do refuses to perform a write that nobody has given a reason for, before it
// opens a transaction — so the check cannot be reached with a half-applied
// change behind it.
func TestDoRequiresAReason(t *testing.T) {
	service := &Console{}
	var ran bool
	err := service.Do(context.Background(), Session{Operator: Operator{ID: "1"}},
		Change{Action: "tenant.suspend", TargetType: "tenant", TargetID: "x"},
		func(context.Context, pgx.Tx) error {
			ran = true
			return nil
		})

	if err == nil {
		t.Fatal("a write with no reason was allowed")
	}
	if ran {
		t.Fatal("the write ran before the reason was checked")
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
