package scope

import "path/filepath"

type Context struct {
	Project string
	Branch  string
}

func ContextFromRepoRoot(repoRoot, branch string) Context {
	project := filepath.Base(repoRoot)
	if branch == "" {
		branch = "main"
	}
	return Context{Project: project, Branch: branch}
}

func CurrentContext() Context {
	repoRoot := RepoRoot()
	branch := BranchName()
	if repoRoot == "" {
		return Context{Project: "unknown", Branch: "main"}
	}
	return ContextFromRepoRoot(repoRoot, branch)
}
