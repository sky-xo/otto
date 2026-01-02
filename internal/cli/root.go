package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"june/internal/claude"
	"june/internal/scope"
	"june/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func Execute() {
	rootCmd := &cobra.Command{
		Use:   "june",
		Short: "Subagent viewer for Claude Code",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWatch()
		},
	}

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
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}
