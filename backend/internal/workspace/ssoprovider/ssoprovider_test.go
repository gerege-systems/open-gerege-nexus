package ssoprovider

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// Unit coverage for the parts that hold without a database. The flow itself is
// exercised end to end in oauth2_flow_test.go against real Postgres, because
// single-use redemption and refresh rotation are enforced in SQL.

func TestVerifyPKCE(t *testing.T) {
	// A verifier of the minimum legal length (43 characters, RFC 7636 §4.1).
	verifier := strings.Repeat("a", 43)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	if !verifyPKCE(verifier, challenge) {
		t.Fatal("the correct verifier was rejected")
	}
	if verifyPKCE(strings.Repeat("b", 43), challenge) {
		t.Error("a wrong verifier was accepted")
	}
	if verifyPKCE("", challenge) {
		t.Error("an empty verifier was accepted")
	}
	// Below the RFC's floor a verifier is brute-forceable, so length is part
	// of the check rather than a detail left to the client.
	short := strings.Repeat("a", 42)
	shortSum := sha256.Sum256([]byte(short))
	if verifyPKCE(short, base64.RawURLEncoding.EncodeToString(shortSum[:])) {
		t.Error("a 42-character verifier was accepted; the RFC floor is 43")
	}
	if verifyPKCE(strings.Repeat("a", 129), challenge) {
		t.Error("a 129-character verifier was accepted; the RFC ceiling is 128")
	}
}

func TestResolveScopes(t *testing.T) {
	client := &Client{Scopes: []string{"openid", "profile", "erp.read"}}

	t.Run("defaults to the client's registered scopes", func(t *testing.T) {
		got, err := resolveScopes("", client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("expected the client's three scopes, got %v", got)
		}
	})

	t.Run("narrows to what was asked for", func(t *testing.T) {
		got, err := resolveScopes("openid profile", client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 || got[0] != "openid" || got[1] != "profile" {
			t.Fatalf("expected [openid profile], got %v", got)
		}
	})

	t.Run("refuses a scope the client is not registered for", func(t *testing.T) {
		// The old token endpoint ignored the scope parameter entirely and
		// always issued the client's full set, so a caller asking for less got
		// more and a caller asking for more was never told no.
		if _, err := resolveScopes("openid erp.write", client); err == nil {
			t.Fatal("erp.write is not registered for this client but was granted")
		}
	})

	t.Run("refuses a scope outside the vocabulary", func(t *testing.T) {
		if _, err := resolveScopes("wildcard.everything", client); err == nil {
			t.Fatal("an unknown scope was accepted")
		}
	})
}

func TestSignJWTVerifiesAgainstItsPublishedKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	token, err := signJWT("kid-1", key, map[string]any{"iss": "https://sso.example", "sub": "user-1"})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected three JWS segments, got %d", len(parts))
	}

	var header map[string]string
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		t.Fatalf("parse header: %v", err)
	}
	if header["alg"] != "RS256" || header["kid"] != "kid-1" {
		t.Errorf("unexpected header: %v", header)
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], signature); err != nil {
		t.Fatalf("the signature does not verify against its own key: %v", err)
	}

	// And the JWK we publish has to describe that same key, or a client
	// following the discovery document could never check the signature.
	jwk := publicJWK("kid-1", &key.PublicKey)
	if jwk["kid"] != "kid-1" || jwk["kty"] != "RSA" || jwk["alg"] != "RS256" {
		t.Errorf("unexpected JWK: %v", jwk)
	}
	if jwk["n"] == "" || jwk["e"] == "" {
		t.Error("the JWK is missing the modulus or exponent")
	}
}

func TestHashSecretIsStableAndNotTheSecret(t *testing.T) {
	const secret = "sec_0123456789abcdef"
	digest := HashSecret(secret)

	if digest == secret {
		t.Fatal("the secret was stored in the clear")
	}
	if len(digest) != 64 {
		t.Fatalf("expected a 64-character SHA-256 hex digest, got %d characters", len(digest))
	}
	if digest != HashSecret(secret) {
		t.Error("hashing is not deterministic, so no stored digest would ever match")
	}
	if digest == HashSecret(secret+"x") {
		t.Error("two different secrets hashed alike")
	}
}

func TestNewIdentifierUsesItsFullLength(t *testing.T) {
	// The original helper drew n bytes, hex-encoded them into 2n characters
	// and then truncated back to n, halving the entropy of every secret.
	const length = 48
	seen := make(map[string]bool, 64)
	for i := 0; i < 64; i++ {
		id := NewIdentifier(length)
		if len(id) != length {
			t.Fatalf("expected %d characters, got %d", length, len(id))
		}
		if seen[id] {
			t.Fatal("NewIdentifier repeated itself within 64 draws")
		}
		seen[id] = true
	}
}

func TestIsSubsetAndUnion(t *testing.T) {
	if !isSubset([]string{"openid"}, []string{"openid", "profile"}) {
		t.Error("openid should be a subset of [openid profile]")
	}
	if isSubset([]string{"erp.write"}, []string{"openid", "profile"}) {
		t.Error("erp.write is not in the granted set but passed as a subset")
	}
	merged := union([]string{"profile"}, []string{"openid", "profile"})
	if len(merged) != 2 {
		t.Fatalf("expected two distinct scopes, got %v", merged)
	}
}
