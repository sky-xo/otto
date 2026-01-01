package cli

import (
	"os"

	"june/internal/cli/commands"

	"github.com/spf13/cobra"
)

func Execute() {
	rootCmd := &cobra.Command{
		Use:   "june",
		Short: "Multi-agent orchestrator for Claude Code and Codex",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.RunWatchDefault(cmd.Context())
		},
	}

	// Add commands
	rootCmd.AddCommand(commands.NewAskCmd())
	rootCmd.AddCommand(commands.NewCompleteCmd())
	rootCmd.AddCommand(commands.NewDMCmd())
	rootCmd.AddCommand(commands.NewMessagesCmd())
	rootCmd.AddCommand(commands.NewStatusCmd())
	rootCmd.AddCommand(commands.NewArchiveCmd())
	rootCmd.AddCommand(commands.NewSpawnCmd())
	rootCmd.AddCommand(commands.NewPromptCmd())
	rootCmd.AddCommand(commands.NewAttachCmd())
	rootCmd.AddCommand(commands.NewWatchCmd())
	rootCmd.AddCommand(commands.NewKillCmd())
	rootCmd.AddCommand(commands.NewInterruptCmd())
	rootCmd.AddCommand(commands.NewLogCmd())
	rootCmd.AddCommand(commands.NewPeekCmd())
	rootCmd.AddCommand(commands.NewInstallSkillsCmd())
	rootCmd.AddCommand(commands.NewWorkerSpawnCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
