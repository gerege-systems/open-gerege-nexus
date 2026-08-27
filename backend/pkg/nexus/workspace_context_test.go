package nexus_test

import (
	"context"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

func TestWorkspaceContextIsolation(t *testing.T) {
	ctx := context.Background()

	_, err := nexus.WorkspaceID(ctx)
	if err == nil {
		t.Fatal("expected error when tenant ID is missing from context")
	}

	ctxWithTenant := nexus.WithWorkspaceID(ctx, "tenant-123")
	workspaceID, err := nexus.WorkspaceID(ctxWithTenant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if workspaceID != "tenant-123" {
		t.Fatalf("expected tenant-123, got %s", workspaceID)
	}
}
