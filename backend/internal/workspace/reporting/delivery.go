/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Getting a finished report to the people who asked for it.
 */

package reporting

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/credentials"
)

// ErrDeliveryNotConfigured is what a scheduled run reports when this deployment
// has no way to send mail. The run still happens and the outcome is still
// recorded — "produced, nowhere to send it" is a different and more useful
// state than "did not run".
var ErrDeliveryNotConfigured = errors.New("no mail transport is configured for scheduled reports")

// Deliverer sends a finished export.
//
// An interface with one implementation, because the implementation is the part
// most likely to be replaced: a deployment behind a corporate relay, one with a
// transactional provider's API, one that would rather drop files on a share.
// The scheduler holds this and knows none of that.
type Deliverer interface {
	Deliver(ctx context.Context, to []string, subject, body, filename string, attachment []byte) error
}

// A note on why this is SMTP and not the platform's own mail path.
//
// The design document said scheduled reports would go out "through the existing
// hosted email service". They cannot: that service (internal/workspace/emailverify)
// sends one thing, a verification link to an address it is asked to prove, and
// its API accepts an address and a return URL. There is no endpoint for a
// subject, a body or an attachment, and inventing one on this side would mean
// sending a link nobody can follow.
//
// So a scheduled report needs a mail transport of its own, and SMTP is the one
// every deployment already has an answer for. It is off unless configured, and
// a schedule on a deployment without it records "delivery not configured"
// rather than failing silently.

// SMTPDeliverer sends over SMTP, configured from the environment.
type SMTPDeliverer struct{}

// NewSMTPDeliverer returns the deliverer, or nil when no relay is configured.
// A nil Deliverer is handled by the scheduler; it is not an error at startup.
//
// Asked again rather than remembered: the relay is a credential an operator can
// set from the console now, and a deployment that had none when it booted would
// otherwise go on producing undelivered reports until somebody restarted it.
func NewSMTPDeliverer() Deliverer {
	if smtpURL() == "" {
		return nil
	}
	return &SMTPDeliverer{}
}

// smtpURL is the relay: what the console holds, then the environment.
func smtpURL() string { return strings.TrimSpace(credentials.Get(credentials.ReportSMTPURL)) }

// smtpTimeout bounds a delivery. A relay that has stopped answering must not
// hold the scheduler's goroutine into the next minute's tick.
const smtpTimeout = 30 * time.Second

// Deliver sends one message with one attachment.
//
// The message is assembled here rather than with a library: it is one
// multipart body with two parts, and the alternative is a dependency whose API
// surface is larger than this function.
func (d *SMTPDeliverer) Deliver(ctx context.Context, to []string, subject, body, filename string, attachment []byte) error {
	config, err := smtpConfigFromStore()
	if err != nil {
		return err
	}
	if len(to) == 0 {
		return errors.New("a scheduled report has no recipients")
	}

	message := buildMessage(config.from, to, subject, body, filename, attachment)

	dialer := &net.Dialer{Timeout: smtpTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", config.address)
	if err != nil {
		return fmt.Errorf("reach the mail relay: %w", err)
	}
	defer func() { _ = conn.Close() }()
	// The deadline covers the whole conversation, not just the dial: a relay
	// that accepts a connection and then stops reading is the failure that
	// hangs, and DialContext's timeout does not reach it.
	_ = conn.SetDeadline(time.Now().Add(smtpTimeout))

	client, err := smtp.NewClient(conn, config.host)
	if err != nil {
		return fmt.Errorf("start the SMTP session: %w", err)
	}
	defer func() { _ = client.Quit() }()

	if config.startTLS {
		if err := client.StartTLS(&tls.Config{ServerName: config.host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("start TLS with the mail relay: %w", err)
		}
	}
	if config.username != "" {
		if err := client.Auth(smtp.PlainAuth("", config.username, config.password, config.host)); err != nil {
			return fmt.Errorf("authenticate with the mail relay: %w", err)
		}
	}
	if err := client.Mail(config.from); err != nil {
		return fmt.Errorf("set the sender: %w", err)
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("set the recipient %s: %w", recipient, err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("open the message body: %w", err)
	}
	if _, err := writer.Write(message); err != nil {
		return fmt.Errorf("write the message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish the message: %w", err)
	}
	return nil
}

type smtpConfig struct {
	address  string
	host     string
	from     string
	username string
	password string
	startTLS bool
}

// smtpConfigFromStore reads the relay URL, in the shape everything else uses:
//
//	smtp://user:password@relay.example.mn:587
//	smtps://user:password@relay.example.mn:465   (implicit TLS is not supported)
//
// A URL rather than five variables, because five variables is five things to
// get wrong separately and this is one string an operator copies from their
// mail provider's page.
func smtpConfigFromStore() (smtpConfig, error) {
	raw := smtpURL()
	if raw == "" {
		return smtpConfig{}, ErrDeliveryNotConfigured
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return smtpConfig{}, fmt.Errorf("REPORT_SMTP_URL is not a URL: %w", err)
	}
	if parsed.Scheme != "smtp" {
		return smtpConfig{}, fmt.Errorf("REPORT_SMTP_URL must start with smtp:// (implicit TLS on 465 is not supported; use STARTTLS on 587)")
	}

	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		port = "587"
	}

	config := smtpConfig{
		address:  net.JoinHostPort(host, port),
		host:     host,
		from:     strings.TrimSpace(os.Getenv("REPORT_MAIL_FROM")),
		startTLS: true,
	}
	if config.from == "" {
		return smtpConfig{}, errors.New("REPORT_MAIL_FROM is required alongside REPORT_SMTP_URL")
	}
	if parsed.User != nil {
		config.username = parsed.User.Username()
		config.password, _ = parsed.User.Password()
	}
	// An operator running a relay on the same host with no TLS says so
	// explicitly rather than having it inferred from a missing password.
	if strings.EqualFold(strings.TrimSpace(os.Getenv("REPORT_SMTP_STARTTLS")), "false") {
		config.startTLS = false
	}
	return config, nil
}

func buildMessage(from string, to []string, subject, body, filename string, attachment []byte) []byte {
	const boundary = "gerege-nexus-report-boundary"

	var message bytes.Buffer
	fmt.Fprintf(&message, "From: %s\r\n", from)
	fmt.Fprintf(&message, "To: %s\r\n", strings.Join(to, ", "))
	// Encoded, because every subject this platform sends is in Mongolian and a
	// raw non-ASCII header is discarded or mangled by half the relays in the
	// world.
	fmt.Fprintf(&message, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&message, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	message.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&message, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", boundary)

	fmt.Fprintf(&message, "--%s\r\n", boundary)
	message.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	message.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	message.WriteString(wrapBase64(base64.StdEncoding.EncodeToString([]byte(body))))

	fmt.Fprintf(&message, "\r\n--%s\r\n", boundary)
	fmt.Fprintf(&message, "Content-Type: application/octet-stream; name=%q\r\n", filename)
	message.WriteString("Content-Transfer-Encoding: base64\r\n")
	fmt.Fprintf(&message, "Content-Disposition: attachment; filename=%q\r\n\r\n", filename)
	message.WriteString(wrapBase64(base64.StdEncoding.EncodeToString(attachment)))

	fmt.Fprintf(&message, "\r\n--%s--\r\n", boundary)
	return message.Bytes()
}

// wrapBase64 breaks the payload into 76-character lines, which RFC 2045
// requires and which some relays enforce by rejecting the message.
func wrapBase64(encoded string) string {
	const width = 76
	var wrapped strings.Builder
	for start := 0; start < len(encoded); start += width {
		end := start + width
		if end > len(encoded) {
			end = len(encoded)
		}
		wrapped.WriteString(encoded[start:end])
		wrapped.WriteString("\r\n")
	}
	return wrapped.String()
}
