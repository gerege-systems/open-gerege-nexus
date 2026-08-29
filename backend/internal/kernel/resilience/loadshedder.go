// Package resilience holds the load shedder, and only the load shedder.
//
// It used to also hold an SRE-style adaptive circuit breaker, an exponential
// backoff retry and a singleflight. All three were removed rather than wired
// up, because the reasons to wire them had already been answered better
// elsewhere:
//
//   - Every outbound client already carries a timeout — integration, gerege,
//     emailverify and eid each set one on their http.Client.
//   - Delivery already retries, in integration.deliver, against a bounded
//     backoff schedule and only for failures attemptDelivery classifies as
//     retryable. The generic DoWithRetry retried everything, so putting it in
//     front of a 400 would have sent the same rejected request three times.
//   - The breaker and the singleflight had never executed outside their own
//     tests, and the singleflight leaked: a panic inside the guarded function
//     skipped both wg.Done and the map delete, so every later caller for that
//     key blocked forever. Introducing that to the eID, DAN and payment paths
//     would have been the opposite of resilience.
//
// If a breaker is wanted later, it should arrive attached to the call it
// guards. An unused one in a package named "resilience" reads, to anyone
// scanning for it, as a protection this platform already has.
package resilience

import (
	"net/http"
	"sync/atomic"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
)

type LoadShedder struct {
	maxInFlight int64
	current     int64
}

func NewLoadShedder(maxInFlight int64) *LoadShedder {
	if maxInFlight <= 0 {
		maxInFlight = 1000 // Default max concurrent requests limit
	}
	return &LoadShedder{
		maxInFlight: maxInFlight,
	}
}

// Middleware returns HTTP middleware that sheds load when system capacity is reached.
//
// Both the depth and the refusals are exported to Prometheus. A shed request is
// a 503 the caller sees and nobody here does — before this, the only trace of
// one was an access-log line among every other 503, and there was no way to ask
// how close to the ceiling a normal hour runs.
func (ls *LoadShedder) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt64(&ls.current, 1)
		telemetry.EnterFlight()
		defer func() {
			atomic.AddInt64(&ls.current, -1)
			telemetry.LeaveFlight()
		}()

		if current > ls.maxInFlight {
			telemetry.RecordLoadShed()
			w.Header().Set("Retry-After", "5")
			httpx.Error(w, http.StatusServiceUnavailable, "service overloaded: load shedding active")
			return
		}

		next.ServeHTTP(w, r)
	})
}
