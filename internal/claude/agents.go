// internal/claude/agents.go
package claude

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sky-xo/june/internal/agent"
)

const activeThreshold = 10 * time.Second
const recentThreshold = 2 * time.Hour

// Agent represents a Claude Code subagent session.
type Agent struct {
	ID          string    // Extracted from filename: agent-{id}.jsonl
	FilePath    string    // Full path to jsonl file
	LastMod     time.Time // File modification time
	Description string    // First line of first user message (task description)
}

// extractDescription reads the first user message from a JSONL file
// and returns its first line as the description.
func extractDescription(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Set larger buffer for potentially long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 256*1024)

	for scanner.Scan() {
		var entry struct {
			Type    string `json:"type"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		// Look for first user message
		if entry.Type == "user" && entry.Message.Role == "user" && entry.Message.Content != "" {
			content := entry.Message.Content
			// Extract first line (up to newline or 80 chars)
			if idx := strings.Index(content, "\n"); idx != -1 {
				content = content[:idx]
			}
			if len(content) > 80 {
				content = content[:77] + "..."
			}
			return strings.TrimSpace(content)
		}
	}
	return ""
}

// IsActive returns true if the agent was modified within the active threshold.
func (a Agent) IsActive() bool {
	return time.Since(a.LastMod) < activeThreshold
}

// IsRecent returns true if the agent was modified within the recent threshold (2 hours).
func (a Agent) IsRecent() bool {
	return time.Since(a.LastMod) < recentThreshold
}

// ToUnified converts a claude.Agent to the unified agent.Agent type.
// repoPath and branch come from the channel context.
func (a Agent) ToUnified(repoPath, branch string) agent.Agent {
	return agent.Agent{
		ID:             a.ID,
		Name:           a.Description, // Use extracted description as display name
		Source:         agent.SourceClaude,
		RepoPath:       repoPath,
		Branch:         branch,
		TranscriptPath: a.FilePath,
		LastActivity:   a.LastMod,
		PID:            0, // Claude agents don't track PID
	}
}

// ScanAgents finds all agent-*.jsonl files in a directory.
// Returns agents sorted by: 1) active first, 2) active by ID (stable), inactive by LastMod (recent first).
// It uses a description cache to look up short descriptions from parent sessions.
func ScanAgents(dir string) ([]Agent, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No agents yet
		}
		return nil, err
	}

	// Load description cache and scan session files for any new descriptions
	cache := LoadDescriptionCache()
	originalSize := len(cache)
	cache = ScanSessionFilesForDescriptions(dir, cache)

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

		// Look up description from cache, fall back to extractDescription if not found
		desc := cache[id]
		if desc == "" {
			desc = extractDescription(filepath.Join(dir, name))
		}

		agents = append(agents, Agent{
			ID:          id,
			FilePath:    filepath.Join(dir, name),
			LastMod:     info.ModTime(),
			Description: desc,
		})
	}

	// Save cache if we found new descriptions
	if len(cache) > originalSize {
		cache.Save()
	}

	// Sort by: 1) active status (active first), 2) secondary sort depends on status
	// - Active agents: alphabetically by ID (stable, so they don't jump while working)
	// - Inactive agents: by LastMod descending (most recently finished first)
	sort.Slice(agents, func(i, j int) bool {
		iActive := agents[i].IsActive()
		jActive := agents[j].IsActive()
		if iActive != jActive {
			return iActive // active agents come first
		}
		if iActive {
			// Both active: sort alphabetically by ID for stable ordering
			return agents[i].ID < agents[j].ID
		}
		// Both inactive: sort by LastMod descending (most recent first)
		return agents[i].LastMod.After(agents[j].LastMod)
	})

	return agents, nil
}
