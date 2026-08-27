package flags

import (
	"context"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// storeWith builds a store around flags held in memory, so the evaluation
// rules can be tested without a database. The database's part — loading these
// rows — is exercised by the control plane's own tests.
func storeWith(list ...Flag) *Store {
	store := &Store{flags: map[string]Flag{}}
	for _, flag := range list {
		if flag.Overrides == nil {
			flag.Overrides = map[string]bool{}
		}
		store.flags[flag.Key] = flag
	}
	return store
}

func ctxFor(tenantID string) context.Context {
	return nexus.WithWorkspaceID(context.Background(), tenantID)
}

// A flag nobody declared is off. The alternative — unknown means on — would
// turn a typo in a key into a feature released to everybody, and the failure
// would look exactly like the feature working.
func TestAnUnknownFlagIsOff(t *testing.T) {
	Default = storeWith()
	t.Cleanup(func() { Default = nil })

	if Enabled(ctxFor("t1"), "nothing.declared.this") {
		t.Fatal("an undeclared flag was on")
	}
}

func TestAnOverrideBeatsEverything(t *testing.T) {
	store := storeWith(Flag{
		Key: "billing.new_invoice", Enabled: false, Rollout: 100,
		Overrides: map[string]bool{"included": true},
	})
	if !store.enabled("billing.new_invoice", "included") {
		t.Fatal("an override could not turn a disabled flag on")
	}
	if store.enabled("billing.new_invoice", "everybody-else") {
		t.Fatal("a disabled flag was on for somebody with no override")
	}

	off := storeWith(Flag{
		Key: "billing.new_invoice", Enabled: true, Rollout: 100,
		Overrides: map[string]bool{"excluded": false},
	})
	if off.enabled("billing.new_invoice", "excluded") {
		t.Fatal("an override could not turn an enabled flag off")
	}
}

// The property the rollout depends on: an organisation's answer does not
// change between two requests, and it does not change as the percentage grows.
// An organisation inside 10% is still inside 50%.
func TestARolloutIsStableAndMonotonic(t *testing.T) {
	const key = "inventory.new_counting"
	tenants := []string{
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
		"33333333-3333-3333-3333-333333333333",
		"44444444-4444-4444-4444-444444444444",
		"55555555-5555-5555-5555-555555555555",
	}

	at := func(percent int, tenantID string) bool {
		return storeWith(Flag{Key: key, Enabled: true, Rollout: percent}).enabled(key, tenantID)
	}

	for _, tenantID := range tenants {
		// Stable: the same question, ten times, is the same answer.
		first := at(50, tenantID)
		for i := 0; i < 10; i++ {
			if at(50, tenantID) != first {
				t.Fatalf("%s flickered inside a 50%% rollout", tenantID)
			}
		}
		// Monotonic: widening the rollout never takes anybody out of it.
		for percent := 0; percent < 100; percent++ {
			if at(percent, tenantID) && !at(percent+1, tenantID) {
				t.Fatalf("%s was inside %d%% and outside %d%%", tenantID, percent, percent+1)
			}
		}
	}

	// 0 is nobody and 100 is everybody, which the percentages in between are
	// meaningless without.
	for _, tenantID := range tenants {
		if at(0, tenantID) {
			t.Fatalf("%s was inside a 0%% rollout", tenantID)
		}
		if !at(100, tenantID) {
			t.Fatalf("%s was outside a 100%% rollout", tenantID)
		}
	}
}

// Two flags at the same percentage must not select the same organisations, or
// the same unlucky few would be the test subjects for every experiment.
func TestTwoFlagsAtTheSamePercentagePickDifferentOrganisations(t *testing.T) {
	same := 0
	const population = 200
	for i := 0; i < population; i++ {
		tenantID := string(rune('a'+i%26)) + "-" + time.Duration(i).String()
		if (bucket("flag.one", tenantID) < 20) == (bucket("flag.two", tenantID) < 20) {
			same++
		}
	}
	// If the two flags selected identically, every organisation would agree.
	// Independent 20% selections agree about 68% of the time, so anything near
	// the whole population means the key is not in the hash.
	if same > population*9/10 {
		t.Fatalf("two flags agreed about %d of %d organisations", same, population)
	}
}

func TestAnExpiredFlagStillWorksAndIsReported(t *testing.T) {
	yesterday := time.Now().Add(-24 * time.Hour)
	store := storeWith(
		Flag{Key: "old.flag", Enabled: true, Rollout: 100, ExpiresAt: &yesterday},
		Flag{Key: "current.flag", Enabled: true, Rollout: 100},
	)

	// Expiry is a reminder, not an off switch: a flag that stopped working on
	// its expiry date would be a scheduled incident.
	if !store.enabled("old.flag", "t1") {
		t.Fatal("an expired flag stopped working")
	}
	total, expired := store.Snapshot(time.Now())
	if total != 2 {
		t.Fatalf("the snapshot counted %d flags", total)
	}
	if len(expired) != 1 || expired[0] != "old.flag" {
		t.Fatalf("the expired list is %v", expired)
	}
}

// A request with no organisation — the platform path — is not a coin toss.
func TestAPartialRolloutIsOffWithoutAnOrganisation(t *testing.T) {
	Default = storeWith(Flag{Key: "some.flag", Enabled: true, Rollout: 50})
	t.Cleanup(func() { Default = nil })

	if Enabled(context.Background(), "some.flag") {
		t.Fatal("a partial rollout was on for a request with no organisation")
	}
}

func TestTheModuleKillSwitchIsAName(t *testing.T) {
	if got := ModuleKillSwitch("io.gerege.nexus.billing"); got != "module.io.gerege.nexus.billing.disabled" {
		t.Fatalf("the kill switch key is %q", got)
	}
}
