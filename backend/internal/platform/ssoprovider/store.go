/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 */

package ssoprovider

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"crypto/rand"
	"crypto/rsa"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned by every lookup that finds nothing, so callers never
// have to reach for pgx.ErrNoRows.
var ErrNotFound = errors.New("not found")

// Client is a registered OAuth2 relying party.
type Client struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	ClientID        string     `json:"client_id"`
	ClientName      string     `json:"client_name"`
	ClientURI       string     `json:"client_uri,omitempty"`
	LogoURI         string     `json:"logo_uri,omitempty"`
	ClientType      string     `json:"client_type"`
	RedirectURIs    []string   `json:"redirect_uris"`
	GrantTypes      []string   `json:"grant_types"`
	Scopes          []string   `json:"scopes"`
	Disabled        bool       `json:"disabled"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	SecretRotatedAt *time.Time `json:"secret_rotated_at,omitempty"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`

	// Secret carries the plaintext client secret exactly once — in the
	// response to the call that created or rotated it. It is never read back
	// out of the database, because only its digest is stored.
	Secret string `json:"client_secret,omitempty"`
}

// IsPublic reports whether the client authenticates with PKCE alone.
func (c *Client) IsPublic() bool { return c.ClientType == clientTypePublic }

const (
	clientTypeConfidential = "confidential"
	clientTypePublic       = "public"
)

// AuthCode is an issued authorization code, bound to its PKCE challenge.
type AuthCode struct {
	CodeHash            string
	ClientID            string
	TenantID            string
	UserID              string
	RedirectURI         string
	Scopes              []string
	CodeChallenge       string
	CodeChallengeMethod string
	Nonce               string
	ExpiresAt           time.Time
}

// Token is an access or refresh token as stored — by digest, never in the clear.
type Token struct {
	ID           string
	TokenHash    string
	TokenType    string
	ClientID     string
	TenantID     string
	UserID       *string
	Scopes       []string
	ParentID     *string
	AuthCodeHash *string
	ExpiresAt    time.Time
	RevokedAt    *time.Time
}

const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

// Store is the Postgres persistence layer for the provider.
//
// It replaces the two Go maps the provider used to keep on its struct. Those
// maps meant a client registered through the developer portal disappeared at
// the next deploy, that tokens survived no restart, and that expired entries
// were never reclaimed — the token map only ever grew.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

// hashSecret digests a client secret or token for storage.
//
// SHA-256 rather than bcrypt/argon2 deliberately: these are 128-bit values this
// server generated from crypto/rand, not passwords a human chose. There is no
// dictionary to attack, so the slow-KDF tradeoff buys nothing and would put a
// deliberate delay on the token endpoint's hot path.
func hashSecret(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

const clientColumns = `id, tenant_id, client_id, client_name, client_uri, logo_uri,
	client_type, redirect_uris, grant_types, scopes, disabled,
	created_at, updated_at, secret_rotated_at, last_used_at`

func scanClient(row pgx.Row) (*Client, error) {
	var c Client
	err := row.Scan(&c.ID, &c.TenantID, &c.ClientID, &c.ClientName, &c.ClientURI, &c.LogoURI,
		&c.ClientType, &c.RedirectURIs, &c.GrantTypes, &c.Scopes, &c.Disabled,
		&c.CreatedAt, &c.UpdatedAt, &c.SecretRotatedAt, &c.LastUsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateClient inserts a client. secretHash is empty for public clients.
func (s *Store) CreateClient(ctx context.Context, c *Client, secretHash, createdBy string) (*Client, error) {
	var hash, creator *string
	if secretHash != "" {
		hash = &secretHash
	}
	if createdBy != "" {
		creator = &createdBy
	}
	row := s.db.QueryRow(ctx, `
		INSERT INTO oauth2_clients
			(tenant_id, client_id, client_secret_hash, client_name, client_uri, logo_uri,
			 client_type, redirect_uris, grant_types, scopes, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING `+clientColumns,
		c.TenantID, c.ClientID, hash, c.ClientName, c.ClientURI, c.LogoURI,
		c.ClientType, c.RedirectURIs, c.GrantTypes, c.Scopes, creator)
	return scanClient(row)
}

// GetClient resolves a client by its OAuth2 identifier, across tenants: the
// token endpoint has to find it before any tenant context exists.
func (s *Store) GetClient(ctx context.Context, clientID string) (*Client, error) {
	return scanClient(s.db.QueryRow(ctx,
		`SELECT `+clientColumns+` FROM oauth2_clients WHERE client_id = $1`, clientID))
}

// GetTenantClient resolves a client that the given tenant owns. Every developer
// portal read and write goes through this, so one tenant cannot name another
// tenant's client_id and act on it.
func (s *Store) GetTenantClient(ctx context.Context, tenantID, clientID string) (*Client, error) {
	return scanClient(s.db.QueryRow(ctx,
		`SELECT `+clientColumns+` FROM oauth2_clients WHERE tenant_id = $1 AND client_id = $2`,
		tenantID, clientID))
}

// ListClients returns one tenant's clients, newest first.
func (s *Store) ListClients(ctx context.Context, tenantID string) ([]*Client, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+clientColumns+` FROM oauth2_clients WHERE tenant_id = $1 ORDER BY created_at DESC`,
		tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	clients := make([]*Client, 0, 8)
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		clients = append(clients, c)
	}
	return clients, rows.Err()
}

// UpdateClient applies the editable fields of a tenant's client.
func (s *Store) UpdateClient(ctx context.Context, tenantID string, c *Client) (*Client, error) {
	return scanClient(s.db.QueryRow(ctx, `
		UPDATE oauth2_clients
		   SET client_name = $3, client_uri = $4, logo_uri = $5,
		       redirect_uris = $6, grant_types = $7, scopes = $8,
		       disabled = $9, updated_at = NOW()
		 WHERE tenant_id = $1 AND client_id = $2
		RETURNING `+clientColumns,
		tenantID, c.ClientID, c.ClientName, c.ClientURI, c.LogoURI,
		c.RedirectURIs, c.GrantTypes, c.Scopes, c.Disabled))
}

// RotateClientSecret replaces the stored digest and stamps the rotation.
func (s *Store) RotateClientSecret(ctx context.Context, tenantID, clientID, secretHash string) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE oauth2_clients
		   SET client_secret_hash = $3, secret_rotated_at = NOW(), updated_at = NOW()
		 WHERE tenant_id = $1 AND client_id = $2 AND client_type = 'confidential'`,
		tenantID, clientID, secretHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteClient removes a client. Codes, tokens and consents cascade with it, so
// deleting a compromised integration cuts off everything it ever issued.
func (s *Store) DeleteClient(ctx context.Context, tenantID, clientID string) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM oauth2_clients WHERE tenant_id = $1 AND client_id = $2`, tenantID, clientID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// VerifyClientSecret reports whether the presented secret matches the stored
// digest. Both the unknown-client and the wrong-secret paths return the same
// error, so the endpoint cannot be used to enumerate client_ids.
func (s *Store) VerifyClientSecret(ctx context.Context, clientID, secret string) (*Client, error) {
	var stored *string
	err := s.db.QueryRow(ctx,
		`SELECT client_secret_hash FROM oauth2_clients WHERE client_id = $1 AND NOT disabled`,
		clientID).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidClient
	}
	if err != nil {
		return nil, err
	}
	if stored == nil || *stored != hashSecret(secret) {
		return nil, ErrInvalidClient
	}
	return s.GetClient(ctx, clientID)
}

// TouchClient records that a client just exchanged a token; the portal shows it
// so an integrator can spot a credential nobody uses any more.
func (s *Store) TouchClient(ctx context.Context, clientID string) {
	_, _ = s.db.Exec(ctx, `UPDATE oauth2_clients SET last_used_at = NOW() WHERE client_id = $1`, clientID)
}

// SaveAuthCode stores an issued authorization code by digest.
func (s *Store) SaveAuthCode(ctx context.Context, ac *AuthCode) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO oauth2_authorization_codes
			(code_hash, client_id, tenant_id, user_id, redirect_uri, scopes,
			 code_challenge, code_challenge_method, nonce, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		ac.CodeHash, ac.ClientID, ac.TenantID, ac.UserID, ac.RedirectURI, ac.Scopes,
		ac.CodeChallenge, ac.CodeChallengeMethod, ac.Nonce, ac.ExpiresAt)
	return err
}

// ErrCodeReplayed signals that a code was presented a second time. The caller
// must revoke everything already minted from it (RFC 6749 §4.1.2).
var ErrCodeReplayed = errors.New("authorization code already used")

// ConsumeAuthCode marks a code used and returns it, in one statement. The
// `consumed_at IS NULL` predicate inside the UPDATE is what makes redemption
// single-use even when two requests race: exactly one UPDATE can match.
func (s *Store) ConsumeAuthCode(ctx context.Context, codeHash string) (*AuthCode, error) {
	var ac AuthCode
	err := s.db.QueryRow(ctx, `
		UPDATE oauth2_authorization_codes
		   SET consumed_at = NOW()
		 WHERE code_hash = $1 AND consumed_at IS NULL AND expires_at > NOW()
		RETURNING code_hash, client_id, tenant_id, user_id, redirect_uri, scopes,
		          code_challenge, code_challenge_method, nonce, expires_at`, codeHash).
		Scan(&ac.CodeHash, &ac.ClientID, &ac.TenantID, &ac.UserID, &ac.RedirectURI, &ac.Scopes,
			&ac.CodeChallenge, &ac.CodeChallengeMethod, &ac.Nonce, &ac.ExpiresAt)

	if errors.Is(err, pgx.ErrNoRows) {
		// Either it never existed, it expired, or it was already redeemed.
		// Only the last case is an attack, and it is worth distinguishing.
		var consumed bool
		if lookupErr := s.db.QueryRow(ctx,
			`SELECT consumed_at IS NOT NULL FROM oauth2_authorization_codes WHERE code_hash = $1`,
			codeHash).Scan(&consumed); lookupErr == nil && consumed {
			return nil, ErrCodeReplayed
		}
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ac, nil
}

// SaveToken records an issued token by digest.
func (s *Store) SaveToken(ctx context.Context, t *Token) (*Token, error) {
	err := s.db.QueryRow(ctx, `
		INSERT INTO oauth2_tokens
			(token_hash, token_type, client_id, tenant_id, user_id, scopes,
			 parent_id, auth_code_hash, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id`,
		t.TokenHash, t.TokenType, t.ClientID, t.TenantID, t.UserID, t.Scopes,
		t.ParentID, t.AuthCodeHash, t.ExpiresAt).Scan(&t.ID)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// GetToken looks up a live token of the given type by its plaintext value.
func (s *Store) GetToken(ctx context.Context, token, tokenType string) (*Token, error) {
	var t Token
	err := s.db.QueryRow(ctx, `
		SELECT id, token_hash, token_type, client_id, tenant_id, user_id, scopes,
		       parent_id, auth_code_hash, expires_at, revoked_at
		  FROM oauth2_tokens
		 WHERE token_hash = $1 AND token_type = $2`,
		hashSecret(token), tokenType).
		Scan(&t.ID, &t.TokenHash, &t.TokenType, &t.ClientID, &t.TenantID, &t.UserID, &t.Scopes,
			&t.ParentID, &t.AuthCodeHash, &t.ExpiresAt, &t.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// RevokeToken invalidates a single token, whatever its type.
func (s *Store) RevokeToken(ctx context.Context, token string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE oauth2_tokens SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`,
		hashSecret(token))
	return err
}

// RevokeFamily revokes a token and everything descended from it.
//
// Refresh rotation makes each new token a child of the one it replaced, so when
// a rotated token is presented again — the signature of a stolen token being
// replayed — this cuts off the whole lineage rather than just the copy that was
// replayed, which is the behaviour RFC 6819 §5.2.2.3 asks for.
func (s *Store) RevokeFamily(ctx context.Context, tokenID string) error {
	_, err := s.db.Exec(ctx, `
		WITH RECURSIVE family AS (
			SELECT id FROM oauth2_tokens WHERE id = $1
			UNION ALL
			SELECT t.id FROM oauth2_tokens t JOIN family f ON t.parent_id = f.id
		)
		UPDATE oauth2_tokens SET revoked_at = NOW()
		 WHERE id IN (SELECT id FROM family) AND revoked_at IS NULL`, tokenID)
	return err
}

// RevokeByAuthCode kills every token minted from one authorization code, which
// is what a replayed code calls for.
func (s *Store) RevokeByAuthCode(ctx context.Context, codeHash string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE oauth2_tokens SET revoked_at = NOW() WHERE auth_code_hash = $1 AND revoked_at IS NULL`,
		codeHash)
	return err
}

// DeleteExpired reclaims spent codes and dead tokens. Nothing did this before:
// the in-memory token map grew for the lifetime of the process.
func (s *Store) DeleteExpired(ctx context.Context) (int64, error) {
	codes, err := s.db.Exec(ctx,
		`DELETE FROM oauth2_authorization_codes WHERE expires_at < NOW() - INTERVAL '1 hour'`)
	if err != nil {
		return 0, err
	}
	// Revoked tokens are kept for a day so that introspection can still answer
	// "inactive" rather than "never existed" for a token just pulled.
	tokens, err := s.db.Exec(ctx,
		`DELETE FROM oauth2_tokens
		  WHERE expires_at < NOW() - INTERVAL '24 hours'
		     OR (revoked_at IS NOT NULL AND revoked_at < NOW() - INTERVAL '24 hours')`)
	if err != nil {
		return codes.RowsAffected(), err
	}
	return codes.RowsAffected() + tokens.RowsAffected(), nil
}

// GetConsent returns the scopes a user has already granted a client.
func (s *Store) GetConsent(ctx context.Context, userID, clientID string) ([]string, error) {
	var scopes []string
	err := s.db.QueryRow(ctx,
		`SELECT scopes FROM oauth2_consents WHERE user_id = $1 AND client_id = $2`,
		userID, clientID).Scan(&scopes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return scopes, err
}

// SaveConsent records a grant, widening it if the user approves more later.
func (s *Store) SaveConsent(ctx context.Context, tenantID, userID, clientID string, scopes []string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO oauth2_consents (tenant_id, user_id, client_id, scopes)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (user_id, client_id)
		DO UPDATE SET scopes = EXCLUDED.scopes, granted_at = NOW()`,
		tenantID, userID, clientID, scopes)
	return err
}

// RevokeConsent withdraws a grant and every token issued under it.
func (s *Store) RevokeConsent(ctx context.Context, userID, clientID string) error {
	if _, err := s.db.Exec(ctx,
		`DELETE FROM oauth2_consents WHERE user_id = $1 AND client_id = $2`, userID, clientID); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx,
		`UPDATE oauth2_tokens SET revoked_at = NOW()
		  WHERE user_id = $1 AND client_id = $2 AND revoked_at IS NULL`, userID, clientID)
	return err
}

// signingKey is the active RSA key pair plus the kid published in the JWKS.
type signingKey struct {
	KID     string
	Private *rsa.PrivateKey
}

// ActiveSigningKey loads the active key, generating and storing one the first
// time. Persisting it matters twice over: a restart would otherwise invalidate
// every id_token in flight, and two replicas would publish different JWKS.
func (s *Store) ActiveSigningKey(ctx context.Context) (*signingKey, error) {
	var kid, privatePEM string
	err := s.db.QueryRow(ctx,
		`SELECT kid, private_key_pem FROM oauth2_signing_keys WHERE active LIMIT 1`).
		Scan(&kid, &privatePEM)

	if errors.Is(err, pgx.ErrNoRows) {
		return s.generateSigningKey(ctx)
	}
	if err != nil {
		return nil, err
	}

	key, err := parsePrivateKeyPEM(privatePEM)
	if err != nil {
		return nil, err
	}
	return &signingKey{KID: kid, Private: key}, nil
}

func (s *Store) generateSigningKey(ctx context.Context) (*signingKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}
	kid := "sig-" + generateRandomString(16)

	privatePEM := string(pem.EncodeToMemory(&pem.Block{
		Type: "PRIVATE KEY", Bytes: mustMarshalPKCS8(key),
	}))
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))

	// A partial unique index allows one active key. If two replicas boot
	// together, one insert loses the race — it then reads the winner's key
	// rather than failing, so both end up signing with the same kid.
	_, err = s.db.Exec(ctx, `
		INSERT INTO oauth2_signing_keys (kid, private_key_pem, public_key_pem)
		VALUES ($1,$2,$3)`, kid, privatePEM, publicPEM)
	if err != nil {
		var loadedKID, loadedPEM string
		if lookupErr := s.db.QueryRow(ctx,
			`SELECT kid, private_key_pem FROM oauth2_signing_keys WHERE active LIMIT 1`).
			Scan(&loadedKID, &loadedPEM); lookupErr == nil {
			parsed, parseErr := parsePrivateKeyPEM(loadedPEM)
			if parseErr != nil {
				return nil, parseErr
			}
			return &signingKey{KID: loadedKID, Private: parsed}, nil
		}
		return nil, fmt.Errorf("store signing key: %w", err)
	}

	return &signingKey{KID: kid, Private: key}, nil
}

// PublicKeys returns every key that should appear in the JWKS: the active one
// plus any retired key whose tokens have not all expired yet.
func (s *Store) PublicKeys(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.db.Query(ctx,
		`SELECT kid, public_key_pem FROM oauth2_signing_keys ORDER BY active DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]map[string]any, 0, 2)
	for rows.Next() {
		var kid, publicPEM string
		if err := rows.Scan(&kid, &publicPEM); err != nil {
			return nil, err
		}
		block, _ := pem.Decode([]byte(publicPEM))
		if block == nil {
			continue
		}
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			continue
		}
		if pub, ok := parsed.(*rsa.PublicKey); ok {
			keys = append(keys, publicJWK(kid, pub))
		}
	}
	return keys, rows.Err()
}

func parsePrivateKeyPEM(data string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(data))
	if block == nil {
		return nil, errors.New("signing key is not valid PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse signing key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("signing key is not RSA")
	}
	return key, nil
}

func mustMarshalPKCS8(key *rsa.PrivateKey) []byte {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		// Marshalling an RSA key we just generated cannot fail.
		panic("ssoprovider: marshal generated key: " + err.Error())
	}
	return der
}

// ClientActivity is one client's live footprint, for the portal's audit screen.
type ClientActivity struct {
	ClientID       string     `json:"client_id"`
	ClientName     string     `json:"client_name"`
	ClientType     string     `json:"client_type"`
	Disabled       bool       `json:"disabled"`
	ActiveAccess   int        `json:"active_access_tokens"`
	ActiveRefresh  int        `json:"active_refresh_tokens"`
	ConsentedUsers int        `json:"consented_users"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
}

// ConsentRecord is one user's standing grant to one client.
type ConsentRecord struct {
	ClientID   string    `json:"client_id"`
	ClientName string    `json:"client_name"`
	UserID     string    `json:"user_id"`
	UserEmail  string    `json:"user_email"`
	UserName   string    `json:"user_name"`
	Scopes     []string  `json:"scopes"`
	GrantedAt  time.Time `json:"granted_at"`
}

// ClientActivityByTenant reports live tokens and consents per client.
//
// Scoped by which tenant *owns the client*, not by the tenant on the token: a
// client owned here can sign in a user from elsewhere, and its owner is the
// one who needs to see that it did.
func (s *Store) ClientActivityByTenant(ctx context.Context, tenantID string) ([]ClientActivity, error) {
	rows, err := s.db.Query(ctx, `
		SELECT c.client_id, c.client_name, c.client_type, c.disabled, c.last_used_at,
		       count(*) FILTER (WHERE t.token_type = 'access'  AND t.revoked_at IS NULL AND t.expires_at > NOW()),
		       count(*) FILTER (WHERE t.token_type = 'refresh' AND t.revoked_at IS NULL AND t.expires_at > NOW()),
		       (SELECT count(*) FROM oauth2_consents oc WHERE oc.client_id = c.client_id)
		  FROM oauth2_clients c
		  LEFT JOIN oauth2_tokens t ON t.client_id = c.client_id
		 WHERE c.tenant_id = $1
		 GROUP BY c.client_id, c.client_name, c.client_type, c.disabled, c.last_used_at, c.created_at
		 ORDER BY c.created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	activity := make([]ClientActivity, 0, 8)
	for rows.Next() {
		var a ClientActivity
		if err := rows.Scan(&a.ClientID, &a.ClientName, &a.ClientType, &a.Disabled, &a.LastUsedAt,
			&a.ActiveAccess, &a.ActiveRefresh, &a.ConsentedUsers); err != nil {
			return nil, err
		}
		activity = append(activity, a)
	}
	return activity, rows.Err()
}

// ConsentsByTenant lists the standing grants against this tenant's clients.
func (s *Store) ConsentsByTenant(ctx context.Context, tenantID string, limit int) ([]ConsentRecord, error) {
	rows, err := s.db.Query(ctx, `
		SELECT c.client_id, c.client_name, u.id::text, u.email, u.name, oc.scopes, oc.granted_at
		  FROM oauth2_consents oc
		  JOIN oauth2_clients c ON c.client_id = oc.client_id
		  JOIN users u ON u.id = oc.user_id
		 WHERE c.tenant_id = $1
		 ORDER BY oc.granted_at DESC
		 LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]ConsentRecord, 0, 16)
	for rows.Next() {
		var r ConsentRecord
		if err := rows.Scan(&r.ClientID, &r.ClientName, &r.UserID, &r.UserEmail, &r.UserName,
			&r.Scopes, &r.GrantedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// RevokeClientTokens kills every live token a tenant's client holds — the
// switch to pull when a credential is suspected leaked but the integration
// itself should survive.
func (s *Store) RevokeClientTokens(ctx context.Context, tenantID, clientID string) (int64, error) {
	tag, err := s.db.Exec(ctx, `
		UPDATE oauth2_tokens t SET revoked_at = NOW()
		  FROM oauth2_clients c
		 WHERE t.client_id = c.client_id
		   AND c.tenant_id = $1 AND c.client_id = $2
		   AND t.revoked_at IS NULL`, tenantID, clientID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// WithdrawConsent removes one user's grant to a tenant's client, and the tokens
// issued under it. The tenant guard is in the subquery: withdrawing consent on
// someone else's client is not this tenant's call.
func (s *Store) WithdrawConsent(ctx context.Context, tenantID, clientID, userID string) error {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM oauth2_consents oc
		 WHERE oc.client_id = $2 AND oc.user_id = $3
		   AND EXISTS (SELECT 1 FROM oauth2_clients c
		                WHERE c.client_id = oc.client_id AND c.tenant_id = $1)`,
		tenantID, clientID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = s.db.Exec(ctx, `
		UPDATE oauth2_tokens SET revoked_at = NOW()
		 WHERE client_id = $1 AND user_id = $2 AND revoked_at IS NULL`, clientID, userID)
	return err
}

// SigningKeyInfo describes a published key without any private material.
type SigningKeyInfo struct {
	KID       string     `json:"kid"`
	Algorithm string     `json:"algorithm"`
	Active    bool       `json:"active"`
	CreatedAt time.Time  `json:"created_at"`
	RetiredAt *time.Time `json:"retired_at,omitempty"`
}

// SigningKeys lists the keys behind the JWKS. Deliberately no private_key_pem
// column in the SELECT: the portal never has a reason to see one, so the query
// cannot leak one by accident.
func (s *Store) SigningKeys(ctx context.Context) ([]SigningKeyInfo, error) {
	rows, err := s.db.Query(ctx,
		`SELECT kid, algorithm, active, created_at, retired_at
		   FROM oauth2_signing_keys ORDER BY active DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]SigningKeyInfo, 0, 2)
	for rows.Next() {
		var k SigningKeyInfo
		if err := rows.Scan(&k.KID, &k.Algorithm, &k.Active, &k.CreatedAt, &k.RetiredAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}
