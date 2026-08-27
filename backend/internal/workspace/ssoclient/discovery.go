/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package ssoclient

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Metadata is the subset of the provider's discovery document this client acts
// on. Everything else in it is ignored rather than stored, so a provider that
// grows a field cannot change what this client does.
type Metadata struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
}

// discoveryTTL is how long a fetched document is trusted. Endpoints move
// rarely, and a provider that has moved one has broken every client until they
// notice — an hour is the compromise between that and asking on every sign-in.
const discoveryTTL = time.Hour

type discoveryCache struct {
	issuer string
	http   *http.Client

	mu        sync.Mutex
	metadata  *Metadata
	refreshed time.Time
}

func newDiscoveryCache(issuer string, client *http.Client) *discoveryCache {
	return &discoveryCache{issuer: issuer, http: client}
}

// get returns the provider's metadata, fetching it at most once per TTL.
//
// A failed fetch is neither cached nor papered over with the previous document:
// the next sign-in tries again, and this one says what went wrong. Serving a
// stale document would only help if the discovery path were unreachable while
// the authorization and token endpoints beside it still answered, which is not
// how a provider goes down — and it would trade a clear error for a sign-in
// that fails one redirect later, somewhere harder to read.
func (d *discoveryCache) get(ctx context.Context) (*Metadata, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.metadata != nil && time.Since(d.refreshed) < discoveryTTL {
		return d.metadata, nil
	}

	meta, err := fetchMetadata(ctx, d.http, d.issuer)
	if err != nil {
		return nil, err
	}
	d.metadata, d.refreshed = meta, time.Now()
	return meta, nil
}

func fetchMetadata(ctx context.Context, client *http.Client, issuer string) (*Metadata, error) {
	url := issuer + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reach the SSO provider's discovery document: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the SSO provider answered %d for its discovery document", res.StatusCode)
	}

	var meta Metadata
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&meta); err != nil {
		return nil, fmt.Errorf("parse the SSO provider's discovery document: %w", err)
	}

	// The issuer in the document must be the one we asked. This is the check
	// that makes every endpoint below trustworthy: without it a redirect to
	// somebody else's document would hand this client an authorization endpoint
	// of the attacker's choosing, and every person signing in would be sent to
	// it. See OpenID Connect Discovery 1.0 §4.3.
	if strings.TrimRight(meta.Issuer, "/") != strings.TrimRight(issuer, "/") {
		return nil, fmt.Errorf("the discovery document names issuer %q, not %q", meta.Issuer, issuer)
	}
	if meta.AuthorizationEndpoint == "" || meta.TokenEndpoint == "" || meta.JWKSURI == "" {
		return nil, fmt.Errorf("the SSO provider's discovery document is missing an endpoint this client needs")
	}
	return &meta, nil
}

// keyCache holds the provider's signing keys by kid.
//
// A key set is refetched when a token names a kid that is not in it, which is
// what a rotation looks like from here, and at most once a minute so that a
// stream of tokens naming a kid nobody has cannot be turned into a stream of
// requests at the provider.
type keyCache struct {
	http *http.Client

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
	uri     string
}

const keyRefetchInterval = time.Minute

func newKeyCache(client *http.Client) *keyCache {
	return &keyCache{http: client, keys: map[string]*rsa.PublicKey{}}
}

func (k *keyCache) key(ctx context.Context, jwksURI, kid string) (*rsa.PublicKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.uri != jwksURI {
		// The provider moved its key set; nothing cached under the old address
		// says anything about the new one.
		k.keys, k.uri, k.fetched = map[string]*rsa.PublicKey{}, jwksURI, time.Time{}
	}
	if key, ok := k.keys[kid]; ok {
		return key, nil
	}
	if time.Since(k.fetched) < keyRefetchInterval {
		return nil, fmt.Errorf("the SSO provider's key set has no key %q", kid)
	}

	keys, err := fetchJWKS(ctx, k.http, jwksURI)
	if err != nil {
		return nil, err
	}
	k.keys, k.fetched = keys, time.Now()

	key, ok := keys[kid]
	if !ok {
		return nil, fmt.Errorf("the SSO provider's key set has no key %q", kid)
	}
	return key, nil
}

func fetchJWKS(ctx context.Context, client *http.Client, uri string) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reach the SSO provider's key set: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the SSO provider answered %d for its key set", res.StatusCode)
	}

	var document struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			Alg string `json:"alg"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&document); err != nil {
		return nil, fmt.Errorf("parse the SSO provider's key set: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(document.Keys))
	for _, jwk := range document.Keys {
		// Only RSA signing keys are collected. An encryption key or an EC key
		// in the set is not an error — it is simply not something the one
		// algorithm this client accepts can be verified with.
		if jwk.Kty != "RSA" || jwk.Kid == "" || (jwk.Use != "" && jwk.Use != "sig") {
			continue
		}
		modulus, err := base64.RawURLEncoding.DecodeString(jwk.N)
		if err != nil {
			continue
		}
		exponent, err := base64.RawURLEncoding.DecodeString(jwk.E)
		if err != nil {
			continue
		}
		e := new(big.Int).SetBytes(exponent)
		if !e.IsInt64() || e.Int64() > 1<<31 {
			continue
		}
		keys[jwk.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: int(e.Int64())}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("the SSO provider's key set carries no usable RSA signing key")
	}
	return keys, nil
}
