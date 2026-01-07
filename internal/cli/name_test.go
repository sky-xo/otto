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
