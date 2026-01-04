# TUI Codex Integration Design

**Date:** 2026-01-03
**Status:** Draft

## Overview

Integrate Codex agents into June's TUI so they appear alongside Claude Code agents in the channel-based sidebar. Users should see all their AI coding agents in one place, regardless of which system spawned them.

## Goals

1. Codex agents appear in the same channel list as Claude agents
2. Unified transcript rendering works for both agent types
3. Extensible for future agent types (Gemini, etc.)

## Design Decisions

### 1. Channel Integration

**Decision:** Codex agents are mixed into channels alongside Claude agents (not a separate section).

**Rationale:**
- Both agent types work on the same codebase
- User-given names for Codex agents are distinct enough
- Icons can be added later if visual distinction is needed

### 2. Git Context Capture

**Decision:** Capture git repo path and branch when spawning Codex agents.

**Implementation:**
- Add `repo_path` and `branch` columns to the `agents` table in SQLite
- Populate at spawn time using existing `internal/scope/` package
- Use these fields to group Codex agents into channels

**Schema change:**
```sql
ALTER TABLE agents ADD COLUMN repo_path TEXT;
ALTER TABLE agents ADD COLUMN branch TEXT;
```

### 3. Package Structure

**Decision:** Create new `internal/agent/` package for shared types.

```
internal/
├── agent/           # NEW - shared types
│   ├── agent.go     # Unified Agent struct
│   └── part.go      # Transcript Part types
├── claude/          # Claude-specific parsing → produces agent.Agent
├── codex/           # Codex-specific parsing → produces agent.Agent
├── db/              # SQLite storage (keeps Cursor internally)
└── tui/             # Consumes agent.Agent and agent.Part
```

**Dependencies:**
- `claude` → `agent`
- `codex` → `agent`
- `tui` → `agent`
- `db` → `agent` (for Agent fields, keeps Cursor separately)

### 4. Unified Agent Struct

```go
package agent

import "time"

// Agent represents any AI coding agent (Claude, Codex, etc.)
type Agent struct {
    // Identity
    ID     string // ULID or extracted from filename
    Name   string // Display name (user-given for Codex, extracted from transcript for Claude)
    Source string // "claude" or "codex"

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
    return time.Since(a.LastActivity) < 10*time.Second
}

// IsRecent returns true if the agent was modified within 2 hours.
func (a Agent) IsRecent() bool {
    return time.Since(a.LastActivity) < 2*time.Hour
}
```

### 5. Part-Based Transcript Model

**Decision:** Use a part-based architecture where transcripts are sequences of typed parts.

```go
package agent

import "time"

// Part is the interface all transcript parts implement.
type Part interface {
    PartType() string
    PartTime() time.Time
}

// TextPart represents user or assistant text content.
type TextPart struct {
    Role    string    // "user" or "assistant"
    Content string
    Time    time.Time
}

func (p TextPart) PartType() string   { return "text" }
func (p TextPart) PartTime() time.Time { return p.Time }

// ThinkingPart represents reasoning/thinking content.
type ThinkingPart struct {
    Content string
    Time    time.Time
}

func (p ThinkingPart) PartType() string   { return "thinking" }
func (p ThinkingPart) PartTime() time.Time { return p.Time }

// ToolStatus represents the state of a tool execution.
type ToolStatus string

const (
    ToolStatusPending   ToolStatus = "pending"
    ToolStatusRunning   ToolStatus = "running"
    ToolStatusCompleted ToolStatus = "completed"
    ToolStatusError     ToolStatus = "error"
)

// ToolPart represents a tool call with its result.
// Combines call and result into single part with status.
type ToolPart struct {
    ID        string     // Tool call ID for correlation
    Name      string     // Tool name (Edit, Bash, Read, etc.)
    Input     string     // Tool input (JSON or formatted)
    Output    string     // Tool output (filled when complete)
    Status    ToolStatus
    StartTime time.Time
    EndTime   time.Time  // Filled when complete
    Error     string     // Filled on error
}

func (p ToolPart) PartType() string   { return "tool" }
func (p ToolPart) PartTime() time.Time { return p.StartTime }

// Duration returns how long the tool took to execute.
// Returns 0 if not yet complete.
func (p ToolPart) Duration() time.Duration {
    if p.EndTime.IsZero() {
        return 0
    }
    return p.EndTime.Sub(p.StartTime)
}

// Transcript represents a parsed agent transcript.
type Transcript struct {
    AgentID string
    Parts   []Part
}
```

### 6. Parser Architecture

Each agent type has its own parser that produces the common `agent.Part` types:

```go
// internal/claude/parser.go
func ParseTranscript(path string) (*agent.Transcript, error) {
    // Read JSONL, convert to agent.Part types
    // - type:"user"/"assistant" → agent.TextPart
    // - content with type:"thinking" → agent.ThinkingPart
    // - content with type:"tool_use" → agent.ToolPart (pending)
    // - tool_result → update matching ToolPart to completed
}

// internal/codex/parser.go
func ParseTranscript(path string) (*agent.Transcript, error) {
    // Read session JSONL, convert to agent.Part types
    // - type:"function_call" → agent.ToolPart (pending)
    // - type:"function_call_output" → update matching ToolPart
    // - agent_reasoning → agent.ThinkingPart
    // - user messages → agent.TextPart
}
```

### 7. Channel Scanning

Modify `ScanChannels` to merge both agent sources:

```go
func ScanChannels(claudeProjectsDir, codexDB, basePath, repoName string) ([]Channel, error) {
    // 1. Scan Claude agents (existing logic)
    claudeAgents := scanClaudeAgents(claudeProjectsDir, basePath, repoName)

    // 2. Load Codex agents from DB, filter by repo/branch
    codexAgents := loadCodexAgents(codexDB, basePath)

    // 3. Group all agents by channel (repo:branch)
    // 4. Sort and return
}
```

### 8. Cursor/Read State

**Decision:** `Cursor` (for `june peek`) stays in the DB layer only, not in the unified Agent struct.

**Rationale:** Cursor is read state for the peek command, not agent metadata. The TUI doesn't need it.

## Implementation Plan

See: `docs/plans/2026-01-03-tui-codex-integration-impl.md`

## Future Considerations

- **Streaming:** Part model supports status updates for live viewing
- **Additional part types:** StepStart/StepFinish for metrics, AgentPart for subagent references
- **More agent types:** Gemini, OpenCode, etc. - just add a new parser
- **Icons:** Could add Source-based icons to distinguish agent types visually

## References

- [sst/opencode](https://github.com/sst/opencode) - Part-based architecture inspiration
- Existing `internal/claude/` and `internal/codex/` packages
