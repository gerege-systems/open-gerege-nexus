package metering

import (
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// The nightly run is scheduled on the platform's clock, not the process's.
//
// A container's clock is whatever the base image decided, which is almost
// always UTC. Left to it, "a little after midnight" would be a little after
// eight in the morning for the office the figures belong to — the collection
// would land in the middle of the working day and yesterday's total would be
// settled eight hours late.
func TestTheCollectionIsScheduledOnThePlatformsClock(t *testing.T) {
	here := nexus.Location()

	// Half past midnight where the platform lives, handed over the way a
	// container hands it over: as an instant with no local zone attached.
	when := time.Date(2026, 8, 16, 0, 30, 0, 0, here).UTC()

	next := when.Add(untilNextRun(when)).In(here)
	if next.Hour() != collectionHour || next.Minute() != 10 {
		t.Errorf("the next run is at %02d:%02d on the platform's clock, want %02d:10",
			next.Hour(), next.Minute(), collectionHour)
	}
	// Forty minutes later, not a day and forty minutes: 01:10 has not passed
	// yet on the clock that matters.
	if gap := next.Sub(when); gap != 40*time.Minute {
		t.Errorf("the next run is %s away, want 40m", gap)
	}
}

// And a run that has already passed today waits for tomorrow's.
func TestACollectionAlreadyPastWaitsForTheNextDay(t *testing.T) {
	here := nexus.Location()
	when := time.Date(2026, 8, 16, 9, 0, 0, 0, here).UTC()

	next := when.Add(untilNextRun(when)).In(here)
	if next.Day() != 17 || next.Hour() != collectionHour {
		t.Errorf("the next run is %s, want the 17th at %02d:10", next.Format(time.RFC3339), collectionHour)
	}
}
