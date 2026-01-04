# Codex Home Isolation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Isolate spawned Codex agents from user's ~/.codex config (AGENTS.md, skills, etc.) by using a dedicated CODEX_HOME at ~/.june/codex/

**Architecture:** Create ensureCodexHome() that sets up ~/.june/codex/ directory, copies only auth.json for API access, and returns the path. Update spawn command to set CODEX_HOME env var. Update session file discovery to look in the new location.

**Tech Stack:** Go, SQLite, Codex CLI

---

## Task 1: Create ensureCodexHome function

**Files:**
- Create: `internal/codex/home.go`
- Test: `internal/codex/home_test.go`

**Step 1: Write the failing test**

```go
// internal/codex/home_test.go
package codex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCodexHome_CreatesDirectory(t *testing.T) {
	// Use temp dir as fake june home
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create fake user codex with auth.json
	userCodex := filepath.Join(tmpDir, ".codex")
	os.MkdirAll(userCodex, 0755)
	os.WriteFile(filepath.Join(userCodex, "auth.json"), []byte(`{"token":"secret"}`), 0600)

	codexHome, err := EnsureCodexHome()
	if err != nil {
		t.Fatalf("EnsureCodexHome failed: %v", err)
	}

	// Should be under ~/.june/codex
	expected := filepath.Join(tmpDir, ".june", "codex")
	if codexHome != expected {
		t.Errorf("codexHome = %q, want %q", codexHome, expected)
	}

	// Directory should exist
	if _, err := os.Stat(codexHome); os.IsNotExist(err) {
		t.Error("codex home directory was not created")
	}
}

func TestEnsureCodexHome_CopiesAuthJson(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create fake user codex with auth.json
	userCodex := filepath.Join(tmpDir, ".codex")
	os.MkdirAll(userCodex, 0755)
	authContent := []byte(`{"token":"test-secret-token"}`)
	os.WriteFile(filepath.Join(userCodex, "auth.json"), authContent, 0600)

	codexHome, err := EnsureCodexHome()
	if err != nil {
		t.Fatalf("EnsureCodexHome failed: %v", err)
	}

	// auth.json should be copied
	copiedAuth := filepath.Join(codexHome, "auth.json")
	data, err := os.ReadFile(copiedAuth)
	if err != nil {
		t.Fatalf("failed to read copied auth.json: %v", err)
	}
	if string(data) != string(authContent) {
		t.Errorf("auth.json content = %q, want %q", string(data), string(authContent))
	}
}

func TestEnsureCodexHome_NoAuthJsonOK(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// No ~/.codex exists at all
	codexHome, err := EnsureCodexHome()
	if err != nil {
		t.Fatalf("EnsureCodexHome should succeed without auth.json: %v", err)
	}

	// Directory should still be created
	if _, err := os.Stat(codexHome); os.IsNotExist(err) {
		t.Error("codex home directory was not created")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/codex/... -v -run TestEnsureCodexHome`
Expected: FAIL - EnsureCodexHome not defined

**Step 3: Write minimal implementation**

```go
// internal/codex/home.go
package codex

import (
	"os"
	"path/filepath"
)

// EnsureCodexHome creates an isolated CODEX_HOME at ~/.june/codex/
// and copies auth.json from the user's ~/.codex/ for API access.
// Returns the path to the isolated codex home directory.
func EnsureCodexHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Create ~/.june/codex/
	codexHome := filepath.Join(home, ".june", "codex")
	if err := os.MkdirAll(codexHome, 0755); err != nil {
		return "", err
	}

	// Copy auth.json from user's ~/.codex/ if it exists
	userCodex := filepath.Join(home, ".codex")
	authSrc := filepath.Join(userCodex, "auth.json")
	authDst := filepath.Join(codexHome, "auth.json")

	// Only copy if source exists and destination doesn't
	if _, err := os.Stat(authDst); os.IsNotExist(err) {
		if authData, err := os.ReadFile(authSrc); err == nil {
			_ = os.WriteFile(authDst, authData, 0600)
		}
		// Ignore errors - auth.json is optional (user may not have authenticated yet)
	}

	return codexHome, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/codex/... -v -run TestEnsureCodexHome`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/codex/home.go internal/codex/home_test.go
git commit -m "feat(codex): add EnsureCodexHome for isolated codex environment"
```

---

## Task 2: Update spawn command to use isolated CODEX_HOME

**Files:**
- Modify: `internal/cli/spawn.go`

**Step 1: Read current spawn.go**

Understand how Codex is currently spawned (around line 70-80 where exec.Command is created).

**Step 2: Update spawn to set CODEX_HOME env var**

Find where `codexCmd := exec.Command(...)` is created and add:

```go
// Before creating the command, ensure isolated codex home
isolatedCodexHome, err := codex.EnsureCodexHome()
if err != nil {
	return fmt.Errorf("failed to setup isolated codex home: %w", err)
}

// Create command with isolated CODEX_HOME
codexCmd := exec.Command("codex", "exec", "--json", task)
codexCmd.Dir = cwd
codexCmd.Env = append(os.Environ(), fmt.Sprintf("CODEX_HOME=%s", isolatedCodexHome))
```

**Step 3: Run build to verify no errors**

Run: `go build ./...`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add internal/cli/spawn.go
git commit -m "feat(spawn): use isolated CODEX_HOME for vanilla agents"
```

---

## Task 3: Update session file discovery to use ~/.june/codex/

**Files:**
- Modify: `internal/codex/session.go`
- Test: `internal/codex/session_test.go`

**Step 1: Write the failing test**

Add to existing session tests:

```go
func TestFindSessionFile_UsesJuneCodexHome(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create session file in ~/.june/codex/sessions/
	juneCodex := filepath.Join(tmpDir, ".june", "codex")
	sessionDir := filepath.Join(juneCodex, "sessions", "2026", "01", "04")
	os.MkdirAll(sessionDir, 0755)

	threadID := "01abc123"
	sessionFile := filepath.Join(sessionDir, threadID+".jsonl")
	os.WriteFile(sessionFile, []byte(`{"test":"data"}`), 0644)

	// FindSessionFile should find it
	found, err := FindSessionFile(threadID)
	if err != nil {
		t.Fatalf("FindSessionFile failed: %v", err)
	}
	if found != sessionFile {
		t.Errorf("FindSessionFile = %q, want %q", found, sessionFile)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/codex/... -v -run TestFindSessionFile_UsesJuneCodexHome`
Expected: FAIL - looks in wrong directory

**Step 3: Update FindSessionFile implementation**

Update `FindSessionFile` to look in `~/.june/codex/sessions/` instead of `~/.codex/sessions/`:

```go
// FindSessionFile finds a session file by thread ID in the June codex home.
func FindSessionFile(threadID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Look in ~/.june/codex/sessions/
	sessionsDir := filepath.Join(home, ".june", "codex", "sessions")
	return findSessionInDir(sessionsDir, threadID)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/codex/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/codex/session.go internal/codex/session_test.go
git commit -m "feat(codex): update session discovery to use ~/.june/codex/"
```

---

## Task 4: Update spawn.go to use new FindSessionFile

**Files:**
- Modify: `internal/cli/spawn.go`

**Step 1: Find where FindSessionFile is called**

Look for the call to `codex.FindSessionFile(codexHome, threadID)` - it currently takes codexHome as a parameter.

**Step 2: Update to use new signature**

Change from:
```go
sessionFile, err := codex.FindSessionFile(codexHome, threadID)
```

To:
```go
sessionFile, err := codex.FindSessionFile(threadID)
```

Since FindSessionFile now knows to look in `~/.june/codex/sessions/`.

**Step 3: Run build and tests**

Run: `go build ./... && go test ./...`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add internal/cli/spawn.go
git commit -m "refactor(spawn): simplify FindSessionFile call"
```

---

## Task 5: Update TUI commands to use new session path

**Files:**
- Modify: `internal/tui/commands.go` (if needed)

**Step 1: Check if TUI uses CodexHome directly**

The TUI gets TranscriptPath from the agent.Agent struct, which is populated from db.Agent.SessionFile. This should already be the full path, so no changes may be needed.

Verify by reading the code path:
1. `db.Agent.SessionFile` stores full path
2. `db.Agent.ToUnified()` copies to `TranscriptPath`
3. `loadTranscriptCmd()` uses `a.TranscriptPath`

**Step 2: If no changes needed, skip to commit**

Run: `go test ./internal/tui/... -v`
Expected: PASS

**Step 3: Commit (if any changes)**

```bash
git add internal/tui/
git commit -m "chore(tui): verify transcript path handling"
```

---

## Final Verification

**Step 1: Run all tests**

Run: `make test`
Expected: All tests pass

**Step 2: Manual verification**

1. Build: `go build -o june .`
2. Spawn agent: `./june spawn codex "Say hello" --name test-isolated`
3. Check isolated home exists: `ls -la ~/.june/codex/`
4. Check auth.json was copied: `cat ~/.june/codex/auth.json`
5. Check session file location: `ls ~/.june/codex/sessions/`
6. Open TUI: `./june` - verify agent appears and transcript loads

**Step 3: Final commit**

```bash
git add -A
git commit -m "feat: isolate Codex agents with dedicated CODEX_HOME at ~/.june/codex/

Spawned Codex agents now use ~/.june/codex/ as their CODEX_HOME,
isolating them from user's ~/.codex/ config (AGENTS.md, skills, etc.).
Only auth.json is copied for API access. This creates 'vanilla' agents
similar to Claude subagents."
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Create EnsureCodexHome function | `codex/home.go`, `codex/home_test.go` |
| 2 | Update spawn to set CODEX_HOME | `cli/spawn.go` |
| 3 | Update FindSessionFile path | `codex/session.go`, `codex/session_test.go` |
| 4 | Simplify spawn FindSessionFile call | `cli/spawn.go` |
| 5 | Verify TUI transcript handling | `tui/commands.go` |
