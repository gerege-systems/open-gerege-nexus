package emailverify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests exercise the flow against a real schema, because what they
// protect lives partly in SQL: the single conditional UPDATE that makes a
// return good exactly once, and the counts the local limits are computed from.
// The hosted service is stubbed — what is under test is this platform's half.
//
// They are skipped unless a migrated throwaway database is provided:
//
//	EMAILVERIFY_TEST_DATABASE_URL=postgres://... go test ./internal/tenant/emailverify/...
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("EMAILVERIFY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set EMAILVERIFY_TEST_DATABASE_URL to a migrated test database to run email verification integration tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

type fixture struct {
	svc      *Service
	stub     *stubProvider
	pool     *pgxpool.Pool
	tenantID string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	t.Setenv("PUBLIC_ORIGIN", "https://nexus.test")
	// These tests are about what happens to a verification after it is sent —
	// claimed once, expired, swept. Where it may point is a separate question,
	// answered in redirect_test.go, so the host they use is simply listed.
	t.Setenv("EMAIL_VERIFY_REDIRECT_HOSTS", "theirapp.com")

	stub := newStubProvider(t, http.StatusOK, `{"ok":true,"email":"user@example.com"}`)
	pool := testPool(t)

	var tenantID string
	slug := fmt.Sprintf("emailverify-test-%d", time.Now().UnixNano())
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO registry.tenants (slug, name) VALUES ($1, $2) RETURNING id::text`,
		slug, "Email verification integration test").Scan(&tenantID); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	t.Cleanup(func() {
		// The schema cascades from registry.tenants, so one delete clears this test's
		// verifications with it.
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1`, tenantID)
	})

	return &fixture{svc: NewService(pool), stub: stub, pool: pool, tenantID: tenantID}
}

// refFromReturnURL reads the single-use reference out of the return address the
// provider was handed — the way the provider's redirect will hand it back.
func refFromReturnURL(t *testing.T, stub *stubProvider) string {
	t.Helper()
	_, payload := stub.seen()
	parsed, err := url.Parse(payload["redirect_url"])
	if err != nil {
		t.Fatalf("the return address is not a URL: %q", payload["redirect_url"])
	}
	ref := parsed.Query().Get("ref")
	if ref == "" {
		t.Fatalf("the return address carries no reference: %q", payload["redirect_url"])
	}
	return ref
}

// The whole point: a return that is honoured once.
func TestAReturnIsGoodExactlyOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	sent, err := f.svc.Send(ctx, f.tenantID, Request{
		Email:       "User@Example.com",
		RedirectURL: "https://theirapp.com/verified",
		Purpose:     "signup",
		Source:      "io.gerege.nexus.contacts",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if sent.Status != StatusPending {
		t.Fatalf("a fresh request is %q, want PENDING", sent.Status)
	}
	if sent.Email != "user@example.com" {
		t.Fatalf("the address was stored as %q, want it normalised", sent.Email)
	}

	ref := refFromReturnURL(t, f.stub)

	confirmed, err := f.svc.Confirm(ctx, ref)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if confirmed.Status != StatusVerified || confirmed.VerifiedAt == nil {
		t.Fatalf("confirm returned %+v, want a verified row with a timestamp", confirmed)
	}
	if confirmed.RedirectURL != "https://theirapp.com/verified" {
		t.Fatalf("the destination came back as %q", confirmed.RedirectURL)
	}

	// A second arrival — a reload, a forwarded mail, a reference replayed out
	// of a browser's history — gets nothing.
	if _, err := f.svc.Confirm(ctx, ref); !errors.Is(err, ErrLinkSpent) {
		t.Fatalf("a replayed return got %v, want ErrLinkSpent", err)
	}
	// And a reference nobody issued is refused the same way, so an attacker
	// cannot tell a spent one from an invented one.
	if _, err := f.svc.Confirm(ctx, "not-a-reference-anybody-issued"); !errors.Is(err, ErrLinkSpent) {
		t.Fatalf("an invented reference got %v, want ErrLinkSpent", err)
	}
}

// Two arrivals at once must not both win. A browser reloading the landing page
// races itself, and a read-then-write would hand both of them a success.
func TestConcurrentReturnsProduceOneVerification(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Send(ctx, f.tenantID, Request{Email: "race@example.com"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	ref := refFromReturnURL(t, f.stub)

	const arrivals = 8
	var wg sync.WaitGroup
	results := make([]error, arrivals)
	start := make(chan struct{})
	for i := 0; i < arrivals; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			<-start
			_, results[slot] = f.svc.Confirm(ctx, ref)
		}(i)
	}
	close(start)
	wg.Wait()

	won := 0
	for _, err := range results {
		switch {
		case err == nil:
			won++
		case errors.Is(err, ErrLinkSpent):
		default:
			t.Fatalf("unexpected error from a concurrent return: %v", err)
		}
	}
	if won != 1 {
		t.Fatalf("%d of %d concurrent returns succeeded, want exactly 1", won, arrivals)
	}
}

func TestAnExpiredRequestIsRefusedAndSweptUp(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	sent, err := f.svc.Send(ctx, f.tenantID, Request{Email: "late@example.com"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	ref := refFromReturnURL(t, f.stub)

	// Age it rather than waiting a day for it.
	if _, err := f.pool.Exec(ctx,
		`UPDATE tenant.email_verifications SET expires_at = NOW() - INTERVAL '1 minute' WHERE id = $1`,
		sent.ID); err != nil {
		t.Fatalf("age the request: %v", err)
	}

	if _, err := f.svc.Confirm(ctx, ref); !errors.Is(err, ErrLinkSpent) {
		t.Fatalf("an expired return got %v, want ErrLinkSpent", err)
	}

	f.svc.sweep(ctx)
	var status string
	if err := f.pool.QueryRow(ctx,
		`SELECT status FROM tenant.email_verifications WHERE id = $1`, sent.ID).Scan(&status); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if status != string(StatusExpired) {
		t.Fatalf("after the sweep the row is %q, want EXPIRED", status)
	}
}

// The provider's deadline is the one that governs the link; ours was only a
// placeholder written before the call.
func TestTheProvidersExpiryReplacesThePlaceholder(t *testing.T) {
	f := newFixture(t)
	deadline := time.Now().Add(3 * time.Hour).UTC().Truncate(time.Second)

	f.stub.mu.Lock()
	f.stub.body = fmt.Sprintf(`{"ok":true,"expires_at":%q}`, deadline.Format(time.RFC3339))
	f.stub.mu.Unlock()

	sent, err := f.svc.Send(context.Background(), f.tenantID, Request{Email: "deadline@example.com"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !sent.ExpiresAt.UTC().Truncate(time.Second).Equal(deadline) {
		t.Fatalf("the row expires at %v, want the provider's %v", sent.ExpiresAt.UTC(), deadline)
	}
}

// A request the provider refused must leave nothing behind: a row would show on
// the Overview screen as a verification nobody was ever asked to complete.
func TestARefusedRequestLeavesNoRowBehind(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	f.stub.mu.Lock()
	f.stub.status = http.StatusBadGateway
	f.stub.body = `{"error":"send_failed"}`
	f.stub.mu.Unlock()

	if _, err := f.svc.Send(ctx, f.tenantID, Request{Email: "nowhere@example.com"}); !errors.Is(err, ErrUpstream) {
		t.Fatalf("a refused send got %v, want ErrUpstream", err)
	}

	var count int
	if err := f.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM tenant.email_verifications WHERE tenant_id = $1 AND email = $2`,
		f.tenantID, "nowhere@example.com").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("%d rows survived a request the provider refused, want none", count)
	}
}

// Without a key nothing can be asked for, and the provider must not be called
// to find that out.
func TestSendingWithoutAKeyIsRefusedLocally(t *testing.T) {
	f := newFixture(t)
	t.Setenv("EMAIL_VERIFY_API_KEY", "")

	if _, err := f.svc.Send(context.Background(), f.tenantID, Request{Email: "nokey@example.com"}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("got %v, want ErrNotConfigured", err)
	}
	if calls, _ := f.stub.seen(); calls != 0 {
		t.Fatalf("the provider was called %d times with no key configured", calls)
	}
}

// The recipient never asked for any of this. Writing to the same address twice
// in a minute is the shape of a mail-bombing tool, whoever is asking — and it
// is refused here rather than by provoking the provider's own limit.
func TestTheSameAddressIsNotWrittenToTwiceInAMinute(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Send(ctx, f.tenantID, Request{Email: "target@example.com"}); err != nil {
		t.Fatalf("first send: %v", err)
	}
	_, err := f.svc.Send(ctx, f.tenantID, Request{Email: "TARGET@example.com"})
	var limited *RateLimitedError
	if !errors.As(err, &limited) {
		t.Fatalf("an immediate resend got %v, want a rate limit", err)
	}
	if limited.RetryAfter <= 0 || limited.RetryAfter > ResendInterval {
		t.Fatalf("Retry-After is %v, want it inside the resend interval", limited.RetryAfter)
	}
	if calls, _ := f.stub.seen(); calls != 1 {
		t.Fatalf("the provider was called %d times, want the refused resend not to have reached it", calls)
	}
}

// A destination outside the rules never reaches the provider either: this
// platform is the one that would issue the onward redirect.
func TestAnUnsafeDestinationIsRefusedBeforeAnythingIsSent(t *testing.T) {
	f := newFixture(t)

	_, err := f.svc.Send(context.Background(), f.tenantID, Request{
		Email: "user@example.com", RedirectURL: "http://theirapp.com/verified",
	})
	var invalidErr *InvalidError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("a plain-HTTP destination got %v, want a refusal", err)
	}
	if calls, _ := f.stub.seen(); calls != 0 {
		t.Fatalf("the provider was called %d times for a request we should have refused", calls)
	}
}

// The Overview screen is one request: the counts, the recent rows, and whether
// the service that does the sending is reachable at all.
func TestOverviewCountsAndReportsTheProvider(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Send(ctx, f.tenantID, Request{Email: "one@example.com", Source: "portal"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if _, err := f.svc.Confirm(ctx, refFromReturnURL(t, f.stub)); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := f.svc.Send(ctx, f.tenantID, Request{Email: "two@example.com", Source: "portal"}); err != nil {
		t.Fatalf("second send: %v", err)
	}

	overview, err := f.svc.Overview(ctx, f.tenantID, 25)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if overview.Stats.Total != 2 || overview.Stats.Verified != 1 || overview.Stats.Pending != 1 {
		t.Fatalf("stats are %+v, want 2 total / 1 verified / 1 pending", overview.Stats)
	}
	if overview.Stats.VerifiedPct != 50 {
		t.Fatalf("verified rate is %v, want 50", overview.Stats.VerifiedPct)
	}
	if len(overview.Recent) != 2 {
		t.Fatalf("%d recent rows, want 2", len(overview.Recent))
	}
	if !overview.Configured || !overview.Reachable {
		t.Fatalf("overview reports configured=%v reachable=%v, want both true", overview.Configured, overview.Reachable)
	}
	if overview.ReturnURL != "https://nexus.test/api/v1/verify/landed" {
		t.Fatalf("the screen was given %q as the return address", overview.ReturnURL)
	}

	// An administrator seeing nothing arrive should be able to tell "nobody
	// asked" from "the service is down".
	f.stub.server.Close()
	down, err := f.svc.Overview(ctx, f.tenantID, 25)
	if err != nil {
		t.Fatalf("overview with the provider down: %v", err)
	}
	if down.Reachable || down.Health == "" {
		t.Fatalf("with the provider down the screen reports reachable=%v health=%q",
			down.Reachable, down.Health)
	}
	if !strings.Contains(strings.ToLower(down.Health), "could not send") &&
		!strings.Contains(strings.ToLower(down.Health), "health check") {
		t.Fatalf("the health message reads %q, which does not say what is wrong", down.Health)
	}
}
