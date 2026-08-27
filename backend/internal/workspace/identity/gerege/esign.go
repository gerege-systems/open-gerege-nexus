package gerege

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
)

// EsignCertRequest is the payload for validating a citizen's digital signature
// certificate (тоон гарын үсэг) against the Gerege eSign login service.
type EsignCertRequest struct {
	PhoneNo string `json:"phoneNo"`
	CivilID string `json:"civilId"`
	Data    string `json:"data"`
}

// EsignCertResponse mirrors the eSign service certificate validation response.
type EsignCertResponse struct {
	IsValid    bool   `json:"IsValid"`
	SignedData string `json:"SignedData"`
	Signature  string `json:"Signature"`
	SubjectDN  struct {
		GivenName  string `json:"givenname"`
		Surname    string `json:"surname"`
		UID        string `json:"uid"`
		CommonName string `json:"cn"`
	} `json:"SubjectDN"`
}

// EsignExtraImage places an additional copy of the signature stamp on another page.
type EsignExtraImage struct {
	ExtraX          uint `json:"extra_x"`
	ExtraY          uint `json:"extra_y"`
	ExtraHeight     uint `json:"extra_height"`
	ExtraWidth      uint `json:"extra_width"`
	ExtraPageNumber uint `json:"extra_page_number"`
}

// EsignDocSignRequest is the PDF signing payload sent to the eSign HSM service.
// The eSign service uses a TOP-LEFT coordinate origin; (X,Y) is the top-left
// corner of the signature box, Width extends right and Height extends down, and
// the signature image is anchored to the BOTTOM of the box.
type EsignDocSignRequest struct {
	ID               string            `json:"id"`
	Passphrase       string            `json:"passphrase"`
	MSISDN           string            `json:"MSISDN"`
	X                uint              `json:"x"`
	Y                uint              `json:"y"`
	Width            uint              `json:"width"`
	Height           uint              `json:"height"`
	PageNumber       uint              `json:"page_number"`
	SignatureText    string            `json:"signature_text"`
	Pdf64            string            `json:"pdf64"`
	SignatureImage64 string            `json:"signature_image64"`
	ExtraImages      []EsignExtraImage `json:"extra_images"`
}

type esignDocSignResponse struct {
	Base64pdf string `json:"base64pdf"`
	Message   string `json:"message"`
}

// EsignService is the platform client for the Gerege eSign HSM: certificate
// validation and PKCS#7 PDF digital signing. The private signing key never
// leaves the eSign service's HSM — this client only exchanges base64 PDFs.
type EsignService struct {
	loginURL string
	signURL  string
	token    string
	mockMode bool
	client   *http.Client
}

func NewEsignService() *EsignService {
	loginURL := os.Getenv("ESIGN_LOGIN_URL")
	if loginURL == "" {
		loginURL = "https://hsm.gerege.mn/esign/login"
	}
	signURL := os.Getenv("ESIGN_SIGN_URL")
	if signURL == "" {
		signURL = "https://hsm.gerege.mn/signer/signpdf"
	}
	// Mock mode is never implicit in production — the same rule the E-ID, DAN
	// and XYP connectors already follow.
	//
	// The previous rule was `os.Getenv("ESIGN_MOCK_MODE") != "false"`, i.e. mock
	// on unless explicitly disabled, and the production compose file sets no
	// ESIGN_* variables at all. Mock SignPDF returns the submitted PDF
	// unchanged, so every document signed on the deployed instance was stored
	// with status SIGNED and byte-identical, entirely unsigned content. A
	// signature that silently is not one is worse than a signing error: the
	// error is visible, the fake is not.
	mock := config.MockEnabled("ESIGN_MOCK_MODE")
	token := os.Getenv("ESIGN_TOKEN")

	return &EsignService{
		loginURL: loginURL,
		signURL:  signURL,
		token:    token,
		mockMode: mock,
		// Signing large PDFs through the HSM can take tens of seconds.
		client: &http.Client{Timeout: 90 * time.Second},
	}
}

// CheckCertificate validates a digital signature certificate via the eSign login
// service and verifies the certificate UID matches the supplied civil ID.
func (s *EsignService) CheckCertificate(ctx context.Context, req EsignCertRequest) (*EsignCertResponse, error) {
	return telemetry.ObserveExternalValue(ctx, telemetry.SystemESign, "check_certificate",
		func(ctx context.Context) (*EsignCertResponse, error) { return s.checkCertificate(ctx, req) })
}

func (s *EsignService) checkCertificate(ctx context.Context, req EsignCertRequest) (*EsignCertResponse, error) {
	if req.PhoneNo == "" || req.CivilID == "" {
		return nil, errors.New("phone number and civil ID are required")
	}

	if s.mockMode {
		res := &EsignCertResponse{IsValid: true, Signature: "MOCK_SIGNATURE"}
		res.SubjectDN.GivenName = "Болд"
		res.SubjectDN.Surname = "Бат"
		res.SubjectDN.UID = req.CivilID
		res.SubjectDN.CommonName = "Бат Болд"
		return res, nil
	}

	var res EsignCertResponse
	if err := s.post(ctx, s.loginURL, req, &res); err != nil {
		return nil, fmt.Errorf("eSign certificate validation failed: %w", err)
	}
	if !res.IsValid {
		return nil, errors.New("digital signature certificate is invalid or not registered")
	}
	if res.SubjectDN.UID != req.CivilID {
		return nil, errors.New("certificate UID does not match the supplied civil ID")
	}
	return &res, nil
}

// SignPDF submits a base64 PDF to the eSign HSM for PKCS#7 digital signing and
// returns the signed PDF as base64.
func (s *EsignService) SignPDF(ctx context.Context, req EsignDocSignRequest) (string, error) {
	return telemetry.ObserveExternalValue(ctx, telemetry.SystemESign, "sign_pdf",
		func(ctx context.Context) (string, error) { return s.signPDF(ctx, req) })
}

func (s *EsignService) signPDF(ctx context.Context, req EsignDocSignRequest) (string, error) {
	if req.Pdf64 == "" {
		return "", errors.New("empty PDF payload")
	}

	if s.mockMode {
		// No HSM available: return the document unchanged so the demo flow completes.
		return req.Pdf64, nil
	}

	var res esignDocSignResponse
	if err := s.post(ctx, s.signURL, req, &res); err != nil {
		return "", fmt.Errorf("eSign PDF signing failed: %w", err)
	}
	if res.Base64pdf == "" {
		msg := res.Message
		if msg == "" {
			msg = "eSign service returned an empty signed PDF"
		}
		return "", errors.New(msg)
	}
	return res.Base64pdf, nil
}

func (s *EsignService) post(ctx context.Context, url string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		httpReq.Header.Set("Authorization", "Basic "+s.token)
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errRes struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &errRes) == nil && errRes.Message != "" {
			return errors.New(errRes.Message)
		}
		return fmt.Errorf("eSign service returned status %d", resp.StatusCode)
	}
	return json.Unmarshal(respBody, out)
}

// PDFPageCount extracts the page count from raw PDF bytes so signatures can be
// placed on the last page regardless of how long the document is.
func PDFPageCount(pdfData []byte) uint {
	re := regexp.MustCompile(`/Type\s*/Page[^s]`)
	if matches := re.FindAll(pdfData, -1); len(matches) > 0 {
		return uint(len(matches))
	}
	reCount := regexp.MustCompile(`/Count\s+(\d+)`)
	if m := reCount.FindSubmatch(pdfData); len(m) > 1 {
		if n, err := strconv.Atoi(string(m[1])); err == nil && n > 0 {
			return uint(n)
		}
	}
	return 1
}
