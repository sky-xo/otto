// internal/agent/agent_test.go
package agent

import (
	"testing"
	"time"
)

func TestAgent_DisplayName(t *testing.T) {
	tests := []struct {
		name     string
		agent    Agent
		expected string
	}{
		{
			name:     "uses Name when set",
			agent:    Agent{ID: "abc", Name: "fix-auth"},
			expected: "fix-auth",
		},
		{
			name:     "falls back to ID when Name empty",
			agent:    Agent{ID: "abc123"},
			expected: "abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.agent.DisplayName(); got != tt.expected {
				t.Errorf("DisplayName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAgent_IsActive(t *testing.T) {
	active := Agent{LastActivity: time.Now().Add(-5 * time.Second)}
	if !active.IsActive() {
		t.Error("agent modified 5s ago should be active")
	}

	inactive := Agent{LastActivity: time.Now().Add(-30 * time.Second)}
	if inactive.IsActive() {
		t.Error("agent modified 30s ago should not be active")
	}
}

func TestAgent_IsRecent(t *testing.T) {
	recent := Agent{LastActivity: time.Now().Add(-1 * time.Hour)}
	if !recent.IsRecent() {
		t.Error("agent modified 1h ago should be recent")
	}

	old := Agent{LastActivity: time.Now().Add(-3 * time.Hour)}
	if old.IsRecent() {
		t.Error("agent modified 3h ago should not be recent")
	}
}
