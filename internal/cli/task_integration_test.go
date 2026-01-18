package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskWorkflowE2E(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JUNE_HOME", tmpDir)

	// Setup git repo
	repoDir := filepath.Join(tmpDir, "myproject")
	os.MkdirAll(repoDir, 0755)

	origDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origDir) })

	os.Chdir(repoDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "checkout", "-b", "feature-auth").Run()

	var stdout bytes.Buffer

	// 1. Create root task
	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Implement auth feature"})
	cmd.SetOut(&stdout)
	cmd.Execute()
	parentID := strings.TrimSpace(stdout.String())
	t.Logf("Created parent: %s", parentID)

	// 2. Create child tasks
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"create", "Add middleware", "Write tests", "Update docs", "--parent", parentID})
	stdout.Reset()
	cmd.SetOut(&stdout)
	cmd.Execute()
	childIDs := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	t.Logf("Created children: %v", childIDs)

	if len(childIDs) != 3 {
		t.Fatalf("Expected 3 children, got %d", len(childIDs))
	}

	// 3. Start work on first child
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"update", childIDs[0], "--status", "in_progress"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.Execute()

	// 4. Add note to first child
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"update", childIDs[0], "--note", "Mock server needs HTTPS"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.Execute()

	// 5. Complete first child
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"update", childIDs[0], "--status", "closed"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.Execute()

	// 6. List parent to see progress
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list", parentID})
	stdout.Reset()
	cmd.SetOut(&stdout)
	cmd.Execute()

	output := stdout.String()
	t.Logf("Task list output:\n%s", output)

	// Verify expected state
	if !strings.Contains(output, "Implement auth feature") {
		t.Error("Parent title not shown")
	}
	if !strings.Contains(output, "[closed]") {
		t.Error("First child should be closed")
	}
	if !strings.Contains(output, "[open]") {
		t.Error("Other children should be open")
	}

	// 6b. Check note on the specific child task
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list", childIDs[0]})
	stdout.Reset()
	cmd.SetOut(&stdout)
	cmd.Execute()

	childOutput := stdout.String()
	if !strings.Contains(childOutput, "Mock server needs HTTPS") {
		t.Errorf("Note should be shown on child task: %s", childOutput)
	}

	// 7. List root tasks
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list"})
	stdout.Reset()
	cmd.SetOut(&stdout)
	cmd.Execute()

	output = stdout.String()
	if !strings.Contains(output, parentID) {
		t.Error("Parent should appear in root list")
	}
	if !strings.Contains(output, "(3 children)") {
		t.Error("Should show children count")
	}

	// 8. Delete parent (cascades to children)
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"delete", parentID})
	cmd.SetOut(&bytes.Buffer{})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// 9. Verify all gone
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list"})
	stdout.Reset()
	cmd.SetOut(&stdout)
	cmd.Execute()

	output = stdout.String()
	if strings.Contains(output, parentID) {
		t.Error("Deleted parent should not appear")
	}
	if output != "No tasks.\n" {
		t.Errorf("Expected 'No tasks.', got: %s", output)
	}
}

func TestTaskScopedToRepoBranch(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("JUNE_HOME", tmpDir)

	// Setup two separate git repos to test scope isolation
	repo1Dir := filepath.Join(tmpDir, "project1")
	repo2Dir := filepath.Join(tmpDir, "project2")
	os.MkdirAll(repo1Dir, 0755)
	os.MkdirAll(repo2Dir, 0755)

	origDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origDir) })

	// Initialize repo1
	os.Chdir(repo1Dir)
	exec.Command("git", "init").Run()
	exec.Command("git", "checkout", "-b", "main").Run()

	// Create task in repo1
	cmd := newTaskCmd()
	cmd.SetArgs([]string{"create", "Project 1 task"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.Execute()
	repo1TaskID := strings.TrimSpace(stdout.String())

	// Initialize repo2
	os.Chdir(repo2Dir)
	exec.Command("git", "init").Run()
	exec.Command("git", "checkout", "-b", "main").Run()

	// Create task in repo2
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"create", "Project 2 task"})
	stdout.Reset()
	cmd.SetOut(&stdout)
	cmd.Execute()

	// List tasks in repo2 - should only see repo2 task
	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list"})
	stdout.Reset()
	cmd.SetOut(&stdout)
	cmd.Execute()

	output := stdout.String()
	if strings.Contains(output, repo1TaskID) {
		t.Error("Should not see project1 task in project2")
	}
	if !strings.Contains(output, "Project 2 task") {
		t.Error("Should see project2 task")
	}

	// Go back to repo1 and verify its task is visible
	os.Chdir(repo1Dir)

	cmd = newTaskCmd()
	cmd.SetArgs([]string{"list"})
	stdout.Reset()
	cmd.SetOut(&stdout)
	cmd.Execute()

	output = stdout.String()
	if !strings.Contains(output, "Project 1 task") {
		t.Error("Should see project1 task in project1")
	}
	if strings.Contains(output, "Project 2 task") {
		t.Error("Should not see project2 task in project1")
	}
}
