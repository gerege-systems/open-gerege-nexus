/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import (
	"context"
	"errors"
)

// ErrSigningUnavailable is what every call answers on an installation that
// cannot sign.
//
// An error rather than a silent no-op: a module that skipped the Enabled()
// check and got an empty session back would record a signature that was never
// asked for. It is also not the citizen's refusal — a caller may retry this one
// after an operator has configured the rail, and must not tell somebody their
// signature was rejected.
var ErrSigningUnavailable = errors.New("nexus: this installation has no signing rail")

// ErrPDFSigningUnavailable is what SignDocument answers on a rail that signs
// digests but not documents.
//
// Separate from ErrSigningUnavailable because the caller should do something
// different: a rail that cannot sign PDFs can still sign the digest of one, and
// a caller that treated the two errors alike would refuse a citizen over a
// format rather than sign what it could.
var ErrPDFSigningUnavailable = errors.New("nexus: this signing rail cannot sign PDFs")

// Signer is the installation's qualified signing rail, as a module sees it.
//
// It is a platform capability for the same reason the Өртөө ring is: an
// installation has one eID registration, one certificate level, one state store
// and one mock mode, not one per app. Two apps that each grew their own signing
// ceremony would disagree eventually — about what a session is, about what is
// checked before a signature is recorded, about what a signature covers — and
// the day they disagreed would be a day with legal consequences.
//
// See docs/adr/0002-one-signing-rail.md for the finding this exists to settle:
// one product held two different things under the word "signature", and only
// one of them was a signature over a document.
//
// # What a signature here is
//
// A ceremony over a SHA-256 digest. The citizen sees the display text, approves
// with PIN2 on their own device, and what comes back covers the digest that was
// sent — which is what makes it a signature on a document rather than a record
// that somebody approved a prompt at a moment in time. A caller that does not
// confirm the digest afterwards (VerifiedDigest) has not established that.
//
// # What is deliberately not here
//
// PDF signing, downloading the signed artifact and listing the organisations a
// citizen may sign for. Those belong to internal/workspace/signing, which is a
// platform package and holds the concrete client; publishing them for a single
// in-repository caller would be a contract carried by every distribution for
// nobody's benefit.
//
// Nor is a channel that can only approve. ДАН has no signing product on this
// platform and is not behind this interface at all: an app that offers it is
// offering an authenticated approval, and it has to say so rather than let one
// word cover both.
type Signer interface {
	// Enabled reports whether this installation can sign at all.
	//
	// False on every deployment that has not been given an eID registration
	// with the SIGNATURE permission, which is most of them. A module should ask
	// before offering a button whose only outcome would be an error.
	Enabled() bool

	// SignDigest asks a citizen to sign a digest, and returns as soon as their
	// device has been asked.
	//
	// It does not wait for the answer: the citizen has to unlock a phone, find
	// a notification and enter a PIN, which is a human amount of time and not a
	// request's. The session it returns is what Poll and VerifiedDigest name.
	SignDigest(ctx context.Context, request SignatureRequest) (SignatureSession, error)

	// PollSignature reports where a ceremony has got to.
	//
	// The registration number is the ceremony's owner and is checked, so one
	// citizen cannot reach another's session by guessing an id.
	PollSignature(ctx context.Context, ownerRegNumber, sessionID string) (SignatureState, error)

	// SignDocument asks a citizen to sign a PDF, and returns as soon as their
	// device has been asked.
	//
	// It is a separate call from SignDigest because the artifact is different,
	// not because the ceremony is: what comes back at the end is a PDF that
	// carries the signature inside it and verifies away from this platform,
	// which a digest signature cannot do. A rail that cannot sign PDFs answers
	// ErrPDFSigningUnavailable, and a caller that gets it should sign the
	// digest instead rather than refuse the citizen.
	SignDocument(ctx context.Context, request DocumentSignatureRequest) (SignatureSession, error)

	// SignedDocument is the finished PDF, once PollSignature reports completed.
	//
	// The bytes are the whole point: they are the artifact a person keeps, a
	// court reads and a verifier checks without asking this platform anything.
	SignedDocument(ctx context.Context, ownerRegNumber, sessionID string) (SignedDocument, error)

	// VerifiedDigest is the digest a completed ceremony actually signed, in
	// base64.
	//
	// This is the half that makes the signature mean something. A caller that
	// asked for one digest and is handed a session that signed another has been
	// told about a document it did not sign, and only this call can tell.
	VerifiedDigest(ctx context.Context, ownerRegNumber, sessionID string) (string, error)
}

// SignatureRequest is one ceremony, as it is asked for.
type SignatureRequest struct {
	// RegNumber is the citizen being asked, and the owner of the session that
	// results. Uppercase and trimmed: the rail compares it.
	RegNumber string
	// FullName is who the certificate is expected to belong to, as the rail
	// shows it. Optional.
	FullName string
	// DigestHex is the SHA-256 of what is being signed, hex-encoded. It is what
	// the signature covers.
	DigestHex string
	// DisplayText is the only thing the citizen sees about what they are
	// approving, so it is theirs rather than the developer's: a document's
	// title, not a request id.
	DisplayText string
	// DocumentName names the artifact in the rail's own records.
	DocumentName string
}

// DocumentSignatureRequest is one PDF ceremony.
//
// The PDF travels rather than a digest of it: the rail embeds the signature in
// the document, which it can only do with the document.
type DocumentSignatureRequest struct {
	// RegNumber is the citizen being asked, and the owner of the session.
	RegNumber string
	// FullName is who the certificate is expected to belong to, as the rail
	// shows it. Optional.
	FullName string
	// FileName names the artifact in the rail's records and in what comes back.
	FileName string
	// PDF is the document to sign. On a second signature it is the already
	// signed copy: PAdES adds signatures to a document rather than replacing
	// them, so a chain of signers signs a growing file.
	PDF []byte
	// DisplayText is the only thing the citizen sees about what they are
	// approving.
	DisplayText string
}

// SignedDocument is a finished PDF.
type SignedDocument struct {
	PDF      []byte
	FileName string
}

// SignatureSession is a started ceremony.
type SignatureSession struct {
	// SessionID names the ceremony at the rail. It is not a signature and must
	// never be recorded as one.
	SessionID string
	// VerificationCode is the number shown here and on the citizen's device, so
	// that the person approving can see they are approving this request and not
	// one that arrived at the same moment.
	VerificationCode string
}

// SignatureState is where a ceremony has got to. The strings are the rail's
// own, because a browser polls for exactly these.
type SignatureState string

const (
	// SignatureRunning — the citizen has been asked and has not answered.
	SignatureRunning SignatureState = "running"
	// SignatureCompleted — they approved, and a signature exists.
	SignatureCompleted SignatureState = "completed"
	// SignatureFailed — the rail could not finish the ceremony.
	SignatureFailed SignatureState = "failed"
	// SignatureExpired — nobody answered in time.
	SignatureExpired SignatureState = "expired"
	// SignatureRejected — the citizen refused, which is an answer and not an
	// error.
	SignatureRejected SignatureState = "rejected"
)

// Settled reports whether a ceremony has stopped moving, so a poller knows when
// to give up rather than deciding for itself which states are final.
func (s SignatureState) Settled() bool { return s != SignatureRunning }
