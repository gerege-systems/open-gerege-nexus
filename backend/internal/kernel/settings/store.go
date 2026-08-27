package settings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/async"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store keeps the current values in memory and refreshes them.
//
// Reading a setting happens on the request path — the idle timeout is consulted
// on every authenticated request — so it must not be a query. What it is
// instead is a map behind a read lock, refreshed on a timer and dropped when
// the console writes.
//
// Thirty seconds is the staleness anybody has to tolerate. The plan allows
// either that or Redis invalidation; this does both, because the timer is what
// still works when Redis is not there, and the invalidation is what makes a
// change feel immediate on the deployment that has it.
type Store struct {
	db *pgxpool.Pool

	mu     sync.RWMutex
	values map[string]string
	loaded time.Time

	// invalidate tells the other replicas to reload. Nil is fine: the timer
	// picks the change up within refreshInterval.
	invalidate func()
}

const refreshInterval = 30 * time.Second

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db, values: map[string]string{}}
}

// OnChange registers what to call after a value is written, so a deployment
// with Redis can tell its other replicas rather than leaving them to notice.
func (s *Store) OnChange(notify func()) { s.invalidate = notify }

// Default is the store the package-level helpers read.
//
// A package-level store, like audit's pool, and for the same reason: the
// consumers of a setting are scattered — auth, the catalogue, the AI client —
// and threading a store through each of their constructors would be a change
// to every signature between here and there, for a value they read once.
//
// Nil until UseStore is called, and nil means "environment then default",
// which is exactly what these call sites did before this package existed.
var Default *Store

// UseStore installs the process-wide store. Called once, from server
// construction.
func UseStore(store *Store) { Default = store }

// Get returns a setting's current value: the database, then the environment,
// then the default.
func Get(key string) string {
	spec, known := Lookup(key)
	if !known {
		// A key nobody registered has no meaning, and answering with something
		// would let a typo in a call site read as an empty setting for ever.
		slog.Error("settings: asked for a key that is not registered", "key", key)
		return ""
	}
	if Default != nil {
		if value, ok := Default.stored(key); ok {
			if err := spec.Validate(value); err == nil {
				return value
			}
			// A stored value that no longer validates — the spec's options
			// changed under it, or somebody wrote to the table by hand. The
			// environment and the default are still good answers.
			slog.Warn("settings: the stored value is not valid; falling back",
				"key", key, "value", value)
		}
	}
	if spec.Env != "" {
		if value := os.Getenv(spec.Env); value != "" {
			if err := spec.Validate(value); err == nil {
				return value
			}
			slog.Warn("settings: the environment value is not valid; using the default",
				"key", key, "env", spec.Env, "value", value)
		}
	}
	return spec.Default
}

// Bool and Duration are Get with the parse the caller would otherwise
// write. Each falls back to the default on a value that cannot be parsed,
// which cannot happen unless the registry and the parse disagree.
func Bool(key string) bool {
	parsed, err := strconv.ParseBool(Get(key))
	if err != nil {
		return false
	}
	return parsed
}

func Duration(key string) time.Duration {
	parsed, err := time.ParseDuration(Get(key))
	if err != nil {
		return 0
	}
	return parsed
}

func (s *Store) stored(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.values[key]
	return value, ok
}

// Load reads every stored value.
func (s *Store) Load(ctx context.Context) error {
	rows, err := s.db.Query(ctx, `SELECT key, value FROM registry.platform_settings`)
	if err != nil {
		return fmt.Errorf("settings: read the stored values: %w", err)
	}
	defer rows.Close()

	values := make(map[string]string, len(registry))
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return fmt.Errorf("settings: read a stored value: %w", err)
		}
		values[key] = value
	}
	if err := rows.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	s.values, s.loaded = values, time.Now()
	s.mu.Unlock()
	return nil
}

// Reload drops what is cached and reads again. It is what the invalidation bus
// calls on the replicas that did not make the change.
func (s *Store) Reload(ctx context.Context) {
	if err := s.Load(ctx); err != nil {
		slog.Warn("settings: could not reload", "error", err)
	}
}

// StartRefresh keeps the cache within refreshInterval of the table.
func (s *Store) StartRefresh(ctx context.Context) {
	async.Go("platform-settings-refresh", func() {
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

// ErrUnknownSetting is a key nothing declared.
var ErrUnknownSetting = errors.New("no such setting")

// Set writes a value and its history row, inside the caller's transaction.
//
// The transaction belongs to the caller because the console's audit row goes
// in the same one: a setting that changed with no operator_audit entry is the
// thing CP-1's middleware exists to prevent, and passing the transaction is
// what makes that impossible rather than merely unlikely.
func (s *Store) Set(ctx context.Context, tx pgx.Tx, key, value, operatorID, reason string) error {
	spec, known := Lookup(key)
	if !known {
		return fmt.Errorf("%w: %s", ErrUnknownSetting, key)
	}
	if err := spec.Validate(value); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}

	var previous *string
	if err := tx.QueryRow(ctx,
		`SELECT value FROM registry.platform_settings WHERE key = $1`, key).Scan(&previous); err != nil &&
		!errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("settings: read the current value: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO registry.platform_settings (key, value, updated_by, updated_at)
		 VALUES ($1, $2, $3::uuid, NOW())
		 ON CONFLICT (key) DO UPDATE
		    SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		key, value, operatorID); err != nil {
		return fmt.Errorf("settings: write the value: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO operator.platform_settings_history (key, previous_value, new_value, reason, changed_by)
		 VALUES ($1, $2, $3, $4, $5::uuid)`,
		key, previous, value, reason, operatorID); err != nil {
		return fmt.Errorf("settings: record the change: %w", err)
	}
	return nil
}

// Value is one setting as the console shows it: what it is, what it holds, and
// where that came from.
type Value struct {
	Spec
	// Current is what Get would answer.
	Current string `json:"current"`
	// Source is "database", "environment" or "default" — the question an
	// operator asks first when a setting is not doing what they expect.
	Source    string     `json:"source"`
	UpdatedAt *time.Time `json:"updated_at"`
}

// List returns every registered setting with its current value.
func (s *Store) List(ctx context.Context) ([]Value, error) {
	updated := map[string]time.Time{}
	rows, err := s.db.Query(ctx, `SELECT key, updated_at FROM registry.platform_settings`)
	if err != nil {
		return nil, fmt.Errorf("settings: read the stored values: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var at time.Time
		if err := rows.Scan(&key, &at); err != nil {
			return nil, err
		}
		updated[key] = at
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	values := make([]Value, 0, len(registry))
	for _, spec := range All() {
		value := Value{Spec: spec, Current: Get(spec.Key), Source: "default"}
		if stored, ok := s.stored(spec.Key); ok && spec.Validate(stored) == nil {
			value.Source = "database"
			if at, ok := updated[spec.Key]; ok {
				value.UpdatedAt = &at
			}
		} else if spec.Env != "" && os.Getenv(spec.Env) != "" {
			value.Source = "environment"
		}
		values = append(values, value)
	}
	return values, nil
}

// Change is one row of a setting's history.
type Change struct {
	ID            string    `json:"id"`
	Key           string    `json:"key"`
	PreviousValue *string   `json:"previous_value"`
	NewValue      string    `json:"new_value"`
	Reason        string    `json:"reason"`
	ChangedBy     string    `json:"changed_by"`
	ChangedAt     time.Time `json:"changed_at"`
}

// History returns what a setting has been, newest first.
func (s *Store) History(ctx context.Context, key string) ([]Change, error) {
	rows, err := s.db.Query(ctx,
		`SELECT h.id::text, h.key, h.previous_value, h.new_value, h.reason,
		        COALESCE(o.email, ''), h.changed_at
		   FROM operator.platform_settings_history h
		   LEFT JOIN operator.operator_accounts o ON o.id = h.changed_by
		  WHERE ($1 = '' OR h.key = $1)
		  ORDER BY h.changed_at DESC
		  LIMIT 100`, key)
	if err != nil {
		return nil, fmt.Errorf("settings: read the history: %w", err)
	}
	defer rows.Close()

	changes := make([]Change, 0, 16)
	for rows.Next() {
		var change Change
		if err := rows.Scan(&change.ID, &change.Key, &change.PreviousValue, &change.NewValue,
			&change.Reason, &change.ChangedBy, &change.ChangedAt); err != nil {
			return nil, fmt.Errorf("settings: read a history row: %w", err)
		}
		changes = append(changes, change)
	}
	return changes, rows.Err()
}

// Changed is called after a write commits, to drop the caches.
func (s *Store) Changed(ctx context.Context) {
	s.Reload(ctx)
	if s.invalidate != nil {
		s.invalidate()
	}
}

// InvalidatePrefix makes this satisfy the cache bus's interface, so a change on
// one replica reaches the others.
//
// The prefix is ignored: there are a handful of values and they are read
// together, so "something changed" is as fine-grained as this needs to be.
func (s *Store) InvalidatePrefix(string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.Reload(ctx)
}
