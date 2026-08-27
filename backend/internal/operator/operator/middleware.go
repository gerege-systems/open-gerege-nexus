package operator

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
)

// sessionKey carries the signed-in operator through a request.
type sessionKey struct{}

// SessionFrom returns the operator this request belongs to. The second result
// is false on any route not behind RequireOperator.
func SessionFrom(ctx context.Context) (Session, bool) {
	sess, ok := ctx.Value(sessionKey{}).(Session)
	return sess, ok
}

// HostGate is the first thing every console request meets: the console answers
// on its own hostname and nowhere else.
//
// What it adds is that a misconfigured proxy, a hand-written location block, or
// somebody pointing the public hostname at the same upstream cannot quietly
// expose the console: those requests arrive with the wrong Host and get the
// same answer as a URL that does not exist.
//
// The address list is checked here as well, and only while the platform is
// private — see address.go for why that decision left nginx and came here.
//
// 404, not 403: a 403 would confirm that something is there. The console is not
// a locked door on a public street, it is an address that is not on the map.
//
// CONTROL_PLANE_HOST decides the behaviour:
//
//	set                        → the hostname must match, exactly
//	unset, ENVIRONMENT=production → the console is off entirely
//	unset, anywhere else       → open, so `npm run dev` on localhost works
//
// The production default is the important one. A deployment that forgets the
// variable gets no console rather than a console reachable on the hostname
// every tenant already uses.
func (c *Console) HostGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := security.ClientIP(r)
		// The same answer for both refusals, and 404 for the same reason: a 403
		// would tell somebody scanning that there is a console here and that
		// their address is the only thing between them and it.
		if !c.hostAllowed(r) || !c.addressAllowed(clientIP) {
			httpx.Error(w, http.StatusNotFound, "not found")
			return
		}
		next.ServeHTTP(w, r.WithContext(withClientIP(r.Context(), clientIP)))
	})
}

func (c *Console) hostAllowed(r *http.Request) bool {
	if c.host == "" {
		return !config.IsProduction()
	}
	return requestHost(r) == c.host
}

// Enabled reports whether this deployment has a console at all. Used by the
// route table so that the frontend's own check has something to agree with.
func (c *Console) Enabled() bool { return c.host != "" || !config.IsProduction() }

// RequireOperator resolves the console session and puts the request on the
// operator's database role.
//
// The two happen together on purpose. dbguard.AsOperator is what lets a query
// see another organisation's rows, and it is applied in exactly one place — the
// middleware that has just proved who is asking — rather than being available
// to any handler that imports dbguard.
func (c *Console) RequireOperator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, err := c.sessions.Resolve(r.Context(), TokenFromRequest(r))
		if err != nil {
			if !errors.Is(err, ErrSessionInvalid) {
				slog.Error("control plane: could not resolve the operator session", "error", err)
			}
			// The cookie is cleared as well as refused. Otherwise a browser
			// holding an expired token sends it on every request for the next
			// eight hours, and each one is a failed database lookup.
			ClearSessionCookie(w)
			httpx.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ctx := context.WithValue(r.Context(), sessionKey{}, sess)
		next.ServeHTTP(w, r.WithContext(Scoped(ctx)))
	})
}

// RequireCapability gates a route on what the operator's role may do.
func (c *Console) RequireCapability(capability Capability) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, ok := SessionFrom(r.Context())
			if !ok {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if !sess.Role.Can(capability) {
				httpx.Error(w, http.StatusForbidden, "forbidden: this role may not "+string(capability))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// StepUpRequiredCode is what the console's UI keys on to open the "enter your
// code" dialog. A message would have had to be matched by its text, in one
// language, by a client that should not be reading prose.
const StepUpRequiredCode = "step_up_required"

// RequireStepUp refuses an action unless the second factor was confirmed
// recently.
//
// CP-1 mounts it on nothing — there is no dangerous action to protect yet, the
// whole console being read-only until CP-2. It is written now because CP-2's
// first commit adds tenant suspension, and a step-up mechanism designed at the
// same time as the first thing that needs it tends to become that thing's
// special case.
func (c *Console) RequireStepUp(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := SessionFrom(r.Context())
		if !ok {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if !sess.SteppedUp(time.Now()) {
			httpx.JSON(w, http.StatusForbidden, map[string]string{
				"error": "this action needs your authenticator code again",
				"code":  StepUpRequiredCode,
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAudit holds back the response to any write that did not record one.
//
// See the note at the top of audit.go for why this is a middleware rather than
// a habit. The mechanics: the handler is given a buffer instead of the real
// response, and the buffer is only replayed if an audit row was committed for
// this request — or if the handler was refusing anyway, since a 4xx changed
// nothing and there is nothing to record.
//
// The cost is that a console response cannot stream. Nothing here streams, and
// the day something does — an export, a log tail — it belongs on a route that
// is a GET and never reaches this.
func (c *Console) RequireAudit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		ctx, ticket := withAuditTicket(r.Context())
		buffered := newBufferedWriter()
		next.ServeHTTP(buffered, r.WithContext(ctx))

		if ticket.recorded || buffered.status >= http.StatusBadRequest {
			buffered.flushTo(w)
			return
		}

		// A successful write with no audit row. The write itself may well have
		// happened — this cannot undo it — so the answer says so plainly and
		// the log carries the route, which is what somebody needs to find the
		// handler that skipped Do.
		slog.Error("control plane: a write answered successfully without an audit record",
			"method", r.Method, "path", r.URL.Path, "status", buffered.status)
		httpx.Error(w, http.StatusInternalServerError,
			"this action was not recorded in the operator audit trail and has been refused")
	})
}
