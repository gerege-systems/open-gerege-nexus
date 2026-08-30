/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import (
	"net/http"
	"slices"
	"strings"
)

// SupportedLocales lists the languages this platform answers in. The first
// entry is the default — Mongolian, the source language.
//
// Server-owned content is translated by the API rather than by the client: a
// menu label, a catalogue description and a report title all arrive already in
// the caller's language. A module that renders any of those needs to know which
// language was asked for, which is why this is here rather than in
// internal/kernel/config, where it was.
//
// docs/TRANSLATION.md is the policy: Mongolian plus the six official
// languages of the United Nations. Growing the list is a decision — every entry
// is one more column every future translation has to fill.
var SupportedLocales = []string{"mn", "ar", "zh", "en", "fr", "ru", "es"}

// LocaleFromRequest is the language this caller asked for.
//
//	?lang=fr            an explicit choice, which wins
//	Accept-Language     what the browser was configured with
//	SupportedLocales[0] neither, so the source language
//
// A language this platform does not answer in is not an error: it falls through
// to the next candidate and finally to the default, which is what a caller
// asking for Portuguese should get — a page in Mongolian, not a 400.
//
// Published here because a module cannot reach internal/kernel/config, and a
// module that renders a title per locale has no other way to learn which locale
// that is. It was the last reason internal/apps/reports imported the platform.
func LocaleFromRequest(r *http.Request) string {
	if lang := normalizeLocale(r.URL.Query().Get("lang")); lang != "" {
		return lang
	}

	// Accept-Language: mn-MN,mn;q=0.9,en;q=0.8
	for _, part := range strings.Split(r.Header.Get("Accept-Language"), ",") {
		tag, _, _ := strings.Cut(part, ";")
		if lang := normalizeLocale(tag); lang != "" {
			return lang
		}
	}

	return SupportedLocales[0]
}

// normalizeLocale reduces a tag to a language this platform answers in, or "".
// `mn-MN` and `MN` both become `mn`; a region this build has no words for is
// dropped rather than guessed at.
func normalizeLocale(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	base, _, _ := strings.Cut(tag, "-")
	if slices.Contains(SupportedLocales, base) {
		return base
	}
	return ""
}
