/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import (
	"context"
	"errors"
	"time"
)

// Filing a document, and watching what happens to it.
//
// Documents and signing stay in the platform rather than becoming a
// distribution, and the module's dependencies are the argument: it reaches into
// the eID rail, the DAN rail, the HSM, the national registry client, quota, the
// async runner. Extracting it would drag half the platform into the SDK and
// freeze it there as a contract. It is not an app that happens to live in the
// core; it is a platform capability wearing an app's clothes — the same shape
// as reporting, where the engine is platform and `reports` is only the screens.
//
// So the capability is published here instead, and any module can use it: a
// government decision that has to be signed by two officers, an invoice a
// customer has to countersign, a permit. That is what makes it a platform
// capability rather than an app somebody remembered to install.
//
// # What is deliberately not here
//
// Signing. There is no Sign method and there should not be: a signature is an
// interactive ceremony — the citizen's eID app, a QR code, a one-time code from
// DAN, a card in an HSM — performed by a person in front of a screen, not by a
// module in a request handler. A module that could "sign" would be a module
// that could sign as somebody else.
//
// What a module does instead is file the document and say what kind it is. How
// many signatures that kind needs is the tenant's signature policy, not the
// caller's opinion, so it is not a field here — the same document filed by the
// same module needs two signatures at one organisation and one at another, and
// neither of them is the module's business.
//
// Nor is retention, renaming, rejection or the approval chain's own screens.
// Those are the documents app, and a module reaching for them is a module
// reimplementing an app somebody can already open.
type DocumentFiler interface {
	// File records a document for this organisation and returns it as filed.
	//
	// It fails when the tenant has not installed the documents app: the
	// capability is the platform's, but the tables and the screens belong to an
	// installation, and filing into an app nobody has is how a record ends up
	// somewhere its owner cannot see it.
	File(ctx context.Context, workspaceID string, draft DocumentDraft) (FiledDocument, error)

	// Document returns one filed document, for a module following what became
	// of something it filed.
	Document(ctx context.Context, workspaceID, documentID string) (FiledDocument, error)
}

// DocumentDraft is what a module knows when it files something.
//
// Two fields. Everything else about a document — how many signatures it needs,
// how long it is kept, who may read it — is decided by the organisation's
// policy and by the platform, which is the point of filing it here rather than
// keeping a table of one's own.
type DocumentDraft struct {
	// Title is what a person will see in the list. Required.
	Title string
	// Type selects the tenant's signature and retention policy: CONTRACT,
	// REQUEST or APPROVAL. Empty takes the platform's default.
	Type string
}

// FiledDocument is a document as a module sees it: enough to show a link and a
// state, and no more. The signature hashes, the signer's registry number and
// the full ceremony history stay inside the platform.
type FiledDocument struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Type   string `json:"type"`
	Status string `json:"status"`
	// SignatureCount and RequiredSignatures are the progress a caller can
	// render without asking what a signature is.
	SignatureCount     int        `json:"signature_count"`
	RequiredSignatures int        `json:"required_signatures"`
	SignedAt           *time.Time `json:"signed_at,omitempty"`
}

// Signed reports whether the document has every signature it was asked for.
//
// A helper rather than a field, because the answer is derived and a stored
// boolean is a second source of truth that eventually disagrees with the count
// beside it.
func (d FiledDocument) Signed() bool {
	return d.RequiredSignatures > 0 && d.SignatureCount >= d.RequiredSignatures
}

// ErrNoDocumentFiler is returned by Documents when this binary has no documents
// module. A distribution built without it can still compile against the
// contract; it just cannot file.
var ErrNoDocumentFiler = errors.New("nexus: this deployment has no document store")

// UseDocumentFiler installs the capability.
//
// Deprecated: use Provide[DocumentFiler] instead. This is a wrapper over it and
// behaves identically, including UseDocumentFiler(nil) to withdraw. It stays
// for one major version so a distribution pinned to v1 keeps compiling, and
// goes in v2 — see docs/MODULES.md.
func UseDocumentFiler(filer DocumentFiler) { Provide[DocumentFiler](filer) }

// Documents returns the document capability, or ErrNoDocumentFiler.
//
// An error rather than a nil interface: a module that forgets to check nil gets
// a panic at the first filing, and a module that forgets to check an error gets
// a vet warning. The failure should be loud where it is written, not where it
// is used. Capability answers the same way and this calls it; the sentinel is
// kept because callers written against v1 may compare against it.
func Documents() (DocumentFiler, error) {
	filer, err := Capability[DocumentFiler]()
	if err != nil {
		return nil, ErrNoDocumentFiler
	}
	return filer, nil
}
