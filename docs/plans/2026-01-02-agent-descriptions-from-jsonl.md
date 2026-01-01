# Agent Descriptions from JSONL Implementation Plan

> **Status:** In Progress - Revised approach after research

**Goal:** Display meaningful agent descriptions in June's TUI sidebar instead of random 8-character IDs.

**Key Insight:** The short description (e.g., "Implement Task 1: Agent field") is stored in the **parent session's JSONL** when the Task tool is called, not in the agent's own JSONL file. The agent's first user message contains the full verbose prompt, which is too long.

**Architecture:**
1. Scan parent session JSONL files for Task tool calls
2. Extract the short `description` parameter and link it to the `agentId` in the tool result
3. Cache the mapping in `~/.june/agent-descriptions.json` for fast lookups
4. Fall back to agent ID if no description found

**Tech Stack:** Go, JSONL parsing, JSON cache file

---

## Completed Work

### ✅ Task 1: Add Description field to Agent struct
- Added `Description` field to `Agent` struct in `internal/claude/agents.go`
- Added `extractDescription` helper (to be replaced with new approach)
- Commit: d933018

### ✅ Task 2: Edge case tests
- Added tests for empty files, malformed JSON, etc.
- Commit: 45f53fe

### ✅ Task 3: Display description in TUI sidebar
- Updated `renderSidebarContent` to show description instead of ID
- Falls back to ID when no description available
- Commit: 628053f

---

## Remaining Work

### Task 4: Implement description lookup from parent session files

**Files:**
- Modify: `internal/claude/agents.go`
- Create: `internal/claude/descriptions.go`
- Test: `internal/claude/descriptions_test.go`

#### Step 1: Create descriptions.go with cache and lookup logic

```go
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

// SaveDescriptionCache saves the cache to disk
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
```

#### Step 2: Update ScanAgents to use the description cache

Modify `internal/claude/agents.go`:

```go
// ScanAgents scans a directory for agent JSONL files and returns them.
// It uses a description cache to look up short descriptions from parent sessions.
func ScanAgents(dir string) ([]Agent, error) {
    // Load description cache
    cache := LoadDescriptionCache()
    originalSize := len(cache)

    // Scan session files for any new descriptions
    cache = ScanSessionFilesForDescriptions(dir, cache)

    // ... existing agent scanning logic ...

    agents = append(agents, Agent{
        ID:          id,
        FilePath:    filepath.Join(dir, name),
        LastMod:     info.ModTime(),
        Description: cache[id], // Look up from cache instead of extractDescription
    })

    // ... rest of function ...

    // Save cache if we found new descriptions
    if len(cache) > originalSize {
        cache.Save()
    }

    return agents, nil
}
```

#### Step 3: Remove or deprecate extractDescription

The old `extractDescription` function that reads the first user message can be removed or kept as a fallback.

#### Step 4: Write tests

```go
// internal/claude/descriptions_test.go
func TestLoadAndSaveDescriptionCache(t *testing.T) {
    // Test cache round-trip
}

func TestScanSessionFilesForDescriptions(t *testing.T) {
    // Create temp dir with mock session file containing Task tool calls
    // Verify descriptions are extracted correctly
}
```

#### Step 5: Run tests and commit

```bash
go test ./internal/claude -v
git add internal/claude/
git commit -m "feat(claude): lookup agent descriptions from parent session files

Instead of extracting the first line of the agent's prompt (too verbose),
look up the short description from Task tool calls in parent session files.
Cache results in ~/.june/agent-descriptions.json for fast lookups."
```

---

### Task 5: Update right panel title to show description with ID

**Files:**
- Modify: `internal/tui/model.go`
- Test: `internal/tui/model_test.go`

#### Step 1: Add tests for new title format

```go
func TestViewShowsDescriptionAndIDInRightPanel(t *testing.T) {
    m := Model{
        agents: []claude.Agent{
            {ID: "abc12345", Description: "Fix login bug", FilePath: "/tmp/test.jsonl"},
        },
        selectedIdx: 0,
        width:       80,
        height:      24,
    }

    view := m.View()

    // Should show "Description (ID) | timestamp" format
    if !strings.Contains(view, "Fix login bug") {
        t.Errorf("expected description in right panel")
    }
    if !strings.Contains(view, "(abc12345)") {
        t.Errorf("expected ID in parentheses")
    }
}

func TestViewShowsOnlyIDWhenNoDescription(t *testing.T) {
    // When no description, fall back to just "ID | timestamp"
}
```

#### Step 2: Update View() function

```go
if agent := m.SelectedAgent(); agent != nil {
    if agent.Description != "" {
        rightTitle = fmt.Sprintf("%s (%s) | %s", agent.Description, agent.ID, formatTimestamp(agent.LastMod))
    } else {
        rightTitle = fmt.Sprintf("%s | %s", agent.ID, formatTimestamp(agent.LastMod))
    }
}
```

#### Step 3: Commit

```bash
git add internal/tui/
git commit -m "feat(tui): show description with ID in right panel title

Format: 'Description (ID) | timestamp' when description available,
falls back to 'ID | timestamp' when no description."
```

---

### Task 6: Final verification

1. Run all tests: `go test ./...`
2. Build: `make build`
3. Manual test: `./june` - verify sidebar shows short descriptions
4. Verify cache file created: `cat ~/.june/agent-descriptions.json`

---

## Summary

| Task | Status | Description |
|------|--------|-------------|
| 1 | ✅ Done | Add Description field to Agent struct |
| 2 | ✅ Done | Edge case tests |
| 3 | ✅ Done | Display in sidebar |
| 4 | 🔄 TODO | Implement parent session lookup + cache |
| 5 | 🔄 TODO | Update right panel title format |
| 6 | 🔄 TODO | Final verification |
