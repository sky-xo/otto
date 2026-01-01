package repo

import (
	"fmt"
	"testing"
	"time"
)

func TestCreateAndListLogs(t *testing.T) {
	db := openTestDB(t)

	if err := CreateLogEntry(db, LogEntry{Project: "june", Branch: "main", AgentName: "agent-1", AgentType: "claude", EventType: "prompt"}); err != nil {
		t.Fatalf("create entry 1: %v", err)
	}
	if err := CreateLogEntry(db, LogEntry{Project: "june", Branch: "main", AgentName: "agent-1", AgentType: "claude", EventType: "response"}); err != nil {
		t.Fatalf("create entry 2: %v", err)
	}
	if err := CreateLogEntry(db, LogEntry{Project: "june", Branch: "main", AgentName: "agent-2", AgentType: "codex", EventType: "output"}); err != nil {
		t.Fatalf("create entry 3: %v", err)
	}

	entries, err := ListLogs(db, "june", "main", "agent-1", "")
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

	if err := CreateLogEntry(db, LogEntry{Project: "june", Branch: "main", AgentName: "agent-1", AgentType: "claude", EventType: "output", Content: nullStr("one")}); err != nil {
		t.Fatalf("create entry 1: %v", err)
	}
	if err := CreateLogEntry(db, LogEntry{Project: "june", Branch: "main", AgentName: "agent-1", AgentType: "claude", EventType: "output", Content: nullStr("two")}); err != nil {
		t.Fatalf("create entry 2: %v", err)
	}
	if err := CreateLogEntry(db, LogEntry{Project: "june", Branch: "main", AgentName: "agent-1", AgentType: "claude", EventType: "output", Content: nullStr("three")}); err != nil {
		t.Fatalf("create entry 3: %v", err)
	}

	entries, err := ListLogs(db, "june", "main", "agent-1", "")
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

	sinceEntries, err := ListLogs(db, "june", "main", "agent-1", entries[0].ID)
	if err != nil {
		t.Fatalf("list since: %v", err)
	}
	if len(sinceEntries) != 2 || sinceEntries[0].ID != entries[1].ID || sinceEntries[1].ID != entries[2].ID {
		t.Fatalf("unexpected since entries: %#v", sinceEntries)
	}
}

func TestListLogsSinceSameSecond(t *testing.T) {
	db := openTestDB(t)

	if err := CreateLogEntry(db, LogEntry{Project: "june", Branch: "main", AgentName: "agent-1", AgentType: "claude", EventType: "output", Content: nullStr("one")}); err != nil {
		t.Fatalf("create entry 1: %v", err)
	}
	if err := CreateLogEntry(db, LogEntry{Project: "june", Branch: "main", AgentName: "agent-1", AgentType: "claude", EventType: "output", Content: nullStr("two")}); err != nil {
		t.Fatalf("create entry 2: %v", err)
	}
	if err := CreateLogEntry(db, LogEntry{Project: "june", Branch: "main", AgentName: "agent-1", AgentType: "claude", EventType: "output", Content: nullStr("three")}); err != nil {
		t.Fatalf("create entry 3: %v", err)
	}

	entries, err := ListLogs(db, "june", "main", "agent-1", "")
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

	sinceEntries, err := ListLogs(db, "june", "main", "agent-1", entries[0].ID)
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
			Project:   "june",
			Branch:    "main",
			AgentName: "agent-1",
			AgentType: "claude",
			EventType: "output",
			Content:   nullStr(fmt.Sprintf("line %d", i)),
		}); err != nil {
			t.Fatalf("create log: %v", err)
		}
	}

	logs, err := ListLogsWithTail(db, "june", "main", "agent-1", 3)
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

func TestCountLogs(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	project := "test-project"
	branch := "main"
	agentName := "test-agent"

	// Create some log entries
	for i := 0; i < 5; i++ {
		if err := CreateLogEntry(db, LogEntry{
			Project:   project,
			Branch:    branch,
			AgentName: agentName,
			AgentType: "codex",
			EventType: "output",
			Content:   nullStr("line"),
		}); err != nil {
			t.Fatalf("create log: %v", err)
		}
	}

	count, err := CountLogs(db, project, branch, agentName)
	if err != nil {
		t.Fatalf("CountLogs failed: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5, got %d", count)
	}
}

func TestGetAgentLastActivity(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	project := "june"
	branch := "main"

	// Create logs for multiple agents with different timestamps
	if err := CreateLogEntry(db, LogEntry{
		Project:   project,
		Branch:    branch,
		AgentName: "agent-1",
		AgentType: "claude",
		EventType: "output",
		Content:   nullStr("first"),
	}); err != nil {
		t.Fatalf("create log: %v", err)
	}
	if err := CreateLogEntry(db, LogEntry{
		Project:   project,
		Branch:    branch,
		AgentName: "agent-1",
		AgentType: "claude",
		EventType: "output",
		Content:   nullStr("second"),
	}); err != nil {
		t.Fatalf("create log: %v", err)
	}
	if err := CreateLogEntry(db, LogEntry{
		Project:   project,
		Branch:    branch,
		AgentName: "agent-2",
		AgentType: "codex",
		EventType: "output",
		Content:   nullStr("only entry"),
	}); err != nil {
		t.Fatalf("create log: %v", err)
	}

	// Set specific timestamps for testing
	entries, err := ListLogs(db, project, branch, "agent-1", "")
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for agent-1, got %d", len(entries))
	}

	// Set agent-1's first entry to an older time, second entry to newer time
	if _, err := db.Exec(`UPDATE logs SET created_at = ? WHERE id = ?`, "2024-01-01 10:00:00", entries[0].ID); err != nil {
		t.Fatalf("update timestamp: %v", err)
	}
	if _, err := db.Exec(`UPDATE logs SET created_at = ? WHERE id = ?`, "2024-01-02 15:30:00", entries[1].ID); err != nil {
		t.Fatalf("update timestamp: %v", err)
	}

	entries2, err := ListLogs(db, project, branch, "agent-2", "")
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(entries2) != 1 {
		t.Fatalf("expected 1 entry for agent-2, got %d", len(entries2))
	}
	if _, err := db.Exec(`UPDATE logs SET created_at = ? WHERE id = ?`, "2024-01-01 12:00:00", entries2[0].ID); err != nil {
		t.Fatalf("update timestamp: %v", err)
	}

	// Test getting last activity for both agents
	result, err := GetAgentLastActivity(db, project, branch, []string{"agent-1", "agent-2"})
	if err != nil {
		t.Fatalf("GetAgentLastActivity: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}

	expected1 := time.Date(2024, 1, 2, 15, 30, 0, 0, time.UTC)
	if !result["agent-1"].Equal(expected1) {
		t.Errorf("agent-1: expected %v, got %v", expected1, result["agent-1"])
	}

	expected2 := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	if !result["agent-2"].Equal(expected2) {
		t.Errorf("agent-2: expected %v, got %v", expected2, result["agent-2"])
	}

	// Test with empty slice returns empty map
	emptyResult, err := GetAgentLastActivity(db, project, branch, []string{})
	if err != nil {
		t.Fatalf("GetAgentLastActivity with empty slice: %v", err)
	}
	if len(emptyResult) != 0 {
		t.Errorf("expected empty map, got %d entries", len(emptyResult))
	}

	// Test with non-existent agent returns empty map (agent not in results)
	noMatchResult, err := GetAgentLastActivity(db, project, branch, []string{"non-existent"})
	if err != nil {
		t.Fatalf("GetAgentLastActivity with non-existent agent: %v", err)
	}
	if len(noMatchResult) != 0 {
		t.Errorf("expected empty map for non-existent agent, got %d entries", len(noMatchResult))
	}

	// Test with subset of agents
	subsetResult, err := GetAgentLastActivity(db, project, branch, []string{"agent-1"})
	if err != nil {
		t.Fatalf("GetAgentLastActivity with subset: %v", err)
	}
	if len(subsetResult) != 1 {
		t.Fatalf("expected 1 result, got %d", len(subsetResult))
	}
	if !subsetResult["agent-1"].Equal(expected1) {
		t.Errorf("agent-1 (subset): expected %v, got %v", expected1, subsetResult["agent-1"])
	}
}

func TestListAgentMessages(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	project := "test"
	branch := "main"
	agentName := "june"

	// Create various log entries
	entries := []LogEntry{
		{Project: project, Branch: branch, AgentName: agentName, AgentType: "codex", EventType: "input", Content: nullStr("user question")},
		{Project: project, Branch: branch, AgentName: agentName, AgentType: "codex", EventType: "thinking", Content: nullStr("let me think...")},
		{Project: project, Branch: branch, AgentName: agentName, AgentType: "codex", EventType: "agent_message", Content: nullStr("Here is my response!")},
		{Project: project, Branch: branch, AgentName: agentName, AgentType: "codex", EventType: "command_execution", Command: nullStr("ls")},
		{Project: project, Branch: branch, AgentName: agentName, AgentType: "codex", EventType: "agent_message", Content: nullStr("And another response")},
		{Project: project, Branch: branch, AgentName: "other-agent", AgentType: "codex", EventType: "agent_message", Content: nullStr("from other agent")},
	}

	for _, e := range entries {
		if err := CreateLogEntry(db, e); err != nil {
			t.Fatalf("create log entry: %v", err)
		}
	}

	// Fetch only agent_message entries for june
	result, err := ListAgentMessages(db, project, branch, agentName, "")
	if err != nil {
		t.Fatalf("ListAgentMessages: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 agent_message entries, got %d", len(result))
	}

	if result[0].Content.String != "Here is my response!" {
		t.Errorf("expected first message 'Here is my response!', got %q", result[0].Content.String)
	}
	if result[1].Content.String != "And another response" {
		t.Errorf("expected second message 'And another response', got %q", result[1].Content.String)
	}
}
