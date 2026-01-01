// internal/claude/agents_test.go
package claude

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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
