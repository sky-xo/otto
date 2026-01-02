// internal/claude/channels_test.go
package claude

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindRelatedProjectDirs(t *testing.T) {
	// Create temp Claude projects directory structure
	tmpDir := t.TempDir()
	claudeProjects := filepath.Join(tmpDir, ".claude", "projects")
	os.MkdirAll(claudeProjects, 0755)

	// Create project dirs
	dirs := []string{
		"-Users-test-code-myproject",
		"-Users-test-code-myproject--worktrees-feature1",
		"-Users-test-code-myproject--worktrees-feature2",
		"-Users-test-code-other", // unrelated
	}
	for _, d := range dirs {
		os.MkdirAll(filepath.Join(claudeProjects, d), 0755)
	}

	// Test finding related dirs
	basePath := "/Users/test/code/myproject"
	related := FindRelatedProjectDirs(claudeProjects, basePath)

	if len(related) != 3 {
		t.Errorf("expected 3 related dirs, got %d: %v", len(related), related)
	}
}

func TestExtractChannelName(t *testing.T) {
	tests := []struct {
		baseDir    string
		projectDir string
		repoName   string
		want       string
	}{
		{
			baseDir:    "-Users-test-code-myproject",
			projectDir: "-Users-test-code-myproject",
			repoName:   "myproject",
			want:       "myproject:main",
		},
		{
			baseDir:    "-Users-test-code-myproject",
			projectDir: "-Users-test-code-myproject--worktrees-feature1",
			repoName:   "myproject",
			want:       "myproject:feature1",
		},
		{
			baseDir:    "-Users-test-code-june",
			projectDir: "-Users-test-code-june--worktrees--worktrees-channels",
			repoName:   "june",
			want:       "june:channels",
		},
		{
			// Hyphenated branch names should be preserved
			baseDir:    "-Users-test-code-june",
			projectDir: "-Users-test-code-june--worktrees-select-mode",
			repoName:   "june",
			want:       "june:select-mode",
		},
	}

	for _, tt := range tests {
		got := ExtractChannelName(tt.baseDir, tt.projectDir, tt.repoName)
		if got != tt.want {
			t.Errorf("ExtractChannelName(%q, %q, %q) = %q, want %q",
				tt.baseDir, tt.projectDir, tt.repoName, got, tt.want)
		}
	}
}

func TestScanChannels(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	claudeProjects := filepath.Join(tmpDir, ".claude", "projects")

	// Create two project dirs with agent files
	mainDir := filepath.Join(claudeProjects, "-Users-test-code-myproject")
	worktreeDir := filepath.Join(claudeProjects, "-Users-test-code-myproject--worktrees-feature1")
	os.MkdirAll(mainDir, 0755)
	os.MkdirAll(worktreeDir, 0755)

	// Create agent files
	os.WriteFile(filepath.Join(mainDir, "agent-abc123.jsonl"), []byte(`{"type":"user","message":{"role":"user","content":"Main task"}}`+"\n"), 0644)
	os.WriteFile(filepath.Join(worktreeDir, "agent-def456.jsonl"), []byte(`{"type":"user","message":{"role":"user","content":"Feature task"}}`+"\n"), 0644)

	// Touch the feature file to make it more recent
	futureTime := time.Now().Add(time.Hour)
	os.Chtimes(filepath.Join(worktreeDir, "agent-def456.jsonl"), futureTime, futureTime)

	channels, err := ScanChannels(claudeProjects, "/Users/test/code/myproject", "myproject")
	if err != nil {
		t.Fatalf("ScanChannels failed: %v", err)
	}

	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}

	// Check channel names (sorted by most recent agent)
	if channels[0].Name != "myproject:feature1" {
		t.Errorf("expected first channel to be myproject:feature1, got %s", channels[0].Name)
	}
	if channels[1].Name != "myproject:main" {
		t.Errorf("expected second channel to be myproject:main, got %s", channels[1].Name)
	}

	// Check agents are present
	if len(channels[0].Agents) != 1 || channels[0].Agents[0].ID != "def456" {
		t.Errorf("unexpected agents in feature1 channel: %v", channels[0].Agents)
	}
	if len(channels[1].Agents) != 1 || channels[1].Agents[0].ID != "abc123" {
		t.Errorf("unexpected agents in main channel: %v", channels[1].Agents)
	}
}

func TestScanChannels_Integration(t *testing.T) {
	// Create a structure mimicking real June worktrees
	tmpDir := t.TempDir()
	claudeProjects := filepath.Join(tmpDir, ".claude", "projects")

	// Mimic: june main + 2 worktrees (including hyphenated name)
	dirs := []struct {
		name   string
		agents []string
	}{
		{"-Users-test-code-june", []string{"agent-main1.jsonl", "agent-main2.jsonl"}},
		{"-Users-test-code-june--worktrees-channels", []string{"agent-ch1.jsonl"}},
		{"-Users-test-code-june--worktrees-select-mode", []string{"agent-sel1.jsonl", "agent-sel2.jsonl"}},
	}

	for _, d := range dirs {
		dir := filepath.Join(claudeProjects, d.name)
		os.MkdirAll(dir, 0755)
		for _, a := range d.agents {
			os.WriteFile(filepath.Join(dir, a), []byte(`{"type":"user","message":{"role":"user","content":"Test task"}}`+"\n"), 0644)
		}
	}

	channels, err := ScanChannels(claudeProjects, "/Users/test/code/june", "june")
	if err != nil {
		t.Fatalf("ScanChannels failed: %v", err)
	}

	// Verify all channels found
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(channels))
	}

	// Verify channel names contain expected patterns
	names := make(map[string]bool)
	for _, ch := range channels {
		names[ch.Name] = true
	}
	if !names["june:main"] {
		t.Error("missing june:main channel")
	}
	if !names["june:channels"] {
		t.Error("missing june:channels channel")
	}
	if !names["june:select-mode"] {
		t.Error("missing june:select-mode channel")
	}
}
