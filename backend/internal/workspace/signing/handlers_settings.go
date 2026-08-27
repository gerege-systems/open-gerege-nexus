/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Tenant configuration: stamp placement, HSM connection and signing policy.
 * Backs the three screens under Тохиргоо.
 */

package signing

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/gerege"
)

func (m *Rails) getSettingsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := m.require(w, r, PermRead)
	if !ok {
		return
	}
	settings, err := m.settings(r, tenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	nexus.JSON(w, http.StatusOK, settings)
}

func (m *Rails) settings(r *http.Request, tenantID string) (*Settings, error) {
	placement, policy, updatedAt, err := m.store.loadSettings(r.Context(), tenantID)
	if err != nil {
		return nil, err
	}
	probe, err := m.store.loadProbe(r.Context(), tenantID)
	if err != nil {
		return nil, err
	}
	return &Settings{
		Placement: placement,
		Policy:    policy,
		HSM:       m.hsmSettings(probe),
		UpdatedAt: updatedAt,
	}, nil
}

// hsmSettings reports the connection as the process actually has it. The
// endpoints and mode come from the environment rather than the database
// because they are deployment facts, not tenant preferences — and reporting a
// stored value the process is not using would make this screen a liar.
//
// The token is never returned; only whether one is present.
func (m *Rails) hsmSettings(probe *Probe) HSMSettings {
	loginURL := os.Getenv("ESIGN_LOGIN_URL")
	if loginURL == "" {
		loginURL = "https://hsm.gerege.mn/esign/login"
	}
	signURL := os.Getenv("ESIGN_SIGN_URL")
	if signURL == "" {
		signURL = "https://hsm.gerege.mn/signer/signpdf"
	}
	mock := os.Getenv("ESIGN_MOCK_MODE") != "false"
	token := strings.TrimSpace(os.Getenv("ESIGN_TOKEN"))

	return HSMSettings{
		LoginURL:  loginURL,
		SignURL:   signURL,
		MockMode:  mock,
		HasToken:  token != "",
		Enabled:   mock || token != "",
		LastProbe: probe,
	}
}

func (m *Rails) updatePlacementHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermManage)
	if !ok {
		return
	}
	var req Placement
	if err := decodeJSON(r, &req); err != nil {
		writeDomainError(w, err)
		return
	}
	req = req.normalize()
	if err := req.validate(); err != nil {
		writeDomainError(w, err)
		return
	}
	if err := m.store.saveSettings(r.Context(), tenantID, actor.UserID, "placement", req); err != nil {
		writeDomainError(w, err)
		return
	}
	nexus.Audit(r.Context(), tenantID, actor.UserID, "esign.placement_updated", "esign", map[string]any{
		"x": req.X, "y": req.Y, "width": req.Width, "height": req.Height, "page": req.PageNumber,
	})
	nexus.JSON(w, http.StatusOK, req)
}

func (m *Rails) updatePolicyHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermManage)
	if !ok {
		return
	}
	var req Policy
	if err := decodeJSON(r, &req); err != nil {
		writeDomainError(w, err)
		return
	}
	// Refusing the HSM rail with no eID connection configured would leave the
	// tenant unable to sign anything at all.
	if req.RequireEID && !m.eid.Enabled() {
		writeDomainError(w, badRequest("EID_NOT_CONFIGURED",
			"eID Mongolia signing is not configured, so it cannot be made mandatory"))
		return
	}
	req = req.normalize()
	if err := m.store.saveSettings(r.Context(), tenantID, actor.UserID, "policy", req); err != nil {
		writeDomainError(w, err)
		return
	}
	nexus.Audit(r.Context(), tenantID, actor.UserID, "esign.policy_updated", "esign", map[string]any{
		"default_provider": req.DefaultProvider, "require_eid": req.RequireEID,
		"allow_self_sign": req.AllowSelfSign, "retention_days": req.RetentionDays,
	})
	nexus.JSON(w, http.StatusOK, req)
}

// testHSMHandler probes the eSign service so an operator can tell a
// misconfigured endpoint from a broken document, which are otherwise
// indistinguishable at the moment somebody tries to sign.
func (m *Rails) testHSMHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermManage)
	if !ok {
		return
	}

	started := time.Now()
	probe := Probe{CheckedAt: started, CheckedBy: actor.Email}

	// The certificate endpoint is the cheapest call that exercises URL,
	// credentials and connectivity together. It is called with a deliberately
	// invalid subject: a rejection proves the service answered, which is what
	// is being tested — nobody's certificate is being checked here.
	_, err := m.hsm.CheckCertificate(r.Context(), gerege.EsignCertRequest{
		PhoneNo: "00000000", CivilID: "CONNECTION-TEST",
	})
	probe.LatencyMs = time.Since(started).Milliseconds()

	switch {
	case config.MockEnabled("ESIGN_MOCK_MODE"):
		probe.OK = true
		probe.Message = "The eSign service is in mock mode; no live HSM was contacted."
	case err == nil:
		probe.OK = true
		probe.Message = "The eSign service answered successfully."
	case isReachableRejection(err):
		// A refusal is a successful round trip. Treating it as a failure would
		// tell an operator their endpoint is wrong when it is working.
		probe.OK = true
		probe.Message = "The eSign service is reachable and rejected the test subject as expected."
	default:
		probe.OK = false
		probe.Message = err.Error()
	}

	// The probe is stored alongside the connection so the screen can show when
	// it was last known good, rather than only at the moment somebody looks.
	if saveErr := m.store.saveSettings(r.Context(), tenantID, actor.UserID, "hsm", map[string]any{
		"last_probe": probe,
	}); saveErr != nil {
		writeDomainError(w, saveErr)
		return
	}

	nexus.Audit(r.Context(), tenantID, actor.UserID, "esign.hsm_probed", "esign", map[string]any{
		"ok": probe.OK, "latency_ms": probe.LatencyMs,
	})
	nexus.JSON(w, http.StatusOK, probe)
}

// isReachableRejection distinguishes "the service said no" from "the service
// could not be reached". Only the latter is a connection failure.
//
// The markers are deliberately narrow. Matching a bare "certificate" looked
// reasonable and was wrong: every eSign refusal says "certificate is invalid
// or not registered", so a working endpoint reported itself unreachable. The
// transport markers here are phrases Go's net stack emits and an application
// refusal does not.
func isReachableRejection(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, transport := range []string{
		"no such host", "connection refused", "connection reset",
		"deadline exceeded", "handshake timeout", "i/o timeout",
		"tls handshake", "x509:", "network is unreachable",
		"no route to host", "unexpected eof",
	} {
		if strings.Contains(msg, transport) {
			return false
		}
	}
	return true
}
