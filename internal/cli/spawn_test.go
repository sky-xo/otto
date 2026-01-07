package cli

import (
	"reflect"
	"testing"

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

func TestSpawnCmdRequiredFlags(t *testing.T) {
	cmd := newSpawnCmd()

	// The --name flag should be marked as required
	nameFlag := cmd.Flags().Lookup("name")
	if nameFlag == nil {
		t.Fatal("flag --name not found")
	}

	// Check if the flag is annotated as required
	annotations := nameFlag.Annotations
	if _, ok := annotations["cobra_annotation_bash_completion_one_required_flag"]; !ok {
		t.Error("flag --name should be marked as required")
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
			name:    "valid args with only required flags",
			args:    []string{"codex", "task", "--name", "test"},
			wantErr: false,
		},
		{
			name:    "missing required name flag",
			args:    []string{"codex", "task"},
			wantErr: true,
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
