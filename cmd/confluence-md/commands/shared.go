package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gosimple/slug"
	"github.com/jackchuka/confluence-md/internal/converter"
	"github.com/jackchuka/confluence-md/internal/downloader"
	"github.com/jackchuka/confluence-md/internal/models"
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

func downloadImages(doc *models.MarkdownDocument, email, apiToken string, outputDir string) error {
	if len(doc.Images) == 0 {
		return nil
	}

	imageDownloader := downloader.NewDownloader(email, apiToken)
	if err := imageDownloader.DownloadImages(doc, outputDir); err != nil {
		fmt.Printf("⚠️  Warning: Failed to download images: %v\n", err)
		return err
	}
	return nil
}

// saveMarkdownDocumentWithOptions saves a markdown document with configurable frontmatter
func saveMarkdownDocumentWithOptions(doc *models.MarkdownDocument, outputPath string, includeFrontmatter bool) error {
	// Create directory if needed
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	// Write frontmatter if title exists and enabled
	if includeFrontmatter {
		doc.Content, err = doc.WithFrontmatter()
		if err != nil {
			return fmt.Errorf("failed to convert document to markdown: %w", err)
		}
	}

	// Write content
	_, _ = file.WriteString(doc.Content)

	return nil
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
func convertSinglePage(page *models.ConfluencePage, baseURL string, opts ConvertOptions) *PageConversionResult {
	return convertSinglePageWithPath(page, baseURL, "", opts)
}

// convertSinglePageWithPath handles conversion with a custom output path (for tree structure)
func convertSinglePageWithPath(page *models.ConfluencePage, baseURL, customOutputPath string, opts ConvertOptions) *PageConversionResult {
	result := &PageConversionResult{
		PageID: page.ID,
		Title:  page.Title,
	}

	// Create converter and convert page
	conv := converter.NewConverter(opts.ImageFolder)
	doc, err := conv.ConvertPage(page, baseURL)
	if err != nil {
		result.Error = fmt.Errorf("failed to convert page: %w", err)
		return result
	}

	// Generate output path
	var outputPath string
	if customOutputPath != "" {
		outputPath = customOutputPath
	} else {
		outputFileName := sanitizeFileName(page.Title) + ".md"
		outputPath = filepath.Join(opts.OutputDir, outputFileName)
	}
	result.OutputPath = outputPath

	// Determine image directory (same directory as the markdown file for tree, outputDir for convert)
	imageDir := opts.OutputDir
	if customOutputPath != "" {
		imageDir = filepath.Dir(outputPath)
	}

	// Download images
	if opts.DownloadImages {
		if err := downloadImages(doc, opts.Email, opts.APIKey, imageDir); err != nil {
			result.Error = fmt.Errorf("failed to download images: %w", err)
		}
		result.ImagesCount = len(doc.Images)
	}

	// Save document
	if err := saveMarkdownDocumentWithOptions(doc, outputPath, opts.IncludeMetadata); err != nil {
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
