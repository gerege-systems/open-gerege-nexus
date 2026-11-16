package eidrp

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// The client certificate, for a relying party eID registered with
// require_mtls.
//
// eID identifies every relying party by its Bearer secret. A registration can
// additionally demand mutual TLS, and once it does, a request that presents no
// client certificate is refused with 401 ("энэ RP-д mTLS client сертификат
// заавал") whatever the secret says. A deployment whose registration turned
// mTLS on and whose client sends no certificate cannot start a single sign-in,
// and the citizen sees "could not reach the server".
//
//	EID_RP_CLIENT_CERT_FILE  the certificate eID issued, PEM. The chain eID
//	                         hands back (leaf first, then its issuing CA) is
//	                         used as it is.
//	EID_RP_CLIENT_KEY_FILE   its private key, PEM.
//
// Both or neither. One without the other is a deployment that believes it
// presents a certificate and does not, so it is an error rather than a quiet
// fallback to sending none.
const (
	envClientCert = "EID_RP_CLIENT_CERT_FILE"
	envClientKey  = "EID_RP_CLIENT_KEY_FILE"
)

// HTTPClient is the client every call to the eID relying-party API makes:
// authentication here, and signing in eidsign. With no certificate configured
// it is a plain client, exactly as before.
func HTTPClient(timeout time.Duration) (*http.Client, error) {
	tlsConfig, err := clientTLSConfig(os.Getenv(envClientCert), os.Getenv(envClientKey), time.Now())
	if err != nil {
		return nil, err
	}
	if tlsConfig == nil {
		return &http.Client{Timeout: timeout}, nil
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	return &http.Client{Timeout: timeout, Transport: transport}, nil
}

// clientTLSConfig loads the pair, or answers nil when none is configured.
//
// An expired certificate is refused here rather than at eID: the 401 eID would
// answer says nothing about dates, and the operator reading it is looking at
// the wrong half of the system.
func clientTLSConfig(certFile, keyFile string, now time.Time) (*tls.Config, error) {
	certFile, keyFile = strings.TrimSpace(certFile), strings.TrimSpace(keyFile)
	switch {
	case certFile == "" && keyFile == "":
		return nil, nil
	case certFile == "" || keyFile == "":
		return nil, fmt.Errorf("eid: %s and %s must be set together", envClientCert, envClientKey)
	}
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("eid: load the client certificate: %w", err)
	}
	if leaf := pair.Leaf; leaf != nil && now.After(leaf.NotAfter) {
		return nil, fmt.Errorf("eid: the client certificate %s expired on %s",
			leaf.Subject.CommonName, leaf.NotAfter.Format(time.DateOnly))
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair}}, nil
}
