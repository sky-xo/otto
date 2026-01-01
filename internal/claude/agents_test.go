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
