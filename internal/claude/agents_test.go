// internal/claude/agents_test.go
package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sky-xo/june/internal/agent"
)

func TestScanAgents(t *testing.T) {
	// Create temp directory with fake agent files
	dir := t.TempDir()

	// Create agent-abc123.jsonl
	f1, _ := os.Create(filepath.Join(dir, "agent-abc123.jsonl"))
	f1.WriteString(`{"type":"user","message":{"content":"test"}}`)
	f1.Close()

	// Create agent-def456.jsonl
	f2, _ := os.Create(filepath.Join(dir, "agent-def456.jsonl"))
	f2.WriteString(`{"type":"user","message":{"content":"test2"}}`)
	f2.Close()

	// Create non-agent file (should be ignored)
	f3, _ := os.Create(filepath.Join(dir, "session-xyz.jsonl"))
	f3.WriteString(`{}`)
	f3.Close()

	agents, err := ScanAgents(dir)
	if err != nil {
		t.Fatalf("ScanAgents: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("got %d agents, want 2", len(agents))
	}

	// Check IDs were extracted correctly
	ids := make(map[string]bool)
	for _, a := range agents {
		ids[a.ID] = true
	}
	if !ids["abc123"] || !ids["def456"] {
		t.Errorf("expected agents abc123 and def456, got %v", ids)
	}
}

func TestAgentIsActive(t *testing.T) {
	agent := Agent{
		LastMod: time.Now().Add(-5 * time.Second),
	}
	if !agent.IsActive() {
		t.Error("agent modified 5s ago should be active")
	}

	agent.LastMod = time.Now().Add(-30 * time.Second)
	if agent.IsActive() {
		t.Error("agent modified 30s ago should not be active")
	}
}

func TestScanAgentsSorting(t *testing.T) {
	// Create temp directory with agent files
	dir := t.TempDir()

	// Create agents with specific IDs: charlie, alice, bob
	// All will be inactive with different mod times to test LastMod sorting
	baseTime := time.Now().Add(-1 * time.Hour)
	for i, id := range []string{"charlie", "alice", "bob"} {
		f, _ := os.Create(filepath.Join(dir, "agent-"+id+".jsonl"))
		f.WriteString(`{"type":"user","message":{"content":"test"}}`)
		f.Close()
		// Set different old mod times: charlie oldest, alice middle, bob newest
		modTime := baseTime.Add(time.Duration(i) * time.Minute)
		os.Chtimes(filepath.Join(dir, "agent-"+id+".jsonl"), modTime, modTime)
	}

	agents, err := ScanAgents(dir)
	if err != nil {
		t.Fatalf("ScanAgents: %v", err)
	}

	if len(agents) != 3 {
		t.Fatalf("got %d agents, want 3", len(agents))
	}

	// All inactive, should be sorted by LastMod descending (most recent first)
	// bob is newest (i=2), alice middle (i=1), charlie oldest (i=0)
	expectedOrder := []string{"bob", "alice", "charlie"}
	for i, expected := range expectedOrder {
		if agents[i].ID != expected {
			t.Errorf("position %d: got %s, want %s", i, agents[i].ID, expected)
		}
	}
}

func TestScanAgentsSortingWithActive(t *testing.T) {
	// Create temp directory with agent files
	dir := t.TempDir()

	// Create 4 agents
	for _, id := range []string{"delta", "alpha", "gamma", "beta"} {
		f, _ := os.Create(filepath.Join(dir, "agent-"+id+".jsonl"))
		f.WriteString(`{"type":"user","message":{"content":"test"}}`)
		f.Close()
	}

	// Make alpha and gamma inactive with different times
	// alpha: oldest, gamma: less old (so gamma should come first in inactive list)
	alphaTime := time.Now().Add(-2 * time.Hour)
	gammaTime := time.Now().Add(-1 * time.Hour)
	os.Chtimes(filepath.Join(dir, "agent-alpha.jsonl"), alphaTime, alphaTime)
	os.Chtimes(filepath.Join(dir, "agent-gamma.jsonl"), gammaTime, gammaTime)

	// delta and beta are active (recent mod time - already set by Create)

	agents, err := ScanAgents(dir)
	if err != nil {
		t.Fatalf("ScanAgents: %v", err)
	}

	if len(agents) != 4 {
		t.Fatalf("got %d agents, want 4", len(agents))
	}

	// Active agents first (alphabetical: beta, delta), then inactive (by LastMod: gamma newer, alpha older)
	expectedOrder := []string{"beta", "delta", "gamma", "alpha"}
	for i, expected := range expectedOrder {
		if agents[i].ID != expected {
			t.Errorf("position %d: got %s, want %s (active: %v)", i, agents[i].ID, expected, agents[i].IsActive())
		}
	}

	// Verify first two are active, last two are not
	if !agents[0].IsActive() || !agents[1].IsActive() {
		t.Error("first two agents should be active")
	}
	if agents[2].IsActive() || agents[3].IsActive() {
		t.Error("last two agents should be inactive")
	}
}

func TestScanAgentsExtractsDescription(t *testing.T) {
	dir := t.TempDir()

	// Create agent file with user message containing task description
	content := `{"type":"user","message":{"role":"user","content":"Implement feature X: add button\n\nMore details here..."}}
{"type":"assistant","message":{"role":"assistant","content":"I'll implement that."}}`
	f, _ := os.Create(filepath.Join(dir, "agent-test123.jsonl"))
	f.WriteString(content)
	f.Close()

	agents, err := ScanAgents(dir)
	if err != nil {
		t.Fatalf("ScanAgents: %v", err)
	}

	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	expected := "Implement feature X: add button"
	if agents[0].Description != expected {
		t.Errorf("expected description %q, got %q", expected, agents[0].Description)
	}
}

func TestAgent_IsRecent(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		lastMod  time.Time
		expected bool
	}{
		{"1 hour ago", now.Add(-1 * time.Hour), true},
		{"just under 2 hours ago", now.Add(-2*time.Hour + time.Minute), true},
		{"3 hours ago", now.Add(-3 * time.Hour), false},
		{"1 day ago", now.Add(-24 * time.Hour), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := Agent{LastMod: tt.lastMod}
			if got := agent.IsRecent(); got != tt.expected {
				t.Errorf("IsRecent() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestScanAgentsDescriptionEdgeCases(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		wantDesc string
	}{
		{
			name:     "empty file",
			content:  "",
			wantDesc: "",
		},
		{
			name:     "no user message",
			content:  `{"type":"assistant","message":{"role":"assistant","content":"Hello"}}`,
			wantDesc: "",
		},
		{
			name:     "malformed json",
			content:  `{not valid json`,
			wantDesc: "",
		},
		{
			name:     "long first line truncated",
			content:  `{"type":"user","message":{"role":"user","content":"` + strings.Repeat("a", 100) + `"}}`,
			wantDesc: strings.Repeat("a", 77) + "...",
		},
	}

	for i, tc := range tests {
		filename := fmt.Sprintf("agent-edge%d.jsonl", i)
		f, _ := os.Create(filepath.Join(dir, filename))
		f.WriteString(tc.content)
		f.Close()
	}

	agents, err := ScanAgents(dir)
	if err != nil {
		t.Fatalf("ScanAgents: %v", err)
	}

	// Sort by ID to have predictable order
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].ID < agents[j].ID
	})

	for i, tc := range tests {
		if i >= len(agents) {
			t.Errorf("test %q: agent not found", tc.name)
			continue
		}
		if agents[i].Description != tc.wantDesc {
			t.Errorf("test %q: expected description %q, got %q", tc.name, tc.wantDesc, agents[i].Description)
		}
	}
}

func TestAgent_ToUnified(t *testing.T) {
	claudeAgent := Agent{
		ID:          "abc123",
		FilePath:    "/path/to/agent-abc123.jsonl",
		LastMod:     time.Now(),
		Description: "Fix the auth bug",
	}

	// Channel context would be provided by the caller
	unified := claudeAgent.ToUnified("/Users/test/project", "main")

	if unified.ID != claudeAgent.ID {
		t.Errorf("ID = %q, want %q", unified.ID, claudeAgent.ID)
	}
	if unified.Name != claudeAgent.Description {
		t.Errorf("Name = %q, want %q (from Description)", unified.Name, claudeAgent.Description)
	}
	if unified.Source != agent.SourceClaude {
		t.Errorf("Source = %q, want %q", unified.Source, agent.SourceClaude)
	}
	if unified.RepoPath != "/Users/test/project" {
		t.Errorf("RepoPath = %q, want %q", unified.RepoPath, "/Users/test/project")
	}
	if unified.Branch != "main" {
		t.Errorf("Branch = %q, want %q", unified.Branch, "main")
	}
	if unified.TranscriptPath != claudeAgent.FilePath {
		t.Errorf("TranscriptPath = %q, want %q", unified.TranscriptPath, claudeAgent.FilePath)
	}
	if !unified.LastActivity.Equal(claudeAgent.LastMod) {
		t.Errorf("LastActivity = %v, want %v", unified.LastActivity, claudeAgent.LastMod)
	}
	if unified.PID != 0 {
		t.Errorf("PID = %d, want 0 (Claude agents don't track PID)", unified.PID)
	}
}
