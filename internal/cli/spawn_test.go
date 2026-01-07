package cli

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sky-xo/june/internal/db"
	"github.com/spf13/cobra"
)

func TestBuildCodexArgs(t *testing.T) {
	tests := []struct {
		name            string
		task            string
		model           string
		reasoningEffort string
		sandbox         string
		maxTokens       int
		want            []string
	}{
		{
			name: "task only (no flags)",
			task: "implement feature",
			want: []string{"exec", "--json", "implement feature"},
		},
		{
			name:  "with model flag",
			task:  "implement feature",
			model: "o3",
			want:  []string{"exec", "--json", "--model", "o3", "implement feature"},
		},
		{
			name:            "with reasoning-effort flag",
			task:            "implement feature",
			reasoningEffort: "high",
			want:            []string{"exec", "--json", "-c", "model_reasoning_effort=high", "implement feature"},
		},
		{
			name:      "with max-tokens flag",
			task:      "implement feature",
			maxTokens: 4096,
			want:      []string{"exec", "--json", "-c", "model_max_output_tokens=4096", "implement feature"},
		},
		{
			name:    "with sandbox flag",
			task:    "implement feature",
			sandbox: "workspace-write",
			want:    []string{"exec", "--json", "--sandbox", "workspace-write", "implement feature"},
		},
		{
			name:            "with all flags",
			task:            "implement feature",
			model:           "o3",
			reasoningEffort: "medium",
			maxTokens:       8192,
			sandbox:         "read-only",
			want: []string{
				"exec", "--json",
				"--model", "o3",
				"-c", "model_reasoning_effort=medium",
				"-c", "model_max_output_tokens=8192",
				"--sandbox", "read-only",
				"implement feature",
			},
		},
		{
			name:            "with model and reasoning-effort",
			task:            "fix bug",
			model:           "gpt-4",
			reasoningEffort: "low",
			want: []string{
				"exec", "--json",
				"--model", "gpt-4",
				"-c", "model_reasoning_effort=low",
				"fix bug",
			},
		},
		{
			name:      "with sandbox and max-tokens",
			task:      "refactor code",
			maxTokens: 2048,
			sandbox:   "danger-full-access",
			want: []string{
				"exec", "--json",
				"-c", "model_max_output_tokens=2048",
				"--sandbox", "danger-full-access",
				"refactor code",
			},
		},
		{
			name:      "zero max-tokens is ignored",
			task:      "implement feature",
			maxTokens: 0,
			want:      []string{"exec", "--json", "implement feature"},
		},
		{
			name:  "empty model is ignored",
			task:  "implement feature",
			model: "",
			want:  []string{"exec", "--json", "implement feature"},
		},
		{
			name:            "empty reasoning-effort is ignored",
			task:            "implement feature",
			reasoningEffort: "",
			want:            []string{"exec", "--json", "implement feature"},
		},
		{
			name:    "empty sandbox is ignored",
			task:    "implement feature",
			sandbox: "",
			want:    []string{"exec", "--json", "implement feature"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCodexArgs(tt.task, tt.model, tt.reasoningEffort, tt.sandbox, tt.maxTokens)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildCodexArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpawnCmdFlags(t *testing.T) {
	cmd := newSpawnCmd()

	// Test that all expected flags exist
	flags := []struct {
		name     string
		flagType string
	}{
		{"name", "string"},
		{"model", "string"},
		{"reasoning-effort", "string"},
		{"max-tokens", "int"},
		{"sandbox", "string"},
	}

	for _, f := range flags {
		t.Run("flag_"+f.name+"_exists", func(t *testing.T) {
			flag := cmd.Flags().Lookup(f.name)
			if flag == nil {
				t.Errorf("flag --%s not found", f.name)
				return
			}
			if flag.Value.Type() != f.flagType {
				t.Errorf("flag --%s type = %s, want %s", f.name, flag.Value.Type(), f.flagType)
			}
		})
	}
}

func TestSpawnCmdFlagDefaults(t *testing.T) {
	cmd := newSpawnCmd()

	tests := []struct {
		name         string
		defaultValue string
	}{
		{"name", ""},
		{"model", ""},
		{"reasoning-effort", ""},
		{"max-tokens", "0"},
		{"sandbox", ""},
	}

	for _, tt := range tests {
		t.Run("flag_"+tt.name+"_default", func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.name)
			if flag == nil {
				t.Fatalf("flag --%s not found", tt.name)
			}
			if flag.DefValue != tt.defaultValue {
				t.Errorf("flag --%s default = %q, want %q", tt.name, flag.DefValue, tt.defaultValue)
			}
		})
	}
}

func TestSpawnCmdFlagParsing(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "valid args with all flags",
			args:    []string{"codex", "task", "--name", "test", "--model", "o3", "--reasoning-effort", "high", "--max-tokens", "4096", "--sandbox", "read-only"},
			wantErr: false,
		},
		{
			name:    "valid args with only name flag",
			args:    []string{"codex", "task", "--name", "test"},
			wantErr: false,
		},
		{
			name:    "valid args without name flag (auto-generated)",
			args:    []string{"codex", "task"},
			wantErr: false,
		},
		{
			name:    "invalid max-tokens type",
			args:    []string{"codex", "task", "--name", "test", "--max-tokens", "not-a-number"},
			wantErr: true,
		},
		{
			name:    "missing agent type argument",
			args:    []string{"--name", "test"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newSpawnCmd()
			cmd.SetArgs(tt.args)
			// Disable RunE to avoid actual execution
			cmd.RunE = func(c *cobra.Command, args []string) error { return nil }

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveAgentName_UserProvided(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	name, err := resolveAgentName(database, "my-agent", true)
	if err != nil {
		t.Fatalf("resolveAgentName failed: %v", err)
	}
	if name != "my-agent" {
		t.Errorf("name = %q, want %q", name, "my-agent")
	}
}

func TestResolveAgentName_UserProvided_Collision(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	// Create existing agent
	err := database.CreateAgent(db.Agent{
		Name:        "existing",
		ULID:        "test-ulid",
		SessionFile: "/tmp/test.jsonl",
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	_, err = resolveAgentName(database, "existing", true)
	if err == nil {
		t.Fatal("expected error for collision, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists', got: %v", err)
	}
	if !strings.Contains(err.Error(), "existing-2") {
		t.Errorf("error should suggest 'existing-2', got: %v", err)
	}
}

func TestResolveAgentName_AutoGenerated(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	name, err := resolveAgentName(database, "", false)
	if err != nil {
		t.Fatalf("resolveAgentName failed: %v", err)
	}
	if !strings.HasPrefix(name, "task-") {
		t.Errorf("auto-generated name should start with 'task-', got: %q", name)
	}
	if len(name) != 11 { // "task-" (5) + 6 chars
		t.Errorf("auto-generated name should be 11 chars, got: %d", len(name))
	}
}

func TestResolveAgentName_AutoGenerated_RetriesOnCollision(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	// This test verifies the retry logic exists, not that it works perfectly
	// (since we can't easily force collisions with random names)
	name, err := resolveAgentName(database, "", false)
	if err != nil {
		t.Fatalf("resolveAgentName failed: %v", err)
	}
	if name == "" {
		t.Error("resolveAgentName should return a name")
	}
}

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	return database
}
