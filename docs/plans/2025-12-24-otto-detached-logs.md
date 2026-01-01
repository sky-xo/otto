# June Detached Logging Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan.

**Goal:** Make `june spawn --detach` behave like codex-subagent: detached agents still capture logs, update status on exit, and record Codex thread IDs.

**Architecture:** Add a hidden detached worker subcommand (`june worker-spawn`) that runs the same transcript-capture and lifecycle logic as non-detached `spawn`. `spawn --detach` creates the agent and prompt as usual, then launches the worker in the background, which reads the stored prompt, runs Codex/Claude with transcript capture, and updates DB state (logs, session ID, exit message, status).

**Tech Stack:** Go, Cobra CLI, SQLite (internal repo), os/exec

---

### Task 0: Spike – Verify detached worker can capture logs + thread_id

**Files:**
- Modify: `internal/cli/commands/spawn.go`
- Create: `internal/cli/commands/worker_spawn.go`
- Modify: `internal/cli/root.go`

**Step 1: Write a failing test for worker reading prompt and writing logs**

```go
// internal/cli/commands/worker_spawn_test.go
func TestWorkerSpawnCapturesPromptAndLogs(t *testing.T) {
  // 1) set up temp DB, create agent row, store prompt message
  // 2) run worker spawn with a fake runner that emits transcript chunks
  // 3) assert logs contain prompt (in) + output (out)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/commands -run TestWorkerSpawnCapturesPromptAndLogs -v`
Expected: FAIL (worker command not implemented)

**Step 3: Implement minimal worker command**

```go
// internal/cli/commands/worker_spawn.go
// Hidden command: `june worker-spawn <agent-id>`
// - open DB
// - load agent + latest prompt
// - run codex/claude with transcript capture
// - update status + exit message
```

**Step 4: Re-run the test**

Run: `go test ./internal/cli/commands -run TestWorkerSpawnCapturesPromptAndLogs -v`
Expected: PASS

**Step 5: Manual smoke (Codex detached)**

Run:
```bash
./june spawn codex "quick skim" --name spike-detach --detach
./june log spike-detach --tail 5
./june status | rg spike-detach
```
Expected:
- `log` shows at least prompt + some stdout/stderr chunks
- `status` eventually shows `complete` or `failed`, not stuck `busy`

**Step 6: Commit**

```bash
git add internal/cli/commands/worker_spawn.go internal/cli/commands/worker_spawn_test.go internal/cli/commands/spawn.go internal/cli/root.go
git commit -m "feat(june): add detached worker spawn"
```

---

### Task 1: Store/retrieve prompt for worker (DB helper)

**Files:**
- Modify: `internal/repo/messages.go`
- Modify: `internal/cli/commands/transcript_capture.go`
- Test: `internal/repo/messages_test.go`

**Step 1: Write failing test for latest prompt lookup**

```go
func TestGetLatestPromptForAgent(t *testing.T) {
  // create multiple messages, including prompt to_id=agent
  // assert latest prompt content returns
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/repo -run TestGetLatestPromptForAgent -v`
Expected: FAIL (function missing)

**Step 3: Implement repo helper**

```go
// internal/repo/messages.go
func GetLatestPromptForAgent(db *sql.DB, agentID string) (Message, error) {
  // SELECT * FROM messages WHERE type='prompt' AND to_id=? ORDER BY created_at DESC, id DESC LIMIT 1
}
```

**Step 4: Wire worker to use prompt**

```go
// internal/cli/commands/worker_spawn.go
promptMsg, err := repo.GetLatestPromptForAgent(db, agentID)
// use promptMsg.Content as the prompt body
```

**Step 5: Re-run tests**

Run:
- `go test ./internal/repo -run TestGetLatestPromptForAgent -v`
- `go test ./internal/cli/commands -run TestWorkerSpawnCapturesPromptAndLogs -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/repo/messages.go internal/repo/messages_test.go internal/cli/commands/worker_spawn.go
git commit -m "feat(june): store and fetch latest prompt for detached worker"
```

---

### Task 2: Make `spawn --detach` launch worker and handle failures

**Files:**
- Modify: `internal/cli/commands/spawn.go`
- Modify: `internal/exec/runner.go`
- Test: `internal/cli/commands/spawn_test.go`

**Step 1: Write failing test for detach path launching worker**

```go
func TestSpawnDetachLaunchesWorker(t *testing.T) {
  // stub runner to capture StartDetached call
  // assert it runs june worker-spawn <agent-id>
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/commands -run TestSpawnDetachLaunchesWorker -v`
Expected: FAIL

**Step 3: Implement detach worker launch**

```go
// internal/cli/commands/spawn.go
// in detach path:
// 1) build june binary path
// 2) StartDetached(juneBin, "worker-spawn", agentID)
// 3) on error: SetAgentFailed + create exit message
```

**Step 4: Add StartDetachedWithEnv (optional) if needed**

If worker requires env overrides:
```go
// internal/exec/runner.go
StartDetachedWithEnv(name string, env []string, args ...string) (pid int, err error)
```

**Step 5: Re-run tests**

Run: `go test ./internal/cli/commands -run TestSpawnDetachLaunchesWorker -v`
Expected: PASS

**Step 6: Manual smoke**

Run:
```bash
./june spawn codex "quick skim" --name detach-worker --detach
./june peek detach-worker
./june log detach-worker --tail 20
./june status | rg detach-worker
```
Expected:
- `peek/log` shows output (not empty)
- status flips to complete/failed

**Step 7: Commit**

```bash
git add internal/cli/commands/spawn.go internal/cli/commands/spawn_test.go internal/exec/runner.go
git commit -m "feat(june): run detached spawn via worker"
```

---

### Task 3: Ensure detached Codex thread_id capture + resume support

**Files:**
- Modify: `internal/cli/commands/worker_spawn.go`
- Test: `internal/cli/commands/worker_spawn_test.go`

**Step 1: Add failing test for thread_id capture**

```go
func TestWorkerSpawnCapturesThreadID(t *testing.T) {
  // fake runner emits thread.started JSON
  // assert repo.UpdateAgentSessionID called (session_id == thread_id)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/commands -run TestWorkerSpawnCapturesThreadID -v`
Expected: FAIL

**Step 3: Implement thread_id parsing in worker**

```go
// use consumeTranscriptEntries with onStdoutLine parser (same as runCodexSpawn)
```

**Step 4: Re-run tests**

Run: `go test ./internal/cli/commands -run TestWorkerSpawnCapturesThreadID -v`
Expected: PASS

**Step 5: Manual smoke**

Run:
```bash
./june spawn codex "quick skim" --name detach-thread --detach
./june status | rg detach-thread
./june prompt detach-thread "continue"
```
Expected:
- `prompt` succeeds (session_id exists)

**Step 6: Commit**

```bash
git add internal/cli/commands/worker_spawn.go internal/cli/commands/worker_spawn_test.go
git commit -m "feat(june): capture thread_id for detached codex agents"
```

---

### Task 4: Launch diagnostics on worker failure (minimal)

**Files:**
- Modify: `internal/cli/commands/spawn.go`
- Modify: `internal/cli/commands/worker_spawn.go`
- Create: `internal/repo/launch_errors.go`
- Test: `internal/repo/launch_errors_test.go`

**Step 1: Write failing test for launch error recording**

```go
func TestRecordLaunchError(t *testing.T) {
  // record error, read back file path/content
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/repo -run TestRecordLaunchError -v`
Expected: FAIL

**Step 3: Implement minimal launch error logging**

```go
// internal/repo/launch_errors.go
// Write error text to ~/.june/orchestrators/<scope>/launch-errors/<agent-id>.log
```

**Step 4: Wire failure paths**

```go
// spawn.go: on StartDetached failure -> RecordLaunchError + SetAgentFailed + exit message
// worker_spawn.go: on runtime error -> RecordLaunchError + SetAgentFailed + exit message
```

**Step 5: Re-run tests**

Run:
- `go test ./internal/repo -run TestRecordLaunchError -v`
- `go test ./internal/cli/commands -run TestSpawnDetachLaunchesWorker -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/repo/launch_errors.go internal/repo/launch_errors_test.go internal/cli/commands/spawn.go internal/cli/commands/worker_spawn.go
git commit -m "feat(june): record launch errors for detached workers"
```

---

### Task 5: End-to-end regression test (manual)

**Files:**
- None

**Step 1: Build**

Run: `make build`
Expected: build succeeds

**Step 2: Detached codex skim**

Run:
```bash
./june spawn codex "skim repo" --name e2e-detach --detach
./june log e2e-detach --tail 10
./june status | rg e2e-detach
```
Expected:
- `log` has content
- `status` is complete/failed (not stuck busy)

**Step 3: Detached prompt/resume**

Run:
```bash
./june prompt e2e-detach "quick followup"
./june log e2e-detach --tail 10
```
Expected: prompt succeeds and new logs appear

---

Plan complete and saved to `docs/plans/2025-12-24-june-detached-logs.md`. Two execution options:

1. Subagent-Driven (this session) – I dispatch fresh subagent per task, review between tasks.
2. Parallel Session (separate) – Open new session with executing-plans for batch execution.

Which approach?
