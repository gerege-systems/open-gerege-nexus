/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Domain types, states and defaults for the PDF e-signature app.
 */

package signing

import (
	"fmt"
	"net/http"
	"time"
)

// The permissions these routes are checked against. The platform's blanket app
// gate cannot express them — signing is not the same authority as uploading —
// so every handler asserts one explicitly.
//
// They read `documents.*` because these rails are part of the documents app
// now, and an app with two permission namespaces would make an administrator
// grant the right to sign twice, in two places, with no way to tell from the
// Access control screen which of them the button in front of them obeys.
//
// The mapping is one to one — esign.read → documents.read, and so on — which is
// what makes it a rename rather than a redesign, and what let migration 00056
// carry every existing grant across without an administrator being asked to
// reconstruct anything.
const (
	PermRead   = "documents.read"
	PermSign   = "documents.sign"
	PermManage = "documents.manage"
)

// Signing providers. HSM is the Gerege eSign hardware module, which stamps a
// drawn signature image server-side. EID is eID Mongolia qualified remote
// signing, where the citizen approves with PIN2 on their own device and the
// private key never leaves it.
const (
	ProviderHSM = "HSM"
	ProviderEID = "EID"
)

// Document states.
const (
	StatusPending = "PENDING"
	StatusSigned  = "SIGNED"
)

// Signing-session states. Lower case, matching the vocabulary the browser
// polls for.
const (
	SessionPending   = "pending"
	SessionCompleted = "completed"
	SessionFailed    = "failed"
	SessionExpired   = "expired"
	SessionRejected  = "rejected"
)

// Batch and batch-item states.
const (
	BatchDraft     = "DRAFT"
	BatchRunning   = "RUNNING"
	BatchCompleted = "COMPLETED"
	BatchFailed    = "FAILED"
	BatchCancelled = "CANCELLED"

	ItemPending = "PENDING"
	ItemRunning = "RUNNING"
	ItemSigned  = "SIGNED"
	ItemFailed  = "FAILED"
	ItemSkipped = "SKIPPED"
)

// Log actions and outcomes.
const (
	ActionCertCheck   = "CERT_CHECK"
	ActionSign        = "SIGN"
	ActionSignStart   = "SIGN_START"
	ActionBatchSign   = "BATCH_SIGN"
	ActionDownload    = "DOWNLOAD"
	OutcomeOK         = "OK"
	OutcomeFailed     = "FAILED"
	OutcomeRejected   = "REJECTED"
	OutcomeExpired    = "EXPIRED"
	OutcomeCancelled  = "CANCELLED"
	OutcomeUnverified = "UNVERIFIED"
)

// Default signature placement, verified live against the eSign service on A4
// (595x842pt) documents: the box top-left sits at (80,216) with the image
// anchored to the box bottom, landing the stamp under the signer block on the
// last page.
//
// The eSign service uses a TOP-LEFT coordinate origin, unlike the PDF
// specification's bottom-left, which is why these numbers look inverted next
// to anything else that measures a page.
const (
	defaultSigX      uint = 80
	defaultSigY      uint = 216
	defaultSigWidth  uint = 200
	defaultSigHeight uint = 56
	defaultSigText        = "Тоон гарын үсгээр баталгаажив."
)

const (
	// maxPDFBytes caps a decoded upload. It matches the frontend guard and the
	// edge proxy's client_max_body_size; raising one without the others turns
	// a clear "file too large" into nginx's 413 HTML page.
	maxPDFBytes = 25 << 20

	// maxUploadBody caps the multipart envelope, leaving room for base64 and
	// form overhead above the PDF limit itself.
	maxUploadBody = 34 << 20

	// sessionTTL is how long a started ceremony stays claimable.
	//
	// It is deliberately generous. An earlier version of the authentication
	// flow invented a two-minute deadline and cut citizens off who were still
	// being pushed a notification; eID keeps a push session alive far longer.
	// This is a sweep horizon for abandoned rows, not a deadline imposed on a
	// person reaching for their phone.
	sessionTTL = 20 * time.Minute

	// logPageSize is the default page for the signature log.
	logPageSize = 50
	maxPageSize = 200
)

// Document is a PDF held for signing.
type Document struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	Title           string     `json:"title"`
	FileName        string     `json:"file_name"`
	Status          string     `json:"status"`
	Provider        string     `json:"provider"`
	PageCount       uint       `json:"page_count"`
	ByteSize        int64      `json:"byte_size"`
	Checksum        string     `json:"checksum,omitempty"`
	SignerName      string     `json:"signer_name,omitempty"`
	SignerRegNo     string     `json:"signer_reg_no,omitempty"`
	SignerPhone     string     `json:"signer_phone,omitempty"`
	SignerEtsi      string     `json:"signer_etsi,omitempty"`
	OnBehalfOfName  string     `json:"on_behalf_of_name,omitempty"`
	CertificateLevl string     `json:"certificate_level,omitempty"`
	SignedAt        *time.Time `json:"signed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// SignatureLog is one auditable event. Unlike the original table this records
// failures too — a refused or expired ceremony is exactly what an auditor
// looks for, and it used to leave no trace at all.
type SignatureLog struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	DocumentID    string    `json:"document_id,omitempty"`
	DocumentTitle string    `json:"document_title,omitempty"`
	SessionID     string    `json:"session_id,omitempty"`
	Provider      string    `json:"provider"`
	Action        string    `json:"action"`
	Outcome       string    `json:"outcome"`
	RegNo         string    `json:"reg_no,omitempty"`
	PhoneNo       string    `json:"phone_no,omitempty"`
	FirstName     string    `json:"first_name,omitempty"`
	LastName      string    `json:"last_name,omitempty"`
	Detail        string    `json:"detail,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// LogQuery filters the signature log.
type LogQuery struct {
	Action     string
	Outcome    string
	Provider   string
	DocumentID string
	Search     string
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
}

// Page is the envelope every paginated listing uses.
type Page[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// SignSession is one remote-signing ceremony.
//
// The PDF bytes are held on the session rather than read back from the
// document at completion. eID's stamp endpoint must be handed exactly the
// bytes whose digest the citizen approved: hashing a document that changed
// mid-ceremony would produce a PDF whose signature does not verify, and the
// failure would only surface when somebody opened it in a reader.
type SignSession struct {
	ID               string     `json:"session_id"`
	TenantID         string     `json:"-"`
	DocumentID       string     `json:"document_id,omitempty"`
	Provider         string     `json:"provider"`
	State            string     `json:"state"`
	FailureReason    string     `json:"failure_reason,omitempty"`
	FileName         string     `json:"filename"`
	DocumentHash     string     `json:"document_hash"`
	VerificationCode string     `json:"verification_code,omitempty"`
	SignerName       string     `json:"signer_name,omitempty"`
	SignerEtsi       string     `json:"signer_etsi,omitempty"`
	OnBehalfOfEtsi   string     `json:"on_behalf_of_etsi,omitempty"`
	OnBehalfOfName   string     `json:"on_behalf_of_name,omitempty"`
	CertificateLevel string     `json:"certificate_level,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	ExpiresAt        time.Time  `json:"expires_at"`
}

// Batch is a set of documents signed in one run.
type Batch struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Provider   string      `json:"provider"`
	Status     string      `json:"status"`
	Total      int         `json:"total"`
	Signed     int         `json:"signed"`
	Failed     int         `json:"failed"`
	CreatedAt  time.Time   `json:"created_at"`
	StartedAt  *time.Time  `json:"started_at,omitempty"`
	FinishedAt *time.Time  `json:"finished_at,omitempty"`
	Items      []BatchItem `json:"items,omitempty"`
}

// BatchItem is one document within a batch.
type BatchItem struct {
	ID            string     `json:"id"`
	DocumentID    string     `json:"document_id"`
	DocumentTitle string     `json:"document_title"`
	FileName      string     `json:"file_name"`
	Position      int        `json:"position"`
	Status        string     `json:"status"`
	SessionID     string     `json:"session_id,omitempty"`
	Error         string     `json:"error,omitempty"`
	SignedAt      *time.Time `json:"signed_at,omitempty"`
}

// Placement is where the stamp lands on the page.
type Placement struct {
	X          uint   `json:"x"`
	Y          uint   `json:"y"`
	Width      uint   `json:"width"`
	Height     uint   `json:"height"`
	PageNumber uint   `json:"page_number"` // 0 = last page
	Text       string `json:"text"`
}

// DefaultPlacement returns the verified A4 placement.
func DefaultPlacement() Placement {
	return Placement{
		X: defaultSigX, Y: defaultSigY,
		Width: defaultSigWidth, Height: defaultSigHeight,
		PageNumber: 0, Text: defaultSigText,
	}
}

// normalize fills unset fields from the default placement and clamps the box
// inside an A4 page, so a bad configuration cannot push the stamp off the
// document where it would be invisible but still counted as signed.
func (p Placement) normalize() Placement {
	d := DefaultPlacement()
	if p.X == 0 {
		p.X = d.X
	}
	if p.Y == 0 {
		p.Y = d.Y
	}
	if p.Width == 0 {
		p.Width = d.Width
	}
	if p.Height == 0 {
		p.Height = d.Height
	}
	if p.Text == "" {
		p.Text = d.Text
	}
	return p
}

// validate rejects a placement that would fall outside an A4 page.
func (p Placement) validate() error {
	const (
		a4Width  uint = 595
		a4Height uint = 842
	)
	if p.Width < 40 || p.Width > a4Width {
		return &Error{Code: "INVALID_PLACEMENT", Message: "signature width must be between 40 and 595 points", Status: http.StatusBadRequest}
	}
	if p.Height < 20 || p.Height > a4Height {
		return &Error{Code: "INVALID_PLACEMENT", Message: "signature height must be between 20 and 842 points", Status: http.StatusBadRequest}
	}
	if p.X+p.Width > a4Width {
		return &Error{Code: "INVALID_PLACEMENT", Message: "the signature box extends past the right edge of an A4 page", Status: http.StatusBadRequest}
	}
	if p.Y+p.Height > a4Height {
		return &Error{Code: "INVALID_PLACEMENT", Message: "the signature box extends past the bottom edge of an A4 page", Status: http.StatusBadRequest}
	}
	if len([]rune(p.Text)) > 120 {
		return &Error{Code: "INVALID_PLACEMENT", Message: "the signature caption may not exceed 120 characters", Status: http.StatusBadRequest}
	}
	return nil
}

// HSMSettings is the tenant-visible view of the HSM connection. It carries no
// secret: the token lives in the environment, and this record is readable by
// anyone holding esign.manage.
type HSMSettings struct {
	LoginURL  string `json:"login_url"`
	SignURL   string `json:"sign_url"`
	Enabled   bool   `json:"enabled"`
	MockMode  bool   `json:"mock_mode"`
	HasToken  bool   `json:"has_token"`
	LastProbe *Probe `json:"last_probe,omitempty"`
}

// Probe is the outcome of a connection test.
type Probe struct {
	OK        bool      `json:"ok"`
	Message   string    `json:"message"`
	LatencyMs int64     `json:"latency_ms"`
	CheckedAt time.Time `json:"checked_at"`
	CheckedBy string    `json:"checked_by,omitempty"`
}

// Policy is the tenant's signing rules.
type Policy struct {
	// DefaultProvider is the rail a new signature uses unless overridden.
	DefaultProvider string `json:"default_provider"`
	// RequireEID refuses the HSM rail outright. A tenant that has moved to
	// qualified signatures uses this to stop the older rail being reachable.
	RequireEID bool `json:"require_eid"`
	// MinCertificateLevel is the lowest eID certificate level accepted.
	MinCertificateLevel string `json:"min_certificate_level"`
	// AllowOnBehalfOf permits signing in an organisation's name.
	AllowOnBehalfOf bool `json:"allow_on_behalf_of"`
	// AllowSelfSign lets the uploader sign their own upload. Tenants that need
	// separation of duties turn it off.
	AllowSelfSign bool `json:"allow_self_sign"`
	// RetentionDays is how long signed documents are kept; 0 means forever.
	RetentionDays int `json:"retention_days"`
	// MaxUploadMB caps uploads below the platform ceiling.
	MaxUploadMB int `json:"max_upload_mb"`
}

// DefaultPolicy is what a tenant gets before anyone configures anything. It
// permits both rails: an existing tenant is mid-flight on the HSM and must not
// have signing taken away by an upgrade.
func DefaultPolicy() Policy {
	return Policy{
		DefaultProvider:     ProviderEID,
		RequireEID:          false,
		MinCertificateLevel: "QUALIFIED",
		AllowOnBehalfOf:     true,
		AllowSelfSign:       true,
		RetentionDays:       0,
		MaxUploadMB:         maxPDFBytes >> 20,
	}
}

func (p Policy) normalize() Policy {
	d := DefaultPolicy()
	if p.DefaultProvider != ProviderHSM && p.DefaultProvider != ProviderEID {
		p.DefaultProvider = d.DefaultProvider
	}
	switch p.MinCertificateLevel {
	case "ADVANCED", "QUALIFIED", "QSCD":
	default:
		p.MinCertificateLevel = d.MinCertificateLevel
	}
	if p.RetentionDays < 0 {
		p.RetentionDays = 0
	}
	if p.MaxUploadMB <= 0 || p.MaxUploadMB > d.MaxUploadMB {
		p.MaxUploadMB = d.MaxUploadMB
	}
	// A policy that demands eID but names the HSM as its default would refuse
	// every signature it started. Resolve it rather than deadlocking the tenant.
	if p.RequireEID {
		p.DefaultProvider = ProviderEID
	}
	return p
}

// Settings is the tenant's full configuration.
type Settings struct {
	Placement Placement   `json:"placement"`
	HSM       HSMSettings `json:"hsm"`
	Policy    Policy      `json:"policy"`
	UpdatedAt *time.Time  `json:"updated_at,omitempty"`
}

// Error is the module's domain error. Carrying an HTTP status and a machine
// code keeps handlers free of status juggling and gives the browser something
// stable to branch on.
type Error struct {
	Code    string
	Message string
	Status  int
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func badRequest(code, message string) *Error {
	return &Error{Code: code, Message: message, Status: http.StatusBadRequest}
}

func notFound(message string) *Error {
	return &Error{Code: "NOT_FOUND", Message: message, Status: http.StatusNotFound}
}

func conflict(code, message string) *Error {
	return &Error{Code: code, Message: message, Status: http.StatusConflict}
}

func forbidden(message string) *Error {
	return &Error{Code: "FORBIDDEN", Message: message, Status: http.StatusForbidden}
}

func upstream(code, message string) *Error {
	return &Error{Code: code, Message: message, Status: http.StatusBadGateway}
}

// Actor is the authenticated caller, resolved once per request.
type Actor struct {
	UserID  string
	Email   string
	IsAdmin bool
	Perms   map[string]bool
	// Etsi is the signer's eID identity when their account is linked. Empty
	// for password and DAN accounts.
	Etsi           string
	DocumentNumber string
	FullName       string
}

func (a Actor) can(permission string) bool {
	return a.IsAdmin || a.Perms[permission]
}
