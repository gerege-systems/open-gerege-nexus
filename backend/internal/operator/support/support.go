package support

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/mailrail"
	"github.com/jackc/pgx/v5"
)

// The help desk (§3.B).
//
// Everything here is about somebody's *access*, never about their data: find
// the account, unlock it, end its sessions, send them a way back in. An
// operator using every function in this file learns a person's name, their
// address and which organisations they belong to — which is what was already
// on the tenant detail page — and nothing else. Reading what they keep on the
// platform is impersonation, and it lives in its own file with its own
// ceremony.
//
// The console cannot change a password, either. It sends a link; the person
// chooses the password themselves, on the platform's own hostname. That is the
// difference between helping somebody back in and being able to become them.
// The database enforces it: migration 00050 grants the console UPDATE on two
// named columns of `users` — the lockout counter and its expiry — and on
// nothing else.

// CredentialLinkTTL is how long an invitation or a reset stays usable.
//
// A day rather than an hour: these are sent to people who are locked out,
// often at the end of an afternoon, and a link that expires while they are
// asleep produces a second support request rather than a signed-in person.
const CredentialLinkTTL = 24 * time.Hour

var (
	// ErrUserNotFound is an account that is not there.
	ErrUserNotFound = errors.New("no such person")
	// ErrMailNotConfigured is a deployment that cannot send anything.
	ErrMailNotConfigured = errors.New("this deployment has no way to send mail")
)

// Person is one account, as the support screen shows it.
type Person struct {
	ID           string             `json:"id"`
	Email        string             `json:"email"`
	Name         string             `json:"name"`
	LockedUntil  *time.Time         `json:"locked_until"`
	FailedLogins int                `json:"failed_logins"`
	Sessions     int                `json:"sessions"`
	Memberships  []PersonMembership `json:"memberships"`
}

// PersonMembership is one organisation somebody belongs to.
type PersonMembership struct {
	TenantID   string   `json:"tenant_id"`
	TenantName string   `json:"tenant_name"`
	TenantSlug string   `json:"tenant_slug"`
	Roles      []string `json:"roles"`
	Suspended  bool     `json:"suspended"`
}

// peoplePageSize bounds a search. Support looks somebody up by address; a
// query that matches two hundred people is a query that needs narrowing, not a
// longer page.
const peoplePageSize = 50

// FindPeople searches by address or name.
//
// Deliberately not by anything else. A search that also matched, say, a
// telephone number or a registration number would turn the support screen into
// a way of finding out who somebody is — the console's remit is to help people
// who have already identified themselves to whoever is helping them.
func (s *Service) FindPeople(ctx context.Context, query string) ([]Person, error) {
	query = strings.TrimSpace(query)
	if len(query) < 3 {
		// Not an error: an empty search must not list every account on the
		// deployment, and "type three characters" is what the screen says.
		return []Person{}, nil
	}

	ctx, cancel := context.WithTimeout(operator.Scoped(ctx), operator.QueryTimeout)
	defer cancel()

	rows, err := s.db.Query(ctx,
		`SELECT u.id::text, u.email, u.name, u.locked_until, u.failed_login_attempts,
		        (SELECT count(*) FROM tenant.sessions s
		          WHERE s.user_id = u.id AND s.revoked_at IS NULL AND s.expires_at > NOW())
		   FROM platform.users u
		  WHERE u.email ILIKE '%' || $1 || '%' OR u.name ILIKE '%' || $1 || '%'
		  ORDER BY u.email
		  LIMIT $2`, query, peoplePageSize)
	if err != nil {
		return nil, fmt.Errorf("control plane: search for people: %w", err)
	}
	defer rows.Close()

	people := make([]Person, 0, 8)
	ids := make([]string, 0, 8)
	for rows.Next() {
		var person Person
		if err := rows.Scan(&person.ID, &person.Email, &person.Name,
			&person.LockedUntil, &person.FailedLogins, &person.Sessions); err != nil {
			return nil, fmt.Errorf("control plane: read a person: %w", err)
		}
		person.Memberships = []PersonMembership{}
		people = append(people, person)
		ids = append(ids, person.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(people) == 0 {
		return people, nil
	}

	// One query for every membership rather than one per person: a support
	// screen showing fifty results would otherwise make fifty-one round trips.
	memberships, err := s.membershipsFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range people {
		if found := memberships[people[i].ID]; found != nil {
			people[i].Memberships = found
		}
	}
	return people, nil
}

func (s *Service) membershipsFor(ctx context.Context, userIDs []string) (map[string][]PersonMembership, error) {
	rows, err := s.db.Query(ctx,
		`SELECT m.user_id::text, t.id::text, t.name, t.slug, t.suspended_at IS NOT NULL,
		        COALESCE(ARRAY_AGG(r.code ORDER BY r.code) FILTER (WHERE r.code IS NOT NULL), '{}')
		   FROM tenant.memberships m
		   JOIN platform.tenants t ON t.id = m.tenant_id
		   LEFT JOIN tenant.membership_roles mr ON mr.membership_id = m.id
		   LEFT JOIN tenant.roles r ON r.id = mr.role_id AND r.tenant_id = m.tenant_id
		  WHERE m.user_id = ANY($1::uuid[])
		  GROUP BY m.user_id, t.id, t.name, t.slug, t.suspended_at
		  ORDER BY t.name`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("control plane: read the memberships: %w", err)
	}
	defer rows.Close()

	byUser := make(map[string][]PersonMembership, len(userIDs))
	for rows.Next() {
		var userID string
		var membership PersonMembership
		if err := rows.Scan(&userID, &membership.TenantID, &membership.TenantName,
			&membership.TenantSlug, &membership.Suspended, &membership.Roles); err != nil {
			return nil, fmt.Errorf("control plane: read a membership: %w", err)
		}
		byUser[userID] = append(byUser[userID], membership)
	}
	return byUser, rows.Err()
}

// person reads one account, for the "before" of a change.
func (s *Service) person(ctx context.Context, userID string) (Person, error) {
	var found Person
	err := s.db.QueryRow(operator.Scoped(ctx),
		`SELECT id::text, email, name, locked_until, failed_login_attempts
		   FROM platform.users WHERE id = $1::uuid`, userID).
		Scan(&found.ID, &found.Email, &found.Name, &found.LockedUntil, &found.FailedLogins)
	if errors.Is(err, pgx.ErrNoRows) {
		return Person{}, ErrUserNotFound
	}
	if err != nil {
		if operator.IsInvalidUUID(err) {
			return Person{}, ErrUserNotFound
		}
		return Person{}, fmt.Errorf("control plane: read the person: %w", err)
	}
	return found, nil
}

// Unlock clears a login lockout.
//
// The columns this writes are the only two of `users` the console's database
// role may write at all. A handler here that tried to change a password would
// be refused by PostgreSQL, which is a better guarantee than this comment.
func (s *Service) Unlock(ctx context.Context, sess operator.Session, userID, reason string) error {
	before, err := s.person(ctx, userID)
	if err != nil {
		return err
	}
	return s.op.Do(ctx, sess, operator.Change{
		Action:     "user.unlock",
		TargetType: "user",
		TargetID:   userID,
		Reason:     reason,
		Before:     map[string]any{"locked_until": before.LockedUntil, "failed_logins": before.FailedLogins},
	}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`UPDATE platform.users SET failed_login_attempts = 0, locked_until = NULL WHERE id = $1::uuid`,
			userID)
		return err
	})
}

// RevokeSessions ends every session somebody holds, everywhere.
//
// What a person needs after leaving themselves signed in on a machine they no
// longer have, and what an operator needs the moment an account is suspected.
func (s *Service) RevokeSessions(ctx context.Context, sess operator.Session, userID, reason string) (int64, error) {
	before, err := s.person(ctx, userID)
	if err != nil {
		return 0, err
	}
	var ended int64
	err = s.op.Do(ctx, sess, operator.Change{
		Action:     "user.sessions.revoke",
		TargetType: "user",
		TargetID:   userID,
		Reason:     reason,
		Before:     map[string]any{"email": before.Email},
	}, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE tenant.sessions SET revoked_at = NOW()
			  WHERE user_id = $1::uuid AND revoked_at IS NULL AND expires_at > NOW()`, userID)
		if err != nil {
			return fmt.Errorf("end the sessions: %w", err)
		}
		ended = tag.RowsAffected()
		return nil
	})
	return ended, err
}

// SendCredentialLink invites somebody in, or lets them back in.
//
// One mechanism for both, because they are the same act: prove you hold this
// address, then choose a password. The difference is one word in the audit
// trail and one word in the mail.
//
// The link goes through the platform's existing verification service, which is
// the only mail rail this deployment has. That service's job is to prove an
// address, and proving the address is exactly the precondition for setting a
// password on the account that carries it.
func (s *Service) SendCredentialLink(ctx context.Context, sess operator.Session, userID, tenantID, purpose, reason string) error {
	if purpose != "invite" && purpose != "reset" {
		return fmt.Errorf("%q is not something this can send", purpose)
	}
	target, err := s.person(ctx, userID)
	if err != nil {
		return err
	}
	return s.op.Do(ctx, sess, operator.Change{
		Action:     "user.credential." + purpose,
		TargetType: "user",
		TargetID:   userID,
		Reason:     reason,
		After:      map[string]any{"email": target.Email, "purpose": purpose},
	}, func(ctx context.Context, tx pgx.Tx) error {
		token, err := s.issueCredentialGrant(ctx, tx, userID, purpose, sess.ID)
		if err != nil {
			return err
		}
		// Inside the transaction on purpose: a mail that could not be sent
		// rolls the grant back, so there is never a live token nobody
		// received. The provider is idempotent from our side — we ask once —
		// and a commit that failed after a successful send would leave a link
		// that does not work, which the person can be sent again.
		return s.deliverCredentialLink(ctx, tenantID, target.Email, purpose, token)
	})
}

// issueCredentialGrant writes the grant and returns the token it is for. The
// token is returned once, never stored, and only its digest is kept.
func (s *Service) issueCredentialGrant(ctx context.Context, tx pgx.Tx, userID, purpose, operatorID string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate a link: %w", err)
	}
	token := hex.EncodeToString(buf)

	// Anything outstanding for this person is spent first. Two live links to
	// the same account are two chances for the older one — the one in the mail
	// somebody forwarded — to still work.
	if _, err := tx.Exec(ctx,
		`UPDATE platform.credential_grants SET redeemed_at = NOW()
		  WHERE user_id = $1::uuid AND redeemed_at IS NULL AND expires_at > NOW()`,
		userID); err != nil {
		return "", fmt.Errorf("retire the outstanding links: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO platform.credential_grants (user_id, purpose, token_hash, issued_by_operator, expires_at)
		 VALUES ($1::uuid, $2, $3, $4::uuid, NOW() + $5::interval)`,
		userID, purpose, operator.HashToken(token), operatorID, CredentialLinkTTL.String()); err != nil {
		return "", fmt.Errorf("record the link: %w", err)
	}
	return token, nil
}

// deliverCredentialLink asks the verification service for a mail.
func (s *Service) deliverCredentialLink(ctx context.Context, tenantID, email, purpose, token string) error {
	if s.mail == nil || !mailrail.Configured() {
		return ErrMailNotConfigured
	}
	// The context loses its cancellation: an operator who closed the tab must
	// not leave a grant written and a mail unsent.
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()

	_, err := s.mail.Send(sendCtx, tenantID, mailrail.Request{
		Email:       email,
		Source:      "control-plane",
		Purpose:     purpose,
		RedirectURL: CredentialLinkURL(token),
	})
	if err != nil {
		return fmt.Errorf("send the link: %w", err)
	}
	return nil
}

// CredentialLinkURL is where somebody lands to choose a password.
//
// Built from PUBLIC_ORIGIN rather than from the console's own hostname, and
// that is the whole point of the redirection: the console is behind an address
// allowlist that the person receiving this mail is not inside. They set their
// password on the platform, where they will then use it.
func CredentialLinkURL(token string) string {
	return mailrail.PublicOrigin() + "/login/set-password?token=" + token
}

// invite is the first administrator's copy of the above, called by
// CreateTenant once the organisation exists.
// Invite gives somebody their first way in: a credential grant and the mail
// that carries it. Exported because creating an organisation is the other
// thing that has to do it, and doing it twice would mean two kinds of link.
func (s *Service) Invite(ctx context.Context, tenantID, userID, email, tenantName string, sess operator.Session) error {
	tx, err := s.db.Begin(operator.Scoped(ctx))
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	token, err := s.issueCredentialGrant(operator.Scoped(ctx), tx, userID, "invite", sess.ID)
	if err != nil {
		return err
	}
	if err := s.deliverCredentialLink(ctx, tenantID, email, "invite", token); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	slog.Info("control plane: invited an organisation's first administrator",
		"tenant_id", tenantID, "tenant", tenantName, "operator_id", sess.ID)
	return nil
}

// unusablePassword is what an account created by an operator carries until its
// owner chooses one.
//
// Random, never shown, never recorded. bcrypt over 32 bytes of entropy is not
// a password anybody guesses, and there is no code path that reveals it — so
// the only way into the account is the invitation, which is the property that
// keeps an operator from being able to sign in as a customer.
// UnusablePassword is what an account created by an operator carries until
// somebody sets one.
func UnusablePassword() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate a placeholder password: %w", err)
	}
	hash, err := security.HashPassword(hex.EncodeToString(buf))
	if err != nil {
		return "", fmt.Errorf("hash the placeholder password: %w", err)
	}
	return hash, nil
}
