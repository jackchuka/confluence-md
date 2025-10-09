package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jackchuka/confluence-md/internal/converter"
	"github.com/spf13/cobra"
)

var htmlCmd = &cobra.Command{
	Use:   "html [input-file]",
	Short: "Convert Confluence HTML to Markdown",
	Long: `Convert Confluence HTML to Markdown format.

Read HTML from a file or stdin and convert it to Markdown.
This is useful for testing or converting exported HTML content.

Examples:
  # Convert from file
  confluence-md html page.html

  # Convert from stdin
  cat page.html | confluence-md html

  # Convert and save to file
  confluence-md html page.html -o output.md

  # Convert from stdin and save
  cat page.html | confluence-md html -o output.md`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHTMLConvert,
}

var htmlOptions struct {
	output      string
	imageFolder string
}

func init() {
	htmlCmd.Flags().StringVarP(&htmlOptions.output, "output", "o", "", "Output file (default: stdout)")
	htmlCmd.Flags().StringVar(&htmlOptions.imageFolder, "image-folder", "assets", "Folder path for images in markdown")

	rootCmd.AddCommand(htmlCmd)
}

func runHTMLConvert(cmd *cobra.Command, args []string) error {
	// Read HTML input
	var htmlContent []byte
	var err error

	if len(args) > 0 {
		// Read from file
		inputFile := args[0]
		htmlContent, err = os.ReadFile(inputFile)
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}
	} else {
		// Read from stdin
		htmlContent, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
	}

	if len(htmlContent) == 0 {
		return fmt.Errorf("no input provided")
	}

	// Create converter (using nil client for HTML-only conversion)
	conv := converter.NewConverter(nil, converter.WithDownloadAttachments(htmlOptions.imageFolder))

	// Convert HTML to Markdown
	markdown, err := conv.ConvertHTML(string(htmlContent))
	if err != nil {
		return fmt.Errorf("failed to convert HTML: %w", err)
	}

	// Write output
	if htmlOptions.output != "" {
		// Create output directory if needed
		outputDir := filepath.Dir(htmlOptions.output)
		if outputDir != "." && outputDir != "" {
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}
		}

		if err := os.WriteFile(htmlOptions.output, []byte(markdown), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		fmt.Fprintf(os.Stderr, "✅ Converted successfully to: %s\n", htmlOptions.output)
	} else {
		fmt.Print(markdown)
	}

	return nil
}
