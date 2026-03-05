package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "coderev",
	Short: "AI-powered code review via AWS Bedrock",
	Long:  "coderev reviews your staged git changes using an LLM on AWS Bedrock.",
}

// Execute is the entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
