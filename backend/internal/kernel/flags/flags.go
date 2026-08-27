/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package flags answers "is this on for this organisation".
//
// The shape follows Unleash's guidance, and three of its recommendations are
// load-bearing here rather than decorative:
//
//   - **A flag has an owner and an expiry.** Not because the platform enforces
//     them, but because the console shows the ones that have lapsed. Flag debt
//     is branches of code nobody remembers deciding to keep, and the only cure
//     that works is somebody being reminded.
//   - **Rollout is by a stable hash.** An organisation that is inside a 20%
//     rollout stays inside it as the percentage grows, and does not flicker
//     between the two code paths from one request to the next.
//   - **A kill switch is a flag like any other.** The panic of switching a
//     module off should not need a deployment, and the mechanism for it should
//     be one everybody has already used for something calmer.
//
// Reads are from memory. Enabled is called inside request paths — the app gate
// asks it for every module request — so it is a map lookup behind a read lock,
// refreshed on a timer and dropped when the console writes.
package flags

import (
	"context"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/async"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Kind is what a flag is for.
const (
	KindRelease    = "release"
	KindKillSwitch = "kill_switch"
	KindExperiment = "experiment"
)

// Flag is one flag and its rollout.
type Flag struct {
	Key         string     `json:"key"`
	Description string     `json:"description"`
	Owner       string     `json:"owner"`
	Kind        string     `json:"kind"`
	Enabled     bool       `json:"enabled"`
	Rollout     int        `json:"rollout"`
	ExpiresAt   *time.Time `json:"expires_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	// Overrides is the per-organisation answer, where one has been set.
	Overrides map[string]bool `json:"overrides,omitempty"`
}

// Expired reports whether a flag has outlived the date somebody gave it. The
// flag still works; what has expired is the excuse for it still existing.
func (f Flag) Expired(now time.Time) bool {
	return f.ExpiresAt != nil && now.After(*f.ExpiresAt)
}

// Store holds the flags in memory.
type Store struct {
	db *pgxpool.Pool

	mu    sync.RWMutex
	flags map[string]Flag

	invalidate func()
}

const refreshInterval = 30 * time.Second

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db, flags: map[string]Flag{}}
}

// OnChange registers what to call after a flag is written, so the other
// replicas reload rather than waiting out their own timer.
func (s *Store) OnChange(notify func()) { s.invalidate = notify }

// Default is the process-wide store, for the same reason settings has one:
// Enabled is called from modules that should not have to be handed a store.
var Default *Store

// UseStore installs the process-wide store.
func UseStore(store *Store) { Default = store }

// Enabled answers for the organisation in ctx.
//
// A flag nobody has declared is **off**. That is the only safe default: a typo
// in a key would otherwise turn an unreleased feature on for everybody, and the
// failure would look like the feature working.
func Enabled(ctx context.Context, key string) bool {
	if Default == nil {
		return false
	}
	tenantID, _ := nexus.WorkspaceID(ctx)
	return Default.enabled(key, tenantID)
}

func (s *Store) enabled(key, tenantID string) bool {
	s.mu.RLock()
	flag, known := s.flags[key]
	s.mu.RUnlock()
	if !known {
		return false
	}

	// An organisation's own answer wins over everything, including the
	// percentage: an override is somebody deciding about *them*, and a rollout
	// is a decision about a population.
	if enabled, overridden := flag.Overrides[tenantID]; overridden {
		return enabled
	}
	if !flag.Enabled {
		return false
	}
	if flag.Rollout >= 100 {
		return true
	}
	if flag.Rollout <= 0 || tenantID == "" {
		// No organisation to hash — a platform-path request — so a partial
		// rollout is off rather than a coin toss.
		return false
	}
	return bucket(flag.Key, tenantID) < flag.Rollout
}

// bucket puts an organisation in 0…99 for one flag, the same way every time.
//
// The key is part of the hash so that two flags at 10% do not select the same
// tenth of organisations — otherwise the same unlucky few would be the test
// subjects for everything.
func bucket(key, tenantID string) int {
	digest := fnv.New32a()
	_, _ = digest.Write([]byte(key))
	_, _ = digest.Write([]byte{':'})
	_, _ = digest.Write([]byte(tenantID))
	return int(digest.Sum32() % 100)
}

// Load reads every flag and its overrides.
func (s *Store) Load(ctx context.Context) error {
	rows, err := s.db.Query(ctx,
		`SELECT key, description, owner, kind, enabled, rollout, expires_at, updated_at
		   FROM registry.feature_flags`)
	if err != nil {
		return err
	}
	defer rows.Close()

	flags := map[string]Flag{}
	for rows.Next() {
		var flag Flag
		if err := rows.Scan(&flag.Key, &flag.Description, &flag.Owner, &flag.Kind,
			&flag.Enabled, &flag.Rollout, &flag.ExpiresAt, &flag.UpdatedAt); err != nil {
			return err
		}
		flag.Overrides = map[string]bool{}
		flags[flag.Key] = flag
	}
	if err := rows.Err(); err != nil {
		return err
	}

	overrides, err := s.db.Query(ctx,
		`SELECT flag_key, tenant_id::text, enabled FROM registry.feature_flag_overrides`)
	if err != nil {
		return err
	}
	defer overrides.Close()
	for overrides.Next() {
		var key, tenantID string
		var enabled bool
		if err := overrides.Scan(&key, &tenantID, &enabled); err != nil {
			return err
		}
		if flag, known := flags[key]; known {
			flag.Overrides[tenantID] = enabled
		}
	}
	if err := overrides.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	s.flags = flags
	s.mu.Unlock()
	return nil
}

// Reload re-reads, reporting a failure without taking the process with it: the
// flags already in memory are a better answer than none.
func (s *Store) Reload(ctx context.Context) {
	if err := s.Load(ctx); err != nil {
		slog.Warn("flags: could not reload", "error", err)
	}
}

// StartRefresh keeps memory within refreshInterval of the table.
func (s *Store) StartRefresh(ctx context.Context) {
	async.Go("feature-flags-refresh", func() {
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

// Changed is called after a write commits.
func (s *Store) Changed(ctx context.Context) {
	s.Reload(ctx)
	if s.invalidate != nil {
		s.invalidate()
	}
}

// List returns every flag, for the console.
func (s *Store) List(ctx context.Context) ([]Flag, error) {
	if err := s.Load(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	flags := make([]Flag, 0, len(s.flags))
	for _, flag := range s.flags {
		flags = append(flags, flag)
	}
	return flags, nil
}

// Snapshot is what the console's home screen needs: how many flags there are
// and which of them have outlived their date.
func (s *Store) Snapshot(now time.Time) (total int, expired []string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	expired = []string{}
	for key, flag := range s.flags {
		total++
		if flag.Expired(now) {
			expired = append(expired, key)
		}
	}
	return total, expired
}

// ModuleKillSwitch is the flag key that turns an app module off for everybody.
//
// A convention rather than a table: the app gate asks for
// "module.<app id>.disabled", so switching a module off is creating one flag
// with a name anybody can work out, and no code has to be changed to make a
// new module killable.
func ModuleKillSwitch(appID string) string { return "module." + appID + ".disabled" }

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
