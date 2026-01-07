package cli

import "testing"

func TestVersion(t *testing.T) {
	// Save original values to restore after tests
	origVersion := version
	origCommit := commit
	defer func() {
		version = origVersion
		commit = origCommit
	}()

	tests := []struct {
		name          string
		version       string
		commit        string
		wantExact     string
		checkNotEmpty bool // when true, just verify non-empty output
	}{
		{
			name:      "tagged release version",
			version:   "v0.2.0",
			commit:    "abc1234",
			wantExact: "v0.2.0",
		},
		{
			name:      "git describe version",
			version:   "v0.2.0-41-g72507bd",
			commit:    "72507bd",
			wantExact: "v0.2.0-41-g72507bd",
		},
		{
			name:      "dirty working tree",
			version:   "v0.2.0-41-g72507bd-dirty",
			commit:    "72507bd",
			wantExact: "v0.2.0-41-g72507bd-dirty",
		},
		{
			name:      "dev version with commit",
			version:   "dev",
			commit:    "abc1234",
			wantExact: "dev (abc1234)",
		},
		{
			name:      "dev version with unknown commit",
			version:   "dev",
			commit:    "unknown",
			wantExact: "dev",
		},
		{
			name:      "dev version without commit",
			version:   "dev",
			commit:    "",
			wantExact: "dev",
		},
		{
			name:          "empty version falls back",
			version:       "",
			commit:        "",
			checkNotEmpty: true, // Will be "dev" or module version
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version = tt.version
			commit = tt.commit

			got := Version()

			if tt.wantExact != "" && got != tt.wantExact {
				t.Errorf("Version() = %q, want %q", got, tt.wantExact)
			}
			if tt.checkNotEmpty && got == "" {
				t.Errorf("Version() returned empty string")
			}
		})
	}
}
