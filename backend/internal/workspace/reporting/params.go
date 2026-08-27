/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Turning what a caller sent into what a report declared it accepts.
 */

package reporting

import (
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"

	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// maxTextParam bounds a free-text parameter. It is a query argument rather than
// interpolated SQL, so length is not an injection question — it is a "do not
// let somebody push a megabyte through a LIKE across every row" question.
const maxTextParam = 200

// Bind validates raw input against a report's declaration.
//
// Everything not declared is dropped rather than passed through, and anything
// declared but malformed is an error the caller sees. Both matter: the first is
// what stops a caller adding a key a report might one day read, and the second
// is what stops a bad date silently becoming the zero time — which in a
// `created_at >= $1` is every row ever written.
func Bind(report Report, raw map[string]string, locale string) (Params, error) {
	values := make(map[string]any, len(report.Params())*2)

	for _, spec := range report.Params() {
		switch spec.Kind {
		case ParamDateRange:
			from, to, err := bindDateRange(spec, raw)
			if err != nil {
				return Params{}, err
			}
			values[spec.Key+"_from"] = from
			values[spec.Key+"_to"] = to

		case ParamUUID:
			given := strings.TrimSpace(raw[spec.Key])
			if given == "" {
				if spec.Required {
					return nexus.Params{}, fmt.Errorf("%s is required", spec.Key)
				}
				values[spec.Key] = ""
				continue
			}
			if _, err := uuid.Parse(given); err != nil {
				return nexus.Params{}, fmt.Errorf("%s is not a valid identifier", spec.Key)
			}
			values[spec.Key] = given

		case ParamSelect:
			given := strings.TrimSpace(raw[spec.Key])
			if given == "" {
				if spec.Required {
					return nexus.Params{}, fmt.Errorf("%s is required", spec.Key)
				}
				if fallback, ok := spec.Default.(string); ok {
					given = fallback
				}
			}
			if given != "" && !hasOption(spec.Options, given) {
				return nexus.Params{}, fmt.Errorf("%s is not one of the values %s accepts", given, spec.Key)
			}
			values[spec.Key] = given

		case ParamText:
			given := strings.TrimSpace(raw[spec.Key])
			if given == "" && spec.Required {
				return nexus.Params{}, fmt.Errorf("%s is required", spec.Key)
			}
			if len(given) > maxTextParam {
				return nexus.Params{}, fmt.Errorf("%s is too long", spec.Key)
			}
			values[spec.Key] = given

		case ParamBool:
			given := strings.TrimSpace(raw[spec.Key])
			if given == "" {
				if fallback, ok := spec.Default.(bool); ok {
					values[spec.Key] = fallback
					continue
				}
				values[spec.Key] = false
				continue
			}
			parsed, err := strconv.ParseBool(given)
			if err != nil {
				return nexus.Params{}, fmt.Errorf("%s must be true or false", spec.Key)
			}
			values[spec.Key] = parsed

		default:
			return nexus.Params{}, fmt.Errorf("%s declares an unknown parameter kind %q", spec.Key, spec.Kind)
		}
	}

	return nexus.NewParams(values, locale), nil
}

// defaultWindow is how far back a date range reaches when the caller gives no
// dates and the report declares no window of its own. A month: long enough to
// be a useful first view, short enough that opening a report on a large tenant
// does not scan a year.
const defaultWindow = 30 * 24 * time.Hour

// maxWindow bounds any range. Five years is past every reporting period anyone
// asks for and stops a `from=0001-01-01` sequential-scanning the table.
const maxWindow = 5 * 365 * 24 * time.Hour

func bindDateRange(spec ParamSpec, raw map[string]string) (from, to time.Time, err error) {
	now := time.Now()

	window := spec.DefaultWindow
	if window <= 0 {
		window = defaultWindow
	}

	from, err = parseDate(raw[spec.Key+"_from"], now.Add(-window))
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%s_from is not a date (YYYY-MM-DD)", spec.Key)
	}
	to, err = parseDate(raw[spec.Key+"_to"], now)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%s_to is not a date (YYYY-MM-DD)", spec.Key)
	}

	if to.Before(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("%s_to is before %s_from", spec.Key, spec.Key)
	}
	if to.Sub(from) > maxWindow {
		return time.Time{}, time.Time{}, fmt.Errorf("%s covers more than five years", spec.Key)
	}

	// The end of the day rather than its start. A range ending "2026-08-12"
	// means through the twelfth; taking midnight would silently drop a day's
	// invoices every time somebody ran a month-end report.
	to = to.Add(24*time.Hour - time.Nanosecond)
	return from, to, nil
}

func parseDate(given string, fallback time.Time) (time.Time, error) {
	given = strings.TrimSpace(given)
	if given == "" {
		return time.Date(fallback.Year(), fallback.Month(), fallback.Day(), 0, 0, 0, 0, fallback.Location()), nil
	}
	// The platform's clock, not the process's. "2026-08-16" typed into a report
	// form means that day in Ulaanbaatar; on a UTC container time.Local would
	// have made it a window shifted by eight hours, quietly dropping the
	// evening's rows out of one report and into the next.
	return time.ParseInLocation("2006-01-02", given, nexus.Location())
}

func hasOption(options []ParamOption, value string) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}
