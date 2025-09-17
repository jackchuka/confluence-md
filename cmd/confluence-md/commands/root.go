package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "confluence-md",
	Short: "Convert Confluence pages to Markdown format",
	Long: `Confluence to Markdown Converter

A CLI tool to convert Confluence pages to Markdown format.
Supports single page conversion and page tree conversion.

Examples:
  confluence-md page <page-url>
  confluence-md tree <page-url>
  confluence-md version`,

	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		os.Exit(1)
	}
}
