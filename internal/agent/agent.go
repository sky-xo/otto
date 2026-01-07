// internal/agent/agent.go
package agent

import "time"

const (
	activeThreshold = 10 * time.Second
	recentThreshold = 2 * time.Hour
)

// Source identifies which system spawned an agent.
const (
	SourceClaude = "claude"
	SourceCodex  = "codex"
	SourceGemini = "gemini"
)

// Agent represents any AI coding agent (Claude, Codex, etc.)
type Agent struct {
	// Identity
	ID     string // ULID or extracted from filename
	Name   string // Display name (user-given for Codex, extracted from transcript for Claude)
	Source string // "claude", "codex", or "gemini"

	// Channel grouping
	RepoPath string // Git repo path
	Branch   string // Git branch

	// Transcript
	TranscriptPath string // Path to JSONL or session file

	// Activity
	LastActivity time.Time
	PID          int // Process ID if running, 0 otherwise
}

// DisplayName returns the best name for UI display.
// Falls back to ID if Name is empty.
func (a Agent) DisplayName() string {
	if a.Name != "" {
		return a.Name
	}
	return a.ID
}

// IsActive returns true if the agent was recently modified.
func (a Agent) IsActive() bool {
	return time.Since(a.LastActivity) < activeThreshold
}

// IsRecent returns true if the agent was modified within 2 hours.
func (a Agent) IsRecent() bool {
	return time.Since(a.LastActivity) < recentThreshold
}

// Channel represents a group of agents from a branch/worktree.
type Channel struct {
	Name   string  // Display name like "june:main"
	Agents []Agent // Mixed Claude and Codex agents
}

// HasRecentActivity returns true if any agent is active or recent.
func (c Channel) HasRecentActivity() bool {
	for _, a := range c.Agents {
		if a.IsActive() || a.IsRecent() {
			return true
		}
	}
	return false
}
