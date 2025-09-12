package commands

import (
	"fmt"
	"os"

	"github.com/jackchuka/confluence-md/internal/confluence"
	"github.com/spf13/cobra"
)

// convertCmd represents the convert command
var convertCmd = &cobra.Command{
	Use:     "convert",
	Aliases: []string{"c"},
	Short:   "Convert a single Confluence page to Markdown",
	Long: `Convert a single Confluence page to Markdown format.

Provide the page URL and your API token to download and convert the page.
The converted content is saved to an output directory with images in an assets folder.

Examples:
  # Convert to default output directory
  confluence-md convert https://example.atlassian.net/wiki/spaces/SPACE/pages/12345/Title
  
  # Convert to custom directory
  confluence-md convert https://example.atlassian.net/wiki/spaces/SPACE/pages/12345/Title --output ./docs
  
  # Convert without downloading images
  confluence-md convert https://example.atlassian.net/wiki/spaces/SPACE/pages/12345/Title --download-images=false`,

	RunE: func(cmd *cobra.Command, args []string) error {
		return runConvert(cmd, args)
	},
}

var convertOpts ConvertOptions

type ConvertOptions struct {
	authOptions
	commonOptions
}

func init() {
	rootCmd.AddCommand(convertCmd)

	convertOpts.authOptions.InitFlags(convertCmd)
	convertOpts.commonOptions.InitFlags(convertCmd)

	// Required flags
	_ = convertCmd.MarkFlagRequired("api-token")
	_ = convertCmd.MarkFlagRequired("email")
}

func runConvert(_ *cobra.Command, args []string) error {
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
	client := confluence.NewClient(pageInfo.BaseURL, convertOpts.Email, convertOpts.APIKey)

	page, err := client.GetPage(pageInfo.PageID)
	if err != nil {
		return fmt.Errorf("failed to get page: %w", err)
	}

	// Create output directory
	if err := os.MkdirAll(convertOpts.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Use shared conversion pipeline
	result := convertSinglePage(
		page,
		pageInfo.BaseURL,
		convertOpts,
	)

	// Print results
	printConversionResult(result)

	if !result.Success {
		return fmt.Errorf("conversion failed: %v", result.Error)
	}

	return nil
}
