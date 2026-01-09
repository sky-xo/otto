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
		name          string
		args          []string
		wantErr       bool
		runValidation bool // if true, let RunE execute to test validation logic
	}{
		{
			name:    "valid args with all flags",
			args:    []string{"codex", "task", "--name", "test", "--model", "o3", "--reasoning-effort", "high", "--max-tokens", "4096", "--sandbox=read-only"},
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
		{
			name:    "sandbox without value followed by name flag",
			args:    []string{"codex", "task", "--sandbox", "--name", "test"},
			wantErr: false,
		},
		{
			name:    "gemini sandbox with name flag",
			args:    []string{"gemini", "task", "--sandbox", "--name", "test"},
			wantErr: false,
		},
		{
			name:          "gemini sandbox with explicit value errors",
			args:          []string{"gemini", "task", "--sandbox=read-only"},
			wantErr:       true,
			runValidation: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newSpawnCmd()
			cmd.SetArgs(tt.args)

			// For tests that check validation errors, let RunE execute.
			// For others, disable to avoid actually spawning agents.
			if !tt.runValidation {
				cmd.RunE = func(c *cobra.Command, args []string) error { return nil }
			}

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
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

func TestStreamLines(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "normal lines",
			input: "line1\nline2\nline3\n",
			want:  []string{"line1", "line2", "line3"},
		},
		{
			name:  "empty input",
			input: "",
			want:  []string{},
		},
		{
			name:  "single line no newline",
			input: "single",
			want:  []string{"single"},
		},
		{
			name:  "line over 1MB (scanner would fail)",
			input: string(make([]byte, 2*1024*1024)) + "\n", // 2MB of zeros
			want:  []string{string(make([]byte, 2*1024*1024))},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			var got []string
			err := streamLines(reader, func(line []byte) error {
				got = append(got, string(line))
				return nil
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("streamLines() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("streamLines() got %d lines, want %d", len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("streamLines() line %d length = %d, want %d", i, len(got[i]), len(tt.want[i]))
				}
			}
		})
	}
}

func TestBuildGeminiArgs(t *testing.T) {
	tests := []struct {
		name    string
		task    string
		model   string
		yolo    bool
		sandbox bool
		want    []string
	}{
		{
			name:    "basic task with defaults",
			task:    "fix the bug",
			model:   "",
			yolo:    false,
			sandbox: false,
			want:    []string{"-p", "fix the bug", "--output-format", "stream-json", "--approval-mode", "auto_edit"},
		},
		{
			name:    "with yolo mode",
			task:    "refactor code",
			model:   "",
			yolo:    true,
			sandbox: false,
			want:    []string{"-p", "refactor code", "--output-format", "stream-json", "--yolo"},
		},
		{
			name:    "with model",
			task:    "write tests",
			model:   "gemini-2.5-pro",
			yolo:    false,
			sandbox: false,
			want:    []string{"-p", "write tests", "--output-format", "stream-json", "--approval-mode", "auto_edit", "-m", "gemini-2.5-pro"},
		},
		{
			name:    "with sandbox",
			task:    "dangerous task",
			model:   "",
			yolo:    true,
			sandbox: true,
			want:    []string{"-p", "dangerous task", "--output-format", "stream-json", "--yolo", "--sandbox"},
		},
		{
			name:    "all options",
			task:    "full task",
			model:   "gemini-2.5-flash",
			yolo:    true,
			sandbox: true,
			want:    []string{"-p", "full task", "--output-format", "stream-json", "--yolo", "-m", "gemini-2.5-flash", "--sandbox"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildGeminiArgs(tt.task, tt.model, tt.yolo, tt.sandbox)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildGeminiArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}
