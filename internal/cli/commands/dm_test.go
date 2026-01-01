package commands

import (
	"testing"

	"june/internal/db"
	"june/internal/repo"
)

func TestRunDM(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()
	ctx := testCtx()

	// Create sender and recipient agents
	repo.CreateAgent(conn, repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "impl-1", Type: "codex", Task: "test", Status: "busy"})
	repo.CreateAgent(conn, repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "reviewer", Type: "codex", Task: "test", Status: "busy"})

	err := runDM(conn, ctx.Project, ctx.Branch, "impl-1", "reviewer", "API contract is ready")
	if err != nil {
		t.Fatalf("runDM failed: %v", err)
	}

	// Verify message was created
	msgs, err := repo.ListMessages(conn, repo.MessageFilter{Project: ctx.Project, Branch: ctx.Branch, Type: "dm"})
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].FromAgent != "impl-1" {
		t.Errorf("expected from_agent 'impl-1', got %q", msgs[0].FromAgent)
	}
	if !msgs[0].ToAgent.Valid || msgs[0].ToAgent.String != "reviewer" {
		t.Errorf("expected to_agent 'reviewer', got %v", msgs[0].ToAgent)
	}
	if msgs[0].Content != "API contract is ready" {
		t.Errorf("expected content 'API contract is ready', got %q", msgs[0].Content)
	}
}

func TestRunDM_CrossBranch(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()
	ctx := testCtx()

	// Create sender in current branch
	repo.CreateAgent(conn, repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "impl-1", Type: "codex", Task: "test", Status: "busy"})
	// Create recipient in different branch
	repo.CreateAgent(conn, repo.Agent{Project: ctx.Project, Branch: "feature/login", Name: "frontend", Type: "codex", Task: "test", Status: "busy"})

	err := runDM(conn, ctx.Project, ctx.Branch, "impl-1", "feature/login:frontend", "your branch needs rebase")
	if err != nil {
		t.Fatalf("runDM failed: %v", err)
	}

	// Verify message was created with resolved address
	msgs, err := repo.ListMessages(conn, repo.MessageFilter{Project: ctx.Project, Branch: ctx.Branch, Type: "dm"})
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	// to_agent should be the full resolved address
	if !msgs[0].ToAgent.Valid || msgs[0].ToAgent.String != "feature/login:frontend" {
		t.Errorf("expected to_agent 'feature/login:frontend', got %v", msgs[0].ToAgent)
	}
}
