/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package reporting

import (
	"context"
	"fmt"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// AsEngine presents the report engine as the SDK's nexus.ReportEngine.
//
// Six methods over fifteen package functions, and the reduction is the point.
// The app that shows reports was calling Get, Bind, Run, Export, Filename,
// ParseFormat, ParseCron, ForApps and LocalizedTitle directly — every one of
// them under internal/, which is what kept the screens in this repository.
//
// Where three calls become one, it is because the three had an order: bind
// before run, parse the format before exporting. A contract that offered them
// separately would be a contract a caller could use in the wrong order, and
// the wrong order here runs a report with parameters nothing checked.
func AsEngine(e *Engine) nexus.ReportEngine {
	if e == nil {
		return nil
	}
	return engineAdapter{e}
}

type engineAdapter struct{ engine *Engine }

func (a engineAdapter) Available(installed map[string]bool) []nexus.ReportDescription {
	permitted := ForApps(installed)
	described := make([]nexus.ReportDescription, 0, len(permitted))
	for _, report := range permitted {
		described = append(described, describe(report))
	}
	return described
}

func (a engineAdapter) Describe(key string) (nexus.ReportDescription, bool) {
	report, found := Get(key)
	if !found {
		return nexus.ReportDescription{}, false
	}
	return describe(report), true
}

// Form fills a report's parameter form, dropdowns included.
//
// The options query is a report's own SQL and is run here, under the caller's
// tenant binding, because running a report's SQL is the engine's to do — the
// app that shows the form did it with a database handle of its own until
// 2026-08-23, which is one of the things that kept it in this repository. What
// comes back never carries the query: it is a statement this platform runs and
// a browser has no use for it.
func (a engineAdapter) ValidateCron(expression string) error {
	_, err := ParseCron(expression)
	return err
}

func (a engineAdapter) NormalizeFormat(raw string) (string, error) {
	format, err := ParseFormat(raw)
	return string(format), err
}

func (a engineAdapter) Form(ctx context.Context, tenantID, key, locale string) (*nexus.ReportForm, error) {
	report, found := Get(key)
	if !found {
		return nil, fmt.Errorf("no report is registered as %q", key)
	}
	_ = locale

	specs := report.Params()
	params := make([]nexus.ParamSpec, 0, len(specs))
	for _, spec := range specs {
		if spec.Kind == nexus.ParamUUID && spec.OptionsQuery != "" {
			options, err := a.options(ctx, tenantID, spec.OptionsQuery)
			if err != nil {
				return nil, err
			}
			spec.Options = options
		}
		spec.OptionsQuery = ""
		params = append(params, spec)
	}
	return &nexus.ReportForm{
		Key: report.Key(), App: report.App(), Titles: report.Titles(),
		Params: params, Columns: report.Columns(),
	}, nil
}

// options fills one dropdown from this organisation's own rows.
func (a engineAdapter) options(ctx context.Context, tenantID, query string) ([]nexus.ParamOption, error) {
	rows, err := a.engine.db.Query(nexus.WithWorkspaceID(ctx, tenantID), query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	options := make([]nexus.ParamOption, 0, 16)
	for rows.Next() {
		var value, label string
		if err := rows.Scan(&value, &label); err != nil {
			return nil, err
		}
		// One label for every locale: it is a row's own name, not a phrase.
		options = append(options, nexus.ParamOption{
			Value: value, Titles: map[string]string{"mn": label},
		})
	}
	return options, rows.Err()
}

func (a engineAdapter) Run(ctx context.Context, tenantID, key string, raw map[string]string, locale string) (*nexus.ReportRun, error) {
	report, params, err := a.bind(key, raw, locale)
	if err != nil {
		return nil, err
	}
	result, err := a.engine.Run(ctx, tenantID, report, params)
	if err != nil {
		return nil, err
	}
	return &nexus.ReportRun{
		Key:    report.Key(),
		Title:  LocalizedTitle(report.Titles(), locale, report.Key()),
		Result: result,
	}, nil
}

func (a engineAdapter) Export(ctx context.Context, tenantID, key string, raw map[string]string, locale, rawFormat string) (*nexus.ReportExport, error) {
	format, err := ParseFormat(rawFormat)
	if err != nil {
		return nil, err
	}
	report, params, err := a.bind(key, raw, locale)
	if err != nil {
		return nil, err
	}
	result, err := a.engine.Run(ctx, tenantID, report, params)
	if err != nil {
		return nil, err
	}
	title := LocalizedTitle(report.Titles(), locale, report.Key())
	payload, err := Export(format, title, result, locale)
	if err != nil {
		return nil, err
	}
	return &nexus.ReportExport{
		Filename:    Filename(report.Key(), format),
		ContentType: format.ContentType(),
		Bytes:       payload,
		Rows:        len(result.Rows),
	}, nil
}

func (a engineAdapter) ValidateSchedule(key string, raw map[string]string, locale, cron, format string) error {
	if _, _, err := a.bind(key, raw, locale); err != nil {
		return err
	}
	if _, err := ParseCron(cron); err != nil {
		return err
	}
	if _, err := ParseFormat(format); err != nil {
		return err
	}
	return nil
}

func (a engineAdapter) Deliverable() bool { return NewSMTPDeliverer() != nil }

// bind is the step a caller cannot skip: it refuses a parameter the report did
// not declare, which is the difference between running a report and running
// whatever a browser sent.
// RunConsolidated runs a report over everything shared with this organisation.
//
// The engine's own method takes a bound report and the actor; this binds and
// forwards, the same way Run does. The actor is here rather than read from the
// context because the audit entry on the *other* organisation's side names it,
// and a caller that could omit it would be a caller that could read somebody
// else's rows anonymously.
func (a engineAdapter) RunConsolidated(ctx context.Context, tenantID, key string,
	raw map[string]string, locale, actorUserID string) (*nexus.ReportRun, error) {

	report, params, err := a.bind(key, raw, locale)
	if err != nil {
		return nil, err
	}
	result, err := a.engine.RunConsolidated(ctx, tenantID, report, params, actorUserID)
	if err != nil {
		return nil, err
	}
	return &nexus.ReportRun{
		Key:    report.Key(),
		Title:  LocalizedTitle(report.Titles(), locale, report.Key()),
		Result: result,
	}, nil
}

func (a engineAdapter) bind(key string, raw map[string]string, locale string) (Report, Params, error) {
	report, found := Get(key)
	if !found {
		return nil, Params{}, fmt.Errorf("no report is registered as %q", key)
	}
	params, err := Bind(report, raw, locale)
	if err != nil {
		return nil, Params{}, err
	}
	return report, params, nil
}

// describe turns a registered report into a catalogue entry.
//
// Scopes come from the Shareable marker rather than from a field: sharing is
// something a report has to be written for, and a report that does not
// implement the marker cannot be named in a grant at all.
func describe(report Report) nexus.ReportDescription {
	described := nexus.ReportDescription{
		Key: report.Key(), App: report.App(), Titles: report.Titles(),
	}
	if shareable, ok := report.(Shareable); ok {
		described.Scopes = shareable.Scopes()
	}
	return described
}
