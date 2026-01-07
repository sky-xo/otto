package cli

import (
	"regexp"
	"strings"
	"testing"

	"github.com/sky-xo/june/internal/db"
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
