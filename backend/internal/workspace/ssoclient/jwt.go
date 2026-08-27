/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * id_token verification (OpenID Connect Core 1.0 §3.1.3.7).
 *
 * Hand-rolled, and deliberately narrow, for the same reason the provider's
 * signing is: this is the side of JWT where the sharp edges live — alg
 * confusion, "none", key substitution — and every one of them is a decision
 * about what to *accept*. One algorithm, one key source, every claim checked
 * before the token is believed, is a hundred lines that can be read in full.
 */

package ssoclient

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// clockSkew is how far ahead of us a provider's clock may be before its freshly
// minted token looks like it comes from the future.
const clockSkew = 2 * time.Minute

// verifyIDToken checks a compact RS256 JWS against the provider's published
// keys and every claim OpenID Connect requires a client to check.
func (c *Client) verifyIDToken(ctx context.Context, meta *Metadata, token, nonce string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("the id_token is not a compact JWS")
	}

	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := decodeSegment(parts[0], &header); err != nil {
		return nil, fmt.Errorf("parse the id_token header: %w", err)
	}
	// RS256 only. "none" and the HMAC family are what alg confusion is made of:
	// accepting HS256 here would let anybody who has read the public key from
	// the provider's JWKS mint tokens with it.
	if header.Alg != "RS256" {
		return nil, fmt.Errorf("the id_token is signed with %q; this client accepts only RS256", header.Alg)
	}
	if header.Kid == "" {
		return nil, errors.New("the id_token names no key")
	}

	key, err := c.keys.key(ctx, meta.JWKSURI, header.Kid)
	if err != nil {
		return nil, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("the id_token signature is not base64url")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
		return nil, errors.New("the id_token signature does not verify against the provider's key")
	}

	var claims map[string]any
	if err := decodeSegment(parts[1], &claims); err != nil {
		return nil, fmt.Errorf("parse the id_token claims: %w", err)
	}

	// Issuer. The signature proves the token came from a holder of that key;
	// this proves the key's owner meant to be the provider we asked.
	if strings.TrimRight(stringClaim(claims, "iss"), "/") != strings.TrimRight(meta.Issuer, "/") {
		return nil, fmt.Errorf("the id_token was issued by %q, not %q", stringClaim(claims, "iss"), meta.Issuer)
	}

	// Audience. A token minted for another client of the same provider is a
	// valid token and still not ours; accepting one is how a hostile relying
	// party replays somebody else's identity into this deployment.
	if !audienceContains(claims["aud"], c.cfg.ClientID) {
		return nil, errors.New("the id_token was not issued for this client")
	}
	// azp names which client a multi-audience token was actually for.
	if azp := stringClaim(claims, "azp"); azp != "" && azp != c.cfg.ClientID {
		return nil, errors.New("the id_token is authorised for a different client")
	}

	now := time.Now()
	expiry, ok := timeClaim(claims, "exp")
	if !ok {
		return nil, errors.New("the id_token carries no expiry")
	}
	if now.After(expiry.Add(clockSkew)) {
		return nil, errors.New("the id_token has expired")
	}
	if issued, ok := timeClaim(claims, "iat"); ok && issued.After(now.Add(clockSkew)) {
		return nil, errors.New("the id_token was issued in the future")
	}

	// Nonce. This is what ties the token to the authorization request this
	// deployment started, and it is the whole defence against a code or token
	// captured from one sign-in being replayed into another.
	returned := stringClaim(claims, "nonce")
	if nonce != "" && subtle.ConstantTimeCompare([]byte(returned), []byte(nonce)) != 1 {
		return nil, errors.New("the id_token answers a different sign-in request")
	}

	return claims, nil
}

func decodeSegment(segment string, into any) error {
	raw, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return errors.New("segment is not base64url")
	}
	return json.Unmarshal(raw, into)
}

// audienceContains handles both shapes the aud claim is allowed to take: a
// single string, or an array of them.
func audienceContains(aud any, clientID string) bool {
	switch value := aud.(type) {
	case string:
		return value == clientID
	case []any:
		for _, entry := range value {
			if text, ok := entry.(string); ok && text == clientID {
				return true
			}
		}
	}
	return false
}

// timeClaim reads a NumericDate. JSON numbers arrive as float64, which is exact
// for every second-precision timestamp any of these claims will ever hold.
func timeClaim(claims map[string]any, name string) (time.Time, bool) {
	seconds, ok := claims[name].(float64)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(int64(seconds), 0), true
}
