/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package eid

import (
	"context"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// AsSigner presents the eID client as the SDK's nexus.EIDSigner.
//
// Five fields of the identity cross, out of fourteen. What stays behind is the
// platform's business: the auth method, the verification status, the timestamps
// and the mock session store. What crosses is who signed and with which
// certificate, which is all a document needs to say afterwards.
func AsSigner(s *EIDService) nexus.EIDSigner {
	if s == nil {
		return nil
	}
	return signer{s}
}

type signer struct{ svc *EIDService }

func (a signer) Mode() string { return a.svc.Mode() }

func (a signer) StartSignature(ctx context.Context, nationalID, displayText, callbackURL string) (*nexus.SignatureCeremony, error) {
	started, err := a.svc.StartSignature(ctx, nationalID, displayText, callbackURL)
	if err != nil || started == nil {
		return nil, err
	}
	return &nexus.SignatureCeremony{
		SessionID:        started.SessionID,
		DeviceLinkURL:    started.DeviceLinkURL,
		VerificationCode: started.VerificationCode,
		ExpiresAt:        started.ExpiresAt,
	}, nil
}

func (a signer) Poll(ctx context.Context, sessionID string) (*nexus.CeremonyState, error) {
	polled, err := a.svc.Poll(ctx, sessionID)
	if err != nil || polled == nil {
		return nil, err
	}
	state := &nexus.CeremonyState{State: polled.State}
	if polled.Identity != nil {
		state.Identity = &nexus.SignerIdentity{
			RegNumber:         polled.Identity.RegNumber,
			FirstName:         polled.Identity.FirstName,
			LastName:          polled.Identity.LastName,
			CertificateSerial: polled.Identity.CertificateSerial,
			CertificateIssuer: polled.Identity.CertificateIssuer,
		}
	}
	return state, nil
}
