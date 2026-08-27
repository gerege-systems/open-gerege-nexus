/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package dan

import (
	"context"
	"errors"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// AsAuthenticator presents the ДАН client as the SDK's nexus.DANAuthenticator.
//
// ErrUnavailable is translated rather than passed through: a module comparing
// against dan.ErrUnavailable would be importing this package to read one
// sentinel, which is exactly the dependency the contract exists to remove.
func AsAuthenticator(s *DANService) nexus.DANAuthenticator {
	if s == nil {
		return nil
	}
	return authenticator{s}
}

type authenticator struct{ svc *DANService }

func (a authenticator) Mode() string { return a.svc.Mode() }

func (a authenticator) AuthenticateCitizen(ctx context.Context, regNumber, otpCode string) (*nexus.DANCitizen, error) {
	profile, err := a.svc.AuthenticateDANCitizen(ctx, regNumber, otpCode)
	if errors.Is(err, ErrUnavailable) {
		return nil, nexus.ErrIdentityRailUnavailable
	}
	if err != nil || profile == nil {
		return nil, err
	}
	return &nexus.DANCitizen{
		SessionID: profile.DANSessionID, RegNumber: profile.RegNumber,
		CivilID: profile.CivilID, LastName: profile.LastName,
		FirstName: profile.FirstName, MobileNumber: profile.MobileNumber,
		Email: profile.Email, VerifiedAt: profile.VerifiedAt,
	}, nil
}
