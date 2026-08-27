/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package emailverify

import (
	"testing"
	"unicode/utf8"
)

// A byte limit that lands mid-rune must not store half a character. Purpose
// and Source are prose a caller writes, and Mongolian prose is two bytes a
// letter, so every odd limit used to cut one in half.
func TestTruncateKeepsValidUTF8(t *testing.T) {
	const cyrillic = "Баталгаажуулах хүсэлт"
	for limit := range len(cyrillic) + 2 {
		got := truncate(cyrillic, limit)
		if !utf8.ValidString(got) {
			t.Fatalf("limit %d produced invalid UTF-8: %q", limit, got)
		}
		if len(got) > limit {
			t.Fatalf("limit %d produced %d bytes", limit, len(got))
		}
	}
}
