/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package staffpin

import (
	"context"
	"errors"
	"testing"
)

func TestAPINIsFourToTwelveDigits(t *testing.T) {
	for _, pin := range []string{"0000", "123456", "123456789012"} {
		if !validPIN.MatchString(pin) {
			t.Errorf("valid PIN rejected: %s", pin)
		}
	}
	for _, pin := range []string{"123", "1234567890123", "12ab", "", "12 34"} {
		if validPIN.MatchString(pin) {
			t.Errorf("invalid PIN accepted: %q", pin)
		}
	}
}

func TestAMalformedSecretIsRejectedWithoutReadingAnything(t *testing.T) {
	s := &Service{}
	if _, err := s.Verify(context.Background(), "tenant", "not-a-pin"); !errors.Is(err, ErrStaffCredentialRejected) {
		t.Errorf("got %v, want ErrStaffCredentialRejected", err)
	}
}
