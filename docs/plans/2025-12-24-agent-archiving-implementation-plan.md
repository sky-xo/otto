# Agent Archiving Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement explicit agent archiving with `archived_at`, CLI commands/flags, TUI archived section, and retention based on archived age.

**Architecture:** Add `archived_at` to agents and filter status output to hide archived agents by default. Introduce `june archive <id>` and `june status --archive/--all` for explicit archiving. Auto-unarchive on orchestrator actions (prompt/attach). Retention cleanup deletes only agents archived for 7+ days.

**Tech Stack:** Go, SQLite (modernc), Cobra CLI, Bubble Tea TUI.

### Task 1: Schema + repo support for archived_at

**Files:**
- Modify: `internal/db/db.go`
- Modify: `internal/repo/agents.go`
- Test: `internal/repo/agents_test.go`
- Test: `internal/db/db_test.go`

**Step 1: Write failing repo test for archived_at**

```go
func TestArchiveAgentSetsArchivedAt(t *testing.T) {
    db := openTestDB(t)
    t.Cleanup(func() { _ = db.Close() })

    err := CreateAgent(db, Agent{ID: "arch-me", Type: "claude", Task: "task", Status: "complete"})
    if err != nil { t.Fatalf("create: %v", err) }

    if err := ArchiveAgent(db, "arch-me"); err != nil {
        t.Fatalf("archive: %v", err)
    }

    got, err := GetAgent(db, "arch-me")
    if err != nil { t.Fatalf("get: %v", err) }
    if !got.ArchivedAt.Valid { t.Fatalf("expected archived_at to be set") }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/repo -run TestArchiveAgentSetsArchivedAt -v`
Expected: FAIL with missing function/field.

**Step 3: Implement archived_at support**

- Add `archived_at` column migration + index in `internal/db/db.go`.
- Extend `Agent` with `ArchivedAt sql.NullTime`.
- Update Create/Get/List scans to include `archived_at`.
- Add helpers:
  - `ArchiveAgent(db, id)` sets `archived_at = CURRENT_TIMESTAMP`.
  - `UnarchiveAgent(db, id)` clears `archived_at = NULL`.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/repo -run TestArchiveAgentSetsArchivedAt -v`
Expected: PASS

**Step 5: Add db schema test**

Add assertion for `archived_at` column + index in `internal/db/db_test.go`.

**Step 6: Run db tests**

Run: `go test ./internal/db -v`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/db/db.go internal/repo/agents.go internal/repo/agents_test.go internal/db/db_test.go
git commit -m "feat: add archived_at to agents"
```

### Task 2: Status filtering + archive command

**Files:**
- Modify: `internal/cli/commands/status.go`
- Modify: `internal/repo/agents.go`
- Create: `internal/cli/commands/archive.go`
- Modify: `internal/cli/root.go`
- Test: `internal/cli/commands/commands_test.go`

**Step 1: Write failing CLI tests**

Add tests in `commands_test.go`:
- `status` excludes archived agents by default.
- `status --all` includes archived agents.
- `status --archive` archives complete/failed agents in view.
- `june archive <id>` archives a complete/failed agent, rejects busy/blocked.

**Step 2: Run tests to verify failure**

Run: `go test ./internal/cli/commands -run Status -v`
Expected: FAIL (flags/command missing).

**Step 3: Implement repo list filters**

Add `ListAgentsFiltered(db, includeArchived bool)` or extend `ListAgents` with a flag. Default exclude archived (`archived_at IS NULL`).

**Step 4: Implement CLI**

- Add flags:
  - `--all` (include archived)
  - `--archive` (bulk archive complete/failed agents shown)
- Implement `june archive <agent-id>` command.
- Register in `internal/cli/root.go`.

**Step 5: Run tests**

Run: `go test ./internal/cli/commands -run Status -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/cli/commands/status.go internal/cli/commands/archive.go internal/repo/agents.go internal/cli/root.go internal/cli/commands/commands_test.go
git commit -m "feat: add archive command and status filters"
```

### Task 3: Auto-unarchive on prompt/attach

**Files:**
- Modify: `internal/cli/commands/prompt.go`
- Modify: `internal/cli/commands/attach.go`
- Test: `internal/cli/commands/prompt_test.go`

**Step 1: Write failing tests**

- Ensure `prompt` clears `archived_at` before resume.
- Ensure `attach` clears `archived_at` even though it only prints command.

**Step 2: Run tests to verify failure**

Run: `go test ./internal/cli/commands -run Prompt -v`
Expected: FAIL

**Step 3: Implement**

- Call `repo.UnarchiveAgent` inside `runPrompt` and `runAttach`.

**Step 4: Run tests**

Run: `go test ./internal/cli/commands -run Prompt -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/commands/prompt.go internal/cli/commands/attach.go internal/cli/commands/prompt_test.go
git commit -m "feat: unarchive agents on prompt/attach"
```

### Task 4: Retention cleanup based on archived_at

**Files:**
- Modify: `internal/db/db.go`
- Test: `internal/db/db_test.go`

**Step 1: Write failing test**

Add a test that:
- Creates archived agent older than 7 days and confirms cleanup deletes it.
- Creates completed but unarchived agent older than 7 days and confirms it remains.

**Step 2: Run test to verify failure**

Run: `go test ./internal/db -run Cleanup -v`
Expected: FAIL

**Step 3: Implement retention change**

Update cleanup SQL to only delete agents (and related logs/messages) where `archived_at < datetime('now', '-7 days')`.

**Step 4: Run tests**

Run: `go test ./internal/db -run Cleanup -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go
git commit -m "feat: cleanup based on archived_at"
```

### Task 5: TUI archived section

**Files:**
- Modify: `internal/tui/watch.go`
- Test: `internal/tui/watch_test.go`

**Step 1: Write failing tests**

- Ensure archived agents are hidden by default.
- Ensure archived agents appear under an "Archived (N)" row when expanded.
- Ensure Enter on the archived row toggles expand/collapse.

**Step 2: Run tests to verify failure**

Run: `go test ./internal/tui -run Archived -v`
Expected: FAIL

**Step 3: Implement UI behavior**

- Track `archivedExpanded` boolean in model.
- Add a selectable Archived row in left panel.
- Toggle on Enter when that row is selected.
- Sort archived agents by `updated_at` (or `archived_at` if more appropriate).
- Mouse click toggles when enabled (if existing mouse events are handled).

**Step 4: Run tests**

Run: `go test ./internal/tui -run Archived -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tui/watch.go internal/tui/watch_test.go
git commit -m "feat: show archived agents in TUI"
```

### Task 6: Docs + CLI help

**Files:**
- Modify: `README.md`
- Modify: `docs/plans/2025-12-24-agent-archiving-design.md`

**Step 1: Update docs**

- Add `june archive` to command list.
- Mention `status --all` and `status --archive`.
- Update retention description if needed.

**Step 2: Commit**

```bash
git add README.md docs/plans/2025-12-24-agent-archiving-design.md
git commit -m "docs: document agent archiving"
```

---

## Progress Log (Checkpoint for Context Compaction)

**Worktree:** `/Users/glowy/code/june/.worktrees/agent-archiving`

**Task 1: Schema + repo support for archived_at** ✅ **Done**
- Commit: `feat: add archived_at to agents` (sha unknown in this file; already committed earlier)
- Files: `internal/db/db.go`, `internal/db/db_test.go`, `internal/repo/agents.go`, `internal/repo/agents_test.go`
- Tests run: `go test ./internal/repo -run TestArchiveAgentSetsArchivedAt -v`, `go test ./internal/db -v`

**Task 2: Status filtering + archive command** ✅ **Done**
- Commit: `feat: add archive command and status filters` (`ee791a8f99acaf68387e4eb6401371d1148c6ae3`)
- Files: `internal/cli/commands/status.go`, `internal/cli/commands/archive.go`, `internal/repo/agents.go`, `internal/cli/root.go`, `internal/cli/commands/commands_test.go`
- Tests added:
  - `TestStatusExcludesArchivedByDefault`
  - `TestStatusIncludesArchivedWithAll`
  - `TestStatusArchiveArchivesCompleteAndFailed`
- Test run: `go test ./internal/cli/commands -run Status -v`
- Spec review v2: ✅ no issues (agent `task-2-spec-review-v2`)

**Task 3: Auto-unarchive on prompt/attach** ✅ **Done** (commit `e97a44d`)
- `repo.UnarchiveAgent` added on prompt/attach; tests updated for unarchive behavior.

**Task 4: Retention cleanup based on archived_at** ✅ **Done** (commit `449fca3`)
- Cleanup deletes only agents/logs/messages with `archived_at < now-7 days`; completed-but-unarchived agents remain.
- Tests updated in `internal/db/db_test.go` for archived retention behavior.

**Task 5: TUI archived section** ✅ **Done** (commit `534e996`)
- TUI archived section added; tests in `internal/tui/watch_test.go` cover hidden-by-default, expanded list, and Enter toggle.
- Test run: `go test ./internal/tui -run Archived -v`

**Task 6: Docs + CLI help** ✅ **Done** (commit `48fb099`)
- README command list updated for `june archive` and `status --all/--archive`.
- Status help text notes archived hidden by default and archive behavior.
- Design doc summary updated to clarify retention based on archived age.
- Code review: non-blocking polish items logged in `TODO.md`.

### Operational Notes
- Orchestrator DB for this worktree lacked `archived_at`; manual migration applied:
  - `~/.june/orchestrators/agent-archiving/agent-archiving/june.db`
  - `ALTER TABLE agents ADD COLUMN archived_at DATETIME;`
  - `CREATE INDEX IF NOT EXISTS idx_agents_archived ON agents(archived_at) WHERE archived_at IS NOT NULL;`
- After migration, spawning via `/Users/glowy/code/june/.worktrees/agent-archiving/june` worked.

### Open Items / Next Steps
- Follow-up polish items tracked in `TODO.md`.
