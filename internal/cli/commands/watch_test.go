package commands

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"otto/internal/db"
	"otto/internal/repo"
)

func TestSinceIDFiltering(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.Close()

	// Create messages with staggered timestamps
	messages := []repo.Message{
		{
			ID:           "m1",
			FromID:       "user1",
			Type:         "text",
			Content:      "First message",
			MentionsJSON: "[]",
			ReadByJSON:   "[]",
		},
		{
			ID:           "m2",
			FromID:       "user1",
			Type:         "text",
			Content:      "Second message",
			MentionsJSON: "[]",
			ReadByJSON:   "[]",
		},
		{
			ID:           "m3",
			FromID:       "user1",
			Type:         "text",
			Content:      "Third message",
			MentionsJSON: "[]",
			ReadByJSON:   "[]",
		},
	}

	// Insert messages with delays to ensure different created_at timestamps
	for i, m := range messages {
		if err := repo.CreateMessage(testDB, m); err != nil {
			t.Fatalf("failed to create message %d: %v", i, err)
		}
		time.Sleep(1100 * time.Millisecond) // SQLite CURRENT_TIMESTAMP has second precision
	}

	// Test 1: List all messages (no SinceID)
	all, err := repo.ListMessages(testDB, repo.MessageFilter{})
	if err != nil {
		t.Fatalf("failed to list all messages: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(all))
	}

	// Test 2: List messages since m1 (should get m2 and m3)
	sinceM1, err := repo.ListMessages(testDB, repo.MessageFilter{SinceID: "m1"})
	if err != nil {
		t.Fatalf("failed to list messages since m1: %v", err)
	}
	if len(sinceM1) != 2 {
		t.Fatalf("expected 2 messages since m1, got %d", len(sinceM1))
	}
	if sinceM1[0].ID != "m2" || sinceM1[1].ID != "m3" {
		t.Fatalf("expected m2 and m3, got %s and %s", sinceM1[0].ID, sinceM1[1].ID)
	}

	// Test 3: List messages since m2 (should get m3)
	sinceM2, err := repo.ListMessages(testDB, repo.MessageFilter{SinceID: "m2"})
	if err != nil {
		t.Fatalf("failed to list messages since m2: %v", err)
	}
	if len(sinceM2) != 1 {
		t.Fatalf("expected 1 message since m2, got %d", len(sinceM2))
	}
	if sinceM2[0].ID != "m3" {
		t.Fatalf("expected m3, got %s", sinceM2[0].ID)
	}

	// Test 4: List messages since m3 (should get nothing)
	sinceM3, err := repo.ListMessages(testDB, repo.MessageFilter{SinceID: "m3"})
	if err != nil {
		t.Fatalf("failed to list messages since m3: %v", err)
	}
	if len(sinceM3) != 0 {
		t.Fatalf("expected 0 messages since m3, got %d", len(sinceM3))
	}

	// Test 5: Defensive - non-existent SinceID (should get all messages)
	sinceNonExistent, err := repo.ListMessages(testDB, repo.MessageFilter{SinceID: "non-existent"})
	if err != nil {
		t.Fatalf("failed to list messages with non-existent SinceID: %v", err)
	}
	if len(sinceNonExistent) != 3 {
		t.Fatalf("expected 3 messages with non-existent SinceID (defensive fallback), got %d", len(sinceNonExistent))
	}
}

// Helper to set up test database
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "otto.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return conn
}
