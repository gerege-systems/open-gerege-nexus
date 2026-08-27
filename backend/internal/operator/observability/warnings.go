/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package observability

// warnings is what the front page shows above everything else: the
// deployment's own complaints about how it is configured.
//
// The configuration screen shows them too, beside the fields they are about.
// Both ask the other plane, which is where the answer is.
func (s *Service) warnings() []string {
	if s.warningsFrom == nil {
		// Empty, never nil. A nil slice marshals as `null`, and the console's
		// front page reads this field as a list — `null.map` is a blank screen
		// with "This page couldn't load" on it, which is what a deployment
		// whose callback was never wired up actually saw.
		return []string{}
	}
	if warnings := s.warningsFrom(); warnings != nil {
		return warnings
	}
	return []string{}
}
