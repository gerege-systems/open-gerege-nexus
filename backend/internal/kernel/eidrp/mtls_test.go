package eidrp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testPKI is a CA, and a client certificate it issued, written to disk the way
// a deployment holds them.
type testPKI struct {
	ca                *x509.Certificate
	certFile, keyFile string
	notAfter          time.Time
}

func newTestPKI(t *testing.T) testPKI {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test organisation issuing CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	notAfter := time.Now().Add(time.Hour)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "rp.example.mn", Organization: []string{"Test RP LLC"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "rp.chain.pem"), filepath.Join(dir, "rp.key")
	// Leaf first, then its issuer: the shape of the chain eID hands back.
	chain := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})...)
	if err := os.WriteFile(certFile, chain, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return testPKI{ca: ca, certFile: certFile, keyFile: keyFile, notAfter: notAfter}
}

// newMTLSServer answers RUNNING to anyone presenting a certificate the CA
// issued, and refuses the handshake to everyone else — eID with require_mtls.
func newMTLSServer(t *testing.T, ca *x509.Certificate) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"state":"RUNNING"}`)
	}))
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	server.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server
}

// clientTrusting is the eID client over cfg, trusting the test server rather
// than the system roots. Only the trust store differs from HTTPClient's.
func clientTrusting(server *httptest.Server, cfg *tls.Config) Client {
	if cfg == nil {
		cfg = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	cfg.RootCAs = x509.NewCertPool()
	cfg.RootCAs.AddCert(server.Certificate())
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = cfg
	return &client{base: server.URL, http: &http.Client{Timeout: 5 * time.Second, Transport: transport}}
}

func TestClientCertificateIsPresentedToAnMTLSRelyingPartyAPI(t *testing.T) {
	pki := newTestPKI(t)
	server := newMTLSServer(t, pki.ca)

	cfg, err := clientTLSConfig(pki.certFile, pki.keyFile, time.Now())
	if err != nil || cfg == nil {
		t.Fatalf("load the pair: cfg = %v, err = %v", cfg, err)
	}
	result, err := clientTrusting(server, cfg).Session(context.Background(), "s-1", 0)
	if err != nil || result.State != StateRunning {
		t.Fatalf("with the certificate: result = %v, err = %v", result, err)
	}

	// The control. The same server, no certificate: if this succeeds the
	// server is not demanding one, and the line above proved nothing.
	if result, err := clientTrusting(server, nil).Session(context.Background(), "s-1", 0); err == nil {
		t.Fatalf("without a certificate the mTLS server answered: %v", result)
	}
}

func TestNoClientCertificateConfiguredIsThePlainClient(t *testing.T) {
	t.Setenv(envClientCert, "")
	t.Setenv(envClientKey, "")
	hc, err := HTTPClient(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if hc.Transport != nil {
		t.Fatalf("no certificate configured, yet the transport was replaced: %T", hc.Transport)
	}
}

func TestClientCertificateConfigurationErrors(t *testing.T) {
	pki := newTestPKI(t)
	cases := []struct {
		name, cert, key string
		now             time.Time
		want            string
	}{
		{"certificate without key", pki.certFile, "", time.Now(), "must be set together"},
		{"key without certificate", "", pki.keyFile, time.Now(), "must be set together"},
		{"missing file", pki.certFile, filepath.Join(t.TempDir(), "absent.key"), time.Now(), "load the client certificate"},
		{"key from another pair", pki.certFile, newTestPKI(t).keyFile, time.Now(), "load the client certificate"},
		{"expired", pki.certFile, pki.keyFile, pki.notAfter.Add(time.Minute), "expired on"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := clientTLSConfig(tc.cert, tc.key, tc.now)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("cfg = %v, err = %v, want an error containing %q", cfg, err, tc.want)
			}
		})
	}
	// The control: the same pair, valid, today.
	if _, err := clientTLSConfig(pki.certFile, pki.keyFile, time.Now()); err != nil {
		t.Fatalf("the valid pair was refused: %v", err)
	}
}

// A certificate that was configured and cannot be used must not degrade into a
// call without one. Counted at the server, because "returned an error" is also
// what a call that went out and was refused looks like.
func TestMisconfiguredClientCertificateSendsNothing(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = io.WriteString(w, `{"state":"RUNNING"}`)
	}))
	defer server.Close()

	t.Setenv(envClientCert, newTestPKI(t).certFile)
	t.Setenv(envClientKey, "")
	if _, err := NewClient(server.URL, "", "", "", "").Session(context.Background(), "s-1", 0); err == nil ||
		!strings.Contains(err.Error(), "must be set together") {
		t.Fatalf("misconfigured certificate: err = %v", err)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("%d request(s) reached the server without the configured certificate", n)
	}

	// The control: unset, the same client reaches the same server.
	t.Setenv(envClientCert, "")
	if _, err := NewClient(server.URL, "", "", "", "").Session(context.Background(), "s-1", 0); err != nil {
		t.Fatalf("plain client: %v", err)
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("plain client: %d requests reached the server, want 1", n)
	}
}
