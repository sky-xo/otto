package commands

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"otto/internal/config"
	"otto/internal/db"
	"otto/internal/repo"
	"otto/internal/scope"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "otto.db")
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

func TestSayCreatesMessage(t *testing.T) {
	db := openTestDB(t)
	err := runSay(db, "orchestrator", "Hello world!")
	if err != nil {
		t.Fatalf("say: %v", err)
	}

	msgs, _ := repo.ListMessages(db, repo.MessageFilter{Type: "say"})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "Hello world!" {
		t.Fatalf("expected 'Hello world!', got %q", msgs[0].Content)
	}
}

func TestParseMentions(t *testing.T) {
	tests := []struct {
		content  string
		expected []string
	}{
		{"Hello @alice and @bob", []string{"alice", "bob"}},
		{"@alice @bob @alice", []string{"alice", "bob"}}, // deduped
		{"No mentions here", []string{}},
		{"@user-123 @test-agent", []string{"user-123", "test-agent"}},
		{"@", []string{}}, // invalid mention
	}

	ctx := scope.Context{Project: "test-project", Branch: "main"}
	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			result := parseMentions(tt.content, ctx)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d mentions, got %d: %v", len(tt.expected), len(result), result)
			}
			for i, exp := range tt.expected {
				// Old tests expected simple names, now we get fully-qualified
				expected := "test-project:main:" + exp
				if result[i] != expected {
					t.Fatalf("expected mention %q at index %d, got %q", expected, i, result[i])
				}
			}
		})
	}
}

func TestParseMentionsWithScope(t *testing.T) {
	ctx := scope.Context{Project: "app", Branch: "feature/login"}

	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "agent only",
			content:  "ping @Impl-1",
			expected: []string{"app:feature/login:impl-1"},
		},
		{
			name:     "branch:agent",
			content:  "ping @main:Otto",
			expected: []string{"app:main:otto"},
		},
		{
			name:     "project:branch:agent",
			content:  "ping @backend:main:Otto",
			expected: []string{"backend:main:otto"},
		},
		{
			name:     "multiple mentions",
			content:  "ping @Impl-1 and @backend:main:Otto",
			expected: []string{"app:feature/login:impl-1", "backend:main:otto"},
		},
		{
			name:     "deduplicate mentions",
			content:  "@impl-1 @IMPL-1 @Impl-1",
			expected: []string{"app:feature/login:impl-1"},
		},
		{
			name:     "preserve project and branch casing",
			content:  "@Backend-API:Feature/Login:agent",
			expected: []string{"Backend-API:Feature/Login:agent"},
		},
		{
			name:     "agent names with dots and underscores",
			content:  "@agent.1 @agent_2",
			expected: []string{"app:feature/login:agent.1", "app:feature/login:agent_2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMentions(tt.content, ctx)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d mentions, got %d: %v", len(tt.expected), len(result), result)
			}
			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Fatalf("expected mention %q at index %d, got %q", exp, i, result[i])
				}
			}
		})
	}
}

func TestMessagesMentionsStoredAsJSON(t *testing.T) {
	db := openTestDB(t)
	ctx := testCtx()
	err := runSay(db, "orchestrator", "Hello @alice and @bob!")
	if err != nil {
		t.Fatalf("say: %v", err)
	}

	msgs, _ := repo.ListMessages(db, repo.MessageFilter{})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	var mentions []string
	if err := json.Unmarshal([]byte(msgs[0].MentionsJSON), &mentions); err != nil {
		t.Fatalf("failed to unmarshal mentions: %v", err)
	}

	// Mentions are now fully-qualified: project:branch:agent
	expected := []string{ctx.Project + ":" + ctx.Branch + ":alice", ctx.Project + ":" + ctx.Branch + ":bob"}
	if len(mentions) != 2 || mentions[0] != expected[0] || mentions[1] != expected[1] {
		t.Fatalf("expected mentions %v, got %v", expected, mentions)
	}
}

func TestOpenDBCreatesDirectory(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Global DB path is now ~/.otto/otto.db
	dbPath := filepath.Join(config.DataDir(), "otto.db")
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
