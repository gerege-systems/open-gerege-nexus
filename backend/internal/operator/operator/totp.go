package operator

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// The second factor is a time-based one-time password: RFC 6238, SHA-1, six
// digits, thirty-second steps — what every authenticator application on a phone
// already does.
//
// WebAuthn is the better answer and the plan says so. It is not here yet
// because it is a browser ceremony with its own storage, and holding CP-1 for
// it would have delayed the thing every later phase depends on. What matters is
// that the console cannot be reached with a password alone, and TOTP settles
// that today; adding WebAuthn later is another way to satisfy the same
// requireSecondFactor, not a change to anything around it.
const (
	// totpIssuer is what shows up beside the code in the authenticator app.
	totpIssuer = "Gerege Nexus Control Plane"
	// TOTPPeriod is the length of one code, in seconds. Exported so a screen's
	// test can produce the code its second factor would.
	TOTPPeriod = 30
	// totpSkew accepts the neighbouring step on either side, which covers a
	// phone whose clock is off by a few seconds. One step, not two: every step
	// of tolerance widens the window an intercepted code stays usable in.
	totpSkew = 1
)

// NewTOTPSecret returns a fresh base32 secret and the otpauth:// URI that
// enrols it.
//
// The URI is what a QR code encodes, and it carries the secret in the clear —
// so it is shown exactly once, at enrolment, and never stored anywhere but the
// operator's own row.
func NewTOTPSecret(email string) (secret, uri string, err error) {
	// 20 bytes is the SHA-1 block size and what RFC 4226 §4 recommends.
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate a TOTP secret: %w", err)
	}
	secret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	return secret, otpauthURI(email, secret), nil
}

// otpauthURI renders the enrolment URI by hand rather than through
// otp.Key.URL(), so that the issuer and account label are escaped once, here,
// and an e-mail address containing a character the format cares about cannot
// produce a URI an authenticator reads as a different account.
func otpauthURI(email, secret string) string {
	label := url.PathEscape(totpIssuer + ":" + email)
	query := url.Values{
		"secret": {secret},
		"issuer": {totpIssuer},
		"period": {fmt.Sprint(TOTPPeriod)},
		"digits": {"6"},
	}
	return "otpauth://totp/" + label + "?" + query.Encode()
}

// verifyTOTP checks a code and returns the time step it was issued for.
//
// The step is the replay defence: a code stays valid for thirty seconds, which
// is thirty seconds in which somebody who watched it typed can type it again.
// The caller stores the step and refuses anything that is not strictly later,
// so a code works once.
func verifyTOTP(secret, code string, at time.Time) (step int64, ok bool) {
	code = strings.TrimSpace(code)
	if secret == "" || code == "" {
		return 0, false
	}
	opts := totp.ValidateOpts{
		Period:    TOTPPeriod,
		Skew:      totpSkew,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	}
	// Which step the code actually belongs to has to be found by asking, since
	// the library answers only yes or no. The candidates are the accepted
	// window, oldest first, so a code that matches more than one step (which
	// cannot happen with a real secret, but must not become a way to replay a
	// code as a later step) is recorded as the earliest it could be.
	for offset := -int64(totpSkew); offset <= int64(totpSkew); offset++ {
		candidate := at.Add(time.Duration(offset) * TOTPPeriod * time.Second)
		valid, err := totp.ValidateCustom(code, secret, candidate, totp.ValidateOpts{
			Period:    opts.Period,
			Skew:      0,
			Digits:    opts.Digits,
			Algorithm: opts.Algorithm,
		})
		if err == nil && valid {
			return candidate.Unix() / TOTPPeriod, true
		}
	}
	return 0, false
}
