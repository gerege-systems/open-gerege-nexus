/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package staffpin implements staff PIN credential management and verification
// for shared tills and kiosks.
package staffpin

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

var validPIN = regexp.MustCompile(`^[0-9]{4,12}$`)

var (
	ErrStaffCredentialRejected = nexus.ErrStaffCredentialRejected
	ErrInvalidPIN              = errors.New("PIN must contain 4-12 digits")
	ErrMembershipNotFound      = errors.New("membership not found")
)

// Service manages staff PINs and authenticates them on devices.
type Service struct {
	db nexus.DB
}

// NewService builds the staff PIN service and publishes StaffCredential capability.
func NewService(db nexus.DB) *Service {
	s := &Service{db: db}
	nexus.Provide[nexus.StaffCredential](s)
	return s
}

// HandleSetPIN handles the administrative endpoint to set a member's PIN.
func (s *Service) HandleSetPIN(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "sign in first")
		return
	}
	var req struct {
		MembershipID string `json:"membership_id"`
		PIN          string `json:"pin"`
	}
	if httpx.DecodeLimited(r, &req, 8<<10) != nil || !validPIN.MatchString(req.PIN) {
		httpx.Error(w, http.StatusBadRequest, "PIN must contain 4-12 digits")
		return
	}
	hash, err := auth.HashPassword(req.PIN)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to protect PIN")
		return
	}
	result, err := s.db.Exec(r.Context(),
		`INSERT INTO tenant.staff_pin_credentials(membership_id,tenant_id,pin_hash)
		 SELECT id,tenant_id,$3 FROM tenant.memberships WHERE id=$1 AND tenant_id=$2
		 ON CONFLICT(membership_id) DO UPDATE
		    SET pin_hash=EXCLUDED.pin_hash,active=true,failed_attempts=0,locked_until=NULL,updated_at=NOW()`,
		req.MembershipID, claims.TenantID, hash)
	if err != nil || result.RowsAffected() != 1 {
		httpx.Error(w, http.StatusNotFound, "membership not found")
		return
	}
	audit.Record(r.Context(), claims.TenantID, claims.UserID, "staff.pin_changed", "membership",
		map[string]any{"membership_id": req.MembershipID})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// Verify authenticates a PIN typed at a device for a given tenant.
func (s *Service) Verify(ctx context.Context, tenantID, secret string) (nexus.StaffIdentity, error) {
	if !validPIN.MatchString(secret) {
		return nexus.StaffIdentity{}, ErrStaffCredentialRejected
	}

	rows, err := s.db.Query(ctx,
		`SELECT p.membership_id::text,p.pin_hash,m.user_id::text,u.name,u.email,p.locked_until
		   FROM tenant.staff_pin_credentials p
		   JOIN tenant.memberships m ON m.id = p.membership_id
		   JOIN registry.users u ON u.id = m.user_id
		  WHERE p.tenant_id = $1 AND p.active`, tenantID)
	if err != nil {
		return nexus.StaffIdentity{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var membershipID, hash, userID, name, email string
		var lockedUntil *time.Time
		if err := rows.Scan(&membershipID, &hash, &userID, &name, &email, &lockedUntil); err != nil {
			return nexus.StaffIdentity{}, err
		}
		if lockedUntil != nil && lockedUntil.After(time.Now()) {
			continue
		}
		if !auth.CheckPasswordHash(secret, hash) {
			continue
		}
		return nexus.StaffIdentity{
			UserID:       userID,
			MembershipID: membershipID,
			Name:         name,
			Email:        email,
		}, nil
	}
	if err := rows.Err(); err != nil {
		return nexus.StaffIdentity{}, err
	}
	return nexus.StaffIdentity{}, ErrStaffCredentialRejected
}
