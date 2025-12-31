package commands

import (
	"testing"

	"otto/internal/repo"
)

func TestCompleteWithoutMessage(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := testCtx()

	// Create an agent first
	agent := repo.Agent{
		Project: ctx.Project,
		Branch:  ctx.Branch,
		Name:    "test-agent",
		Type:    "codex",
		Task:    "test task",
		Status:  "busy",
	}
	if err := repo.CreateAgent(db, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// Complete without message - should succeed
	err := runComplete(db, "test-agent", "")
	if err != nil {
		t.Fatalf("runComplete with empty message should succeed: %v", err)
	}

	// Verify agent is complete
	got, err := repo.GetAgent(db, ctx.Project, ctx.Branch, "test-agent")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if got.Status != "complete" {
		t.Errorf("expected status 'complete', got %q", got.Status)
	}

	// Verify no complete message was created when content is empty
	messages, err := repo.ListMessages(db, repo.MessageFilter{
		Project: ctx.Project,
		Branch:  ctx.Branch,
		Type:    "complete",
	})
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected no complete messages when content is empty, got %d", len(messages))
	}
}
