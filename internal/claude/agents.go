// internal/claude/agents.go
package claude

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const activeThreshold = 10 * time.Second

// Agent represents a Claude Code subagent session.
type Agent struct {
	ID       string    // Extracted from filename: agent-{id}.jsonl
	FilePath string    // Full path to jsonl file
	LastMod  time.Time // File modification time
}

// IsActive returns true if the agent was modified within the active threshold.
func (a Agent) IsActive() bool {
	return time.Since(a.LastMod) < activeThreshold
}

// ScanAgents finds all agent-*.jsonl files in a directory.
// Returns agents sorted by: 1) active status (active first), 2) agent ID (alphabetical).
func ScanAgents(dir string) ([]Agent, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No agents yet
		}
		return nil, err
	}

	var agents []Agent
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "agent-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}

		// Extract ID: agent-abc123.jsonl -> abc123
		id := strings.TrimPrefix(name, "agent-")
		id = strings.TrimSuffix(id, ".jsonl")

		info, err := e.Info()
		if err != nil {
			continue
		}

		agents = append(agents, Agent{
			ID:       id,
			FilePath: filepath.Join(dir, name),
			LastMod:  info.ModTime(),
		})
	}

	// Sort by: 1) active status (active first), 2) agent ID (alphabetical, stable)
	sort.Slice(agents, func(i, j int) bool {
		iActive := agents[i].IsActive()
		jActive := agents[j].IsActive()
		if iActive != jActive {
			return iActive // active agents come first
		}
		// Within same status, sort alphabetically by ID for stable ordering
		return agents[i].ID < agents[j].ID
	})

	return agents, nil
}
