package claude

import (
	"os"
	"testing"
)

func TestIntegrationWithRealFiles(t *testing.T) {
	// Skip if not in the otto project
	projectDir := ProjectDir("/Users/glowy/code/otto")
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		t.Skip("Skipping integration test: no Claude project directory found")
	}

	// Test ScanAgents with real files
	agents, err := ScanAgents(projectDir)
	if err != nil {
		t.Fatalf("ScanAgents: %v", err)
	}

	t.Logf("Found %d agents", len(agents))
	if len(agents) == 0 {
		t.Skip("No agent files found")
	}

	// Test ParseTranscript with first agent
	agent := agents[0]
	t.Logf("Testing with agent: %s", agent.ID)

	entries, err := ParseTranscript(agent.FilePath)
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}

	t.Logf("Parsed %d entries", len(entries))
	if len(entries) == 0 {
		t.Error("Expected at least one entry")
	}

	// Verify entry structure
	for i, e := range entries {
		if i >= 5 {
			break
		}
		t.Logf("  Entry %d: type=%s, content=%q", i, e.Type, truncate(e.TextContent(), 50))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
