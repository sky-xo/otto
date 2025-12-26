package repo

import (
	"fmt"
	"testing"
)

func TestCreateAndListLogs(t *testing.T) {
	db := openTestDB(t)

	if err := CreateLogEntry(db, LogEntry{Project: "otto", Branch: "main", AgentName: "agent-1", AgentType: "claude", EventType: "prompt"}); err != nil {
		t.Fatalf("create entry 1: %v", err)
	}
	if err := CreateLogEntry(db, LogEntry{Project: "otto", Branch: "main", AgentName: "agent-1", AgentType: "claude", EventType: "response"}); err != nil {
		t.Fatalf("create entry 2: %v", err)
	}
	if err := CreateLogEntry(db, LogEntry{Project: "otto", Branch: "main", AgentName: "agent-2", AgentType: "codex", EventType: "output"}); err != nil {
		t.Fatalf("create entry 3: %v", err)
	}

	entries, err := ListLogs(db, "otto", "main", "agent-1", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	if entries[0].AgentName != "agent-1" || entries[1].AgentName != "agent-1" {
		t.Fatalf("unexpected agent names: %#v", entries)
	}
}

func TestListLogsSince(t *testing.T) {
	db := openTestDB(t)

	if err := CreateLogEntry(db, LogEntry{Project: "otto", Branch: "main", AgentName: "agent-1", AgentType: "claude", EventType: "output", Content: nullStr("one")}); err != nil {
		t.Fatalf("create entry 1: %v", err)
	}
	if err := CreateLogEntry(db, LogEntry{Project: "otto", Branch: "main", AgentName: "agent-1", AgentType: "claude", EventType: "output", Content: nullStr("two")}); err != nil {
		t.Fatalf("create entry 2: %v", err)
	}
	if err := CreateLogEntry(db, LogEntry{Project: "otto", Branch: "main", AgentName: "agent-1", AgentType: "claude", EventType: "output", Content: nullStr("three")}); err != nil {
		t.Fatalf("create entry 3: %v", err)
	}

	entries, err := ListLogs(db, "otto", "main", "agent-1", "")
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

	sinceEntries, err := ListLogs(db, "otto", "main", "agent-1", entries[0].ID)
	if err != nil {
		t.Fatalf("list since: %v", err)
	}
	if len(sinceEntries) != 2 || sinceEntries[0].ID != entries[1].ID || sinceEntries[1].ID != entries[2].ID {
		t.Fatalf("unexpected since entries: %#v", sinceEntries)
	}
}

func TestListLogsSinceSameSecond(t *testing.T) {
	db := openTestDB(t)

	if err := CreateLogEntry(db, LogEntry{Project: "otto", Branch: "main", AgentName: "agent-1", AgentType: "claude", EventType: "output", Content: nullStr("one")}); err != nil {
		t.Fatalf("create entry 1: %v", err)
	}
	if err := CreateLogEntry(db, LogEntry{Project: "otto", Branch: "main", AgentName: "agent-1", AgentType: "claude", EventType: "output", Content: nullStr("two")}); err != nil {
		t.Fatalf("create entry 2: %v", err)
	}
	if err := CreateLogEntry(db, LogEntry{Project: "otto", Branch: "main", AgentName: "agent-1", AgentType: "claude", EventType: "output", Content: nullStr("three")}); err != nil {
		t.Fatalf("create entry 3: %v", err)
	}

	entries, err := ListLogs(db, "otto", "main", "agent-1", "")
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

	sinceEntries, err := ListLogs(db, "otto", "main", "agent-1", entries[0].ID)
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
		if err := CreateLogEntry(db, LogEntry{
			Project:   "otto",
			Branch:    "main",
			AgentName: "agent-1",
			AgentType: "claude",
			EventType: "output",
			Content:   nullStr(fmt.Sprintf("line %d", i)),
		}); err != nil {
			t.Fatalf("create log: %v", err)
		}
	}

	logs, err := ListLogsWithTail(db, "otto", "main", "agent-1", 3)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(logs) != 3 {
		t.Errorf("expected 3 logs, got %d", len(logs))
	}
	if !logs[0].Content.Valid || logs[0].Content.String != "line 7" {
		t.Errorf("expected first to be 'line 7', got %v", logs[0].Content)
	}
}
