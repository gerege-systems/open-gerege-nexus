package ai_test

import (
	"context"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/ai"
)

func TestCopilotServiceQueryValidation(t *testing.T) {
	svc := ai.NewCopilotService(nil)

	_, err := svc.Query(context.Background(), ai.CopilotRequest{Prompt: ""})
	if err == nil {
		t.Fatal("expected error on empty prompt")
	}
}
