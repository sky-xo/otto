# Agent Naming Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `--name` optional with auto-generation, improve collision errors, and print name on success.

**Architecture:** Add `generateName()` for `task-{6-char}` format, update spawn command to handle optional name with different collision behavior for user vs auto names. Print name only after successful process completion.

**Tech Stack:** Go, `crypto/rand` for secure randomness, `errors` package

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

func TestGenerateName_Format(t *testing.T) {
	name := generateName()

	// Should match pattern: task-XXXXXX (6 alphanumeric chars)
	pattern := regexp.MustCompile(`^task-[a-zA-Z0-9]{6}$`)
	if !pattern.MatchString(name) {
		t.Errorf("generateName() = %q, want pattern task-XXXXXX", name)
	}
}

func TestGenerateName_NotConstant(t *testing.T) {
	// Generate several names and verify they're not all the same
	first := generateName()
	for i := 0; i < 10; i++ {
		if generateName() != first {
			return // Success - we got a different name
		}
	}
	t.Error("generateName() returned the same value 11 times in a row")
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
- Create: `internal/cli/format.go`
- Test: `internal/cli/format_test.go`

**Step 1: Write the failing test**

Create `internal/cli/format_test.go`:

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
		{"30 seconds", 30 * time.Second, "just now"},
		{"59 seconds", 59 * time.Second, "just now"},
		{"61 seconds", 61 * time.Second, "1 minute ago"},
		{"5 minutes", 5*time.Minute + 30*time.Second, "5 minutes ago"},
		{"61 minutes", 61 * time.Minute, "1 hour ago"},
		{"2 hours", 2*time.Hour + 30*time.Minute, "2 hours ago"},
		{"23 hours", 23*time.Hour + 59*time.Minute, "23 hours ago"},
		{"25 hours", 25 * time.Hour, "1 day ago"},
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

Create `internal/cli/format.go`:

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
git add internal/cli/format.go internal/cli/format_test.go
git commit -m "feat(cli): add relativeTime helper for human-readable timestamps"
```

---

## Task 3: Add collision error formatter

**Files:**
- Modify: `internal/cli/format.go`
- Modify: `internal/cli/format_test.go`

**Step 1: Write the failing test**

Add to `internal/cli/format_test.go`:

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

func TestFormatCollisionError_NoPercentInterpolation(t *testing.T) {
	// Ensure % in name doesn't cause issues
	spawnedAt := time.Now().Add(-1 * time.Hour)
	errMsg := formatCollisionError("test%s", spawnedAt)

	if errMsg == "" {
		t.Error("formatCollisionError should handle % in names")
	}
	// Should contain the literal %s, not interpolate it
	if !strings.Contains(errMsg, "test%s") {
		t.Errorf("formatCollisionError should preserve literal %%s, got: %s", errMsg)
	}
}
```

Also add to the imports at the top: `"strings"`

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestFormatCollisionError -v`
Expected: FAIL with "undefined: formatCollisionError"

**Step 3: Write the implementation**

Add to `internal/cli/format.go`:

```go
// formatCollisionError creates a helpful error message when an agent name already exists
func formatCollisionError(name string, spawnedAt time.Time) string {
	return fmt.Sprintf("agent %q already exists (spawned %s)\nHint: use --name %s-2 or another unique name",
		name, relativeTime(spawnedAt), name)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -run TestFormatCollisionError -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/format.go internal/cli/format_test.go
git commit -m "feat(cli): add formatCollisionError with timestamp and suggestion"
```

---

## Task 4: Add spawn command tests for new behavior

**Files:**
- Create: `internal/cli/spawn_test.go`

**Step 1: Write failing tests for the new spawn behavior**

Create `internal/cli/spawn_test.go`:

```go
package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sky-xo/june/internal/db"
)

func TestResolveAgentName_UserProvided(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	name, err := resolveAgentName(database, "my-agent", true)
	if err != nil {
		t.Fatalf("resolveAgentName failed: %v", err)
	}
	if name != "my-agent" {
		t.Errorf("name = %q, want %q", name, "my-agent")
	}
}

func TestResolveAgentName_UserProvided_Collision(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	// Create existing agent
	err := database.CreateAgent(db.Agent{
		Name:        "existing",
		ULID:        "test-ulid",
		SessionFile: "/tmp/test.jsonl",
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	_, err = resolveAgentName(database, "existing", true)
	if err == nil {
		t.Fatal("expected error for collision, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists', got: %v", err)
	}
	if !strings.Contains(err.Error(), "existing-2") {
		t.Errorf("error should suggest 'existing-2', got: %v", err)
	}
}

func TestResolveAgentName_AutoGenerated(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	name, err := resolveAgentName(database, "", false)
	if err != nil {
		t.Fatalf("resolveAgentName failed: %v", err)
	}
	if !strings.HasPrefix(name, "task-") {
		t.Errorf("auto-generated name should start with 'task-', got: %q", name)
	}
	if len(name) != 11 { // "task-" (5) + 6 chars
		t.Errorf("auto-generated name should be 11 chars, got: %d", len(name))
	}
}

func TestResolveAgentName_AutoGenerated_RetriesOnCollision(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	// This test verifies the retry logic exists, not that it works perfectly
	// (since we can't easily force collisions with random names)
	name, err := resolveAgentName(database, "", false)
	if err != nil {
		t.Fatalf("resolveAgentName failed: %v", err)
	}
	if name == "" {
		t.Error("resolveAgentName should return a name")
	}
}

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	return database
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli -run TestResolveAgentName -v`
Expected: FAIL with "undefined: resolveAgentName"

**Step 3: Commit the tests (red phase)**

```bash
git add internal/cli/spawn_test.go
git commit -m "test(cli): add tests for resolveAgentName (red)"
```

---

## Task 5: Implement resolveAgentName and update spawn command

**Files:**
- Modify: `internal/cli/spawn.go`

**Step 1: Add the errors import**

In `internal/cli/spawn.go`, add `"errors"` to the import block.

**Step 2: Add resolveAgentName function**

Add after the imports in `internal/cli/spawn.go`:

```go
// resolveAgentName returns the agent name to use.
// If userProvided is true, it validates the name doesn't exist and returns an error on collision.
// If userProvided is false, it generates a unique name with retries.
func resolveAgentName(database *db.DB, name string, userProvided bool) (string, error) {
	if userProvided {
		existing, err := database.GetAgent(name)
		if err == nil {
			return "", errors.New(formatCollisionError(name, existing.SpawnedAt))
		} else if err != db.ErrAgentNotFound {
			return "", fmt.Errorf("failed to check for existing agent: %w", err)
		}
		return name, nil
	}

	// Auto-generate name, retry on collision (unlikely)
	for attempts := 0; attempts < 10; attempts++ {
		name = generateName()
		if _, err := database.GetAgent(name); err == db.ErrAgentNotFound {
			return name, nil
		} else if err != nil {
			return "", fmt.Errorf("failed to check for existing agent: %w", err)
		}
		// Collision with auto-generated name, try again
	}
	return "", errors.New("failed to generate unique agent name after 10 attempts")
}
```

**Step 3: Run tests to verify they pass**

Run: `go test ./internal/cli -run TestResolveAgentName -v`
Expected: PASS

**Step 4: Update newSpawnCmd to make --name optional**

Find in `internal/cli/spawn.go` the block:

```go
		if name == "" {
			return fmt.Errorf("--name is required")
		}
```

Replace with:

```go
		userProvidedName := name != ""
```

**Step 5: Update the runSpawnCodex call**

Find:

```go
		return runSpawnCodex(name, task)
```

Replace with:

```go
		return runSpawnCodex(name, task, userProvidedName)
```

**Step 6: Remove the MarkFlagRequired call**

Find and delete this line:

```go
	cmd.MarkFlagRequired("name")
```

**Step 7: Update the flag help text**

Find:

```go
	cmd.Flags().StringVar(&name, "name", "", "Name for the agent (required)")
```

Replace with:

```go
	cmd.Flags().StringVar(&name, "name", "", "Name for the agent (auto-generated if omitted)")
```

**Step 8: Update runSpawnCodex signature and use resolveAgentName**

Find:

```go
func runSpawnCodex(name, task string) error {
```

Replace with:

```go
func runSpawnCodex(name, task string, userProvidedName bool) error {
```

Find the collision check block:

```go
	// Check if agent already exists
	if _, err := database.GetAgent(name); err == nil {
		return fmt.Errorf("agent %q already exists", name)
	} else if err != db.ErrAgentNotFound {
		return fmt.Errorf("failed to check for existing agent: %w", err)
	}
```

Replace with:

```go
	// Resolve agent name (validate user-provided or auto-generate)
	resolvedName, err := resolveAgentName(database, name, userProvidedName)
	if err != nil {
		return err
	}
	name = resolvedName
```

**Step 9: Run all tests**

Run: `make test`
Expected: All tests PASS

**Step 10: Commit**

```bash
git add internal/cli/spawn.go
git commit -m "feat(cli): make --name optional with auto-generation"
```

---

## Task 6: Print name on successful spawn (at the end)

**Files:**
- Modify: `internal/cli/spawn.go`

**Step 1: Locate the end of runSpawnCodex**

Find the end of the function, after the session file update block:

```go
	// Update session file if we didn't have it
	if sessionFile == "" {
		if found, err := codex.FindSessionFile(threadID); err == nil {
			// Update the agent record with the session file
			if err := database.UpdateSessionFile(name, found); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to update session file: %v\n", err)
			}
		}
	}

	return nil
}
```

**Step 2: Add the print statement before return nil**

Replace:

```go
	return nil
}
```

With:

```go
	// Print the agent name to confirm what was created
	fmt.Println(name)

	return nil
}
```

**Step 3: Run all tests**

Run: `make test`
Expected: All tests PASS

**Step 4: Commit**

```bash
git add internal/cli/spawn.go
git commit -m "feat(cli): print agent name on successful spawn"
```

---

## Task 7: Manual verification

**Step 1: Build the binary**

Run: `make build`

**Step 2: Test auto-generated name (dry run - just check build works)**

The following would spawn a real agent, so only run if you want to test end-to-end:

```bash
# ./june spawn codex "test task"
# Expected: Prints something like "task-f3WlaB"
```

**Step 3: Verify help text updated**

Run: `./june spawn --help`
Expected: Should show `--name string   Name for the agent (auto-generated if omitted)`

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Name generation (`task-XXXXXX`) | `name.go`, `name_test.go` |
| 2 | Relative time helper | `format.go`, `format_test.go` |
| 3 | Collision error formatter | `format.go`, `format_test.go` |
| 4 | Tests for resolveAgentName | `spawn_test.go` |
| 5 | Implement resolveAgentName + update spawn | `spawn.go` |
| 6 | Print name at end of spawn | `spawn.go` |
| 7 | Manual verification | - |

## Review Fixes Applied

- **Retry loop bug**: Now returns error after 10 failed attempts
- **fmt.Errorf format string issue**: Uses `errors.New()` instead
- **Print timing**: Prints at the very end after process completion
- **Missing flag help update**: Changed to "(auto-generated if omitted)"
- **Missing unit tests**: Added `spawn_test.go` with tests for `resolveAgentName`
- **File organization**: Moved formatCollisionError to `format.go` (not `time.go`)
- **Edge cases in relativeTime**: Added tests for 59s, 23h59m boundaries
- **Code patterns over line numbers**: Removed line number references
