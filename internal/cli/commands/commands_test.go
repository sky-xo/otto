package commands

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"otto/internal/db"
	"otto/internal/repo"
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

func TestCompleteSetsDone(t *testing.T) {
	db := openTestDB(t)
	_ = repo.CreateAgent(db, repo.Agent{ID: "authbackend", Type: "claude", Task: "task", Status: "working"})
	err := runComplete(db, "authbackend", "All done!")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	agents, _ := repo.ListAgents(db)
	if agents[0].Status != "done" {
		t.Fatalf("expected done, got %q", agents[0].Status)
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
