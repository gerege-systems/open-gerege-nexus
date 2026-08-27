/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The signing rail, as modules are handed it.
 */

package signing

import (
	"context"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/eidmongolia"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// signingRail is nexus.Signer over the eID Mongolia client.
//
// A wrapper rather than methods on the client itself, so that eidmongolia stays
// what it is — an adapter over the platform's signing packages — and does not
// grow a second vocabulary. What crosses into the SDK is this deployment's
// answers in the SDK's own types; eidsign's types stop here, which is the
// point: a distribution implementing a module against nexus.Signer must not
// have to depend on however this installation happens to sign.
//
// See docs/adr/0002-one-signing-rail.md.
type signingRail struct{ eid *eidmongolia.Service }

// Signing publishes the installation's signing rail. A deployment with no eID
// registration still gets one: it answers Enabled() false, which is the honest
// state and the one a module is meant to ask about.
// Rail publishes the signing rail. Named for what it is now that it lives
// beside the rest of the signing code: Signing(...) inside package signing
// would say the word twice and the type once.
func Rail(eid *eidmongolia.Service) nexus.Signer { return signingRail{eid: eid} }

func (r signingRail) Enabled() bool { return r.eid != nil && r.eid.Enabled() }

func (r signingRail) SignDigest(ctx context.Context, request nexus.SignatureRequest) (nexus.SignatureSession, error) {
	if !r.Enabled() {
		return nexus.SignatureSession{}, nexus.ErrSigningUnavailable
	}
	started, err := r.eid.SignDigest(ctx, request.RegNumber, request.FullName,
		request.DigestHex, request.DisplayText, request.DocumentName)
	if err != nil {
		return nexus.SignatureSession{}, err
	}
	return nexus.SignatureSession{
		SessionID:        started.SessionID,
		VerificationCode: started.VerificationCode,
	}, nil
}

func (r signingRail) SignDocument(ctx context.Context, request nexus.DocumentSignatureRequest) (nexus.SignatureSession, error) {
	if !r.Enabled() {
		return nexus.SignatureSession{}, nexus.ErrSigningUnavailable
	}
	if len(request.PDF) == 0 {
		return nexus.SignatureSession{}, nexus.ErrPDFSigningUnavailable
	}
	started, err := r.eid.SignPDF(ctx, eidmongolia.SignRequest{
		RegNo:    request.RegNumber,
		FullName: request.FullName,
		FileName: request.FileName,
		PDF:      request.PDF,
	})
	if err != nil {
		return nexus.SignatureSession{}, err
	}
	return nexus.SignatureSession{
		SessionID:        started.SessionID,
		VerificationCode: started.VerificationCode,
	}, nil
}

func (r signingRail) SignedDocument(ctx context.Context, ownerRegNumber, sessionID string) (nexus.SignedDocument, error) {
	if !r.Enabled() {
		return nexus.SignedDocument{}, nexus.ErrSigningUnavailable
	}
	signed, err := r.eid.DownloadSigned(ctx, ownerRegNumber, sessionID)
	if err != nil {
		return nexus.SignedDocument{}, err
	}
	return nexus.SignedDocument{PDF: signed.PDF, FileName: signed.Filename}, nil
}

func (r signingRail) PollSignature(ctx context.Context, ownerRegNumber, sessionID string) (nexus.SignatureState, error) {
	if !r.Enabled() {
		return "", nexus.ErrSigningUnavailable
	}
	state, err := r.eid.PollSign(ctx, ownerRegNumber, sessionID)
	if err != nil {
		return "", err
	}
	return nexus.SignatureState(state), nil
}

func (r signingRail) VerifiedDigest(ctx context.Context, ownerRegNumber, sessionID string) (string, error) {
	if !r.Enabled() {
		return "", nexus.ErrSigningUnavailable
	}
	return r.eid.VerifiedDigest(ctx, ownerRegNumber, sessionID)
}
