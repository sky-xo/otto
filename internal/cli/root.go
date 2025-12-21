package cli

import (
	"os"

	"github.com/spf13/cobra"
)

func Execute() {
	rootCmd := &cobra.Command{Use: "otto"}
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
