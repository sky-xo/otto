# Agent Diff Capture Design

> **Status: DRAFT** - Needs further design work before implementation.

## Problem

The TUI transcript shows file change events but not the actual diffs. When an agent modifies code, users see "file.go (update)" but can't see what changed without manually running `git diff`.

## Current State

### What We Capture

| Agent Type | File Changes | Diffs |
|------------|--------------|-------|
| Claude Code | ❌ Not parsed (stored as raw "output") | Available in JSON but not extracted |
| Codex | ✅ Parsed (`file_change` events) | ❌ Not available in `exec` output |

### Claude Code Output (Verified)

Claude Code's `--output-format stream-json` includes full diff information in `tool_use_result`:

```json
{
  "type": "user",
  "tool_use_result": {
    "filePath": "/path/to/file.go",
    "oldString": "hello world",
    "newString": "goodbye world",
    "originalFile": "hello world\n",
    "structuredPatch": [
      {
        "oldStart": 1,
        "oldLines": 1,
        "newStart": 1,
        "newLines": 1,
        "lines": ["-hello world", "+goodbye world"]
      }
    ]
  }
}
```

**We already have this data - we just need to parse it.**

### Codex Output (Verified)

Codex `exec --json` only emits metadata:

```json
{
  "type": "item.completed",
  "item": {
    "type": "file_change",
    "changes": [{"path": "/path/file.go", "kind": "update"}]
  }
}
```

**No diff content in `exec` mode.**

---

## Codex `app-server` Discovery

Research revealed Codex has a separate `app-server` mode that provides richer output.

### What is `app-server`?

- JSON-RPC 2.0 protocol over stdio (similar to MCP/LSP)
- Long-running process (vs one-shot `exec`)
- Used by VS Code extension
- Marked as **experimental** - may change without notice

### Key Difference: `turn/diff/updated` Event

App-server emits streaming diffs:

```json
{
  "method": "turn/diff/updated",
  "params": {
    "diff": "<unified diff string>",
    "threadId": "...",
    "turnId": "..."
  }
}
```

Schema confirmed via `codex app-server generate-json-schema`.

### Trade-offs

| Aspect | `exec` (current) | `app-server` |
|--------|------------------|--------------|
| Stability | Documented, stable | Experimental, may change |
| Complexity | Simple (spawn, read stdout) | Complex (JSON-RPC client, lifecycle) |
| Auth | `CODEX_API_KEY` supported | May not support env var auth |
| Diffs | ❌ Not available | ✅ `turn/diff/updated` |
| Use case | CI/automation | IDE integrations |

---

## Proposed Approaches

### Option A: Parse Claude + Git Diff for Codex (Recommended for V0)

**Claude Code:**
1. Parse JSON output properly (currently stored as raw text)
2. Extract `tool_use_result` blocks with file changes
3. Store `filePath`, `oldString`, `newString`, `structuredPatch`

**Codex:**
1. After each `file_change` event, run `git diff <file>`
2. Capture the diff at that moment
3. Store alongside the file_change log entry

**Pros:**
- Simpler implementation
- No new protocols to implement
- Works with current architecture

**Cons:**
- Git diff approach has race conditions (multiple rapid changes)
- Requires git to be initialized in working directory

### Option B: Codex App-Server Integration

**Approach:**
1. Switch Codex agents from `exec` to `app-server`
2. Implement JSON-RPC client in Go
3. Subscribe to `turn/diff/updated` events
4. Store diffs as they stream

**Pros:**
- First-class diff support
- Streaming updates
- Richer protocol (thread management, etc.)

**Cons:**
- Significant rewrite of Codex spawn/prompt code
- Protocol is experimental and may change
- More complex error handling
- Auth differences (`CODEX_API_KEY` may not work)

### Option C: Hybrid (Future)

- Keep `exec` for spawning/prompting
- Spawn parallel `app-server` process just for diff capture
- Correlate events by thread/turn ID

---

## Schema Changes (Either Option)

Add columns to `logs` table:

```sql
ALTER TABLE logs ADD COLUMN file_path TEXT;
ALTER TABLE logs ADD COLUMN file_diff TEXT;
```

Or create normalized table:

```sql
CREATE TABLE file_changes (
  id TEXT PRIMARY KEY,
  log_id TEXT REFERENCES logs(id),
  file_path TEXT NOT NULL,
  change_kind TEXT NOT NULL,  -- 'create', 'update', 'delete'
  old_content TEXT,
  new_content TEXT,
  diff TEXT,  -- unified diff format
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## TUI Display

For transcript view:

```
$ command output...

📝 internal/cli/commands/spawn.go (update)
┌──────────────────────────────────────────────
│ -    tempDir, err := os.MkdirTemp("", "june-codex-*")
│ +    codexHome, err := ensureCodexHome()
│      if err != nil {
│ -        return fmt.Errorf("create temp CODEX_HOME: %w", err)
│ +        return err
│      }
└──────────────────────────────────────────────

∴ Reasoning about next steps...
```

Consider:
- Collapsible diff sections (large diffs)
- Syntax highlighting (lipgloss)
- Line number display
- Context lines (configurable)

---

## Implementation Order

### Phase 1: Claude JSON Parsing (Quick Win)
- [ ] Add Claude event parser (similar to `codex_events.go`)
- [ ] Extract `tool_use_result` file changes
- [ ] Store in logs table with file_path, file_diff
- [ ] Display in TUI

### Phase 2: Codex Git-Based Diffs (Fallback)
- [ ] After `file_change` event, run `git diff <file>`
- [ ] Store captured diff
- [ ] Handle edge cases (untracked files, binary files)

### Phase 3: Codex App-Server (Future)
- [ ] Evaluate stability of app-server protocol
- [ ] Prototype JSON-RPC client
- [ ] Decide on migration path

---

## Open Questions

1. **Schema choice**: Single `file_diff` column vs normalized `file_changes` table?
2. **Storage limits**: Should we truncate large diffs? How large is too large?
3. **Binary files**: How to handle non-text file changes?
4. **App-server auth**: Does it work with `CODEX_API_KEY` or require OAuth flow?
5. **TUI performance**: Will rendering large diffs impact scroll performance?

---

## References

- [Codex CLI Reference](https://developers.openai.com/codex/cli/reference/)
- [Codex SDK Docs](https://developers.openai.com/codex/sdk/)
- [Codex Changelog](https://developers.openai.com/codex/changelog/)
- App-server JSON schema: `codex app-server generate-json-schema --out <dir>`
- Claude Code: `claude --print --verbose --output-format stream-json`
