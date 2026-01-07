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
