package quota

import "testing"

// The arithmetic the storage check rests on, which is easy to get wrong in a
// direction nobody notices: an organisation at its limit that can still upload.

func TestALimitIsCrossedByTheFileThatWouldCrossIt(t *testing.T) {
	limit := Limit{Max: 100, Used: 99, Hard: true}
	if limit.Exceeded(1) {
		t.Fatal("a file that fits exactly was refused")
	}
	if !limit.Exceeded(2) {
		t.Fatal("a file that would cross the limit was allowed")
	}
}

func TestAnUnlimitedOrganisationIsNeverExceeded(t *testing.T) {
	limit := Limit{Unlimited: true}
	if limit.Exceeded(1_000_000) {
		t.Fatal("an organisation with no limit was refused")
	}
}

// Zero is a limit, not the absence of one — the distinction the console keeps
// in its nullable columns and the reason this type has its own flag.
func TestALimitOfZeroRefusesEverything(t *testing.T) {
	limit := Limit{Max: 0, Used: 0, Hard: true}
	if !limit.Exceeded(1) {
		t.Fatal("a limit of zero admitted a file")
	}
}
