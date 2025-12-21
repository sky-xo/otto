package commands

import "testing"

func TestSpawnBuildsCommand(t *testing.T) {
	cmd := buildSpawnCommand("claude", "task", "sess-123")
	if got := cmd[0]; got != "claude" {
		t.Fatalf("expected claude, got %q", got)
	}
}
