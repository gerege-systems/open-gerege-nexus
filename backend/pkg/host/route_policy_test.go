package host

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/cache"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Which routes a stranger may reach.
//
// Every module is handed the root router — RegisterRoutes takes chi.Router, not
// a pre-gated group — so mounting a path outside the tenant gate is one line
// and looks exactly like mounting one inside it. That is deliberate: the
// registry module the App Store is becoming has to serve a public catalogue,
// and a platform that could not express "public" would have to run it as a
// second service, which is the arrangement being undone.
//
// The cost of allowing it is that a private route can become public by
// accident, and nothing about the diff would say so. This list is what says so:
// a route reachable without a session must be named here, and adding a name is
// a visible act in a review rather than a side effect of where a line was put.
//
// The entries are exact paths or prefixes ending in "/*". They are matched
// against chi's pattern, not against a request, so "/api/v1/registry/*" admits
// everything the registry module mounts under that prefix and nothing else.
var publicRoutes = []string{
	// Liveness and readiness. An orchestrator cannot hold a session.
	"/health",
	"/ready",
	"/metrics",

	// OIDC discovery and the keys that verify what this issuer signs. Public by
	// specification: a relying party reads them before anybody has signed in.
	"/.well-known/openid-configuration",
	"/.well-known/jwks.json",

	// The OAuth2 endpoints. Authorization and consent authenticate the end user
	// themselves; the token, introspection and revocation endpoints
	// authenticate the *client*, by secret or by PKCE, which is not a session
	// and does not come through authMiddleware.
	"/oauth2/auth",
	"/oauth2/token",
	"/oauth2/introspect",
	"/oauth2/revoke",
	"/oauth2/userinfo",
	// RP-initiated logout. It reads the session cookie itself and ends the
	// session it names; requiring one would mean a relying party could not send
	// somebody here to sign out unless they still had a live session, which is
	// the one case where it does not matter.
	"/oauth2/logout",

	// The first-run wizard. Nobody can hold a session on a deployment with no
	// organisation, which is the only state in which any of these answer at
	// all: /status reports the one bit somebody would learn by trying to sign
	// in, and the ones that act carry the setup token instead — 256 bits,
	// minted in memory at boot, written once to the log, dropped the moment an
	// organisation exists. Unarmed, they are 404 rather than 401, so a stranger
	// is not told there is a token to guess. See internal/operator/setup.
	"/api/v1/setup/status",
	"/api/v1/setup/organisation",
	// The console's first operator, on the same token and only while this
	// deployment has none: see internal/operator/setup.
	"/api/v1/setup/operator",
	"/api/v1/setup/operator/confirm",
	"/api/v1/setup/complete",

	// Signing in, and the identity flows that precede a session by definition.
	"/api/v1/auth/login",
	"/api/v1/auth/logout",
	"/api/v1/auth/eid/login",
	"/api/v1/auth/eid/start",
	"/api/v1/auth/eid/start-id",
	"/api/v1/auth/eid/poll",
	"/api/v1/auth/dan/login",
	// Choosing a password from an invitation or a reset, and stepping into an
	// organisation as an operator. Both journeys begin in the control plane,
	// on its own hostname, and finish here — where the
	// person has no session yet and the operator's console session means
	// nothing. What authorises each is a single-use token: 256 bits, stored as
	// a digest, claimed with a conditional UPDATE, and short-lived (an hour
	// for a handover, a day for a password link). See access_recovery.go.
	"/api/v1/auth/credential",
	"/api/v1/auth/credential/redeem",
	"/api/v1/auth/impersonation/redeem",

	// Signing in through the provider this deployment is a client of, when it
	// is one. Each carries its own authority rather than a session: the config
	// endpoint says nothing secret — whether sign-in is federated is visible
	// from the redirect anyway, and the login screen has to know before it can
	// render — the start endpoint mints the state it later requires, and the
	// callback is answerable only to a browser holding the cookie that start
	// set. See sso_client_handlers.go.
	// The display name of the client behind an authorization request. Read by
	// the sign-in screen before anybody is signed in, which is the whole point;
	// it discloses a registered name to somebody who already holds the
	// client_id, which travels in every authorization URL and is not a secret.
	"/api/v1/oauth2/client-info",

	"/api/v1/auth/sso/config",
	"/api/v1/auth/sso/start",
	"/api/v1/auth/sso/callback",
	// Google sign-in, when this deployment offers it. Unauthenticated for
	// exactly the reasons the pair above are: a person signing in has no
	// session yet, and the state cookie is what makes the callback answerable.
	"/api/v1/auth/google/start",
	"/api/v1/auth/google/callback",
	// Finishing a first external sign-in with eID. No session exists yet by
	// definition; the binding token is what carries the authority, and the
	// account it will create does not exist until eID has answered.
	"/api/v1/auth/bind/session",
	"/api/v1/auth/bind/consent",
	"/api/v1/auth/bind/eid/start",
	"/api/v1/auth/bind/eid/poll",

	// The App Store's public routes used to be listed here — the signed
	// catalogue and the keys that verify it. They left with the product
	// (github.com/gerege-systems/appstore-gerege-nexus), and a name on this
	// list is a permission: leaving "/api/v1/registry/*" behind would quietly
	// bless the next core route that happened to be mounted under it.
	//
	// The distribution needs this guard of its own. It has public routes and
	// nothing there is checking them.

	// A landing reached by somebody who is not signed in and may hold no
	// account here at all: a single-use reference in the query is the whole
	// authority — see handleVerifyLanded.
	//
	// /api/v1/integrations/oauth/callback was the other one. It left with the
	// connectors (client-gerege-nexus, modules/integrations), and the name left
	// with it: a name on this list is a permission, and the distribution that
	// serves that route needs this guard of its own.
	"/api/v1/verify/landed",

	// The Өртөө channel's four public routes were here — the redemption, the
	// two exchange endpoints and the installation's own public key. They left
	// with the channel (client-gerege-nexus, modules/urtuu/channel), and the
	// names left with them for the reason the App Store's did above: a name on
	// this list is a permission, and one left behind blesses whatever the next
	// change happens to mount under it. That distribution needs this guard of
	// its own.
}

// isPublic reports whether a chi pattern is on the list.
func isPublic(pattern string) bool {
	for _, allowed := range publicRoutes {
		if allowed == pattern {
			return true
		}
		if prefix, wildcard := strings.CutSuffix(allowed, "/*"); wildcard {
			if pattern == prefix || strings.HasPrefix(pattern, prefix+"/") {
				return true
			}
		}
	}
	return false
}

// walkRoutes lists every (method, pattern) the router serves.
func walkRoutes(t *testing.T, router chi.Routes) map[string][]string {
	t.Helper()
	found := map[string][]string{}
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		// chi reports a mounted subtree's pattern with a trailing /* on the
		// mount point; the leaf patterns come through separately.
		route = strings.TrimSuffix(route, "/*")
		if route == "" {
			route = "/"
		}
		found[route] = append(found[route], method)
		return nil
	})
	if err != nil {
		t.Fatalf("walk routes: %v", err)
	}
	return found
}

// A route not on the list must not be reachable without a session.
//
// This is asserted by making the request rather than by inspecting middleware:
// what matters is what a stranger gets, and a chain that looks right while
// answering 200 is exactly the failure worth catching. 401 or 403 both count —
// the first is "who are you", the second is "not you", and neither served
// anything.
//
// It runs against a real schema, so a handler that gets past the gate really
// runs and really answers — which is what makes a 200 here mean what it says.
func TestEveryRouteIsGatedUnlessItIsOnThePublicList(t *testing.T) {
	router := routerUnderTest(t)

	for pattern, methods := range walkRoutes(t, router) {
		if isPublic(pattern) {
			continue
		}
		for _, method := range methods {
			// A pattern with parameters needs a concrete path to be routed.
			target := concreteFor(pattern)
			req, err := http.NewRequest(method, target, strings.NewReader("{}"))
			if err != nil {
				t.Fatalf("%s %s: %v", method, target, err)
			}
			req.Header.Set("Content-Type", "application/json")

			rec := newRecorder()
			func() {
				// A handler reached without a database panics; the Recoverer
				// middleware turns that into a 500, which is still a refusal to
				// serve. Guarding here keeps a panic from ending the whole test.
				defer func() { _ = recover() }()
				router.(http.Handler).ServeHTTP(rec, req)
			}()

			if rec.Code == http.StatusOK || rec.Code == http.StatusCreated {
				t.Errorf("%s %s answered %d without a session; add it to publicRoutes if that is intended",
					method, pattern, rec.Code)
			}
		}
	}
}

// Nothing on the list may be there without still existing. A public route that
// has been renamed or removed leaves an entry that quietly widens the next
// route to take its name.
func TestThePublicListHasNoStaleEntries(t *testing.T) {
	served := walkRoutes(t, routerUnderTest(t))
	for _, allowed := range publicRoutes {
		if prefix, wildcard := strings.CutSuffix(allowed, "/*"); wildcard {
			matched := false
			for pattern := range served {
				if pattern == prefix || strings.HasPrefix(pattern, prefix+"/") {
					matched = true
					break
				}
			}
			if !matched {
				t.Errorf("publicRoutes carries %q but nothing is mounted under it", allowed)
			}
			continue
		}
		if _, ok := served[allowed]; !ok {
			t.Errorf("publicRoutes carries %q but the router does not serve it", allowed)
		}
	}
}

// routerUnderTest builds the real router, against the real schema.
//
// A stub router would assert only that this test's own wiring is gated. What is
// being checked is the routing table the process actually serves, so it is
// built by the same constructor the process uses.
//
//	AUTH_TEST_DATABASE_URL=postgres://... go test ./internal/operator/...
func routerUnderTest(t *testing.T) chi.Routes {
	t.Helper()
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to a migrated test database to run the route policy tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	// The bundled catalogue, so the module routes under test are the ones this
	// build ships rather than whatever a registry is serving today.
	t.Setenv("APP_CATALOG_URL", "")
	// No Redis: the bus degrades to in-process, which is all a routing table
	// needs — nothing here crosses replicas.
	server, err := newServer(pool, filepath.FromSlash("../../../catalog/apps.json"),
		cache.NewBus(context.Background(), nil))
	if err != nil {
		t.Fatalf("build the server: %v", err)
	}
	return server.router
}

// concreteFor turns a chi pattern into a path that will route: every {param}
// becomes a value that is syntactically plausible for the ones that are parsed.
var routeParam = regexp.MustCompile(`\{[^}]+\}`)

func concreteFor(pattern string) string {
	return routeParam.ReplaceAllStringFunc(pattern, func(param string) string {
		if strings.Contains(param, "id") || strings.Contains(param, "ID") {
			// A UUID, because a handler that parses one before checking the
			// session would otherwise answer 400 and hide what this asserts.
			return "00000000-0000-0000-0000-000000000000"
		}
		return "probe"
	})
}

func newRecorder() *httptest.ResponseRecorder { return httptest.NewRecorder() }
