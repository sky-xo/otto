package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestTaskCommandExists(t *testing.T) {
	cmd := newTaskCmd()
	if cmd.Use != "task" {
		t.Errorf("Use = %q, want %q", cmd.Use, "task")
	}

	// Check subcommands exist
	subcommands := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subcommands[sub.Name()] = true
	}

	expected := []string{"create", "list", "update", "delete"}
	for _, name := range expected {
		if !subcommands[name] {
			t.Errorf("Missing subcommand: %s", name)
		}
	}
}

func setupTestRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("JUNE_HOME", tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	// Save current dir before changing
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	// Register cleanup before chdir so it runs after the test
	t.Cleanup(func() { os.Chdir(origDir) })

	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	if out, err := exec.Command("git", "init").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "checkout", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git checkout failed: %v\n%s", err, out)
	}

	return repoDir
}

func TestTaskCreateSingle(t *testing.T) {
	setupTestRepo(t)

	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Test task"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	output := stdout.String()
	// Should output task ID like "t-a3f8b"
	pattern := regexp.MustCompile(`t-[a-f0-9]{5}`)
	if !pattern.MatchString(output) {
		t.Errorf("Output %q doesn't contain task ID", output)
	}
}

func TestTaskCreateWithParent(t *testing.T) {
	setupTestRepo(t)

	// Create parent first
	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Parent task"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	parentID := strings.TrimSpace(stdout.String())

	// Create child
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"create", "Child task", "--parent", parentID})
	stdout.Reset()
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	childID := strings.TrimSpace(stdout.String())
	if childID == parentID {
		t.Error("Child should have different ID than parent")
	}
}

func TestTaskCreateMultiple(t *testing.T) {
	setupTestRepo(t)

	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Task 1", "Task 2", "Task 3"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 task IDs, got %d", len(lines))
	}
}

func TestTaskListRoot(t *testing.T) {
	setupTestRepo(t)

	// Create tasks
	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Task 1"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	cmd = newTaskCmd()
	cmd.SetArgs([]string{"create", "Task 2"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// List root tasks
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Task 1") || !strings.Contains(output, "Task 2") {
		t.Errorf("List output should contain both tasks: %s", output)
	}
}

func TestTaskListSpecific(t *testing.T) {
	setupTestRepo(t)

	// Create parent
	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Parent task"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	parentID := strings.TrimSpace(stdout.String())

	// Create children
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"create", "Child 1", "Child 2", "--parent", parentID})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// List specific task
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list", parentID})
	stdout.Reset()
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Parent task") {
		t.Errorf("Should show task title: %s", output)
	}
	if !strings.Contains(output, "Child 1") || !strings.Contains(output, "Child 2") {
		t.Errorf("Should show children: %s", output)
	}
}

func TestTaskUpdateStatus(t *testing.T) {
	setupTestRepo(t)

	// Create task
	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Test task"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	taskID := strings.TrimSpace(stdout.String())

	// Update status
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"update", taskID, "--status", "in_progress"})
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify via list
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list", taskID})
	stdout.Reset()
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(stdout.String(), "[in_progress]") {
		t.Errorf("Status not updated: %s", stdout.String())
	}
}

func TestTaskUpdateNote(t *testing.T) {
	setupTestRepo(t)

	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Test task"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	taskID := strings.TrimSpace(stdout.String())

	// Update note
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"update", taskID, "--note", "Important note"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list", taskID})
	stdout.Reset()
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(stdout.String(), "Important note") {
		t.Errorf("Note not shown: %s", stdout.String())
	}
}

func TestTaskDelete(t *testing.T) {
	setupTestRepo(t)

	// Create task
	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Test task"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	taskID := strings.TrimSpace(stdout.String())

	// Delete task
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"delete", taskID})
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify not in list
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list"})
	stdout.Reset()
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if strings.Contains(stdout.String(), taskID) {
		t.Errorf("Deleted task still appears in list")
	}
}

func TestTaskListDeletedByID(t *testing.T) {
	setupTestRepo(t)

	// Create task
	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Test task"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	taskID := strings.TrimSpace(stdout.String())

	// Delete task
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"delete", taskID})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Try to list the deleted task by ID - should fail
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list", taskID})
	stdout.Reset()
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when listing deleted task by ID")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestTaskListJSON(t *testing.T) {
	setupTestRepo(t)

	// Create task
	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Test task"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// List with JSON
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list", "--json"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should be valid JSON array
	var tasks []map[string]any
	err := json.Unmarshal(stdout.Bytes(), &tasks)
	if err != nil {
		t.Fatalf("Invalid JSON: %v\nOutput: %s", err, stdout.String())
	}

	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}
}

func TestTaskCreateJSON(t *testing.T) {
	setupTestRepo(t)

	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Test task", "--json"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var result map[string]string
	err := json.Unmarshal(stdout.Bytes(), &result)
	if err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	if result["id"] == "" {
		t.Error("Expected id in JSON output")
	}
}

func TestTaskCreateWithNote(t *testing.T) {
	setupTestRepo(t)

	cmd := newTaskCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"create", "Test task", "--note", "This is a note", "--json"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Parse JSON output to get task ID
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify note was set by listing the task
	out.Reset()
	cmd = newTaskCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list", result.ID, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Failed to list task: %v", err)
	}

	// Check output contains the note
	if !bytes.Contains(out.Bytes(), []byte("This is a note")) {
		t.Errorf("Expected note in output, got: %s", out.String())
	}
}
