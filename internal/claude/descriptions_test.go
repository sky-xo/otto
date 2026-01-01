// internal/claude/descriptions_test.go
package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDescriptionCacheRoundTrip(t *testing.T) {
	// Save original home dir and restore after test
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Use temp dir as home
	tempHome := t.TempDir()
	os.Setenv("HOME", tempHome)

	// Create and save a cache
	cache := DescriptionCache{
		"agent123": "Fix login bug",
		"agent456": "Add dark mode toggle",
	}

	if err := cache.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was created
	cachePath := filepath.Join(tempHome, ".june", "agent-descriptions.json")
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("cache file not created")
	}

	// Load it back
	loaded := LoadDescriptionCache()

	if len(loaded) != 2 {
		t.Errorf("expected 2 entries, got %d", len(loaded))
	}
	if loaded["agent123"] != "Fix login bug" {
		t.Errorf("expected 'Fix login bug', got %q", loaded["agent123"])
	}
	if loaded["agent456"] != "Add dark mode toggle" {
		t.Errorf("expected 'Add dark mode toggle', got %q", loaded["agent456"])
	}
}

func TestLoadDescriptionCacheMissingFile(t *testing.T) {
	// Save original home dir and restore after test
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Use temp dir as home (no cache file exists)
	tempHome := t.TempDir()
	os.Setenv("HOME", tempHome)

	cache := LoadDescriptionCache()

	// Should return empty cache, not error
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
	if len(cache) != 0 {
		t.Errorf("expected empty cache, got %d entries", len(cache))
	}
}

func TestScanSessionFilesForDescriptions(t *testing.T) {
	dir := t.TempDir()

	// Create a session file with Task tool call and result
	sessionContent := `{"message":{"content":[{"type":"tool_use","id":"toolu_abc123","name":"Task","input":{"description":"Fix login bug","prompt":"Please fix the login bug..."}}]}}
{"message":{"content":[{"type":"tool_result","tool_use_id":"toolu_abc123"}]},"toolUseResult":{"agentId":"xyz789"}}
{"message":{"content":[{"type":"tool_use","id":"toolu_def456","name":"Task","input":{"description":"Add dark mode","prompt":"Implement dark mode toggle..."}}]}}
{"message":{"content":[{"type":"tool_result","tool_use_id":"toolu_def456"}]},"toolUseResult":{"agentId":"uvw321"}}
`
	sessionFile := filepath.Join(dir, "session-main.jsonl")
	if err := os.WriteFile(sessionFile, []byte(sessionContent), 0644); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	// Create an agent file (should be ignored)
	agentFile := filepath.Join(dir, "agent-xyz789.jsonl")
	if err := os.WriteFile(agentFile, []byte(`{"type":"user","message":{"content":"test"}}`), 0644); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}

	cache := make(DescriptionCache)
	cache = ScanSessionFilesForDescriptions(dir, cache)

	// Should have extracted two descriptions
	if len(cache) != 2 {
		t.Errorf("expected 2 descriptions, got %d", len(cache))
	}

	if cache["xyz789"] != "Fix login bug" {
		t.Errorf("expected 'Fix login bug' for xyz789, got %q", cache["xyz789"])
	}
	if cache["uvw321"] != "Add dark mode" {
		t.Errorf("expected 'Add dark mode' for uvw321, got %q", cache["uvw321"])
	}
}

func TestScanSessionFilesForDescriptionsEmptyDir(t *testing.T) {
	dir := t.TempDir()

	cache := make(DescriptionCache)
	cache = ScanSessionFilesForDescriptions(dir, cache)

	if len(cache) != 0 {
		t.Errorf("expected empty cache for empty dir, got %d entries", len(cache))
	}
}

func TestScanSessionFilesForDescriptionsNonexistentDir(t *testing.T) {
	cache := make(DescriptionCache)
	cache = ScanSessionFilesForDescriptions("/nonexistent/path", cache)

	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
	if len(cache) != 0 {
		t.Errorf("expected empty cache, got %d entries", len(cache))
	}
}

func TestScanSessionFilesForDescriptionsMalformedJSON(t *testing.T) {
	dir := t.TempDir()

	// Create a session file with malformed JSON
	sessionContent := `{not valid json}
{"message":{"content":[{"type":"tool_use","id":"toolu_good","name":"Task","input":{"description":"Valid task","prompt":"..."}}]}}
{"message":{"content":[{"type":"tool_result","tool_use_id":"toolu_good"}]},"toolUseResult":{"agentId":"goodagent"}}
`
	sessionFile := filepath.Join(dir, "session-test.jsonl")
	if err := os.WriteFile(sessionFile, []byte(sessionContent), 0644); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	cache := make(DescriptionCache)
	cache = ScanSessionFilesForDescriptions(dir, cache)

	// Should have extracted the valid entry despite malformed line
	if len(cache) != 1 {
		t.Errorf("expected 1 description, got %d", len(cache))
	}
	if cache["goodagent"] != "Valid task" {
		t.Errorf("expected 'Valid task', got %q", cache["goodagent"])
	}
}

func TestScanSessionFilesForDescriptionsNoDescription(t *testing.T) {
	dir := t.TempDir()

	// Create a session file with Task tool call but no description field
	sessionContent := `{"message":{"content":[{"type":"tool_use","id":"toolu_nodesc","name":"Task","input":{"prompt":"Do something..."}}]}}
{"message":{"content":[{"type":"tool_result","tool_use_id":"toolu_nodesc"}]},"toolUseResult":{"agentId":"nodescagent"}}
`
	sessionFile := filepath.Join(dir, "session-nodesc.jsonl")
	if err := os.WriteFile(sessionFile, []byte(sessionContent), 0644); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	cache := make(DescriptionCache)
	cache = ScanSessionFilesForDescriptions(dir, cache)

	// Should not have extracted anything (no description in input)
	if len(cache) != 0 {
		t.Errorf("expected empty cache when no description, got %d entries", len(cache))
	}
}

func TestScanSessionFilesForDescriptionsPreservesExisting(t *testing.T) {
	dir := t.TempDir()

	// Create a session file
	sessionContent := `{"message":{"content":[{"type":"tool_use","id":"toolu_new","name":"Task","input":{"description":"New task","prompt":"..."}}]}}
{"message":{"content":[{"type":"tool_result","tool_use_id":"toolu_new"}]},"toolUseResult":{"agentId":"newagent"}}
`
	sessionFile := filepath.Join(dir, "session-test.jsonl")
	if err := os.WriteFile(sessionFile, []byte(sessionContent), 0644); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	// Start with existing cache entries
	cache := DescriptionCache{
		"existing": "Existing description",
	}
	cache = ScanSessionFilesForDescriptions(dir, cache)

	// Should have both old and new entries
	if len(cache) != 2 {
		t.Errorf("expected 2 entries, got %d", len(cache))
	}
	if cache["existing"] != "Existing description" {
		t.Errorf("existing entry was lost")
	}
	if cache["newagent"] != "New task" {
		t.Errorf("new entry not added")
	}
}

func TestScanSessionFilesIgnoresAgentFiles(t *testing.T) {
	dir := t.TempDir()

	// Create an agent file that happens to contain Task tool format (should be ignored)
	agentContent := `{"message":{"content":[{"type":"tool_use","id":"toolu_fake","name":"Task","input":{"description":"Fake task","prompt":"..."}}]}}
{"message":{"content":[{"type":"tool_result","tool_use_id":"toolu_fake"}]},"toolUseResult":{"agentId":"fakeagent"}}
`
	agentFile := filepath.Join(dir, "agent-shouldignore.jsonl")
	if err := os.WriteFile(agentFile, []byte(agentContent), 0644); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}

	cache := make(DescriptionCache)
	cache = ScanSessionFilesForDescriptions(dir, cache)

	// Should not have extracted from agent file
	if len(cache) != 0 {
		t.Errorf("expected empty cache (agent files should be ignored), got %d entries", len(cache))
	}
}
