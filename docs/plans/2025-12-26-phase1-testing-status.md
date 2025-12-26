# Phase 1 Testing Status

**Date:** 2025-12-26
**Goal:** Test Phase 1 implementation by spawning a Codex agent

## What We Did

1. **Spawned a Codex agent** (`fmt-1-4`) to implement Phase 2 Task 1 (log formatting)
2. **Verified Phase 1 features work in production:**
   - Project/branch-aware agents: ✅ Working (`super|super`)
   - Session ID capture: ✅ Working (real Codex thread IDs captured)
   - Codex event parsing: ✅ Working (reasoning, command_execution, agent_message events)
   - Messages with project/branch: ✅ Working

3. **Agent completed successfully** - created `internal/tui/formatting.go` and `internal/tui/formatting_test.go`

## Bugs Found & Fixed

### 1. NOT NULL Constraint Violations (FIXED)
Old database schema has NOT NULL constraints on columns we no longer use. Fixed by adding backwards-compat columns to INSERTs:

**Files modified:**
- `internal/repo/messages.go` - Added `from_id` to INSERT
- `internal/repo/logs.go` - Added `agent_id`, `direction` to INSERT
- `internal/repo/agents.go` - Added `id` to INSERT
- `internal/db/db.go` - Added backwards-compat columns to new schema

### 2. Test Scope Mismatch (PARTIALLY FIXED)
Tests create agents with hardcoded `Project: "test-project", Branch: "main"` but commands use `scope.CurrentContext()` which returns actual worktree values (`super`, `super`).

**Fix pattern:**
```go
// Added helper in commands_test.go
func testCtx() scope.Context {
    return scope.CurrentContext()
}

// Tests now use:
ctx := testCtx()
agent := repo.Agent{Project: ctx.Project, Branch: ctx.Branch, ...}
```

**Files updated:**
- `internal/cli/commands/commands_test.go` - All tests fixed
- `internal/cli/commands/attach_test.go` - Fixed
- `internal/cli/commands/interrupt_test.go` - Fixed
- `internal/cli/commands/log_test.go` - Fixed
- `internal/cli/commands/peek_test.go` - Fixed
- `internal/cli/commands/prompt_test.go` - Fully rewritten with ctx
- `internal/cli/commands/spawn_test.go` - Partially fixed (some hardcoded values remain)
- `internal/cli/commands/worker_spawn_test.go` - Partially fixed

### 3. TUI Tests Using Old `ID` Field (FIXED)
`internal/tui/watch_test.go` was using `repo.Agent{ID: ...}` but struct now uses `Name`.
Changed all `ID:` to `Name:` in test structs.

## Remaining Work

### Tests Still Failing (8 tests)

Run `make test` to see current status. Failures are:

1. **TestRunPeek** - Peek cursor tracking not working correctly
2. **TestPromptStoresPromptAndResumesAgent** - No transcript entries recorded
3. **TestPromptCapturesOutput** - No output entries recorded
4. **TestGenerateAgentIDUnique** - Still has hardcoded "test-project" at line 85, 102
5. **TestResolveAgentNameUnique** - Still has hardcoded "test-project" at line 153, 170
6. **TestSpawnStoresPromptAndTranscript** - No transcript entries + hardcoded values at line 242
7. **TestSpawnWithCustomNameCollision** - Hardcoded "test-project" at line 564
8. **TestWorkerSpawnCapturesPromptAndLogs** - No prompt entries recorded

### To Fix These Tests

1. **Replace remaining hardcoded values in spawn_test.go:**
   ```bash
   # Lines with "test-project" that need ctx:
   # 85, 102, 153, 170, 242, 564
   ```

2. **Investigate transcript/log entry issues:**
   - Tests expect `event_type = "input"` or `"prompt"` but we might be storing different types
   - Check what `storePrompt` and transcript capture actually store
   - The schema changed - verify tests match new schema expectations

### Quick Fix Commands

```bash
# Check remaining hardcoded values:
grep -n '"test-project"' internal/cli/commands/spawn_test.go

# Run specific failing test to debug:
go test ./internal/cli/commands -run TestGenerateAgentIDUnique -v

# Run all tests:
make test
```

## Files Changed This Session

```
internal/repo/messages.go      # Added from_id backwards compat
internal/repo/logs.go          # Added agent_id, direction backwards compat
internal/repo/agents.go        # Added id backwards compat
internal/db/db.go              # Added backwards compat columns to schema
internal/tui/watch_test.go     # Changed ID to Name
internal/tui/formatting.go     # Created by Codex agent
internal/tui/formatting_test.go # Created by Codex agent
internal/cli/commands/commands_test.go    # Added testCtx(), fixed all tests
internal/cli/commands/attach_test.go      # Fixed ctx usage
internal/cli/commands/interrupt_test.go   # Fixed ctx usage
internal/cli/commands/log_test.go         # Fixed ctx usage
internal/cli/commands/peek_test.go        # Fixed ctx usage
internal/cli/commands/prompt_test.go      # Fully rewritten
internal/cli/commands/spawn_test.go       # Partially fixed
internal/cli/commands/worker_spawn_test.go # Partially fixed
```

## Next Steps

1. Fix remaining hardcoded "test-project" in spawn_test.go
2. Investigate why transcript entries aren't being recorded (check event_type values)
3. Run `make test` to verify all pass
4. Commit the backward-compat fixes with message like:
   ```
   fix: backfill backwards-compat columns for schema migration

   - Add from_id, agent_id, direction to INSERTs for old schema compat
   - Update tests to use scope.CurrentContext() for project/branch
   ```
