# Agent Naming Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `--name` optional with auto-generation, improve collision errors, and print name on success.

**Architecture:** Add `generateName()` for `task-{6-char}` format, update spawn command to handle optional name with different collision behavior for user vs auto names.

**Tech Stack:** Go, `crypto/rand` for secure randomness

---

## Task 1: Add name generation helper

**Files:**
- Create: `internal/cli/name.go`
- Test: `internal/cli/name_test.go`

**Step 1: Write the failing test**

Create `internal/cli/name_test.go`:

```go
package cli

import (
	"regexp"
	"testing"
)

func TestGenerateName(t *testing.T) {
	name := generateName()

	// Should match pattern: task-XXXXXX (6 alphanumeric chars)
	pattern := regexp.MustCompile(`^task-[a-zA-Z0-9]{6}$`)
	if !pattern.MatchString(name) {
		t.Errorf("generateName() = %q, want pattern task-XXXXXX", name)
	}
}

func TestGenerateName_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		name := generateName()
		if seen[name] {
			t.Errorf("generateName() produced duplicate: %q", name)
		}
		seen[name] = true
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestGenerateName -v`
Expected: FAIL with "undefined: generateName"

**Step 3: Write minimal implementation**

Create `internal/cli/name.go`:

```go
package cli

import (
	"crypto/rand"
	"fmt"
)

const base62Chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// generateName creates a random name like "task-f3WlaB"
func generateName() string {
	suffix := make([]byte, 6)
	randomBytes := make([]byte, 6)
	if _, err := rand.Read(randomBytes); err != nil {
		panic(fmt.Sprintf("failed to generate random bytes: %v", err))
	}
	for i := 0; i < 6; i++ {
		suffix[i] = base62Chars[randomBytes[i]%62]
	}
	return "task-" + string(suffix)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -run TestGenerateName -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/name.go internal/cli/name_test.go
git commit -m "feat(cli): add generateName for auto-generated agent names"
```

---

## Task 2: Add relative time helper

**Files:**
- Create: `internal/cli/time.go`
- Test: `internal/cli/time_test.go`

**Step 1: Write the failing test**

Create `internal/cli/time_test.go`:

```go
package cli

import (
	"testing"
	"time"
)

func TestRelativeTime(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"just now", 30 * time.Second, "just now"},
		{"1 minute", 1 * time.Minute, "1 minute ago"},
		{"5 minutes", 5 * time.Minute, "5 minutes ago"},
		{"1 hour", 1 * time.Hour, "1 hour ago"},
		{"2 hours", 2 * time.Hour, "2 hours ago"},
		{"1 day", 24 * time.Hour, "1 day ago"},
		{"3 days", 72 * time.Hour, "3 days ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			past := time.Now().Add(-tt.duration)
			got := relativeTime(past)
			if got != tt.want {
				t.Errorf("relativeTime() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestRelativeTime -v`
Expected: FAIL with "undefined: relativeTime"

**Step 3: Write minimal implementation**

Create `internal/cli/time.go`:

```go
package cli

import (
	"fmt"
	"time"
)

// relativeTime returns a human-readable string like "2 hours ago"
func relativeTime(t time.Time) string {
	d := time.Since(t)

	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -run TestRelativeTime -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/time.go internal/cli/time_test.go
git commit -m "feat(cli): add relativeTime helper for human-readable timestamps"
```

---

## Task 3: Add collision error formatter

**Files:**
- Modify: `internal/cli/time.go` (add formatCollisionError)
- Modify: `internal/cli/time_test.go` (add test)

**Step 1: Write the failing test**

Add to `internal/cli/time_test.go`:

```go
func TestFormatCollisionError(t *testing.T) {
	spawnedAt := time.Now().Add(-2 * time.Hour)
	errMsg := formatCollisionError("myagent", spawnedAt)

	expected := `agent "myagent" already exists (spawned 2 hours ago)
Hint: use --name myagent-2 or another unique name`

	if errMsg != expected {
		t.Errorf("formatCollisionError() = %q, want %q", errMsg, expected)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestFormatCollisionError -v`
Expected: FAIL with "undefined: formatCollisionError"

**Step 3: Write the implementation**

Add to `internal/cli/time.go`:

```go
// formatCollisionError creates a helpful error message when an agent name already exists
func formatCollisionError(name string, spawnedAt time.Time) string {
	return fmt.Sprintf(`agent %q already exists (spawned %s)
Hint: use --name %s-2 or another unique name`, name, relativeTime(spawnedAt), name)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -run TestFormatCollisionError -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/time.go internal/cli/time_test.go
git commit -m "feat(cli): add formatCollisionError with timestamp and suggestion"
```

---

## Task 4: Update spawn command for optional --name

**Files:**
- Modify: `internal/cli/spawn.go`

**Step 1: Remove the required flag and validation**

In `internal/cli/spawn.go`, find and remove line 42:
```go
cmd.MarkFlagRequired("name")
```

Also remove lines 33-35:
```go
if name == "" {
    return fmt.Errorf("--name is required")
}
```

**Step 2: Update runSpawnCodex signature**

Change the function signature to accept a flag indicating if name was user-provided:

Replace the function call at line 37:
```go
return runSpawnCodex(name, task)
```

With:
```go
return runSpawnCodex(name, task, name != "")
```

**Step 3: Update runSpawnCodex to handle auto-generation**

Change function signature at line 47:
```go
func runSpawnCodex(name, task string) error {
```

To:
```go
func runSpawnCodex(name, task string, userProvidedName bool) error {
```

**Step 4: Add name generation and collision handling**

Replace lines 65-70:
```go
	// Check if agent already exists
	if _, err := database.GetAgent(name); err == nil {
		return fmt.Errorf("agent %q already exists", name)
	} else if err != db.ErrAgentNotFound {
		return fmt.Errorf("failed to check for existing agent: %w", err)
	}
```

With:
```go
	// Handle name: generate if not provided, check collisions
	if !userProvidedName {
		// Auto-generate name, retry on collision (unlikely)
		for attempts := 0; attempts < 10; attempts++ {
			name = generateName()
			if _, err := database.GetAgent(name); err == db.ErrAgentNotFound {
				break
			} else if err != nil {
				return fmt.Errorf("failed to check for existing agent: %w", err)
			}
			// Collision with auto-generated name, try again
		}
	} else {
		// User provided name: error on collision with helpful message
		if existing, err := database.GetAgent(name); err == nil {
			return fmt.Errorf(formatCollisionError(name, existing.SpawnedAt))
		} else if err != db.ErrAgentNotFound {
			return fmt.Errorf("failed to check for existing agent: %w", err)
		}
	}
```

**Step 5: Run tests to verify nothing broke**

Run: `make test`
Expected: All tests PASS

**Step 6: Commit**

```bash
git add internal/cli/spawn.go
git commit -m "feat(cli): make --name optional with auto-generation"
```

---

## Task 5: Print name on successful spawn

**Files:**
- Modify: `internal/cli/spawn.go`

**Step 1: Add success output**

After the `CreateAgent` call at approximately line 130, add the print statement. Find:
```go
	if err := database.CreateAgent(agent); err != nil {
		return fmt.Errorf("failed to create agent record: %w", err)
	}
```

Add immediately after:
```go
	// Print the agent name to confirm what was created
	fmt.Println(name)
```

**Step 2: Run all tests**

Run: `make test`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add internal/cli/spawn.go
git commit -m "feat(cli): print agent name on successful spawn"
```

---

## Task 6: Manual verification

**Step 1: Build the binary**

Run: `make build`

**Step 2: Test auto-generated name**

```bash
./june spawn codex "test task"
```
Expected: Prints something like `task-f3WlaB`

**Step 3: Test user-provided name**

```bash
./june spawn codex "test task" --name my-agent
```
Expected: Prints `my-agent`

**Step 4: Test collision error**

```bash
./june spawn codex "another task" --name my-agent
```
Expected:
```
Error: agent "my-agent" already exists (spawned just now)
Hint: use --name my-agent-2 or another unique name
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Name generation (`task-XXXXXX`) | `name.go`, `name_test.go` |
| 2 | Relative time helper | `time.go`, `time_test.go` |
| 3 | Collision error formatter | `time.go`, `time_test.go` |
| 4 | Optional `--name` with auto-gen | `spawn.go` |
| 5 | Print name on success | `spawn.go` |
| 6 | Manual verification | - |
