/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * A five-field cron expression, and whether this minute matches it.
 */

package reporting

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule is a parsed cron expression: minute, hour, day of month, month, day
// of week.
//
// Written here rather than taken from a library, and the reason is the size of
// the question. What is needed is "does this minute match" — the scheduler
// ticks every minute and asks. A cron library brings its own clock, its own
// goroutine per entry and its own idea of when a missed run should be caught
// up, none of which fit a scheduler that has to survive a replica restarting
// and must not send a report twice. This is a hundred lines with no state.
//
// Supported syntax is the common subset: `*`, a number, a comma list, a range
// `a-b`, and a step `*/n` or `a-b/n`. Names (`MON`, `JAN`) are not supported;
// the UI offers a picker rather than a text box, and a name in a stored
// schedule would be a value the picker cannot round-trip.
type Schedule struct {
	minutes  fieldSet
	hours    fieldSet
	days     fieldSet
	months   fieldSet
	weekdays fieldSet
}

type fieldSet [60]bool

// ParseCron validates and parses a five-field expression.
func ParseCron(expression string) (Schedule, error) {
	fields := strings.Fields(strings.TrimSpace(expression))
	if len(fields) != 5 {
		return Schedule{}, fmt.Errorf("a cron expression has five fields (minute hour day month weekday), got %d", len(fields))
	}

	var schedule Schedule
	var err error
	if schedule.minutes, err = parseField(fields[0], 0, 59); err != nil {
		return Schedule{}, fmt.Errorf("minute: %w", err)
	}
	if schedule.hours, err = parseField(fields[1], 0, 23); err != nil {
		return Schedule{}, fmt.Errorf("hour: %w", err)
	}
	if schedule.days, err = parseField(fields[2], 1, 31); err != nil {
		return Schedule{}, fmt.Errorf("day of month: %w", err)
	}
	if schedule.months, err = parseField(fields[3], 1, 12); err != nil {
		return Schedule{}, fmt.Errorf("month: %w", err)
	}
	if schedule.weekdays, err = parseField(fields[4], 0, 6); err != nil {
		return Schedule{}, fmt.Errorf("day of week: %w", err)
	}
	return schedule, nil
}

// Matches reports whether the given minute is one the schedule fires on.
//
// Day-of-month and day-of-week are OR-ed when both are restricted, which is
// what cron has always done and what surprises everybody once: `0 9 1 * 1`
// means "the first of the month, and every Monday", not "Mondays that fall on
// the first".
func (s Schedule) Matches(when time.Time) bool {
	if !s.minutes[when.Minute()] || !s.hours[when.Hour()] || !s.months[int(when.Month())] {
		return false
	}

	dayRestricted := !isFull(s.days, 1, 31)
	weekdayRestricted := !isFull(s.weekdays, 0, 6)

	switch {
	case dayRestricted && weekdayRestricted:
		return s.days[when.Day()] || s.weekdays[int(when.Weekday())]
	case dayRestricted:
		return s.days[when.Day()]
	case weekdayRestricted:
		return s.weekdays[int(when.Weekday())]
	default:
		return true
	}
}

func parseField(field string, min, max int) (fieldSet, error) {
	var set fieldSet
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return set, fmt.Errorf("empty value in %q", field)
		}

		step := 1
		if slash := strings.Index(part, "/"); slash >= 0 {
			parsed, err := strconv.Atoi(part[slash+1:])
			if err != nil || parsed < 1 {
				return set, fmt.Errorf("%q is not a step", part)
			}
			step = parsed
			part = part[:slash]
		}

		from, to := min, max
		switch {
		case part == "*":
			// The whole range, already set above.
		case strings.Contains(part, "-"):
			bounds := strings.SplitN(part, "-", 2)
			var err error
			if from, err = strconv.Atoi(strings.TrimSpace(bounds[0])); err != nil {
				return set, fmt.Errorf("%q is not a range", part)
			}
			if to, err = strconv.Atoi(strings.TrimSpace(bounds[1])); err != nil {
				return set, fmt.Errorf("%q is not a range", part)
			}
		default:
			value, err := strconv.Atoi(part)
			if err != nil {
				return set, fmt.Errorf("%q is not a number", part)
			}
			from, to = value, value
		}

		if from < min || to > max || from > to {
			return set, fmt.Errorf("%q is outside %d-%d", part, min, max)
		}
		for value := from; value <= to; value += step {
			set[value] = true
		}
	}
	return set, nil
}

func isFull(set fieldSet, min, max int) bool {
	for value := min; value <= max; value++ {
		if !set[value] {
			return false
		}
	}
	return true
}
