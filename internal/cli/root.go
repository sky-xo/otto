package cli

import (
	"os"

	"otto/internal/cli/commands"

	"github.com/spf13/cobra"
)

func Execute() {
	rootCmd := &cobra.Command{Use: "otto"}

	// Add commands
	rootCmd.AddCommand(commands.NewSayCmd())
	rootCmd.AddCommand(commands.NewAskCmd())
	rootCmd.AddCommand(commands.NewCompleteCmd())
	rootCmd.AddCommand(commands.NewMessagesCmd())
	rootCmd.AddCommand(commands.NewStatusCmd())
	rootCmd.AddCommand(commands.NewSpawnCmd())
	rootCmd.AddCommand(commands.NewPromptCmd())
	rootCmd.AddCommand(commands.NewAttachCmd())
	rootCmd.AddCommand(commands.NewWatchCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
