package commands

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gosimple/slug"
	"github.com/jackchuka/confluence-md/internal/confluence"
	confluenceModel "github.com/jackchuka/confluence-md/internal/confluence/model"
	"github.com/jackchuka/confluence-md/internal/converter"
)

// sanitizeFileName uses the mature gosimple/slug library for robust filename sanitization
func sanitizeFileName(name string) string {
	if name == "" {
		return "untitled"
	}

	sanitized := slug.MakeLang(name, "en")

	if sanitized == "" {
		return name
	}

	return sanitized
}

func buildOutputNamer(template string) (converter.OutputNamer, error) {
	if strings.TrimSpace(template) == "" {
		return nil, nil
	}

	namer, err := converter.NewTemplateOutputNamer(template)
	if err != nil {
		return nil, err
	}

	return namer, nil
}

// PageConversionResult represents the result of converting a single page
type PageConversionResult struct {
	OutputPath  string
	PageID      string
	Title       string
	ImagesCount int
	Success     bool
	Error       error
}

// convertSinglePage handles the full conversion pipeline for a single page
func convertSinglePage(client confluence.Client, page *confluenceModel.ConfluencePage, baseURL string, opts PageOptions) *PageConversionResult {
	return convertSinglePageWithPath(client, page, baseURL, "", opts)
}

// convertSinglePageWithPath handles conversion with a custom output path (for tree structure)
func convertSinglePageWithPath(client confluence.Client, page *confluenceModel.ConfluencePage, baseURL, outputPath string, opts PageOptions) *PageConversionResult {
	result := &PageConversionResult{
		PageID: page.ID,
		Title:  page.Title,
	}

	if outputPath == "" {
		fileName, err := converter.GenerateFileName(page, opts.OutputNamer)
		if err != nil {
			result.Error = fmt.Errorf("failed to generate output filename: %w", err)
			return result
		}
		outputPath = filepath.Join(opts.OutputDir, fileName)
	}
	result.OutputPath = outputPath

	// Create converter and convert page
	var options []converter.Option
	if opts.DownloadImages {
		options = append(options, converter.WithDownloadAttachments(opts.ImageFolder))
	}
	conv := converter.NewConverter(client, options...)
	doc, err := conv.ConvertPage(page, baseURL, filepath.Dir(outputPath))
	if err != nil {
		result.Error = fmt.Errorf("failed to convert page: %w", err)
		return result
	}
	result.ImagesCount = len(doc.Images)

	if err := converter.SaveMarkdownDocument(doc, outputPath, opts.IncludeMetadata); err != nil {
		result.Error = fmt.Errorf("failed to save document: %w", err)
		return result
	}

	result.Success = true
	return result
}

// printConversionResult prints the result of a page conversion in a consistent format
func printConversionResult(result *PageConversionResult) {
	if result.Success {
		fmt.Printf("✅ Successfully converted page: %s\n", result.OutputPath)
		fmt.Printf("   Page ID: %s\n", result.PageID)
		fmt.Printf("   Title: %s\n", result.Title)
		if result.ImagesCount > 0 {
			fmt.Printf("   📥 Images downloaded: %d\n", result.ImagesCount)
		}
	} else {
		fmt.Printf("❌ Failed to convert page: %s\n", result.Title)
		if result.Error != nil {
			fmt.Printf("   Error: %v\n", result.Error)
		}
	}
	fmt.Println()
}

func urlToPageInfo(pageURL string) (confluenceModel.PageURLInfo, error) {
	if pageURL == "" {
		return confluenceModel.PageURLInfo{}, fmt.Errorf("URL is empty")
	}

	u, err := url.Parse(pageURL)
	if err != nil {
		return confluenceModel.PageURLInfo{}, fmt.Errorf("invalid URL: %w", err)
	}

	baseURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	var pageID string
	var spaceKey string
	var title string

	// Extract page ID from path
	// Path format: /wiki/spaces/SPACE/pages/12345/Title
	parts := strings.Split(u.Path, "/")
	for i, part := range parts {
		if part == "spaces" && i+1 < len(parts) {
			spaceKey = parts[i+1]
		}
		if part == "pages" && i+1 < len(parts) {
			pageID = parts[i+1]
		}
		if i == len(parts)-1 {
			title = part
		}
	}

	if pageID == "" {
		return confluenceModel.PageURLInfo{}, fmt.Errorf("could not extract page ID from URL")
	}

	return confluenceModel.PageURLInfo{
		BaseURL:  baseURL,
		PageID:   pageID,
		SpaceKey: spaceKey,
		Title:    title,
	}, nil
}
