/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 */

package ssoprovider

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
)

// Compact JWS signing for id_tokens (RFC 7515) and the JWK representation of
// the public half (RFC 7517).
//
// Hand-rolled rather than pulled from a JWT library because this provider only
// ever *signs*: it never accepts a token minted elsewhere, which is where the
// sharp edges of JWT live (alg confusion, "none", key substitution). Signing
// with a fixed algorithm is a hundred lines and no dependency.

// signJWT returns a compact RS256 JWS over claims, carrying kid in the header
// so a verifier can pick the right key out of the published JWKS.
func signJWT(kid string, key *rsa.PrivateKey, claims map[string]any) (string, error) {
	header := map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal jwt header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal jwt claims: %w", err)
	}

	signingInput := b64(headerJSON) + "." + b64(claimsJSON)

	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}

	return signingInput + "." + b64(signature), nil
}

// publicJWK renders an RSA public key as a JWK. "use":"sig" and "alg":"RS256"
// are advisory but let strict clients select without guessing.
func publicJWK(kid string, pub *rsa.PublicKey) map[string]any {
	return map[string]any{
		"kty": "RSA",
		"use": "sig",
		"alg": "RS256",
		"kid": kid,
		"n":   b64(pub.N.Bytes()),
		"e":   b64(big.NewInt(int64(pub.E)).Bytes()),
	}
}

// b64 is base64url without padding, which is what JOSE requires everywhere.
func b64(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}
