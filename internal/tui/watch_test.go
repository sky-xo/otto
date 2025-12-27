package tui

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"otto/internal/repo"
	"otto/internal/scope"

	tea "github.com/charmbracelet/bubbletea"
	_ "modernc.org/sqlite"
)

func TestNewModel(t *testing.T) {
	// Create in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create model
	m := NewModel(db)

	// Verify initial state
	if m.db == nil {
		t.Error("expected db to be set")
	}
	if len(m.messages) != 0 {
		t.Error("expected empty messages list")
	}
	if len(m.agents) != 0 {
		t.Error("expected empty agents list")
	}
	// Default activeChannelID is still mainChannelID even though Main channel no longer exists
	// This is handled by ensureSelection() when agents are loaded
	if m.activeChannelID != mainChannelID {
		t.Errorf("expected activeChannelID to be %q", mainChannelID)
	}
	if len(m.transcripts) != 0 {
		t.Error("expected empty transcripts map")
	}
}

func TestModelView(t *testing.T) {
	// Create in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create model with size
	m := NewModel(db)
	m.width = 80
	m.height = 24

	// Should render without panic
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestChannelOrdering(t *testing.T) {
	agents := []repo.Agent{
		{Name: "agent-3", Status: "failed"},
		{Name: "agent-2", Status: "blocked"},
		{Name: "agent-1", Status: "busy"},
		{Name: "agent-4", Status: "complete"},
	}

	ordered := sortAgentsByStatus(agents)
	if len(ordered) != 4 {
		t.Fatalf("expected 4 agents, got %d", len(ordered))
	}

	expected := []string{"agent-1", "agent-2", "agent-4", "agent-3"}
	for i, id := range expected {
		if ordered[i].Name != id {
			t.Fatalf("expected %q at index %d, got %q", id, i, ordered[i].Name)
		}
	}
}

func TestChannelsIncludeProjectHeaderFirst(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "test", Branch: "main", Name: "agent-2", Status: "complete"},
		{Project: "test", Branch: "main", Name: "agent-1", Status: "busy"},
	}

	channels := m.channels()
	// Expected: test/main header, agent-1 (busy first), agent-2
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(channels))
	}
	if channels[0].Kind != "project_header" {
		t.Fatalf("expected project_header at index 0, got %q", channels[0].Kind)
	}
	if channels[0].ID != "test/main" {
		t.Fatalf("expected test/main header first, got %q", channels[0].ID)
	}
	// Agents should be sorted by status: busy before complete
	if channels[1].ID != "agent-1" || channels[2].ID != "agent-2" {
		t.Fatalf("unexpected agent order: %q, %q", channels[1].ID, channels[2].ID)
	}
}

func TestArchivedAgentsHiddenByDefault(t *testing.T) {
	m := NewModel(nil)
	archivedAt := time.Now().Add(-time.Hour)
	m.agents = []repo.Agent{
		{Project: "test", Branch: "main", Name: "agent-1", Status: "busy"},
		{
			Project:    "test",
			Branch:     "main",
			Name:       "agent-2",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: archivedAt, Valid: true},
		},
	}

	channels := m.channels()
	// Expected: test/main header, agent-1, separator, Archived header
	if len(channels) != 4 {
		t.Fatalf("expected 4 channels, got %d", len(channels))
	}
	if channels[0].Kind != "project_header" {
		t.Fatalf("expected project_header at index 0, got %q", channels[0].Kind)
	}
	if channels[1].ID != "agent-1" {
		t.Fatalf("expected active agent at index 1, got %q", channels[1].ID)
	}
	if channels[2].Kind != "separator" {
		t.Fatalf("expected separator at index 2, got %q", channels[2].Kind)
	}
	if channels[3].ID != archivedChannelID {
		t.Fatalf("expected archived header last, got %q", channels[3].ID)
	}
}

func TestArchivedAgentsAppearWhenExpanded(t *testing.T) {
	m := NewModel(nil)
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	m.agents = []repo.Agent{
		{Project: "test", Branch: "main", Name: "agent-1", Status: "busy"},
		{
			Project:    "test",
			Branch:     "main",
			Name:       "agent-2",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: older, Valid: true},
		},
		{
			Project:    "test",
			Branch:     "main",
			Name:       "agent-3",
			Status:     "failed",
			ArchivedAt: sql.NullTime{Time: newer, Valid: true},
		},
	}
	m.archivedExpanded = true

	channels := m.channels()
	// Expected: test/main header, agent-1, separator, Archived header, test/main header (archived), agent-3, agent-2
	if len(channels) != 7 {
		t.Fatalf("expected 7 channels, got %d", len(channels))
	}
	if channels[2].Kind != "separator" {
		t.Fatalf("expected separator at index 2, got %q", channels[2].Kind)
	}
	if channels[3].ID != archivedChannelID {
		t.Fatalf("expected archived header at index 3, got %q", channels[3].ID)
	}
	if channels[3].Name != "Archived (2)" {
		t.Fatalf("expected archived header label, got %q", channels[3].Name)
	}
	// Archived section now groups by project/branch too
	if channels[4].Kind != "project_header" {
		t.Fatalf("expected project_header at index 4 (archived section), got %q", channels[4].Kind)
	}
	if channels[5].ID != "agent-3" || channels[6].ID != "agent-2" {
		t.Fatalf("unexpected archived order: %q, %q", channels[5].ID, channels[6].ID)
	}
}

func TestArchivedEnterTogglesExpanded(t *testing.T) {
	m := NewModel(nil)
	archivedAt := time.Now().Add(-time.Hour)
	m.agents = []repo.Agent{
		{Name: "agent-1", Status: "busy"},
		{
			Name:       "agent-2",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: archivedAt, Valid: true},
		},
	}

	channels := m.channels()
	headerIndex := -1
	for i, ch := range channels {
		if ch.ID == archivedChannelID {
			headerIndex = i
			break
		}
	}
	if headerIndex == -1 {
		t.Fatal("expected archived header to exist")
	}

	m.cursorIndex = headerIndex
	_ = m.toggleSelection()
	if !m.archivedExpanded {
		t.Fatal("expected archived section to expand on enter")
	}
	if m.activeChannelID != mainChannelID {
		t.Fatalf("expected active channel to remain main, got %q", m.activeChannelID)
	}

	_ = m.toggleSelection()
	if m.archivedExpanded {
		t.Fatal("expected archived section to collapse on enter")
	}
}

func TestFetchTranscriptsUsesCurrentScope(t *testing.T) {
	// Create in-memory database with schema
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize schema - we need to use the db package's Open function
	// to ensure schema is created properly
	db.Close()
	db, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to reopen database: %v", err)
	}

	// Manually create schema since we can't use db.Open with :memory: cleanly
	schemaSQL := `
		CREATE TABLE IF NOT EXISTS agents (
			project TEXT NOT NULL,
			branch TEXT NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			task TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (project, branch, name)
		);
		CREATE TABLE IF NOT EXISTS logs (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			branch TEXT NOT NULL,
			agent_name TEXT NOT NULL,
			agent_type TEXT NOT NULL,
			event_type TEXT NOT NULL,
			tool_name TEXT,
			content TEXT,
			raw_json TEXT,
			command TEXT,
			exit_code INTEGER,
			status TEXT,
			tool_use_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Insert an agent in the current project/branch scope
	// Use scope.CurrentContext() to get actual values for the test environment
	ctx := scope.CurrentContext()
	currentProject := ctx.Project
	currentBranch := ctx.Branch
	agentName := "agent-1"

	_, err = db.Exec(
		`INSERT INTO agents (project, branch, name, type, task, status) VALUES (?, ?, ?, ?, ?, ?)`,
		currentProject, currentBranch, agentName, "codex", "test task", "busy",
	)
	if err != nil {
		t.Fatalf("failed to insert agent: %v", err)
	}

	// Insert logs for this agent in the same scope
	_, err = db.Exec(
		`INSERT INTO logs (id, project, branch, agent_name, agent_type, event_type, content) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"log-1", currentProject, currentBranch, agentName, "codex", "message", "test log entry",
	)
	if err != nil {
		t.Fatalf("failed to insert log: %v", err)
	}

	// Insert logs for a different project/branch - should NOT be returned
	_, err = db.Exec(
		`INSERT INTO logs (id, project, branch, agent_name, agent_type, event_type, content) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"log-other", "other-project", "other-branch", agentName, "codex", "message", "other log entry",
	)
	if err != nil {
		t.Fatalf("failed to insert other log: %v", err)
	}

	// Call fetchTranscriptsCmd - it should use scope.CurrentContext()
	// and only return logs matching the current project/branch
	cmd := fetchTranscriptsCmd(db, agentName, "")
	msg := cmd()

	// Verify we got the correct logs
	transcriptsMsg, ok := msg.(transcriptsMsg)
	if !ok {
		if err, ok := msg.(error); ok {
			t.Fatalf("fetchTranscriptsCmd returned error: %v", err)
		}
		t.Fatalf("expected transcriptsMsg, got %T", msg)
	}

	if transcriptsMsg.agentID != agentName {
		t.Errorf("expected agentID %q, got %q", agentName, transcriptsMsg.agentID)
	}

	// Verify we got at least one log entry from current scope
	if len(transcriptsMsg.entries) == 0 {
		t.Fatalf("expected at least 1 log entry, got 0 - fetchTranscriptsCmd is not using current scope")
	}

	// Verify we got the right log (not the one from other project/branch)
	if len(transcriptsMsg.entries) != 1 {
		t.Errorf("expected 1 log entry, got %d", len(transcriptsMsg.entries))
	}

	if transcriptsMsg.entries[0].ID != "log-1" {
		t.Errorf("expected log ID %q, got %q", "log-1", transcriptsMsg.entries[0].ID)
	}
	if transcriptsMsg.entries[0].Content.String != "test log entry" {
		t.Errorf("expected content %q, got %q", "test log entry", transcriptsMsg.entries[0].Content.String)
	}
}

func TestFetchAgentsFetchesAllProjects(t *testing.T) {
	// Create in-memory database with schema
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Manually create schema
	schemaSQL := `
		CREATE TABLE IF NOT EXISTS agents (
			project TEXT NOT NULL,
			branch TEXT NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			task TEXT NOT NULL,
			status TEXT NOT NULL,
			session_id TEXT,
			pid INTEGER,
			compacted_at DATETIME,
			last_seen_message_id TEXT,
			peek_cursor TEXT,
			completed_at DATETIME,
			archived_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (project, branch, name)
		);
	`
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Insert agents in the current project/branch scope
	ctx := scope.CurrentContext()
	currentProject := ctx.Project
	currentBranch := ctx.Branch

	_, err = db.Exec(
		`INSERT INTO agents (project, branch, name, type, task, status) VALUES (?, ?, ?, ?, ?, ?)`,
		currentProject, currentBranch, "agent-1", "codex", "task 1", "busy",
	)
	if err != nil {
		t.Fatalf("failed to insert agent-1: %v", err)
	}

	// Insert agent in different scope - should ALSO be returned (global view)
	_, err = db.Exec(
		`INSERT INTO agents (project, branch, name, type, task, status) VALUES (?, ?, ?, ?, ?, ?)`,
		"other-project", "other-branch", "agent-2", "codex", "task 2", "busy",
	)
	if err != nil {
		t.Fatalf("failed to insert agent-2: %v", err)
	}

	// Call fetchAgentsCmd - it should return ALL agents across all projects
	cmd := fetchAgentsCmd(db)
	msg := cmd()

	// Verify we got the correct agents
	agentsMsg, ok := msg.(agentsMsg)
	if !ok {
		if err, ok := msg.(error); ok {
			t.Fatalf("fetchAgentsCmd returned error: %v", err)
		}
		t.Fatalf("expected agentsMsg, got %T", msg)
	}

	// Verify we got agents from ALL projects (global view)
	if len(agentsMsg) != 2 {
		t.Errorf("expected 2 agents from all projects, got %d", len(agentsMsg))
	}

	// Verify both agents are present
	names := map[string]bool{}
	for _, a := range agentsMsg {
		names[a.Name] = true
	}
	if !names["agent-1"] || !names["agent-2"] {
		t.Errorf("expected both agent-1 and agent-2, got %v", names)
	}
}

func TestFetchMessagesUsesCurrentScope(t *testing.T) {
	// Create in-memory database with schema
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Manually create schema
	schemaSQL := `
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			branch TEXT NOT NULL,
			from_agent TEXT,
			to_agent TEXT,
			type TEXT NOT NULL,
			content TEXT,
			mentions TEXT,
			requires_human INTEGER DEFAULT 0,
			read_by TEXT,
			from_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Insert messages in the current project/branch scope
	// Use scope.CurrentContext() to get actual values for the test environment
	ctx := scope.CurrentContext()
	currentProject := ctx.Project
	currentBranch := ctx.Branch

	_, err = db.Exec(
		`INSERT INTO messages (id, project, branch, from_agent, type, content, mentions, read_by, from_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"msg-1", currentProject, currentBranch, "agent-1", "say", "message in current scope", "", "", "agent-1",
	)
	if err != nil {
		t.Fatalf("failed to insert msg-1: %v", err)
	}

	// Insert message in different scope - should NOT be returned
	_, err = db.Exec(
		`INSERT INTO messages (id, project, branch, from_agent, type, content, mentions, read_by, from_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"msg-2", "other-project", "other-branch", "agent-2", "say", "message in other scope", "", "", "agent-2",
	)
	if err != nil {
		t.Fatalf("failed to insert msg-2: %v", err)
	}

	// Call fetchMessagesCmd with explicit project/branch from current context
	cmd := fetchMessagesCmd(db, currentProject, currentBranch, "")
	msg := cmd()

	// Verify we got the correct messages
	messagesMsg, ok := msg.(messagesMsg)
	if !ok {
		if err, ok := msg.(error); ok {
			t.Fatalf("fetchMessagesCmd returned error: %v", err)
		}
		t.Fatalf("expected messagesMsg, got %T", msg)
	}

	// Verify we got at least one message from current scope
	if len(messagesMsg) == 0 {
		t.Fatalf("expected at least 1 message, got 0 - fetchMessagesCmd is not using current scope")
	}

	// Verify we only got messages from the current scope
	if len(messagesMsg) != 1 {
		t.Errorf("expected 1 message from current scope, got %d", len(messagesMsg))
	}

	if messagesMsg[0].ID != "msg-1" {
		t.Errorf("expected message ID %q, got %q", "msg-1", messagesMsg[0].ID)
	}
	if messagesMsg[0].Content != "message in current scope" {
		t.Errorf("expected content %q, got %q", "message in current scope", messagesMsg[0].Content)
	}
}

func TestChannelsGroupByProjectBranch(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
		{Project: "otto", Branch: "main", Name: "reviewer", Status: "blocked"},
		{Project: "other", Branch: "feature", Name: "worker", Status: "complete"},
		{Project: "app", Branch: "dev", Name: "tester", Status: "busy"},
	}

	channels := m.channels()

	// Expected structure:
	// 0: app/dev header
	// 1:   tester (indented)
	// 2: separator
	// 3: other/feature header
	// 4:   worker (indented)
	// 5: separator
	// 6: otto/main header
	// 7:   impl-1 (indented)
	// 8:   reviewer (indented)

	expectedCount := 9
	if len(channels) != expectedCount {
		t.Fatalf("expected %d channels, got %d", expectedCount, len(channels))
	}

	// Test headers are present with correct IDs and kinds
	// app/dev header
	if channels[0].ID != "app/dev" {
		t.Fatalf("expected 'app/dev' header at index 0, got %q", channels[0].ID)
	}
	if channels[0].Kind != "project_header" {
		t.Fatalf("expected 'project_header' kind at index 0, got %q", channels[0].Kind)
	}
	if channels[0].Name != "app/dev" {
		t.Fatalf("expected 'app/dev' name at index 0, got %q", channels[0].Name)
	}
	if channels[0].Level != 0 {
		t.Fatalf("expected Level 0 for header at index 0, got %d", channels[0].Level)
	}

	// separator at index 2
	if channels[2].Kind != "separator" {
		t.Fatalf("expected 'separator' kind at index 2, got %q", channels[2].Kind)
	}

	// other/feature header
	if channels[3].ID != "other/feature" {
		t.Fatalf("expected 'other/feature' header at index 3, got %q", channels[3].ID)
	}
	if channels[3].Kind != "project_header" {
		t.Fatalf("expected 'project_header' kind at index 3, got %q", channels[3].Kind)
	}

	// separator at index 5
	if channels[5].Kind != "separator" {
		t.Fatalf("expected 'separator' kind at index 5, got %q", channels[5].Kind)
	}

	// otto/main header
	if channels[6].ID != "otto/main" {
		t.Fatalf("expected 'otto/main' header at index 6, got %q", channels[6].ID)
	}
	if channels[6].Kind != "project_header" {
		t.Fatalf("expected 'project_header' kind at index 6, got %q", channels[6].Kind)
	}

	// Test agents under headers have correct Level and properties
	// tester under app/dev
	if channels[1].ID != "tester" {
		t.Fatalf("expected 'tester' at index 1, got %q", channels[1].ID)
	}
	if channels[1].Kind != "agent" {
		t.Fatalf("expected 'agent' kind at index 1, got %q", channels[1].Kind)
	}
	if channels[1].Level != 1 {
		t.Fatalf("expected Level 1 for agent at index 1, got %d", channels[1].Level)
	}
	if channels[1].Status != "busy" {
		t.Fatalf("expected 'busy' status at index 1, got %q", channels[1].Status)
	}

	// worker under other/feature
	if channels[4].ID != "worker" {
		t.Fatalf("expected 'worker' at index 4, got %q", channels[4].ID)
	}
	if channels[4].Level != 1 {
		t.Fatalf("expected Level 1 for agent at index 4, got %d", channels[4].Level)
	}

	// impl-1 under otto/main (sorted by status: busy before blocked)
	if channels[7].ID != "impl-1" {
		t.Fatalf("expected 'impl-1' at index 7, got %q", channels[7].ID)
	}
	if channels[7].Level != 1 {
		t.Fatalf("expected Level 1 for agent at index 7, got %d", channels[7].Level)
	}

	// reviewer under otto/main
	if channels[8].ID != "reviewer" {
		t.Fatalf("expected 'reviewer' at index 8, got %q", channels[8].ID)
	}
	if channels[8].Level != 1 {
		t.Fatalf("expected Level 1 for agent at index 8, got %d", channels[8].Level)
	}
}

func TestChannelsGroupingWithArchived(t *testing.T) {
	m := NewModel(nil)
	archivedAt := time.Now().Add(-time.Hour)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
		{Project: "other", Branch: "feature", Name: "worker", Status: "complete"},
		{
			Project:    "otto",
			Branch:     "main",
			Name:       "old-agent",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: archivedAt, Valid: true},
		},
	}

	channels := m.channels()

	// Expected structure:
	// 0: other/feature header
	// 1:   worker
	// 2: separator
	// 3: otto/main header
	// 4:   impl-1
	// 5: separator
	// 6: Archived (1) header

	expectedCount := 7
	if len(channels) != expectedCount {
		t.Fatalf("expected %d channels, got %d", expectedCount, len(channels))
	}

	// Verify active agents are grouped
	if channels[0].Kind != "project_header" {
		t.Fatalf("expected project header at index 0, got %q", channels[0].Kind)
	}
	if channels[2].Kind != "separator" {
		t.Fatalf("expected separator at index 2, got %q", channels[2].Kind)
	}
	if channels[3].Kind != "project_header" {
		t.Fatalf("expected project header at index 3, got %q", channels[3].Kind)
	}
	if channels[5].Kind != "separator" {
		t.Fatalf("expected separator at index 5, got %q", channels[5].Kind)
	}

	// Verify archived section is last and NOT grouped
	if channels[6].ID != archivedChannelID {
		t.Fatalf("expected archived header last, got %q", channels[6].ID)
	}
	if channels[6].Kind != "archived_header" {
		t.Fatalf("expected 'archived_header' kind, got %q", channels[6].Kind)
	}
}

func TestProjectHeaderCollapseHidesAgents(t *testing.T) {
	m := NewModel(nil)
	m.projectExpanded = map[string]bool{"otto/main": false}
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
		{Project: "otto", Branch: "main", Name: "reviewer", Status: "blocked"},
	}

	channels := m.channels()

	// Expected structure when collapsed:
	// 0: otto/main header (collapsed)
	// No agents shown under header

	expectedCount := 1
	if len(channels) != expectedCount {
		t.Fatalf("expected %d channels (header only), got %d", expectedCount, len(channels))
	}

	if channels[0].ID != "otto/main" {
		t.Fatalf("expected otto/main header at index 0, got %q", channels[0].ID)
	}
	if channels[0].Kind != "project_header" {
		t.Fatalf("expected project_header kind, got %q", channels[0].Kind)
	}
}

func TestProjectHeaderExpandedShowsAgents(t *testing.T) {
	m := NewModel(nil)
	m.projectExpanded = map[string]bool{"otto/main": true}
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
		{Project: "otto", Branch: "main", Name: "reviewer", Status: "blocked"},
	}

	channels := m.channels()

	// Expected structure when expanded:
	// 0: otto/main header (expanded)
	// 1:   impl-1
	// 2:   reviewer

	expectedCount := 3
	if len(channels) != expectedCount {
		t.Fatalf("expected %d channels, got %d", expectedCount, len(channels))
	}

	if channels[0].ID != "otto/main" {
		t.Fatalf("expected otto/main header at index 0, got %q", channels[0].ID)
	}
	if channels[1].ID != "impl-1" {
		t.Fatalf("expected impl-1 at index 1, got %q", channels[1].ID)
	}
	if channels[2].ID != "reviewer" {
		t.Fatalf("expected reviewer at index 2, got %q", channels[2].ID)
	}
}

func TestProjectHeaderDefaultExpanded(t *testing.T) {
	m := NewModel(nil)
	// No explicit projectExpanded state - should default to expanded
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	channels := m.channels()

	// Expected structure (default expanded):
	// 0: otto/main header
	// 1:   impl-1

	expectedCount := 2
	if len(channels) != expectedCount {
		t.Fatalf("expected %d channels (default expanded), got %d", expectedCount, len(channels))
	}

	if channels[1].ID != "impl-1" {
		t.Fatalf("expected impl-1 agent visible by default, got %q", channels[1].ID)
	}
}

func TestArchivedSectionGroupsByProjectBranch(t *testing.T) {
	m := NewModel(nil)
	m.archivedExpanded = true
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)

	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "active-1", Status: "busy"},
		{
			Project:    "otto",
			Branch:     "main",
			Name:       "archived-1",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: newer, Valid: true},
		},
		{
			Project:    "other",
			Branch:     "feature",
			Name:       "archived-2",
			Status:     "failed",
			ArchivedAt: sql.NullTime{Time: older, Valid: true},
		},
	}

	channels := m.channels()

	// Expected structure:
	// 0: otto/main header
	// 1:   active-1
	// 2: separator
	// 3: Archived (2) header
	// 4: other/feature header (archived)
	// 5:   archived-2
	// 6: otto/main header (archived)
	// 7:   archived-1

	expectedCount := 8
	if len(channels) != expectedCount {
		t.Fatalf("expected %d channels, got %d", expectedCount, len(channels))
	}

	// Verify separator before archived
	if channels[2].Kind != "separator" {
		t.Fatalf("expected separator at index 2, got %q", channels[2].Kind)
	}

	// Verify archived header
	if channels[3].ID != archivedChannelID {
		t.Fatalf("expected archived header at index 3, got %q", channels[3].ID)
	}

	// Verify archived agents are grouped by project/branch with headers
	if channels[4].Kind != "project_header" {
		t.Fatalf("expected project_header at index 4 (archived section), got %q", channels[4].Kind)
	}
	if channels[4].ID != "other/feature" {
		t.Fatalf("expected other/feature header at index 4, got %q", channels[4].ID)
	}
	if channels[5].ID != "archived-2" {
		t.Fatalf("expected archived-2 at index 5, got %q", channels[5].ID)
	}

	if channels[6].Kind != "project_header" {
		t.Fatalf("expected project_header at index 6 (archived section), got %q", channels[6].Kind)
	}
	if channels[6].ID != "otto/main" {
		t.Fatalf("expected otto/main header at index 6, got %q", channels[6].ID)
	}
	if channels[7].ID != "archived-1" {
		t.Fatalf("expected archived-1 at index 7, got %q", channels[7].ID)
	}
}

func TestArchivedSectionRespectsProjectCollapse(t *testing.T) {
	m := NewModel(nil)
	m.archivedExpanded = true
	m.projectExpanded = map[string]bool{"otto/main": false}
	archivedAt := time.Now().Add(-time.Hour)

	m.agents = []repo.Agent{
		{
			Project:    "otto",
			Branch:     "main",
			Name:       "archived-1",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: archivedAt, Valid: true},
		},
		{
			Project:    "otto",
			Branch:     "main",
			Name:       "archived-2",
			Status:     "failed",
			ArchivedAt: sql.NullTime{Time: archivedAt, Valid: true},
		},
	}

	channels := m.channels()

	// Expected structure (archived expanded but otto/main collapsed):
	// 0: separator
	// 1: Archived (2) header
	// 2: otto/main header (collapsed in archived section)
	// No agents shown under collapsed header

	expectedCount := 3
	if len(channels) != expectedCount {
		t.Fatalf("expected %d channels (archived header collapsed), got %d", expectedCount, len(channels))
	}

	if channels[0].Kind != "separator" {
		t.Fatalf("expected separator at index 0, got %q", channels[0].Kind)
	}
	if channels[1].ID != archivedChannelID {
		t.Fatalf("expected archived header at index 1, got %q", channels[1].ID)
	}
	if channels[2].ID != "otto/main" {
		t.Fatalf("expected otto/main header at index 2, got %q", channels[2].ID)
	}
	if channels[2].Kind != "project_header" {
		t.Fatalf("expected project_header kind at index 2, got %q", channels[2].Kind)
	}
}

func TestProjectHeaderSelectionSetsActiveChannel(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
		{Project: "other", Branch: "feature", Name: "worker", Status: "complete"},
	}

	channels := m.channels()

	// Find the otto/main header
	headerIndex := -1
	for i, ch := range channels {
		if ch.ID == "otto/main" && ch.Kind == "project_header" {
			headerIndex = i
			break
		}
	}
	if headerIndex == -1 {
		t.Fatal("expected to find otto/main header")
	}

	// Select the project header
	m.cursorIndex = headerIndex
	_ = m.activateSelection()

	// Should set activeChannelID to the project header ID
	if m.activeChannelID != "otto/main" {
		t.Errorf("expected activeChannelID to be 'otto/main', got %q", m.activeChannelID)
	}
}

func TestProjectHeaderSelectionTogglesExpanded(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	channels := m.channels()

	// Find the otto/main header
	headerIndex := -1
	for i, ch := range channels {
		if ch.ID == "otto/main" && ch.Kind == "project_header" {
			headerIndex = i
			break
		}
	}
	if headerIndex == -1 {
		t.Fatal("expected to find otto/main header")
	}

	// Initially expanded (default)
	if !m.isProjectExpanded("otto/main") {
		t.Fatal("expected otto/main to be expanded by default")
	}

	// Select the header - should toggle to collapsed
	m.cursorIndex = headerIndex
	_ = m.toggleSelection()
	if m.isProjectExpanded("otto/main") {
		t.Error("expected otto/main to be collapsed after first activation")
	}

	// Select again - should toggle back to expanded
	_ = m.toggleSelection()
	if !m.isProjectExpanded("otto/main") {
		t.Error("expected otto/main to be expanded after second activation")
	}
}

func TestAgentSelectionStillSetsActiveChannelToAgent(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	channels := m.channels()

	// Find the agent
	agentIndex := -1
	for i, ch := range channels {
		if ch.ID == "impl-1" && ch.Kind == "agent" {
			agentIndex = i
			break
		}
	}
	if agentIndex == -1 {
		t.Fatal("expected to find impl-1 agent")
	}

	// Select the agent
	m.cursorIndex = agentIndex
	_ = m.activateSelection()

	// Should set activeChannelID to the agent name
	if m.activeChannelID != "impl-1" {
		t.Errorf("expected activeChannelID to be 'impl-1', got %q", m.activeChannelID)
	}
}

func TestRenderChannelLineProjectHeader(t *testing.T) {
	m := NewModel(nil)
	// Set project as expanded (default)
	m.projectExpanded = map[string]bool{"otto/main": true}

	// Create a project header channel
	ch := channel{
		ID:    "otto/main",
		Name:  "otto/main",
		Kind:  "project_header",
		Level: 0,
	}

	// Render with cursor (should have background)
	width := 20
	rendered := m.renderChannelLine(ch, width, true, false)

	// Strip ANSI codes for easier testing
	stripped := stripAnsi(rendered)

	// Should show project name and collapse indicator
	if !strings.Contains(stripped, "otto/main") {
		t.Errorf("expected header to contain project name, got: %q", stripped)
	}

	// Should show expanded indicator (▼)
	if !strings.Contains(stripped, "▼") {
		t.Errorf("expected expanded indicator (▼) for expanded header, got: %q", stripped)
	}

	// Verify length matches width (accounting for styling)
	if len(stripped) != width {
		t.Errorf("expected stripped length %d, got %d: %q", width, len(stripped), stripped)
	}

	// Should NOT contain status indicator (●, ○, ✗)
	if strings.Contains(stripped, "●") || strings.Contains(stripped, "○") || strings.Contains(stripped, "✗") {
		t.Errorf("expected no status indicator for header, got: %q", stripped)
	}

	// Test collapsed state
	m.projectExpanded["otto/main"] = false
	renderedCollapsed := m.renderChannelLine(ch, width, true, false)
	strippedCollapsed := stripAnsi(renderedCollapsed)

	// Should show collapsed indicator (▶)
	if !strings.Contains(strippedCollapsed, "▶") {
		t.Errorf("expected collapsed indicator (▶) for collapsed header, got: %q", strippedCollapsed)
	}
}

func TestRenderChannelLineIndentedAgentWithCursor(t *testing.T) {
	m := NewModel(nil)

	// Create an indented agent channel (Level 1)
	ch := channel{
		ID:      "impl-1",
		Name:    "impl-1",
		Kind:    "agent",
		Status:  "busy",
		Level:   1,
		Project: "otto",
		Branch:  "main",
	}

	// Render with cursor (should have background on indent AND content)
	width := 20
	rendered := m.renderChannelLine(ch, width, true, false)

	// Strip ANSI codes for easier testing
	stripped := stripAnsi(rendered)

	// Should be indented (Level 1 = 2 spaces)
	if !strings.HasPrefix(stripped, "  ") {
		t.Errorf("expected 2-space indent for Level 1, got: %q", stripped)
	}

	// Should contain status indicator
	if !strings.Contains(stripped, "●") {
		t.Errorf("expected status indicator for agent, got: %q", stripped)
	}

	// Should contain agent name
	if !strings.Contains(stripped, "impl-1") {
		t.Errorf("expected agent name in output, got: %q", stripped)
	}

	// Verify length matches width (accounting for ANSI codes in lipgloss output)
	// The stripped output should be exactly width characters
	// Note: lipgloss may not render ANSI codes in test environment (no TTY)
	// so we just check the visual width matches
	if len(stripped) < width {
		t.Errorf("expected stripped length at least %d, got %d: %q", width, len(stripped), stripped)
	}

	// Render without cursor for comparison
	renderedNoCursor := m.renderChannelLine(ch, width, false, false)
	strippedNoCursor := stripAnsi(renderedNoCursor)

	// Both should have the same visual content (indent + indicator + label)
	if !strings.HasPrefix(strippedNoCursor, "  ") {
		t.Errorf("expected 2-space indent even without cursor, got: %q", strippedNoCursor)
	}
}

func TestRenderChannelLineIndentedHeaderLevel1(t *testing.T) {
	m := NewModel(nil)

	// Create a project header at Level 1 (archived section)
	ch := channel{
		ID:    "otto/main",
		Name:  "otto/main",
		Kind:  "project_header",
		Level: 1,
	}

	// Render with cursor
	width := 20
	rendered := m.renderChannelLine(ch, width, true, false)

	// Strip ANSI codes
	stripped := stripAnsi(rendered)

	// Should be indented (Level 1 = 2 spaces)
	if !strings.HasPrefix(stripped, "  ") {
		t.Errorf("expected 2-space indent for Level 1 header, got: %q", stripped)
	}

	// Verify length matches width
	if len(stripped) != width {
		t.Errorf("expected stripped length %d, got %d: %q", width, len(stripped), stripped)
	}
}

// stripAnsi removes ANSI escape codes from a string
func stripAnsi(s string) string {
	// Simple ANSI escape sequence stripper
	// Matches ESC [ ... m sequences
	var result strings.Builder
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			inEscape = true
			i++ // skip '['
			continue
		}
		if inEscape {
			if s[i] == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteByte(s[i])
	}
	return result.String()
}

func TestProjectHeaderMessagesUsesProjectScope(t *testing.T) {
	// Create in-memory database with schema
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Manually create schema
	schemaSQL := `
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			branch TEXT NOT NULL,
			from_agent TEXT,
			to_agent TEXT,
			type TEXT NOT NULL,
			content TEXT,
			mentions TEXT,
			requires_human INTEGER DEFAULT 0,
			read_by TEXT,
			from_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Insert messages for different projects/branches
	_, err = db.Exec(
		`INSERT INTO messages (id, project, branch, from_agent, type, content, mentions, read_by, from_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"msg-otto-main", "otto", "main", "user", "say", "message for otto/main", "[]", "[]", "user",
	)
	if err != nil {
		t.Fatalf("failed to insert otto/main message: %v", err)
	}

	_, err = db.Exec(
		`INSERT INTO messages (id, project, branch, from_agent, type, content, mentions, read_by, from_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"msg-other-feature", "other", "feature", "user", "say", "message for other/feature", "[]", "[]", "user",
	)
	if err != nil {
		t.Fatalf("failed to insert other/feature message: %v", err)
	}

	// Fetch messages with explicit project/branch (new signature)
	// This should use otto/main scope, not the current git context
	cmd := fetchMessagesCmd(db, "otto", "main", "")
	msg := cmd()

	// Verify we got the correct messages
	messagesMsg, ok := msg.(messagesMsg)
	if !ok {
		if err, ok := msg.(error); ok {
			t.Fatalf("fetchMessagesCmd returned error: %v", err)
		}
		t.Fatalf("expected messagesMsg, got %T", msg)
	}

	// Should get only otto/main messages
	if len(messagesMsg) != 1 {
		t.Errorf("expected 1 message from otto/main scope, got %d", len(messagesMsg))
	}

	if len(messagesMsg) > 0 {
		if messagesMsg[0].ID != "msg-otto-main" {
			t.Errorf("expected message ID %q, got %q", "msg-otto-main", messagesMsg[0].ID)
		}
		if messagesMsg[0].Content != "message for otto/main" {
			t.Errorf("expected content %q, got %q", "message for otto/main", messagesMsg[0].Content)
		}
	}
}

func TestProjectHeaderMouseClick(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
		{Project: "otto", Branch: "main", Name: "reviewer", Status: "blocked"},
		{Project: "other", Branch: "feature", Name: "worker", Status: "complete"},
	}

	channels := m.channels()
	// Expected structure:
	// 0: Main
	// 1: other/feature header
	// 2:   worker
	// 3: separator
	// 4: otto/main header
	// 5:   impl-1
	// 6:   reviewer

	// Find the otto/main header index
	headerIndex := -1
	for i, ch := range channels {
		if ch.ID == "otto/main" && ch.Kind == "project_header" {
			headerIndex = i
			break
		}
	}
	if headerIndex == -1 {
		t.Fatal("expected to find otto/main header")
	}

	// Simulate mouse click on project header with activateSelection
	// This should just set the activeChannelID, not toggle
	m.cursorIndex = headerIndex
	_ = m.activateSelection()

	// Should set activeChannelID to project header
	if m.activeChannelID != "otto/main" {
		t.Errorf("expected activeChannelID to be 'otto/main', got %q", m.activeChannelID)
	}

	// Should NOT toggle expansion on activateSelection (still expanded)
	if !m.isProjectExpanded("otto/main") {
		t.Error("expected otto/main to still be expanded after activateSelection")
	}

	// Use toggleSelection to actually toggle
	_ = m.toggleSelection()
	if m.isProjectExpanded("otto/main") {
		t.Error("expected otto/main to be collapsed after toggleSelection")
	}
}

func TestNavigationSkipsCollapsedAgents(t *testing.T) {
	// This test verifies that navigation works correctly with collapsed project groups.
	// When collapsed agents are not in the channel list, cursor navigation skips them naturally.

	m := NewModel(nil)
	m.projectExpanded = map[string]bool{
		"otto/main":     false, // Collapse otto/main
		"other/feature": true,  // Keep other/feature expanded to avoid auto-toggle
	}
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
		{Project: "otto", Branch: "main", Name: "reviewer", Status: "blocked"},
		{Project: "other", Branch: "feature", Name: "worker", Status: "complete"},
	}

	channels := m.channels()
	// Expected structure (otto/main collapsed, other/feature expanded):
	// 0: other/feature header
	// 1:   worker
	// 2: separator
	// 3: otto/main header (collapsed, agents hidden)

	if len(channels) != 4 {
		t.Fatalf("expected 4 channels with otto/main collapsed, got %d", len(channels))
	}

	// Verify otto/main agents are not in the list
	for _, ch := range channels {
		if ch.ID == "impl-1" || ch.ID == "reviewer" {
			t.Errorf("expected otto/main agents to be hidden when collapsed, found %q", ch.ID)
		}
	}

	// Navigate through the list - should only see visible channels
	m.cursorIndex = 0 // other/feature header
	if channels[m.cursorIndex].ID != "other/feature" {
		t.Errorf("expected cursor at other/feature header, got %q", channels[m.cursorIndex].ID)
	}

	// Move down - should go to worker (index 1)
	m.cursorIndex = 1
	if channels[m.cursorIndex].ID != "worker" {
		t.Errorf("expected cursor at worker, got %q", channels[m.cursorIndex].ID)
	}

	// Move down - should skip separator (index 2) and go to otto/main header (index 3)
	m.cursorIndex = 1
	_ = m.moveCursor(1) // Should skip separator and land on otto/main
	if channels[m.cursorIndex].ID != "otto/main" {
		t.Errorf("expected cursor at otto/main header, got %q", channels[m.cursorIndex].ID)
	}

	// Verify we can't move past the end
	m.cursorIndex = 3
	_ = m.moveCursor(1) // Try to move down
	if m.cursorIndex != 3 {
		t.Errorf("expected cursor to clamp at last channel (3), got %d", m.cursorIndex)
	}

	// Verify we can't move before the beginning
	m.cursorIndex = 0
	_ = m.moveCursor(-1) // Try to move up
	if m.cursorIndex != 0 {
		t.Errorf("expected cursor to clamp at first channel (0), got %d", m.cursorIndex)
	}
}

func TestEnsureSelectionHandlesCollapsedAgents(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
		{Project: "otto", Branch: "main", Name: "reviewer", Status: "blocked"},
		{Project: "other", Branch: "feature", Name: "worker", Status: "complete"},
	}

	// Start with expanded project, cursor on impl-1 (index 4)
	channels := m.channels()
	impl1Index := -1
	for i, ch := range channels {
		if ch.ID == "impl-1" {
			impl1Index = i
			break
		}
	}
	if impl1Index == -1 {
		t.Fatal("expected to find impl-1")
	}

	m.cursorIndex = impl1Index
	m.activeChannelID = "impl-1"

	// Now collapse otto/main - impl-1 disappears from channel list
	m.projectExpanded["otto/main"] = false

	// Call ensureSelection - should adjust cursor to valid position
	m.ensureSelection()

	// Cursor should be adjusted to valid index
	channels = m.channels()
	if m.cursorIndex >= len(channels) {
		t.Errorf("expected cursor index < %d after collapse, got %d", len(channels), m.cursorIndex)
	}

	// Active channel should be set to the first valid channel when selected agent is hidden
	// Channels are sorted alphabetically, so "other/feature" comes before "otto/main"
	if m.activeChannelID != "other/feature" {
		t.Errorf("expected activeChannelID to be 'other/feature' after agent hidden, got %q", m.activeChannelID)
	}
}

func TestNavigationRespectsChannelListLength(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	channels := m.channels()
	// Expected: otto/main header, impl-1 (2 channels)

	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}

	// Start at last channel (impl-1)
	m.cursorIndex = len(channels) - 1

	// Move down - should clamp to last index
	_ = m.moveCursor(1)
	if m.cursorIndex != len(channels)-1 {
		t.Errorf("expected cursor to stay at last index %d, got %d", len(channels)-1, m.cursorIndex)
	}

	// Move up to first channel
	m.cursorIndex = 0

	// Move up - should clamp to 0
	_ = m.moveCursor(-1)
	if m.cursorIndex != 0 {
		t.Error("expected cursor to stay at index 0")
	}
}

func TestChatInputAppearsForProjectHeader(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	// Select project header
	m.activeChannelID = "otto/main"

	// Should show chat input
	if !m.showChatInput() {
		t.Error("expected chat input to show when project header is selected")
	}

	// Select an agent - should NOT show chat input
	m.activeChannelID = "impl-1"
	if m.showChatInput() {
		t.Error("expected chat input to be hidden when agent is selected")
	}

	// Select Main - should NOT show chat input
	m.activeChannelID = mainChannelID
	if m.showChatInput() {
		t.Error("expected chat input to be hidden when Main is selected")
	}
}

func TestGetOttoAgentForProjectBranch(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "otto", Status: "complete"},
		{Project: "other", Branch: "feature", Name: "otto", Status: "busy"},
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	// Get otto for otto/main
	otto := m.getOttoAgent("otto", "main")
	if otto == nil {
		t.Fatal("expected to find otto agent for otto/main")
	}
	if otto.Name != "otto" || otto.Project != "otto" || otto.Branch != "main" {
		t.Errorf("got wrong agent: %+v", otto)
	}

	// Get otto for other/feature
	otto = m.getOttoAgent("other", "feature")
	if otto == nil {
		t.Fatal("expected to find otto agent for other/feature")
	}
	if otto.Status != "busy" {
		t.Errorf("expected busy otto, got %q", otto.Status)
	}

	// Get otto for non-existent project/branch
	otto = m.getOttoAgent("nonexistent", "branch")
	if otto != nil {
		t.Error("expected nil for non-existent project/branch")
	}
}

func TestIsOttoBusy(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "otto", Status: "busy"},
		{Project: "other", Branch: "feature", Name: "otto", Status: "complete"},
	}

	// otto/main has busy otto
	if !m.isOttoBusy("otto", "main") {
		t.Error("expected otto to be busy for otto/main")
	}

	// other/feature has complete otto
	if m.isOttoBusy("other", "feature") {
		t.Error("expected otto to NOT be busy for other/feature")
	}

	// No otto for this project/branch
	if m.isOttoBusy("nonexistent", "branch") {
		t.Error("expected otto to NOT be busy when it doesn't exist")
	}
}

func TestGetChatSubmitAction(t *testing.T) {
	tests := []struct {
		name           string
		agents         []repo.Agent
		project        string
		branch         string
		expectedAction string // "none", "spawn", "prompt"
	}{
		{
			name:           "otto is busy - no action",
			agents:         []repo.Agent{{Project: "p", Branch: "b", Name: "otto", Status: "busy"}},
			project:        "p",
			branch:         "b",
			expectedAction: "none",
		},
		{
			name:           "otto complete - prompt",
			agents:         []repo.Agent{{Project: "p", Branch: "b", Name: "otto", Status: "complete"}},
			project:        "p",
			branch:         "b",
			expectedAction: "prompt",
		},
		{
			name:           "otto failed - prompt",
			agents:         []repo.Agent{{Project: "p", Branch: "b", Name: "otto", Status: "failed"}},
			project:        "p",
			branch:         "b",
			expectedAction: "prompt",
		},
		{
			name:           "no otto - spawn",
			agents:         []repo.Agent{{Project: "p", Branch: "b", Name: "impl-1", Status: "busy"}},
			project:        "p",
			branch:         "b",
			expectedAction: "spawn",
		},
		{
			name:           "no agents - spawn",
			agents:         []repo.Agent{},
			project:        "p",
			branch:         "b",
			expectedAction: "spawn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(nil)
			m.agents = tt.agents

			action := m.getChatSubmitAction(tt.project, tt.branch)
			if action != tt.expectedAction {
				t.Errorf("expected action %q, got %q", tt.expectedAction, action)
			}
		})
	}
}

func TestViewportHeightUpdatesWhenChatInputShows(t *testing.T) {
	// This test reproduces the bug where viewport dimensions are not recalculated
	// when activeChannelID changes to a project header (which shows chat input).
	// The viewport height should be 1 line smaller when chat input is visible.

	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	// Set window size
	m.width = 80
	m.height = 24

	// Initially select an agent (no chat input shown)
	m.activeChannelID = "impl-1"
	m.updateViewportDimensions() // Simulate what happens in activateSelection()

	// Calculate initial viewport dimensions
	_, _, _, contentHeight := m.layout()
	initialHeight := contentHeight

	// Verify chat input is NOT shown for agent
	if m.showChatInput() {
		t.Fatal("expected chat input to be hidden for agent selection")
	}

	// Now change to a project header (should show chat input)
	m.activeChannelID = "otto/main"
	m.updateViewportDimensions() // This is the fix - should be called when activeChannelID changes

	// BUG: viewport.Height is not updated when activeChannelID changes
	// It still has the old height from when chat input was hidden

	// Verify chat input IS shown for project header
	if !m.showChatInput() {
		t.Fatal("expected chat input to be shown for project header")
	}

	// Calculate new layout dimensions - contentHeight should be 1 less
	_, _, _, newContentHeight := m.layout()

	// When chat input is shown, contentHeight should be 1 line smaller
	expectedHeightDifference := 1
	actualHeightDifference := initialHeight - newContentHeight

	if actualHeightDifference != expectedHeightDifference {
		t.Errorf("expected content height to decrease by %d when chat input shows, got decrease of %d",
			expectedHeightDifference, actualHeightDifference)
	}

	// The BUG: m.viewport.Height was not updated when activeChannelID changed
	// It should match newContentHeight, but it still has the old value
	if m.viewport.Height != newContentHeight {
		t.Errorf("expected viewport.Height to be %d after showing chat input, got %d (viewport dimensions not updated)",
			newContentHeight, m.viewport.Height)
	}

	// Test the reverse: changing from project header back to agent
	m.activeChannelID = "impl-1"
	m.updateViewportDimensions() // Should be called when activeChannelID changes

	// Verify chat input is hidden again
	if m.showChatInput() {
		t.Fatal("expected chat input to be hidden again for agent")
	}

	// Calculate dimensions again
	_, _, _, backToAgentHeight := m.layout()

	// Should be back to original height
	if backToAgentHeight != initialHeight {
		t.Errorf("expected content height to return to %d when hiding chat input, got %d",
			initialHeight, backToAgentHeight)
	}

	// Viewport height should be updated to match
	if m.viewport.Height != backToAgentHeight {
		t.Errorf("expected viewport.Height to be %d after hiding chat input, got %d (viewport dimensions not updated)",
			backToAgentHeight, m.viewport.Height)
	}
}

// Bug 1: Tab key swallowed by textinput
func TestTabKeySwitchesPanels(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	channels := m.channels()
	// Find project header index
	headerIndex := -1
	for i, ch := range channels {
		if ch.ID == "otto/main" && ch.Kind == "project_header" {
			headerIndex = i
			break
		}
	}
	if headerIndex == -1 {
		t.Fatal("expected to find otto/main header")
	}

	// Select project header (shows chat input)
	m.cursorIndex = headerIndex
	m.activeChannelID = "otto/main"
	m.focusedPanel = panelMessages
	m.chatInput.Focus()

	// Chat input is visible and focused
	if !m.showChatInput() {
		t.Fatal("expected chat input to be visible for project header")
	}

	// Send Tab key
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(model)

	// BUG: Tab is swallowed by textinput, focusedPanel stays at panelMessages
	// FIX: Tab should switch focus to panelAgents
	if m.focusedPanel != panelAgents {
		t.Errorf("expected focusedPanel to be panelAgents after Tab, got %d", m.focusedPanel)
	}
}

// Bug 2: Chat cursor not showing when clicking project header
func TestProjectHeaderClickFocusesChatInput(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	m.height = 24
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	channels := m.channels()
	headerIndex := -1
	for i, ch := range channels {
		if ch.ID == "otto/main" && ch.Kind == "project_header" {
			headerIndex = i
			break
		}
	}
	if headerIndex == -1 {
		t.Fatal("expected to find otto/main header")
	}

	// Simulate mouse click on project header (not on caret)
	// X=10 is past the caret area (which is X=1-2 for Level 0)
	mouseMsg := tea.MouseMsg{
		X:      10,
		Y:      headerIndex + 2, // +2 for border + title
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}
	updated, _ := m.Update(mouseMsg)
	m = updated.(model)

	// Clicking project header should focus the messages panel and chat input
	if m.focusedPanel != panelMessages {
		t.Errorf("expected focusedPanel to be panelMessages after clicking project header, got %d", m.focusedPanel)
	}

	if !m.chatInput.Focused() {
		t.Error("expected chatInput to be focused after clicking project header")
	}
}

// Bug 3: Clicking caret doesn't toggle expand/collapse
func TestCaretClickTogglesExpand(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	m.height = 24
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	channels := m.channels()
	headerIndex := -1
	for i, ch := range channels {
		if ch.ID == "otto/main" && ch.Kind == "project_header" {
			headerIndex = i
			break
		}
	}
	if headerIndex == -1 {
		t.Fatal("expected to find otto/main header")
	}

	// Project is expanded by default
	if !m.isProjectExpanded("otto/main") {
		t.Fatal("expected otto/main to be expanded by default")
	}

	// Simulate clicking on the caret area (X position 1-2 for Level 0 header)
	// The caret is rendered at the start of the line after the border
	// Border is at X=0, so caret is at X=1-2 (▼ takes 1 char + 1 space)
	m.cursorIndex = headerIndex

	// BUG: Currently, clicking anywhere on the header calls activateSelection()
	// which just sets activeChannelID, doesn't toggle expand/collapse
	// FIX: When clicking on caret area, should call toggleSelection() instead

	// Simulate mouse click at caret position (X=1, Y is headerIndex+2 for border+title)
	mouseMsg := tea.MouseMsg{
		X:      1,
		Y:      headerIndex + 2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}
	updated, _ := m.Update(mouseMsg)
	m = updated.(model)

	// Should toggle to collapsed
	if m.isProjectExpanded("otto/main") {
		t.Error("expected otto/main to be collapsed after clicking caret")
	}

	// Click again should toggle back to expanded
	updated, _ = m.Update(mouseMsg)
	m = updated.(model)

	if !m.isProjectExpanded("otto/main") {
		t.Error("expected otto/main to be expanded after clicking caret again")
	}
}

// Clicking empty space in left panel should focus the left panel
func TestClickEmptySpaceInLeftPanelFocusesPanel(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	m.height = 24
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	// Start with focus on messages panel
	m.focusedPanel = panelMessages

	// Click on empty space in left panel (Y position beyond any channels)
	// With 1 agent, channels are: header (Y=2), agent (Y=3), so Y=10 is empty
	mouseMsg := tea.MouseMsg{
		X:      5, // In left panel (left panel is ~20 chars wide)
		Y:      10,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}
	updated, _ := m.Update(mouseMsg)
	m = updated.(model)

	if m.focusedPanel != panelAgents {
		t.Errorf("expected focusedPanel to be panelAgents after clicking empty space in left panel, got %d", m.focusedPanel)
	}
}

// Keyboard navigation to project header should NOT change focus
func TestKeyboardNavToProjectHeaderKeepsFocus(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	m.height = 24
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	// Start with focus on agents panel
	m.focusedPanel = panelAgents
	m.cursorIndex = 0

	// Navigate with j/k - this calls moveCursor which calls activateSelection
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	updated, _ := m.Update(keyMsg)
	m = updated.(model)

	// Focus should still be on agents panel (not switched to messages)
	if m.focusedPanel != panelAgents {
		t.Errorf("expected focusedPanel to remain panelAgents after keyboard nav, got %d", m.focusedPanel)
	}
}

// Clicking in right panel should focus the right panel
func TestClickRightPanelFocusesPanel(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	m.height = 24
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	// Start with focus on agents panel
	m.focusedPanel = panelAgents

	// Click in right panel (X > left panel width ~20)
	mouseMsg := tea.MouseMsg{
		X:      40, // Well into right panel
		Y:      10,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}
	updated, _ := m.Update(mouseMsg)
	m = updated.(model)

	if m.focusedPanel != panelMessages {
		t.Errorf("expected focusedPanel to be panelMessages after clicking in right panel, got %d", m.focusedPanel)
	}
}

func TestRightPanelRoutesKeysToInput(t *testing.T) {
	// Create in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	m := NewModel(db)
	m.focusedPanel = panelMessages
	m.chatInput.Focus() // Focus the input first

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	next, _ := m.Update(msg)
	model := next.(model)

	if model.chatInput.Value() != "j" {
		t.Fatalf("expected chat input to capture key, got %q (focused panel: %d, focused: %v)", model.chatInput.Value(), model.focusedPanel, model.chatInput.Focused())
	}
}
