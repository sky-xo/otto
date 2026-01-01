// internal/claude/descriptions.go
package claude

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const cacheFile = ".june/agent-descriptions.json"

// DescriptionCache maps agentId -> short description
type DescriptionCache map[string]string

// LoadDescriptionCache loads the cache from ~/.june/agent-descriptions.json
func LoadDescriptionCache() DescriptionCache {
	home, err := os.UserHomeDir()
	if err != nil {
		return make(DescriptionCache)
	}

	path := filepath.Join(home, cacheFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return make(DescriptionCache)
	}

	var cache DescriptionCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return make(DescriptionCache)
	}
	return cache
}

// Save saves the cache to disk
func (c DescriptionCache) Save() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dir := filepath.Join(home, ".june")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(home, cacheFile)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ScanSessionFilesForDescriptions scans non-agent JSONL files in a directory
// looking for Task tool calls and their results to extract agentId -> description mappings.
func ScanSessionFilesForDescriptions(dir string, cache DescriptionCache) DescriptionCache {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return cache
	}

	for _, entry := range entries {
		name := entry.Name()
		// Skip agent files (agent-*.jsonl), only look at session files
		if !strings.HasSuffix(name, ".jsonl") || strings.HasPrefix(name, "agent-") {
			continue
		}

		scanSessionFile(filepath.Join(dir, name), cache)
	}

	return cache
}

// scanSessionFile scans a single session file for Task tool descriptions
func scanSessionFile(path string, cache DescriptionCache) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB buffer for large lines

	// Track pending Task tool calls by tool_use_id
	pendingTasks := make(map[string]string) // tool_use_id -> description

	for scanner.Scan() {
		line := scanner.Bytes()

		// Look for Task tool calls with description
		if strings.Contains(string(line), `"name":"Task"`) {
			var entry struct {
				Message struct {
					Content []struct {
						Type  string `json:"type"`
						ID    string `json:"id"`
						Name  string `json:"name"`
						Input struct {
							Description string `json:"description"`
						} `json:"input"`
					} `json:"content"`
				} `json:"message"`
			}
			if err := json.Unmarshal(line, &entry); err == nil {
				for _, content := range entry.Message.Content {
					if content.Type == "tool_use" && content.Name == "Task" && content.Input.Description != "" {
						pendingTasks[content.ID] = content.Input.Description
					}
				}
			}
		}

		// Look for tool results with agentId
		if strings.Contains(string(line), `"agentId"`) && strings.Contains(string(line), `"tool_result"`) {
			var entry struct {
				Message struct {
					Content []struct {
						Type      string `json:"type"`
						ToolUseID string `json:"tool_use_id"`
					} `json:"content"`
				} `json:"message"`
				ToolUseResult struct {
					AgentID string `json:"agentId"`
				} `json:"toolUseResult"`
			}
			if err := json.Unmarshal(line, &entry); err == nil {
				if entry.ToolUseResult.AgentID != "" {
					// Find the tool_use_id from content
					for _, content := range entry.Message.Content {
						if content.Type == "tool_result" {
							if desc, ok := pendingTasks[content.ToolUseID]; ok {
								cache[entry.ToolUseResult.AgentID] = desc
							}
						}
					}
				}
			}
		}
	}
}
