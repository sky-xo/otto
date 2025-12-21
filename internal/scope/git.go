package scope

import (
	"os"
	"os/exec"
	"strings"
)

// RepoRoot returns git repo root or empty string if not in a git repo.
func RepoRoot() string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// BranchName returns current branch or empty string if not in a git repo.
func BranchName() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// CurrentScope returns the scope for the current directory.
// It uses git to determine repo root and branch name.
// Falls back to using the current directory basename if git is unavailable.
// Defaults to "main" as the branch name if the branch cannot be determined.
func CurrentScope() string {
	repoRoot := RepoRoot()
	if repoRoot == "" {
		// Fallback: use current directory
		cwd, err := os.Getwd()
		if err != nil {
			return "unknown"
		}
		repoRoot = cwd
	}

	branch := BranchName()
	if branch == "" {
		// Default to "main" when branch cannot be determined
		branch = "main"
	}

	return Scope(repoRoot, branch)
}
