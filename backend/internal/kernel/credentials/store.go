/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package credentials

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/async"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store keeps the decrypted values in memory and refreshes them.
//
// In memory because these are read on the request path — the copilot asks for
// its key on every question — and a decrypt per request would be a decrypt per
// request. The same arrangement as settings.Store, for the same reasons: a
// timer that works everywhere and an invalidation that makes a rotation
// immediate where Redis is configured.
type Store struct {
	db *pgxpool.Pool

	mu     sync.RWMutex
	values map[string]string
	hints  map[string]string

	invalidate func()
}

const refreshInterval = 30 * time.Second

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db, values: map[string]string{}, hints: map[string]string{}}
}

// OnChange registers what to call after a value is written.
func (s *Store) OnChange(notify func()) { s.invalidate = notify }

// Default is the store the package-level Get reads. Nil means "the environment
// only", which is what every call site did before this package existed.
var Default *Store

// UseStore installs the process-wide store. Called once, from server
// construction.
func UseStore(store *Store) { Default = store }

// Get returns a credential: the database, then the environment, then empty.
//
// Empty is an ordinary answer and every caller has to handle it — a deployment
// with no AI key is a deployment without the copilot, not a broken one.
func Get(name string) string {
	spec, known := Lookup(name)
	if !known {
		slog.Error("credentials: asked for a name that is not registered", "name", name)
		return ""
	}
	if Default != nil {
		if value := Default.stored(name); value != "" {
			return value
		}
	}
	return strings.TrimSpace(os.Getenv(spec.Env))
}

func (s *Store) stored(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.values[name]
}

// Load reads and decrypts every stored credential.
//
// A row that will not decrypt is dropped with a warning rather than failing the
// load: the deployment's key has been rotated or the row is corrupt, and the
// other credentials — and the environment fallback for this one — are still
// good. Failing the whole load would take the working ones down with it.
func (s *Store) Load(ctx context.Context) error {
	rows, err := s.db.Query(ctx, `SELECT name, ciphertext, hint FROM operator.platform_credentials`)
	if err != nil {
		return fmt.Errorf("credentials: read the stored values: %w", err)
	}
	defer rows.Close()

	values := make(map[string]string, len(registry))
	hints := make(map[string]string, len(registry))
	for rows.Next() {
		var name, storedHint string
		var ciphertext []byte
		if err := rows.Scan(&name, &ciphertext, &storedHint); err != nil {
			return fmt.Errorf("credentials: read a stored value: %w", err)
		}
		if _, known := Lookup(name); !known {
			continue
		}
		plaintext, err := security.Open(ciphertext)
		if err != nil {
			slog.Warn("credentials: a stored credential could not be decrypted; "+
				"the environment is being used instead", "name", name, "error", err)
			continue
		}
		values[name] = string(plaintext)
		hints[name] = storedHint
	}
	if err := rows.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	s.values, s.hints = values, hints
	s.mu.Unlock()
	return nil
}

// Reload is what the invalidation bus calls on the replicas that did not make
// the change.
func (s *Store) Reload(ctx context.Context) {
	if err := s.Load(ctx); err != nil {
		slog.Warn("credentials: could not reload", "error", err)
	}
}

// StartRefresh keeps the cache within refreshInterval of the table.
func (s *Store) StartRefresh(ctx context.Context) {
	async.Go("platform-credentials-refresh", func() {
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			refreshCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			s.Reload(refreshCtx)
			cancel()
		}
	})
}

// ErrUnknownCredential is a name nothing declared.
var ErrUnknownCredential = errors.New("no such credential")

// Set seals a value and writes it, inside the caller's transaction.
//
// The transaction is the caller's for the reason settings.Set's is: the
// console's audit row goes in the same one, and a credential that changed with
// no operator_audit entry is exactly what the console's middleware exists to
// make impossible.
func (s *Store) Set(ctx context.Context, tx pgx.Tx, name, value, operatorID string) error {
	if _, known := Lookup(name); !known {
		return fmt.Errorf("%w: %s", ErrUnknownCredential, name)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("a credential cannot be empty; clear it instead")
	}
	sealed, err := security.Seal([]byte(value))
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO operator.platform_credentials (name, ciphertext, hint, updated_by, updated_at)
		 VALUES ($1, $2, $3, $4::uuid, NOW())
		 ON CONFLICT (name) DO UPDATE
		    SET ciphertext = EXCLUDED.ciphertext, hint = EXCLUDED.hint,
		        updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		name, sealed, hint(value), operatorID); err != nil {
		return fmt.Errorf("credentials: write the value: %w", err)
	}
	return nil
}

// Clear removes a stored credential, so the deployment falls back to its
// environment variable — or to having none.
func (s *Store) Clear(ctx context.Context, tx pgx.Tx, name, operatorID string) error {
	if _, known := Lookup(name); !known {
		return fmt.Errorf("%w: %s", ErrUnknownCredential, name)
	}
	_ = operatorID // the audit row carries who; the table keeps no tombstone
	if _, err := tx.Exec(ctx,
		`DELETE FROM operator.platform_credentials WHERE name = $1`, name); err != nil {
		return fmt.Errorf("credentials: clear the value: %w", err)
	}
	return nil
}

// Status is one credential as the console may see it. There is no field here
// that holds the value, and that is the point of the type.
type Status struct {
	Spec
	// Source is "database", "environment" or "unset" — the question an
	// operator asks first when an integration is not working.
	Source string `json:"source"`
	// Hint is the last four characters of a value stored here, so that two
	// keys can be told apart and a rotation can be seen to have landed. Empty
	// for a value short enough that four characters would give it away, and
	// for one that lives in the environment — this platform does not hand back
	// pieces of a variable it was given.
	Hint      string     `json:"hint,omitempty"`
	UpdatedAt *time.Time `json:"updated_at"`
	UpdatedBy *string    `json:"updated_by"`
}

// List returns every registered credential and where its value comes from.
func (s *Store) List(ctx context.Context) ([]Status, error) {
	type row struct {
		at *time.Time
		by *string
	}
	stored := map[string]row{}
	rows, err := s.db.Query(ctx,
		`SELECT name, updated_at, updated_by::text FROM operator.platform_credentials`)
	if err != nil {
		return nil, fmt.Errorf("credentials: read the stored values: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var record row
		if err := rows.Scan(&name, &record.at, &record.by); err != nil {
			return nil, fmt.Errorf("credentials: read a stored value: %w", err)
		}
		stored[name] = record
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	hints := make(map[string]string, len(s.hints))
	for name, value := range s.hints {
		hints[name] = value
	}
	s.mu.RUnlock()

	statuses := make([]Status, 0, len(registry))
	for _, spec := range All() {
		status := Status{Spec: spec, Source: "unset"}
		if record, ok := stored[spec.Name]; ok {
			status.Source, status.UpdatedAt, status.UpdatedBy = "database", record.at, record.by
			status.Hint = hints[spec.Name]
		} else if strings.TrimSpace(os.Getenv(spec.Env)) != "" {
			status.Source = "environment"
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

// Changed is called after a write commits, to drop the caches.
//
// Both halves matter and for different reasons: the reload is what makes the
// key this process presents the key that was just set — without it the replica
// that performed the write is the last one to honour it, up to a refresh
// interval later — and the invalidate is what tells the others.
func (s *Store) Changed(ctx context.Context) {
	s.Reload(ctx)
	if s.invalidate != nil {
		s.invalidate()
	}
}

// InvalidatePrefix is what the invalidation bus calls when another replica
// wrote a credential. The prefix is ignored: there are three of these and
// reading all of them costs one query, so a rotation lands everywhere at once
// rather than within the refresh interval.
func (s *Store) InvalidatePrefix(string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.Reload(ctx)
}
