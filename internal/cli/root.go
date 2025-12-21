package cli

import "github.com/spf13/cobra"

func Execute() {
	rootCmd := &cobra.Command{Use: "otto"}
	_ = rootCmd.Execute()
}
