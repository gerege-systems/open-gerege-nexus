/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package backup

import (
	"context"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
)

// A deployment with no token cannot be triggered, and the refusal names the
// reason rather than failing somewhere inside GitHub's API.
func TestDeployingWithoutATokenIsRefusedBeforeAnythingHappens(t *testing.T) {
	t.Setenv("GITHUB_DEPLOY_TOKEN", "")
	t.Setenv("GITHUB_REPOSITORY", "gerege-systems/open-gerege-nexus")

	service := New(nil, Deps{})
	if _, err := service.TriggerDeploy(context.Background(),
		operator.Session{Operator: operator.Operator{ID: "1"}}, "v1.2.3", "a release"); err == nil {
		t.Fatal("a deployment was triggered with no token")
	}
}
