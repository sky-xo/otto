# Always-Unique Agent Names Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make all agent names unique by appending a 4-char ULID suffix, with adjective-noun auto-generation when no prefix provided.

**Architecture:** Replace current `resolveAgentName()` (called before spawn) with `buildAgentName()` (called after getting ULID). Add word lists for adjective-noun generation. Collision fallback uses random hex.

**Tech Stack:** Go, crypto/rand for fallback randomness

---

### Task 1: Add Word Lists and Adjective-Noun Generator

**Files:**
- Modify: `internal/cli/name.go`
- Test: `internal/cli/name_test.go` (create)

**Step 1: Write the failing test**

Create `internal/cli/name_test.go`:

```go
package cli

import (
	"regexp"
	"testing"
)

func TestGenerateAdjectiveNoun(t *testing.T) {
	name := generateAdjectiveNoun()

	// Should match pattern: adjective-noun (lowercase, hyphenated)
	pattern := regexp.MustCompile(`^[a-z]+-[a-z]+$`)
	if !pattern.MatchString(name) {
		t.Errorf("generateAdjectiveNoun() = %q, want adjective-noun pattern", name)
	}
}

func TestGenerateAdjectiveNoun_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		name := generateAdjectiveNoun()
		if seen[name] {
			// Collisions are possible but unlikely in 100 tries with 2500 combos
			// This is a sanity check, not a guarantee
			t.Logf("collision on %q (acceptable)", name)
		}
		seen[name] = true
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestGenerateAdjectiveNoun -v`
Expected: FAIL with "undefined: generateAdjectiveNoun"

**Step 3: Write minimal implementation**

Update `internal/cli/name.go`:

```go
package cli

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

var adjectives = []string{
	"swift", "quiet", "bold", "clever", "bright",
	"calm", "eager", "fair", "gentle", "happy",
	"keen", "lively", "merry", "noble", "proud",
	"quick", "ready", "sharp", "steady", "true",
	"warm", "wise", "young", "brave", "clear",
	"crisp", "deft", "dry", "fast", "firm",
	"fresh", "grand", "great", "kind", "light",
	"neat", "plain", "prime", "pure", "rare",
	"rich", "safe", "slim", "smooth", "soft",
	"sound", "spare", "strong", "sweet", "tidy",
}

var nouns = []string{
	"falcon", "river", "spark", "stone", "wave",
	"arrow", "blade", "brook", "cloud", "crane",
	"crown", "dawn", "flame", "frost", "grove",
	"hawk", "helm", "horn", "lake", "lance",
	"leaf", "light", "marsh", "mesa", "mist",
	"moon", "oak", "path", "peak", "pine",
	"pond", "rain", "reef", "ridge", "rose",
	"sage", "shade", "shell", "shore", "sky",
	"slope", "snow", "spire", "spring", "star",
	"storm", "stream", "sun", "tide", "wind",
}

// generateAdjectiveNoun creates a random name like "swift-falcon"
func generateAdjectiveNoun() string {
	adjIdx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(adjectives))))
	nounIdx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(nouns))))
	return adjectives[adjIdx.Int64()] + "-" + nouns[nounIdx.Int64()]
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -run TestGenerateAdjectiveNoun -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/name.go internal/cli/name_test.go
git commit -m "feat(cli): add adjective-noun name generator"
```

---

### Task 2: Add buildAgentName Function

**Files:**
- Modify: `internal/cli/name.go`
- Modify: `internal/cli/name_test.go`

**Step 1: Write the failing test**

Add to `internal/cli/name_test.go`:

```go
func TestBuildAgentName_WithPrefix(t *testing.T) {
	name := buildAgentName("refactor", "01JGXYZ123456789ABCD")
	if name != "refactor-abcd" {
		t.Errorf("buildAgentName() = %q, want %q", name, "refactor-abcd")
	}
}

func TestBuildAgentName_NoPrefix(t *testing.T) {
	name := buildAgentName("", "01JGXYZ123456789WXYZ")

	// Should be adjective-noun-wxyz
	pattern := regexp.MustCompile(`^[a-z]+-[a-z]+-wxyz$`)
	if !pattern.MatchString(name) {
		t.Errorf("buildAgentName() = %q, want adjective-noun-wxyz pattern", name)
	}
}

func TestBuildAgentName_SuffixLowercase(t *testing.T) {
	name := buildAgentName("test", "01JGXYZ123456789ABCD")
	if name != "test-abcd" {
		t.Errorf("buildAgentName() = %q, want lowercase suffix", name)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestBuildAgentName -v`
Expected: FAIL with "undefined: buildAgentName"

**Step 3: Write minimal implementation**

Add to `internal/cli/name.go`:

```go
import (
	"strings"
)

// buildAgentName creates a name from prefix + ULID suffix.
// If prefix is empty, generates adjective-noun prefix.
// Suffix is last 4 chars of ULID, lowercased.
func buildAgentName(prefix, ulid string) string {
	if prefix == "" {
		prefix = generateAdjectiveNoun()
	}
	suffix := strings.ToLower(ulid[len(ulid)-4:])
	return prefix + "-" + suffix
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -run TestBuildAgentName -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/name.go internal/cli/name_test.go
git commit -m "feat(cli): add buildAgentName with ULID suffix"
```

---

### Task 3: Add Random Suffix Fallback

**Files:**
- Modify: `internal/cli/name.go`
- Modify: `internal/cli/name_test.go`

**Step 1: Write the failing test**

Add to `internal/cli/name_test.go`:

```go
func TestRandomHexSuffix(t *testing.T) {
	suffix := randomHexSuffix()
	if len(suffix) != 4 {
		t.Errorf("randomHexSuffix() length = %d, want 4", len(suffix))
	}
	// Should be lowercase hex
	pattern := regexp.MustCompile(`^[0-9a-f]{4}$`)
	if !pattern.MatchString(suffix) {
		t.Errorf("randomHexSuffix() = %q, want hex pattern", suffix)
	}
}

func TestRandomHexSuffix_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		suffix := randomHexSuffix()
		if seen[suffix] {
			t.Logf("collision on %q (acceptable)", suffix)
		}
		seen[suffix] = true
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestRandomHexSuffix -v`
Expected: FAIL with "undefined: randomHexSuffix"

**Step 3: Write minimal implementation**

Add to `internal/cli/name.go`:

```go
import (
	"encoding/hex"
)

// randomHexSuffix generates 4 random hex chars for collision fallback
func randomHexSuffix() string {
	bytes := make([]byte, 2)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -run TestRandomHexSuffix -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/name.go internal/cli/name_test.go
git commit -m "feat(cli): add randomHexSuffix for collision fallback"
```

---

### Task 4: Add resolveAgentNameWithULID Function

**Files:**
- Modify: `internal/cli/name.go`
- Modify: `internal/cli/name_test.go`

**Step 1: Write the failing test**

Add to `internal/cli/name_test.go`:

```go
import (
	"path/filepath"
	"strings"
	"github.com/sky-xo/june/internal/db"
)

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

func TestResolveAgentNameWithULID_NoCollision(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	name, err := resolveAgentNameWithULID(database, "refactor", "01JGXYZ123456789ABCD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "refactor-abcd" {
		t.Errorf("name = %q, want %q", name, "refactor-abcd")
	}
}

func TestResolveAgentNameWithULID_Collision_FallsBackToRandom(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	// Create existing agent with same name
	err := database.CreateAgent(db.Agent{
		Name: "refactor-abcd",
		ULID: "existing-ulid",
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	name, err := resolveAgentNameWithULID(database, "refactor", "01JGXYZ123456789ABCD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have random suffix instead of "abcd"
	if name == "refactor-abcd" {
		t.Error("should have fallen back to random suffix")
	}
	if len(name) != len("refactor-xxxx") {
		t.Errorf("name length = %d, want %d", len(name), len("refactor-xxxx"))
	}
}

func TestResolveAgentNameWithULID_EmptyPrefix(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	name, err := resolveAgentNameWithULID(database, "", "01JGXYZ123456789WXYZ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should end with -wxyz (adjective-noun-wxyz)
	if !regexp.MustCompile(`^[a-z]+-[a-z]+-wxyz$`).MatchString(name) {
		t.Errorf("name = %q, want adjective-noun-wxyz pattern", name)
	}
}

func TestResolveAgentNameWithULID_DBError_Propagates(t *testing.T) {
	database := openTestDB(t)
	database.Close() // Close DB to force errors

	_, err := resolveAgentNameWithULID(database, "test", "01JGXYZ123456789ABCD")
	if err == nil {
		t.Error("expected error from closed DB, got nil")
	}
	if !strings.Contains(err.Error(), "failed to check for existing agent") {
		t.Errorf("error = %q, want 'failed to check for existing agent'", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestResolveAgentNameWithULID -v`
Expected: FAIL with "undefined: resolveAgentNameWithULID"

**Step 3: Write minimal implementation**

Add to `internal/cli/name.go`:

```go
import (
	"errors"
	"fmt"
	"github.com/sky-xo/june/internal/db"
)

// resolveAgentNameWithULID builds a name from prefix + ULID suffix.
// If collision, falls back to random suffix with retries.
func resolveAgentNameWithULID(database *db.DB, prefix, ulid string) (string, error) {
	if prefix == "" {
		prefix = generateAdjectiveNoun()
	}

	// Try ULID-based suffix first
	suffix := strings.ToLower(ulid[len(ulid)-4:])
	name := prefix + "-" + suffix

	_, err := database.GetAgent(name)
	if err == db.ErrAgentNotFound {
		return name, nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to check for existing agent: %w", err)
	}

	// Collision (rare) - fall back to random suffix
	for attempts := 0; attempts < 10; attempts++ {
		name = prefix + "-" + randomHexSuffix()
		_, err := database.GetAgent(name)
		if err == db.ErrAgentNotFound {
			return name, nil
		}
		if err != nil {
			return "", fmt.Errorf("failed to check for existing agent: %w", err)
		}
	}

	return "", errors.New("failed to generate unique agent name after 10 attempts")
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -run TestResolveAgentNameWithULID -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/name.go internal/cli/name_test.go
git commit -m "feat(cli): add resolveAgentNameWithULID with collision fallback"
```

---

### Task 5: Update spawn.go to Use New Naming

**Files:**
- Modify: `internal/cli/spawn.go`

**Step 1: Review current flow**

Current `runSpawnCodex()` calls `resolveAgentName()` before spawning. We need to:
1. Remove the pre-spawn `resolveAgentName()` call
2. Keep the prefix (user-provided or empty)
3. After getting ULID, call `resolveAgentNameWithULID()`

**Step 2: Update runSpawnCodex signature and flow**

In `internal/cli/spawn.go`, update `runSpawnCodex`:

```go
func runSpawnCodex(prefix, task string, model, reasoningEffort, sandbox string, maxTokens int) error {
	// Capture git context before spawning
	repoPath := scope.RepoRoot()
	branch := scope.BranchName()

	// Open database
	home, err := juneHome()
	if err != nil {
		return fmt.Errorf("failed to get june home: %w", err)
	}
	dbPath := filepath.Join(home, "june.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Before creating the command, ensure isolated codex home
	isolatedCodexHome, err := codex.EnsureCodexHome()
	if err != nil {
		return fmt.Errorf("failed to setup isolated codex home: %w", err)
	}

	// Build codex command arguments dynamically
	args := buildCodexArgs(task, model, reasoningEffort, sandbox, maxTokens)

	// Start codex exec --json
	codexCmd := exec.Command("codex", args...)
	codexCmd.Stderr = os.Stderr
	codexCmd.Env = append(os.Environ(), fmt.Sprintf("CODEX_HOME=%s", isolatedCodexHome))

	stdout, err := codexCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := codexCmd.Start(); err != nil {
		return fmt.Errorf("failed to start codex: %w", err)
	}

	// Read first line to get thread_id
	scanner := bufio.NewScanner(stdout)
	var threadID string
	if scanner.Scan() {
		var event struct {
			Type     string `json:"type"`
			ThreadID string `json:"thread_id"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err == nil {
			if event.Type == "thread.started" {
				threadID = event.ThreadID
			}
		}
	}

	if threadID == "" {
		codexCmd.Process.Kill()
		codexCmd.Wait()
		return fmt.Errorf("failed to get thread_id from codex output")
	}

	// NOW resolve the name using the ULID
	name, err := resolveAgentNameWithULID(database, prefix, threadID)
	if err != nil {
		codexCmd.Process.Kill()
		codexCmd.Wait()
		return fmt.Errorf("failed to resolve agent name: %w", err)
	}

	// Find the session file
	sessionFile, err := codex.FindSessionFile(threadID)
	if err != nil {
		sessionFile = ""
	}

	// Create agent record
	agent := db.Agent{
		Name:        name,
		ULID:        threadID,
		SessionFile: sessionFile,
		PID:         codexCmd.Process.Pid,
		RepoPath:    repoPath,
		Branch:      branch,
	}
	if err := database.CreateAgent(agent); err != nil {
		return fmt.Errorf("failed to create agent record: %w", err)
	}

	// Drain remaining output
	for scanner.Scan() {
	}

	// Wait for process to finish
	if err := codexCmd.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "codex exited with error: %v\n", err)
	}

	// Update session file if we didn't have it
	if sessionFile == "" {
		if found, err := codex.FindSessionFile(threadID); err == nil {
			if err := database.UpdateSessionFile(name, found); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to update session file: %v\n", err)
			}
		}
	}

	// Print the agent name
	fmt.Println(name)

	return nil
}
```

**Step 3: Update newSpawnCmd to pass prefix directly**

```go
func newSpawnCmd() *cobra.Command {
	var (
		name            string
		model           string
		reasoningEffort string
		maxTokens       int
		sandbox         string
	)

	cmd := &cobra.Command{
		Use:   "spawn <type> <task>",
		Short: "Spawn an agent",
		Long:  "Spawn a Codex agent to perform a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentType := args[0]
			task := args[1]

			if agentType != "codex" {
				return fmt.Errorf("unsupported agent type: %s (only 'codex' is supported)", agentType)
			}

			// Pass name as prefix (empty string if not provided)
			return runSpawnCodex(name, task, model, reasoningEffort, sandbox, maxTokens)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Prefix for the agent name (auto-generated if omitted)")
	cmd.Flags().StringVar(&model, "model", "", "Codex model to use")
	cmd.Flags().StringVar(&reasoningEffort, "reasoning-effort", "", "Reasoning effort (minimal|low|medium|high|xhigh)")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 0, "Max output tokens")
	cmd.Flags().StringVar(&sandbox, "sandbox", "", "Sandbox mode (read-only|workspace-write|danger-full-access)")

	return cmd
}
```

**Step 4: Remove old resolveAgentName function**

Delete `resolveAgentName()` and `formatCollisionError()` from `spawn.go` (they're no longer needed).

**Step 5: Commit**

```bash
git add internal/cli/spawn.go
git commit -m "feat(cli): use ULID-based naming in spawn flow"
```

---

### Task 6: Update spawn_test.go

**Files:**
- Modify: `internal/cli/spawn_test.go`

**Step 1: Remove old resolveAgentName tests**

Delete these test functions:
- `TestResolveAgentName_UserProvided`
- `TestResolveAgentName_UserProvided_Collision`
- `TestResolveAgentName_AutoGenerated`
- `TestResolveAgentName_AutoGenerated_RetriesOnCollision`

Also remove `openTestDB` (moved to name_test.go).

**Step 2: Update flag help text test if needed**

The `--name` flag help changed from "Name for the agent" to "Prefix for the agent name". No test currently checks this, so no change needed.

**Step 3: Run all tests**

Run: `go test ./internal/cli -v`
Expected: PASS (all remaining tests pass)

**Step 4: Commit**

```bash
git add internal/cli/spawn_test.go
git commit -m "test(cli): remove obsolete resolveAgentName tests"
```

---

### Task 7: Clean Up Old generateName

**Files:**
- Modify: `internal/cli/name.go`

**Step 1: Remove old generateName function**

Delete `generateName()` and `base62Chars` constant - they're no longer used.

**Step 2: Verify imports are clean**

Ensure only needed imports remain.

**Step 3: Run tests**

Run: `go test ./internal/cli -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/cli/name.go
git commit -m "refactor(cli): remove unused generateName function"
```

---

### Task 8: Final Verification

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All PASS

**Step 2: Build**

Run: `go build -o june .`
Expected: Success

**Step 3: Manual smoke test (optional)**

If codex is available:
```bash
./june spawn codex "echo hello" --name test
# Should output: test-XXXX (4 char suffix)

./june spawn codex "echo hello"
# Should output: adjective-noun-XXXX
```

**Step 4: Final commit if any cleanup needed**

```bash
git status
# If clean, done. Otherwise commit any remaining changes.
```
