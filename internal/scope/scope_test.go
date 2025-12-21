package scope

import "testing"

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
