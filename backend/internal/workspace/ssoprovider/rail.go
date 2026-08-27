/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// The authorization server, published as nexus.SSOClientRegistry.
//
// The app that administers OAuth2 clients left this repository on 2026-08-25
// and is the App Store's now. Everything it used to reach for was exported from
// this package and under internal/, which is the only reason it could not have
// been built anywhere else. This is the whole of what it needed, restated as
// the contract in pkg/nexus so that any distribution can carry it — and stated
// once, so the provider's own shape stays free to change.

package ssoprovider

import (
	"context"
	"errors"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// AsClientRegistry lends the provider to whichever app administers it.
func AsClientRegistry(p *SSOProvider) nexus.SSOClientRegistry { return clientRegistry{p} }

type clientRegistry struct{ p *SSOProvider }

func (r clientRegistry) Issuer() string { return r.p.Issuer() }

func (clientRegistry) SupportedScopes() []nexus.SSOScope {
	out := make([]nexus.SSOScope, 0, len(SupportedScopes))
	for _, s := range SupportedScopes {
		out = append(out, nexus.SSOScope(s))
	}
	return out
}

func (clientRegistry) SupportedGrantTypes() []string { return SupportedGrantTypes }

func (clientRegistry) IsSupportedScope(name string) bool { return IsSupportedScope(name) }

func (clientRegistry) AllowedRedirect(raw string) error { return ValidateRedirectURI(raw) }

func (clientRegistry) NewIdentifier(n int) string { return NewIdentifier(n) }

func (r clientRegistry) ListClients(ctx context.Context, tenantID string) ([]nexus.SSOClient, error) {
	clients, err := r.p.Store().ListClients(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]nexus.SSOClient, 0, len(clients))
	for _, c := range clients {
		out = append(out, nexus.SSOClient(*c))
	}
	return out, nil
}

func (r clientRegistry) GetClient(ctx context.Context, tenantID, clientID string) (nexus.SSOClient, error) {
	c, err := r.p.Store().GetTenantClient(ctx, tenantID, clientID)
	if err != nil {
		return nexus.SSOClient{}, absent(err)
	}
	return nexus.SSOClient(*c), nil
}

// CreateClient hashes the secret here rather than taking a digest.
//
// An app that had to hand over a digest would have to be told which function
// produced it, and the day that function changes is the day two deployments
// disagree about what a stored secret is. What an app has is a secret it just
// generated; what this deployment stores is its own business.
func (r clientRegistry) CreateClient(ctx context.Context, tenantID string, c nexus.SSOClient, secret, createdBy string) (nexus.SSOClient, error) {
	client := Client(c)
	client.WorkspaceID = tenantID

	secretHash := ""
	if secret != "" {
		secretHash = HashSecret(secret)
	}

	created, err := r.p.Store().CreateClient(ctx, &client, secretHash, createdBy)
	if err != nil {
		return nexus.SSOClient{}, err
	}
	out := nexus.SSOClient(*created)
	// The only time it is ever readable.
	out.Secret = secret
	return out, nil
}

func (r clientRegistry) UpdateClient(ctx context.Context, tenantID string, c nexus.SSOClient) (nexus.SSOClient, error) {
	client := Client(c)
	updated, err := r.p.Store().UpdateClient(ctx, tenantID, &client)
	if err != nil {
		return nexus.SSOClient{}, absent(err)
	}
	return nexus.SSOClient(*updated), nil
}

func (r clientRegistry) DeleteClient(ctx context.Context, tenantID, clientID string) error {
	return absent(r.p.Store().DeleteClient(ctx, tenantID, clientID))
}

func (r clientRegistry) RotateClientSecret(ctx context.Context, tenantID, clientID, secret string) error {
	return absent(r.p.Store().RotateClientSecret(ctx, tenantID, clientID, HashSecret(secret)))
}

func (r clientRegistry) ClientActivity(ctx context.Context, tenantID string) ([]nexus.SSOClientActivity, error) {
	activity, err := r.p.Store().ClientActivityByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]nexus.SSOClientActivity, 0, len(activity))
	for _, a := range activity {
		out = append(out, nexus.SSOClientActivity(a))
	}
	return out, nil
}

func (r clientRegistry) Consents(ctx context.Context, tenantID string, limit int) ([]nexus.SSOConsent, error) {
	consents, err := r.p.Store().ConsentsByTenant(ctx, tenantID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]nexus.SSOConsent, 0, len(consents))
	for _, c := range consents {
		out = append(out, nexus.SSOConsent(c))
	}
	return out, nil
}

func (r clientRegistry) RevokeClientTokens(ctx context.Context, tenantID, clientID string) (int64, error) {
	return r.p.Store().RevokeClientTokens(ctx, tenantID, clientID)
}

func (r clientRegistry) WithdrawConsent(ctx context.Context, tenantID, clientID, userID string) error {
	return absent(r.p.Store().WithdrawConsent(ctx, tenantID, clientID, userID))
}

func (r clientRegistry) SigningKeys(ctx context.Context) ([]nexus.SSOSigningKey, error) {
	keys, err := r.p.Store().SigningKeys(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]nexus.SSOSigningKey, 0, len(keys))
	for _, k := range keys {
		out = append(out, nexus.SSOSigningKey(k))
	}
	return out, nil
}

// absent translates this package's sentinel into the contract's, so a caller
// outside the repository can tell "no such client" from "the database is down"
// without importing internal/.
func absent(err error) error {
	if errors.Is(err, ErrNotFound) {
		return nexus.ErrSSOClientNotFound
	}
	return err
}
