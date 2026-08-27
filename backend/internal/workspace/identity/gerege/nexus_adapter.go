/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package gerege

import (
	"context"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// AsStateRegistry presents the ХУР client as the SDK's nexus.StateRegistry.
//
// The conversion is the point of the adapter, not an inconvenience of it: a
// module gets the record and cannot reach the endpoint, the client secret or
// the token behind it. It lives here rather than in the module that consumes it
// because the types being converted from are this package's, and this is where
// a change to them has to be noticed — the same arrangement, and the same
// sentence, as integration.AsMeetingBooker.
func AsStateRegistry(s *GeregeService) nexus.StateRegistry {
	if s == nil {
		return nil
	}
	return stateRegistry{s}
}

type stateRegistry struct{ svc *GeregeService }

func (r stateRegistry) Citizen(ctx context.Context, regNumber string) (*nexus.CitizenRecord, error) {
	info, err := r.svc.GetCitizenInfo(ctx, regNumber)
	if err != nil || info == nil {
		return nil, err
	}
	return &nexus.CitizenRecord{
		RegNumber: info.RegNumber, CivilID: info.CivilID,
		LastName: info.LastName, FirstName: info.FirstName,
		Gender: info.Gender, Address: info.Address,
		PassportStatus: info.PassportStatus, Verified: info.Verified,
	}, nil
}

func (r stateRegistry) Company(ctx context.Context, companyReg string) (*nexus.CompanyRecord, error) {
	info, err := r.svc.GetCompanyInfo(ctx, companyReg)
	if err != nil || info == nil {
		return nil, err
	}
	return &nexus.CompanyRecord{
		CompanyReg: info.CompanyReg, Name: info.Name, Executive: info.Executive,
		Address: info.Address, VatPayer: info.VatPayer, Status: info.Status,
		FoundingDate: info.FoundingDate,
	}, nil
}
