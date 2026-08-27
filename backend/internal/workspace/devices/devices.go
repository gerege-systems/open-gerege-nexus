package devices

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/jackc/pgx/v5"
)

const enrollmentTTL = 10 * time.Minute

type deviceContextKey struct{}
type deviceClaims struct{ ID, TenantID, Name, Platform, FormFactor string }

func opaqueSecret(bytes int) (string, error) {
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func enrollmentCode() (string, error) {
	value := make([]byte, 10)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	raw := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(value)
	return raw[:4] + "-" + raw[4:8] + "-" + raw[8:12] + "-" + raw[12:], nil
}

func secretHash(value string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), "-", ""))))
	return hex.EncodeToString(sum[:])
}

func caseSensitiveSecretHash(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func validDeviceKind(platform, formFactor string) bool {
	platforms := map[string]bool{"windows": true, "android": true, "macos": true, "ios": true}
	factors := map[string]bool{"desktop": true, "mobile": true, "tablet": true, "kiosk": true, "pos": true}
	return platforms[platform] && factors[formFactor]
}

func (h *Handlers) HandleCreateEnrollmentCode(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	code, err := enrollmentCode()
	if err != nil {
		httpx.Error(w, 500, "failed to create enrollment code")
		return
	}
	expires := time.Now().Add(enrollmentTTL)
	_, err = h.db.Exec(r.Context(), `INSERT INTO workspace.device_enrollment_codes(tenant_id,code_hash,created_by,expires_at) VALUES($1,$2,$3,$4)`, claims.WorkspaceID, secretHash(code), claims.UserID, expires)
	if err != nil {
		httpx.Error(w, 500, "failed to persist enrollment code")
		return
	}
	audit.Record(r.Context(), claims.WorkspaceID, claims.UserID, "device.enrollment_code_created", "device", nil)
	httpx.JSON(w, http.StatusCreated, map[string]any{"code": code, "expires_at": expires})
}

func (h *Handlers) HandleEnrollDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code       string `json:"code"`
		Name       string `json:"name"`
		Platform   string `json:"platform"`
		FormFactor string `json:"form_factor"`
		Site       string `json:"site"`
		AppVersion string `json:"app_version"`
		OSVersion  string `json:"os_version"`
	}
	if httpx.DecodeLimited(r, &req, 16<<10) != nil {
		httpx.Error(w, 400, "invalid enrollment request")
		return
	}
	req.Name, req.Platform, req.FormFactor = strings.TrimSpace(req.Name), strings.ToLower(strings.TrimSpace(req.Platform)), strings.ToLower(strings.TrimSpace(req.FormFactor))
	if req.Name == "" || !validDeviceKind(req.Platform, req.FormFactor) || len(strings.TrimSpace(req.Code)) < 12 {
		httpx.Error(w, 400, "invalid device identity")
		return
	}
	token, err := opaqueSecret(32)
	if err != nil {
		httpx.Error(w, 500, "failed to create device token")
		return
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		httpx.Error(w, 503, "enrollment unavailable")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var codeID, tenantID string
	err = tx.QueryRow(r.Context(), `SELECT id::text,tenant_id::text FROM public.resolve_device_enrollment($1)`, secretHash(req.Code)).Scan(&codeID, &tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, 401, "enrollment code is invalid or expired")
		return
	}
	if err != nil {
		httpx.Error(w, 503, "enrollment unavailable")
		return
	}
	if _, err = tx.Exec(r.Context(), `SELECT set_config('app.current_tenant',$1,true)`, tenantID); err != nil {
		httpx.Error(w, 503, "enrollment unavailable")
		return
	}
	var deviceID string
	err = tx.QueryRow(r.Context(), `INSERT INTO workspace.devices(tenant_id,name,platform,form_factor,site,token_hash,app_version,os_version,last_seen_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,NOW()) RETURNING id::text`, tenantID, req.Name, req.Platform, req.FormFactor, strings.TrimSpace(req.Site), secretHash(token), strings.TrimSpace(req.AppVersion), strings.TrimSpace(req.OSVersion)).Scan(&deviceID)
	if err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE workspace.device_enrollment_codes SET used_at=NOW() WHERE id=$1 AND used_at IS NULL`, codeID)
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		httpx.Error(w, 503, "enrollment could not be completed")
		return
	}
	audit.Record(r.Context(), tenantID, "device:"+deviceID, "device.enrolled", "device", map[string]any{"platform": req.Platform, "form_factor": req.FormFactor})
	httpx.JSON(w, http.StatusCreated, map[string]any{"device_id": deviceID, "device_token": token, "tenant_id": tenantID})
}

func deviceTokenFromRequest(r *http.Request) string {
	const prefix = "Device "
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(value, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(value, prefix))
	}
	if cookie, err := r.Cookie("device_token"); err == nil {
		return strings.TrimSpace(cookie.Value)
	}
	return ""
}

func (h *Handlers) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := deviceTokenFromRequest(r)
		if token == "" {
			httpx.Error(w, 401, "missing device token")
			return
		}
		var claims deviceClaims
		err := h.db.QueryRow(r.Context(), `SELECT id::text,tenant_id::text,name,platform,form_factor FROM public.authenticate_device($1)`, secretHash(token)).Scan(&claims.ID, &claims.TenantID, &claims.Name, &claims.Platform, &claims.FormFactor)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, 401, "invalid device token")
			return
		}
		if err != nil {
			httpx.Error(w, 503, "device authentication unavailable")
			return
		}
		ctx := context.WithValue(r.Context(), deviceContextKey{}, claims)
		ctx = nexus.WithWorkspaceID(ctx, claims.TenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handlers) HandleDeviceMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(deviceContextKey{}).(deviceClaims)
	if !ok {
		httpx.Error(w, 401, "unauthorized device")
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": claims.ID, "tenant_id": claims.TenantID, "name": claims.Name, "platform": claims.Platform, "form_factor": claims.FormFactor, "status": "ACTIVE"})
}

func (h *Handlers) HandleRotateDeviceToken(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(deviceContextKey{}).(deviceClaims)
	if !ok {
		httpx.Error(w, 401, "unauthorized device")
		return
	}
	token, err := opaqueSecret(32)
	if err != nil {
		httpx.Error(w, 500, "failed to rotate device token")
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE workspace.devices SET token_hash=$3,updated_at=NOW() WHERE id=$1 AND tenant_id=$2 AND status='ACTIVE'`, claims.ID, claims.TenantID, secretHash(token))
	if err != nil || result.RowsAffected() != 1 {
		httpx.Error(w, 503, "device token rotation failed")
		return
	}
	audit.Record(r.Context(), claims.TenantID, "device:"+claims.ID, "device.token_rotated", "device", nil)
	httpx.JSON(w, 200, map[string]string{"device_token": token})
}

func (h *Handlers) HandleUpdateDeviceStatus(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := nexus.RequireWorkspace(w, r)
	if !ok {
		return
	}
	var req struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if httpx.DecodeLimited(r, &req, 8<<10) != nil {
		httpx.Error(w, 400, "invalid device status")
		return
	}
	req.Status = strings.ToUpper(strings.TrimSpace(req.Status))
	if req.Status != "ACTIVE" && req.Status != "DISABLED" && req.Status != "RETIRED" {
		httpx.Error(w, 400, "invalid device status")
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE workspace.devices SET status=$3,updated_at=NOW() WHERE id=$1 AND tenant_id=$2`, req.ID, tenantID, req.Status)
	if err != nil || result.RowsAffected() != 1 {
		httpx.Error(w, 404, "device not found")
		return
	}
	httpx.JSON(w, 200, map[string]string{"status": req.Status})
}

func (h *Handlers) HandleListDevices(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := nexus.RequireWorkspace(w, r)
	if !ok {
		return
	}
	rows, err := h.db.Query(r.Context(), `SELECT id::text,name,platform,form_factor,site,status,app_version,os_version,last_seen_at,enrolled_at FROM workspace.devices WHERE tenant_id=$1 ORDER BY name`, tenantID)
	if err != nil {
		httpx.Error(w, 500, "failed to load devices")
		return
	}
	defer rows.Close()
	devices := make([]map[string]any, 0)
	for rows.Next() {
		var id, name, platform, factor, site, status, appVersion, osVersion string
		var lastSeen *time.Time
		var enrolled time.Time
		if rows.Scan(&id, &name, &platform, &factor, &site, &status, &appVersion, &osVersion, &lastSeen, &enrolled) != nil {
			httpx.Error(w, 500, "failed to read devices")
			return
		}
		devices = append(devices, map[string]any{"id": id, "name": name, "platform": platform, "form_factor": factor, "site": site, "status": status, "app_version": appVersion, "os_version": osVersion, "last_seen_at": lastSeen, "enrolled_at": enrolled})
	}
	if rows.Err() != nil {
		httpx.Error(w, 500, "failed to read devices")
		return
	}
	httpx.JSON(w, 200, map[string]any{"devices": devices})
}
