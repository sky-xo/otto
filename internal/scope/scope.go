package scope

import "path/filepath"

// Scope returns the orchestrator scope in the format "project/branch".
// The scope is used to determine where Otto stores data for each orchestrator.
// For example: "my-app/feature-auth" -> ~/.otto/orchestrators/my-app/feature-auth/otto.db
func Scope(repoRoot, branch string) string {
	project := filepath.Base(repoRoot)
	if branch == "" {
		return project
	}
	return project + "/" + branch
}
