package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/sky-xo/june/internal/claude"
	"github.com/sky-xo/june/internal/scope"
	"github.com/sky-xo/june/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// version and commit can be set via ldflags at build time (e.g., make build)
var (
	version = ""
	commit  = ""
)

// Version returns the version string, checking ldflags first, then Go module info
func Version() string {
	// If version is set via ldflags and isn't "dev", use it
	if version != "" && version != "dev" {
		return version
	}

	// If version is "dev" and commit is set, show "dev (commit)"
	if version == "dev" && commit != "" && commit != "unknown" {
		return "dev (" + commit + ")"
	}

	// If version is empty, try to read from Go's embedded module info (go install @version)
	if version == "" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}

	// Final fallback
	return "dev"
}

func Execute() {
	rootCmd := &cobra.Command{
		Use:     "june",
		Short:   "Subagent viewer for Claude Code",
		Version: Version(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWatch()
		},
	}

	rootCmd.AddCommand(newSpawnCmd())
	rootCmd.AddCommand(newPeekCmd())
	rootCmd.AddCommand(newLogsCmd())
	rootCmd.AddCommand(newTaskCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// runWatch starts the TUI.
func runWatch() error {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("june requires a terminal")
	}

	// Get current git project root
	repoRoot := scope.RepoRoot()
	if repoRoot == "" {
		return fmt.Errorf("not in a git repository")
	}

	// Get absolute path and repo name
	basePath, err := filepath.Abs(repoRoot)
	if err != nil {
		return err
	}
	repoName := filepath.Base(basePath)

	// Get Claude projects directory
	claudeProjectsDir := claude.ClaudeProjectsDir()

	// Check if any project directory exists for this repo
	projectDir := claude.ProjectDir(basePath)
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		return fmt.Errorf("no Claude Code sessions found for this project\n\nExpected: %s", projectDir)
	}

	// Run TUI
	model := tui.NewModel(claudeProjectsDir, basePath, repoName)
	defer model.Close()
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}
