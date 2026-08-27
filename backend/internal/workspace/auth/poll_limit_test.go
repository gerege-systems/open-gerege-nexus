package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
)

const officeAddr = "203.0.113.7:44001"

func exhaust(t *testing.T, limiter *security.IPRateLimiter, n int) int {
	t.Helper()
	handler := security.RateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	allowed := 0
	for i := 0; i < n; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/eid/poll", nil)
		req.RemoteAddr = officeAddr
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			allowed++
		}
	}
	return allowed
}

// Polling used to share the sign-in budget, so a handful of citizens waiting on
// their phones behind one office address spent it and the next person could not
// sign in at all. The two counters have to be separate, and separate by more
// than name — copying the sign-in numbers into a second limiter would leave the
// office throttling itself out just the same.
func TestPollAndLoginBudgetsAreIndependent(t *testing.T) {
	loginLimiter, pollLimiter := NewLoginLimiter(), NewPollLimiter()

	if spent := exhaust(t, pollLimiter, PollBurst); spent == 0 {
		t.Fatal("the poll limiter refused everything")
	}
	if allowed := exhaust(t, loginLimiter, 1); allowed != 1 {
		t.Errorf("polling consumed the sign-in budget: a citizen at %s cannot sign in", officeAddr)
	}
	if PollRatePerMinute <= LoginRatePerMinute || PollBurst <= LoginBurst {
		t.Errorf("the poll budget (%d/min, burst %d) is no more generous than sign-in (%d/min, burst %d), so separating them changed nothing",
			PollRatePerMinute, PollBurst, LoginRatePerMinute, LoginBurst)
	}
}

// A citizen polls about once every 25s while they reach for their phone. The
// budget is stated per minute, so it has to cover a plausible number of them
// waiting behind one shared address at once.
func TestPollBudgetCoversAnOfficeWaitingAtOnce(t *testing.T) {
	const pollsPerCitizenPerMinute = 60.0 / 25.0

	limiter := NewPollLimiter()
	burst := exhaust(t, limiter, 100)
	if concurrent := float64(burst) / pollsPerCitizenPerMinute; concurrent < 5 {
		t.Errorf("the burst covers only %.1f citizens starting together, too few for a shared address", concurrent)
	}

	if perMinute := float64(PollRatePerMinute); perMinute/pollsPerCitizenPerMinute < 20 {
		t.Errorf("%.0f polls per minute sustains only %.1f waiting citizens per address", perMinute, perMinute/pollsPerCitizenPerMinute)
	}
}

// Sign-in itself stays tight: it is the endpoint worth guessing against, and
// starting a session pushes a notification to a real person's phone.
func TestLoginBudgetStaysTight(t *testing.T) {
	if LoginRatePerMinute > 10 {
		t.Errorf("login rate %d/min is no longer a meaningful brake on guessing", LoginRatePerMinute)
	}
	if allowed := exhaust(t, NewLoginLimiter(), 50); allowed > 10 {
		t.Errorf("a burst of %d sign-in attempts got through from one address", allowed)
	}
}
