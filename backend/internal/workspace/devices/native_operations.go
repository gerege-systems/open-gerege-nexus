package devices

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"golang.org/x/crypto/chacha20poly1305"
)

type telemetryEvent struct {
	Level, Event string
	Payload      map[string]any
	OccurredAt   time.Time `json:"occurred_at"`
}

func (h *Handlers) HandleDeviceTelemetry(w http.ResponseWriter, r *http.Request) {
	device := r.Context().Value(deviceContextKey{}).(deviceClaims)
	var req struct {
		Events []telemetryEvent `json:"events"`
	}
	if httpx.DecodeLimited(r, &req, 256<<10) != nil || len(req.Events) == 0 || len(req.Events) > 100 {
		httpx.Error(w, 400, "invalid telemetry batch")
		return
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		httpx.Error(w, 503, "telemetry unavailable")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	for _, event := range req.Events {
		level := strings.ToUpper(event.Level)
		if level != "INFO" && level != "WARN" && level != "ERROR" || strings.TrimSpace(event.Event) == "" {
			httpx.Error(w, 400, "invalid telemetry event")
			return
		}
		if event.OccurredAt.IsZero() {
			event.OccurredAt = time.Now()
		}
		raw, _ := json.Marshal(event.Payload)
		if _, err = tx.Exec(r.Context(), `INSERT INTO workspace.device_telemetry(tenant_id,device_id,level,event,payload,occurred_at) VALUES($1,$2,$3,$4,$5,$6)`, device.TenantID, device.ID, level, event.Event, raw, event.OccurredAt); err != nil {
			httpx.Error(w, 503, "telemetry unavailable")
			return
		}
	}
	if tx.Commit(r.Context()) != nil {
		httpx.Error(w, 503, "telemetry unavailable")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func encryptPushToken(token string) (string, error) {
	keyText := os.Getenv("PUSH_TOKEN_ENCRYPTION_KEY")
	if keyText == "" {
		return "", errors.New("push token encryption key is missing")
	}
	key, err := base64.StdEncoding.DecodeString(keyText)
	if err != nil || len(key) != chacha20poly1305.KeySize {
		return "", errors.New("push token encryption key is invalid")
	}
	aead, _ := chacha20poly1305.NewX(key)
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(append(nonce, aead.Seal(nil, nonce, []byte(token), nil)...)), nil
}

func (h *Handlers) HandleRegisterPushToken(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.UserFromContext(r.Context())
	var req struct {
		Token    string `json:"token"`
		Provider string `json:"provider"`
		AppID    string `json:"app_id"`
	}
	if httpx.DecodeLimited(r, &req, 16<<10) != nil || len(req.Token) < 16 {
		httpx.Error(w, 400, "invalid push token")
		return
	}
	req.Provider = strings.ToUpper(req.Provider)
	if req.Provider != "APNS" && req.Provider != "FCM" {
		httpx.Error(w, 400, "invalid push provider")
		return
	}
	encrypted, err := encryptPushToken(req.Token)
	if err != nil {
		httpx.Error(w, 503, "push registration is not configured")
		return
	}
	_, err = h.db.Exec(r.Context(), `INSERT INTO workspace.push_tokens(tenant_id,user_id,provider,token_hash,token_ciphertext,app_id) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(token_hash) DO UPDATE SET user_id=EXCLUDED.user_id,tenant_id=EXCLUDED.tenant_id,provider=EXCLUDED.provider,token_ciphertext=EXCLUDED.token_ciphertext,app_id=EXCLUDED.app_id,updated_at=NOW()`, claims.TenantID, claims.UserID, req.Provider, caseSensitiveSecretHash(req.Token), encrypted, req.AppID)
	if err != nil {
		httpx.Error(w, 503, "push registration failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleDeviceStaffPIN signs a person in on an enrolled shared device.
//
// The sign-in is the platform's and the credential is not. What the secret is —
// a PIN today — belongs to whichever app implements nexus.StaffCredential, and
// this handler knows only that somebody typed something at a till and that a
// session may be opened for whoever it turns out to be. Minting the session is
// the part no app may do, which is why the route stayed here when the PIN left
// for internal/apps/staffpin on 2026-08-23.
//
// The route stays mounted on a deployment carrying no such app and answers 404,
// the same rule /ai/stock-forecast follows: a route table that changes shape
// with the environment is a route table nobody can reason about.
func (h *Handlers) HandleDeviceStaffPIN(w http.ResponseWriter, r *http.Request) {
	device := r.Context().Value(deviceContextKey{}).(deviceClaims)
	if device.FormFactor != "pos" && device.FormFactor != "tablet" {
		httpx.Error(w, http.StatusForbidden, "staff switching is unavailable on this device")
		return
	}
	// "pin" is the wire's history rather than this handler's opinion: the field
	// was named when the platform held the PIN itself, and the tills in the
	// field send it.
	var req struct {
		PIN string `json:"pin"`
	}
	if httpx.DecodeLimited(r, &req, 4<<10) != nil || strings.TrimSpace(req.PIN) == "" {
		httpx.Error(w, http.StatusUnauthorized, "invalid PIN")
		return
	}

	identity, err := h.staffPIN.Verify(r.Context(), device.TenantID, req.PIN)
	switch {
	case errors.Is(err, nexus.ErrStaffCredentialRejected):
		// One answer for a wrong secret, a locked credential and an
		// organisation without the app: the caller is a shared till in a shop,
		// and telling it which of the three would tell everybody standing at it.
		audit.Record(r.Context(), device.TenantID, "device:"+device.ID, "staff.pin_failed", "device", nil)
		httpx.Error(w, http.StatusUnauthorized, "invalid PIN")
		return
	case err != nil:
		// A credential that could not be read is not a credential that was
		// wrong, and answering 401 here would have somebody retyping a correct
		// PIN at a database outage.
		slog.Error("staff sign-in: the credential could not be read", "tenant_id", device.TenantID, "error", err)
		httpx.Error(w, http.StatusServiceUnavailable, "staff authentication unavailable")
		return
	}

	token, expires, err := h.authn.IssueSession(r, identity.UserID, device.TenantID, "staff-pin")
	if err != nil {
		auth.ReportSessionFailure(w, err)
		return
	}
	auth.SetSessionCookie(w, token, expires)
	audit.Record(r.Context(), device.TenantID, identity.UserID, "staff.pin_success", "device",
		map[string]any{"device_id": device.ID})
	httpx.JSON(w, http.StatusOK, map[string]any{
		"expires_at":    expires,
		"membership_id": identity.MembershipID,
		"user": map[string]any{
			"id": identity.UserID, "name": identity.Name, "email": identity.Email,
			"tenant_id": device.TenantID,
		},
	})
}
