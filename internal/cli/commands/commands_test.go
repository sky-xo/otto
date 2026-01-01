package commands

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"june/internal/config"
	"june/internal/db"
	"june/internal/repo"
	"june/internal/scope"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "june.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return conn
}

// testCtx returns the scope context that commands will use during tests.
// Tests must create agents with this project/branch for commands to find them.
func testCtx() scope.Context {
	return scope.CurrentContext()
}

func TestAskSetsBlocked(t *testing.T) {
	db := openTestDB(t)
	ctx := testCtx()
	_ = repo.CreateAgent(db, repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "authbackend", Type: "claude", Task: "task", Status: "busy"})
	err := runAsk(db, "authbackend", "Question?")
	if err != nil {
		t.Fatalf("ask: %v", err)
	}

	agents, _ := repo.ListAgents(db, repo.AgentFilter{Project: ctx.Project, Branch: ctx.Branch})
	if agents[0].Status != "blocked" {
		t.Fatalf("expected blocked, got %q", agents[0].Status)
	}
}

func TestCompleteSetsAgentComplete(t *testing.T) {
	db := openTestDB(t)
	ctx := testCtx()
	_ = repo.CreateAgent(db, repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "authbackend", Type: "claude", Task: "task", Status: "busy"})
	err := runComplete(db, "authbackend", "All done!")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	agent, err := repo.GetAgent(db, ctx.Project, ctx.Branch, "authbackend")
	if err != nil {
		t.Fatalf("expected agent to exist, got err=%v", err)
	}
	if agent.Status != "complete" {
		t.Fatalf("expected status complete, got %q", agent.Status)
	}
	if !agent.CompletedAt.Valid {
		t.Fatal("expected completed_at to be set")
	}

	// Message should still exist
	msgs, _ := repo.ListMessages(db, repo.MessageFilter{Type: "complete"})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 complete message, got %d", len(msgs))
	}
}

func TestOpenDBCreatesDirectory(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Global DB path is now ~/.june/june.db
	dbPath := filepath.Join(config.DataDir(), "june.db")
	dbDir := filepath.Dir(dbPath)

	if _, err := os.Stat(dbDir); !os.IsNotExist(err) {
		t.Fatalf("expected db dir to not exist, got err=%v", err)
	}

	conn, err := openDB()
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	if _, err := os.Stat(dbDir); err != nil {
		t.Fatalf("expected db dir to exist, got err=%v", err)
	}
}

func TestArchiveArchivesCompleteAgent(t *testing.T) {
	db := openTestDB(t)
	ctx := testCtx()
	_ = repo.CreateAgent(db, repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "archiver", Type: "claude", Task: "task", Status: "complete"})

	if err := runArchive(db, "archiver"); err != nil {
		t.Fatalf("archive: %v", err)
	}

	agent, err := repo.GetAgent(db, ctx.Project, ctx.Branch, "archiver")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !agent.ArchivedAt.Valid {
		t.Fatal("expected archived_at to be set")
	}
}

func TestArchiveRejectsBusyAgent(t *testing.T) {
	db := openTestDB(t)
	ctx := testCtx()
	_ = repo.CreateAgent(db, repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "busy-agent", Type: "claude", Task: "task", Status: "busy"})

	if err := runArchive(db, "busy-agent"); err == nil {
		t.Fatal("expected error for busy agent")
	}
}

func TestStatusExcludesArchivedByDefault(t *testing.T) {
	db := openTestDB(t)
	ctx := testCtx()
	_ = repo.CreateAgent(db, repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "active-agent", Type: "claude", Task: "task", Status: "busy"})
	_ = repo.CreateAgent(db, repo.Agent{
		Project:    ctx.Project,
		Branch:     ctx.Branch,
		Name:       "archived-agent",
		Type:       "claude",
		Task:       "task",
		Status:     "complete",
		ArchivedAt: sql.NullTime{Time: time.Now(), Valid: true},
	})

	output := captureStdout(t, func() {
		if err := runStatus(db, false, false); err != nil {
			t.Fatalf("runStatus: %v", err)
		}
	})

	if !strings.Contains(output, "active-agent") {
		t.Fatalf("expected active agent in output, got %q", output)
	}
	if strings.Contains(output, "archived-agent") {
		t.Fatalf("did not expect archived agent in output, got %q", output)
	}
}

func TestStatusIncludesArchivedWithAll(t *testing.T) {
	db := openTestDB(t)
	ctx := testCtx()
	_ = repo.CreateAgent(db, repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "active-agent", Type: "claude", Task: "task", Status: "busy"})
	_ = repo.CreateAgent(db, repo.Agent{
		Project:    ctx.Project,
		Branch:     ctx.Branch,
		Name:       "archived-agent",
		Type:       "claude",
		Task:       "task",
		Status:     "complete",
		ArchivedAt: sql.NullTime{Time: time.Now(), Valid: true},
	})

	output := captureStdout(t, func() {
		if err := runStatus(db, true, false); err != nil {
			t.Fatalf("runStatus: %v", err)
		}
	})

	if !strings.Contains(output, "active-agent") {
		t.Fatalf("expected active agent in output, got %q", output)
	}
	if !strings.Contains(output, "archived-agent") {
		t.Fatalf("expected archived agent in output, got %q", output)
	}
	if !strings.Contains(output, "archived-agent") || !strings.Contains(output, "(archived)") {
		t.Fatalf("expected archived suffix in output, got %q", output)
	}
}

func TestStatusArchiveArchivesCompleteAndFailed(t *testing.T) {
	db := openTestDB(t)
	ctx := testCtx()
	_ = repo.CreateAgent(db, repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "complete-agent", Type: "claude", Task: "task", Status: "complete"})
	_ = repo.CreateAgent(db, repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "failed-agent", Type: "claude", Task: "task", Status: "failed"})
	_ = repo.CreateAgent(db, repo.Agent{Project: ctx.Project, Branch: ctx.Branch, Name: "busy-agent", Type: "claude", Task: "task", Status: "busy"})
	_ = repo.CreateAgent(db, repo.Agent{
		Project:    ctx.Project,
		Branch:     ctx.Branch,
		Name:       "archived-agent",
		Type:       "claude",
		Task:       "task",
		Status:     "complete",
		ArchivedAt: sql.NullTime{Time: time.Now(), Valid: true},
	})

	if err := runStatus(db, true, true); err != nil {
		t.Fatalf("runStatus: %v", err)
	}

	completeAgent, err := repo.GetAgent(db, ctx.Project, ctx.Branch, "complete-agent")
	if err != nil {
		t.Fatalf("get complete: %v", err)
	}
	if !completeAgent.ArchivedAt.Valid {
		t.Fatal("expected complete agent to be archived")
	}

	failedAgent, err := repo.GetAgent(db, ctx.Project, ctx.Branch, "failed-agent")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !failedAgent.ArchivedAt.Valid {
		t.Fatal("expected failed agent to be archived")
	}

	busyAgent, err := repo.GetAgent(db, ctx.Project, ctx.Branch, "busy-agent")
	if err != nil {
		t.Fatalf("get busy: %v", err)
	}
	if busyAgent.ArchivedAt.Valid {
		t.Fatal("did not expect busy agent to be archived")
	}

	archivedAgent, err := repo.GetAgent(db, ctx.Project, ctx.Branch, "archived-agent")
	if err != nil {
		t.Fatalf("get archived: %v", err)
	}
	if !archivedAgent.ArchivedAt.Valid {
		t.Fatal("expected archived agent to remain archived")
	}
}
