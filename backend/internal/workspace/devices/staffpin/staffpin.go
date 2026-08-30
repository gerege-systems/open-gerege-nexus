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
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
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
	hash, err := security.HashPIN(req.PIN)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to protect PIN")
		return
	}
	result, err := s.db.Exec(r.Context(),
		`INSERT INTO workspace.staff_pin_credentials(membership_id,tenant_id,pin_hash)
		 SELECT id,tenant_id,$3 FROM workspace.memberships WHERE id=$1 AND tenant_id=$2
		 ON CONFLICT(membership_id) DO UPDATE
		    SET pin_hash=EXCLUDED.pin_hash,active=true,failed_attempts=0,locked_until=NULL,updated_at=NOW()`,
		req.MembershipID, claims.WorkspaceID, hash)
	if err != nil || result.RowsAffected() != 1 {
		httpx.Error(w, http.StatusNotFound, "membership not found")
		return
	}
	audit.Record(r.Context(), claims.WorkspaceID, claims.UserID, "staff.pin_changed", "membership",
		map[string]any{"membership_id": req.MembershipID})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// Verify authenticates a PIN typed at a device for a given tenant.
// Түгжээний хэмжүүрүүд.
//
// Таван оролдлого нь бичих алдаанд өгөөмөр, таахад хатуу: дөрвөн оронтой PIN-ий
// арван мянган хувилбарыг арван таван минутын завсарлагатайгаар туулахад
// зуу гаруй хоног шаардана.
const (
	staffPINMaxFailures = 5
	staffPINLockFor     = 15 * time.Minute
)

// VerifyOnDevice шалгаад буруу оролдлогыг ТӨХӨӨРӨМЖ дээр тоолно.
//
// PIN дангаараа ирдэг тул аль ажилтных болохыг мэдэхгүй — буруу оролдлогыг
// credential-д онооход хэн нэгэн хажуугийн хүнийг санаатай түгжиж чадна.
// Таагаад байгаа нь касс бөгөөд тэр нь мэдэгддэг.
func (s *Service) VerifyOnDevice(ctx context.Context, tenantID, deviceID, secret string) (nexus.StaffIdentity, error) {
	var lockedUntil *time.Time
	if err := s.db.QueryRow(ctx,
		`SELECT staff_pin_locked_until FROM workspace.devices WHERE id = $1::uuid AND tenant_id = $2::uuid`,
		deviceID, tenantID).Scan(&lockedUntil); err != nil {
		return nexus.StaffIdentity{}, err
	}
	if lockedUntil != nil && lockedUntil.After(time.Now()) {
		// Түгжигдсэн касс буруу PIN-тэй ижил хариу авна: аль нь болохыг
		// хэлэх нь кассны өмнө зогсож байгаа хүнд аль хэдийн хэлсэн нь болно.
		return nexus.StaffIdentity{}, ErrStaffCredentialRejected
	}

	identity, err := s.Verify(ctx, tenantID, secret)
	if errors.Is(err, ErrStaffCredentialRejected) {
		// Тоолол ба түгжээ нэг statement-д: хоёр касс зэрэг таавал хоёулаа
		// уншаад нэгийг нь бичих уралдаан үүсэхгүй.
		if _, updateErr := s.db.Exec(ctx, `
			UPDATE workspace.devices
			   SET staff_pin_failures = staff_pin_failures + 1,
			       staff_pin_locked_until = CASE
			           WHEN staff_pin_failures + 1 >= $3 THEN NOW() + $4::interval
			           ELSE staff_pin_locked_until END,
			       updated_at = NOW()
			 WHERE id = $1::uuid AND tenant_id = $2::uuid`,
			deviceID, tenantID, staffPINMaxFailures, staffPINLockFor.String()); updateErr != nil {
			slog.Warn("could not count a failed staff PIN", "device_id", deviceID, "error", updateErr)
		}
		return nexus.StaffIdentity{}, err
	}
	if err != nil {
		return nexus.StaffIdentity{}, err
	}

	if _, err := s.db.Exec(ctx, `
		UPDATE workspace.devices
		   SET staff_pin_failures = 0, staff_pin_locked_until = NULL, updated_at = NOW()
		 WHERE id = $1::uuid AND tenant_id = $2::uuid
		   AND (staff_pin_failures <> 0 OR staff_pin_locked_until IS NOT NULL)`,
		deviceID, tenantID); err != nil {
		slog.Warn("could not clear the staff PIN failures", "device_id", deviceID, "error", err)
	}
	return identity, nil
}

func (s *Service) Verify(ctx context.Context, tenantID, secret string) (nexus.StaffIdentity, error) {
	if !validPIN.MatchString(secret) {
		return nexus.StaffIdentity{}, ErrStaffCredentialRejected
	}

	rows, err := s.db.Query(ctx,
		`SELECT p.membership_id::text,p.pin_hash,m.user_id::text,u.name,u.email,p.locked_until
		   FROM workspace.staff_pin_credentials p
		   JOIN workspace.memberships m ON m.id = p.membership_id
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
