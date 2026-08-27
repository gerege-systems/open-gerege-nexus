package host

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/identity/eid"
)

// /auth/eid/poll blocks for eid.PollWindow while the citizen decides. A write
// deadline shorter than that does not fail the request politely — Go closes the
// connection with nothing written, and nginx serves the citizen its own 502.
func TestWriteTimeoutOutlastsTheEIDPollWindow(t *testing.T) {
	if writeTimeout <= eid.PollWindow {
		t.Fatalf("writeTimeout %s does not outlast the eID poll window %s: every unanswered poll would 502", writeTimeout, eid.PollWindow)
	}
	if margin := writeTimeout - eid.PollWindow; margin < 5*time.Second {
		t.Errorf("only %s of margin over the eID poll window, too little for the round trip", margin)
	}
}

// The failure the citizen saw, reproduced in miniature: a handler that outlives
// the write deadline yields no response at all, not an error status.
func TestHandlerOutlivingTheWriteDeadlineYieldsNoResponse(t *testing.T) {
	for _, tc := range []struct {
		name         string
		writeTimeout time.Duration
		wantResponse bool
	}{
		{"deadline shorter than the handler", 150 * time.Millisecond, false},
		{"deadline outlasting the handler", 3 * time.Second, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(600 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"state":"RUNNING"}`))
			}))
			srv.Config.WriteTimeout = tc.writeTimeout
			srv.Start()
			defer srv.Close()

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(srv.URL)
			if err != nil {
				if tc.wantResponse {
					t.Fatalf("expected a response, got transport error: %v", err)
				}
				return // the connection closed unwritten — exactly what nginx reports as 502
			}
			defer func() { _ = resp.Body.Close() }()
			body, readErr := io.ReadAll(resp.Body)
			if !tc.wantResponse {
				if readErr == nil && len(body) > 0 {
					t.Fatalf("expected the connection to close unwritten, got %q", body)
				}
				return
			}
			if readErr != nil {
				t.Fatalf("reading the body failed: %v", readErr)
			}
			if string(body) != `{"state":"RUNNING"}` {
				t.Errorf("body = %q, want the RUNNING payload", body)
			}
		})
	}
}

// A long write deadline must not become a slowloris foothold: the header
// deadline is what bounds a client that opens a connection and dribbles.
func TestReadHeaderTimeoutBoundsSlowClients(t *testing.T) {
	if readHeaderTimeout <= 0 {
		t.Fatal("readHeaderTimeout is unset, so a stalled client holds a connection until writeTimeout")
	}
	if readHeaderTimeout >= writeTimeout {
		t.Errorf("readHeaderTimeout %s is not tighter than writeTimeout %s", readHeaderTimeout, writeTimeout)
	}

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.Config.ReadHeaderTimeout = 200 * time.Millisecond
	srv.Config.WriteTimeout = 5 * time.Second
	srv.Start()
	defer srv.Close()

	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Send a request line and then stall, never completing the headers.
	if _, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err = io.ReadAll(conn); err != nil {
		t.Fatalf("the server did not drop a stalled client: %v", err)
	}
}
