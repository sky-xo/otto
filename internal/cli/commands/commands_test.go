package commands

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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

func TestAskSetsWaiting(t *testing.T) {
	db := openTestDB(t)
	_ = repo.CreateAgent(db, repo.Agent{ID: "authbackend", Type: "claude", Task: "task", Status: "working"})
	err := runAsk(db, "authbackend", "Question?")
	if err != nil {
		t.Fatalf("ask: %v", err)
	}

	agents, _ := repo.ListAgents(db)
	if agents[0].Status != "waiting" {
		t.Fatalf("expected waiting, got %q", agents[0].Status)
	}
}

func TestCompleteDeletesAgent(t *testing.T) {
	db := openTestDB(t)
	_ = repo.CreateAgent(db, repo.Agent{ID: "authbackend", Type: "claude", Task: "task", Status: "working"})
	err := runComplete(db, "authbackend", "All done!")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	// Agent should be deleted after completion
	_, err = repo.GetAgent(db, "authbackend")
	if err != sql.ErrNoRows {
		t.Fatalf("expected agent to be deleted, got err=%v", err)
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

	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			result := parseMentions(tt.content)
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

	if len(mentions) != 2 || mentions[0] != "alice" || mentions[1] != "bob" {
		t.Fatalf("expected mentions [alice, bob], got %v", mentions)
	}
}

func TestOpenDBCreatesDirectory(t *testing.T) {
	repoRoot := scope.RepoRoot()
	if repoRoot == "" {
		t.Skip("not in a git repository")
	}

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	branch := scope.BranchName()
	if branch == "" {
		branch = "main"
	}

	scopePath := scope.Scope(repoRoot, branch)
	dbPath := filepath.Join(config.DataDir(), "orchestrators", scopePath, "otto.db")
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
