package repo

import (
	"fmt"
	"testing"
)

func TestCreateAndListLogs(t *testing.T) {
	db := openTestDB(t)

	if err := CreateLogEntry(db, "agent-1", "in", "", "prompt"); err != nil {
		t.Fatalf("create entry 1: %v", err)
	}
	if err := CreateLogEntry(db, "agent-1", "out", "stdout", "response"); err != nil {
		t.Fatalf("create entry 2: %v", err)
	}
	if err := CreateLogEntry(db, "agent-2", "out", "stdout", "other"); err != nil {
		t.Fatalf("create entry 3: %v", err)
	}

	entries, err := ListLogs(db, "agent-1", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	if entries[0].AgentID != "agent-1" || entries[1].AgentID != "agent-1" {
		t.Fatalf("unexpected agent IDs: %#v", entries)
	}
}

func TestListLogsSince(t *testing.T) {
	db := openTestDB(t)

	if err := CreateLogEntry(db, "agent-1", "out", "stdout", "one"); err != nil {
		t.Fatalf("create entry 1: %v", err)
	}
	if err := CreateLogEntry(db, "agent-1", "out", "stdout", "two"); err != nil {
		t.Fatalf("create entry 2: %v", err)
	}
	if err := CreateLogEntry(db, "agent-1", "out", "stdout", "three"); err != nil {
		t.Fatalf("create entry 3: %v", err)
	}

	entries, err := ListLogs(db, "agent-1", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("unexpected entries: %#v", entries)
	}

	timestamps := map[string]string{
		entries[0].ID: "2024-01-01 00:00:00",
		entries[1].ID: "2024-01-02 00:00:00",
		entries[2].ID: "2024-01-03 00:00:00",
	}
	for id, ts := range timestamps {
		if _, err := db.Exec(`UPDATE logs SET created_at = ? WHERE id = ?`, ts, id); err != nil {
			t.Fatalf("set created_at: %v", err)
		}
	}

	sinceEntries, err := ListLogs(db, "agent-1", entries[0].ID)
	if err != nil {
		t.Fatalf("list since: %v", err)
	}
	if len(sinceEntries) != 2 || sinceEntries[0].ID != entries[1].ID || sinceEntries[1].ID != entries[2].ID {
		t.Fatalf("unexpected since entries: %#v", sinceEntries)
	}
}

func TestListLogsSinceSameSecond(t *testing.T) {
	db := openTestDB(t)

	if err := CreateLogEntry(db, "agent-1", "out", "stdout", "one"); err != nil {
		t.Fatalf("create entry 1: %v", err)
	}
	if err := CreateLogEntry(db, "agent-1", "out", "stdout", "two"); err != nil {
		t.Fatalf("create entry 2: %v", err)
	}
	if err := CreateLogEntry(db, "agent-1", "out", "stdout", "three"); err != nil {
		t.Fatalf("create entry 3: %v", err)
	}

	entries, err := ListLogs(db, "agent-1", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("unexpected entries: %#v", entries)
	}

	if _, err := db.Exec(`UPDATE logs SET created_at = ? WHERE id IN (?, ?)`, "2024-01-01 00:00:00", entries[0].ID, entries[1].ID); err != nil {
		t.Fatalf("set created_at: %v", err)
	}
	if _, err := db.Exec(`UPDATE logs SET created_at = ? WHERE id = ?`, "2024-01-01 00:00:01", entries[2].ID); err != nil {
		t.Fatalf("set created_at: %v", err)
	}

	sinceEntries, err := ListLogs(db, "agent-1", entries[0].ID)
	if err != nil {
		t.Fatalf("list since: %v", err)
	}
	if len(sinceEntries) != 2 || sinceEntries[0].ID != entries[1].ID || sinceEntries[1].ID != entries[2].ID {
		t.Fatalf("unexpected since entries: %#v", sinceEntries)
	}
}

func TestListLogsWithTail(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	for i := 0; i < 10; i++ {
		if err := CreateLogEntry(db, "agent-1", "out", "stdout", fmt.Sprintf("line %d", i)); err != nil {
			t.Fatalf("create log: %v", err)
		}
	}

	logs, err := ListLogsWithTail(db, "agent-1", 3)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(logs) != 3 {
		t.Errorf("expected 3 logs, got %d", len(logs))
	}
	if logs[0].Content != "line 7" {
		t.Errorf("expected first to be 'line 7', got %q", logs[0].Content)
	}
}
