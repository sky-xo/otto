package scope

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestScopeFromRepoAndBranch(t *testing.T) {
	got := Scope("/Users/alice/code/my-app", "feature-auth")
	want := "my-app/feature-auth"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestScopeWithoutBranch(t *testing.T) {
	got := Scope("/Users/alice/code/my-app", "")
	want := "my-app"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestScopeWithDifferentPaths(t *testing.T) {
	tests := []struct {
		repoRoot string
		branch   string
		want     string
	}{
		{"/Users/bob/projects/cool-project", "main", "cool-project/main"},
		{"/home/dev/work/api-service", "develop", "api-service/develop"},
		{"/tmp/test", "hotfix/bug-123", "test/hotfix/bug-123"},
		{"/var/lib/repos/my-repo", "feature-x", "my-repo/feature-x"},
	}

	for _, tt := range tests {
		got := Scope(tt.repoRoot, tt.branch)
		if got != tt.want {
			t.Errorf("Scope(%q, %q) = %q; want %q", tt.repoRoot, tt.branch, got, tt.want)
		}
	}
}

func TestCurrentContextFromRepoRoot(t *testing.T) {
	ctx := ContextFromRepoRoot("/Users/alice/code/my-app", "feature-login")
	if ctx.Project != "my-app" {
		t.Fatalf("project = %q", ctx.Project)
	}
	if ctx.Branch != "feature-login" {
		t.Fatalf("branch = %q", ctx.Branch)
	}
}

func TestRepoRootRegularRepo(t *testing.T) {
	// This test runs in a regular git repo (june itself)
	// Verify RepoRoot() returns the correct path
	root := RepoRoot()
	if root == "" {
		t.Fatal("RepoRoot() returned empty string")
	}

	// Verify the path exists
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("RepoRoot() returned %q which does not exist: %v", root, err)
	}

	// Verify it has a .git directory (or file for worktrees)
	gitPath := filepath.Join(root, ".git")
	if _, err := os.Stat(gitPath); err != nil {
		t.Fatalf("Expected .git at %q but got error: %v", gitPath, err)
	}

	// The project name should be "june" since we're in the june repo
	project := filepath.Base(root)
	if project != "otto" {
		t.Fatalf("Expected project name 'otto', got %q", project)
	}
}

func TestRepoRootWorktree(t *testing.T) {
	// This test verifies that RepoRoot works correctly when run from a worktree
	// We'll create a temporary git repo with a worktree for testing

	// Create temp directory for test repo
	tmpDir := t.TempDir()
	mainRepo := filepath.Join(tmpDir, "main-repo")
	worktreePath := filepath.Join(tmpDir, "my-worktree")

	// Initialize main repo
	if err := os.Mkdir(mainRepo, 0755); err != nil {
		t.Fatal(err)
	}

	runGit := func(dir string, args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	// Setup main repo
	runGit(mainRepo, "init")
	runGit(mainRepo, "config", "user.email", "test@test.com")
	runGit(mainRepo, "config", "user.name", "Test User")

	// Create initial commit
	readmePath := filepath.Join(mainRepo, "README.md")
	if err := os.WriteFile(readmePath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(mainRepo, "add", "README.md")
	runGit(mainRepo, "commit", "-m", "Initial commit")

	// Create worktree
	runGit(mainRepo, "worktree", "add", worktreePath, "-b", "feature-branch")

	// Now test RepoRoot() from within the worktree
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(worktreePath); err != nil {
		t.Fatal(err)
	}

	// Get repo root from worktree
	root := RepoRoot()
	if root == "" {
		t.Fatal("RepoRoot() returned empty string from worktree")
	}

	// The root should be the main repo, not the worktree
	// Use filepath.EvalSymlinks to handle macOS /var -> /private/var symlink
	expectedRoot, err := filepath.EvalSymlinks(mainRepo)
	if err != nil {
		t.Fatal(err)
	}
	actualRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if actualRoot != expectedRoot {
		t.Errorf("Expected RepoRoot() to return %q (main repo), got %q", expectedRoot, actualRoot)
	}

	// Verify the project name is derived from main repo
	project := filepath.Base(root)
	if project != "main-repo" {
		t.Errorf("Expected project name 'main-repo', got %q", project)
	}

	// Verify we're actually in a worktree (branch name should be feature-branch)
	branch := BranchName()
	if branch != "feature-branch" {
		t.Errorf("Expected branch 'feature-branch', got %q", branch)
	}
}
