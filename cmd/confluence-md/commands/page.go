package commands

import (
	"fmt"
	"os"

	"github.com/jackchuka/confluence-md/internal/confluence"
	"github.com/spf13/cobra"
)

// pageCmd represents the page command
var pageCmd = &cobra.Command{
	Use:   "page",
	Short: "Convert a single Confluence page to Markdown",
	Long: `Convert a single Confluence page to Markdown format.

Provide the page URL and your API token to download and convert the page.
The converted content is saved to an output directory with images in an assets folder.

Examples:
  # Convert to default output directory
  confluence-md page https://example.atlassian.net/wiki/spaces/SPACE/pages/12345/Title

  # Convert to custom directory
  confluence-md page https://example.atlassian.net/wiki/spaces/SPACE/pages/12345/Title --output ./docs

  # Convert without downloading images
  confluence-md page https://example.atlassian.net/wiki/spaces/SPACE/pages/12345/Title --download-images=false`,

	RunE: func(cmd *cobra.Command, args []string) error {
		return runPage(cmd, args)
	},
}

var pageOpts PageOptions

type PageOptions struct {
	authOptions
	commonOptions
}

func init() {
	rootCmd.AddCommand(pageCmd)

	pageOpts.authOptions.InitFlags(pageCmd)
	pageOpts.commonOptions.InitFlags(pageCmd)

	// Required flags
	_ = pageCmd.MarkFlagRequired("api-token")
	_ = pageCmd.MarkFlagRequired("email")
}

func runPage(_ *cobra.Command, args []string) error {
	// Get required flags
	if len(args) < 1 {
		return fmt.Errorf("missing required argument: page URL")
	}
	pageURL := args[0]

	// Extract base URL from page URL
	pageInfo, err := confluence.ParseURL(pageURL)
	if err != nil {
		return fmt.Errorf("invalid Confluence URL: %w", err)
	}

	// Create Confluence client
	client := confluence.NewClient(pageInfo.BaseURL, pageOpts.Email, pageOpts.APIKey)

	page, err := client.GetPage(pageInfo.PageID)
	if err != nil {
		return fmt.Errorf("failed to get page: %w", err)
	}

	// Create output directory
	if err := os.MkdirAll(pageOpts.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Use shared conversion pipeline
	result := convertSinglePage(
		page,
		pageInfo.BaseURL,
		pageOpts,
	)

	// Print results
	printConversionResult(result)

	if !result.Success {
		return fmt.Errorf("conversion failed: %v", result.Error)
	}

	return nil
}
