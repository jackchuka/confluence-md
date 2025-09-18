package converter

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/jackchuka/confluence-md/internal/attachments"
	"github.com/jackchuka/confluence-md/internal/models"
)

// Converter handles HTML to Markdown conversion
type Converter struct {
	mdConverter *converter.Converter
	imageFolder string
	plugin      *confluencePlugin
}

// NewConverter creates a new HTML to Markdown converter
func NewConverter(resolver attachments.Resolver, imageFolder string) *Converter {
	plugin := NewConfluencePlugin(resolver, imageFolder)
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
			// official table plugin doesn't handle complex cells well
			// table.NewTablePlugin(),
			plugin,
		),
	)

	return &Converter{
		mdConverter: conv,
		imageFolder: imageFolder,
		plugin:      plugin,
	}
}

// ConvertHtml converts HTML string to Markdown (for testing)
func (c *Converter) ConvertHtml(html string) (string, error) {
	// Preprocess CDATA content before HTML parsing strips it
	processedHTML := c.preprocessCDATA(html)

	md, err := c.mdConverter.ConvertString(processedHTML)
	if err != nil {
		fmt.Printf("Conversion error: %v\n", err)
	}
	return c.postprocessMarkdown(md), nil
}

// ConvertPage converts a Confluence page to Markdown
func (c *Converter) ConvertPage(page *models.ConfluencePage, baseURL string) (*models.MarkdownDocument, error) {
	if err := page.Validate(); err != nil {
		return nil, fmt.Errorf("invalid page: %w", err)
	}

	if c.plugin != nil {
		c.plugin.SetCurrentPage(page)
	}

	// Create markdown document
	doc, err := models.NewMarkdownDocument(page, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create markdown document: %w", err)
	}

	htmlContent := page.Content.Storage.Value

	markdown, err := c.ConvertHtml(htmlContent)
	if err != nil {
		return nil, fmt.Errorf("failed to convert HTML to Markdown: %w", err)
	}
	doc.Content = markdown
	// Extract image references for downloading
	imageRefs := c.extractImageReferences(htmlContent, doc.Frontmatter.Confluence.PageID, baseURL)
	doc.Images = imageRefs

	return doc, nil
}

// postprocessMarkdown cleans up the converted Markdown
func (c *Converter) postprocessMarkdown(markdown string) string {
	// Remove excessive whitespace
	markdown = regexp.MustCompile(`\n{3,}`).ReplaceAllString(markdown, "\n\n")

	// Fix nested list spacing - html-to-markdown still adds extra blank lines for nested lists
	markdown = fixNestedListSpacing(markdown)

	// Fix link formatting
	markdown = fixMarkdownLinks(markdown)

	// Trim whitespace
	markdown = strings.TrimSpace(markdown)

	return markdown
}

// extractImageReferences finds all images in the HTML and creates ImageRef objects
func (c *Converter) extractImageReferences(html, pageID, baseURL string) []models.ImageRef {
	var imageRefs []models.ImageRef

	// Find all Confluence ac:image elements
	acImageRegex := regexp.MustCompile(`<ac:image[^>]*>[\s\S]*?</ac:image>`)
	matches := acImageRegex.FindAllString(html, -1)

	for _, imageHTML := range matches {
		fileName := parseConfluenceImage(imageHTML)

		if fileName == "" {
			continue
		}

		// Build Confluence attachment download URL
		// Format: {baseURL}/wiki/download/attachments/{pageId}/{encodedFilename}
		encodedFilename := url.QueryEscape(fileName)
		actualURL := fmt.Sprintf("%s/wiki/download/attachments/%s/%s",
			strings.TrimSuffix(baseURL, "/"), pageID, encodedFilename)

		localPath := c.imageFolder + "/" + fileName

		imageRef := models.ImageRef{
			OriginalURL: actualURL,
			LocalPath:   localPath,
			FileName:    fileName,
		}

		imageRefs = append(imageRefs, imageRef)
	}

	return imageRefs
}

// fixMarkdownLinks improves link formatting
func fixMarkdownLinks(markdown string) string {
	// Fix Confluence internal links
	confLinkRegex := regexp.MustCompile(`\[([^\]]+)\]\(/wiki/spaces/([^/]+)/pages/(\d+)/[^)]+\)`)
	markdown = confLinkRegex.ReplaceAllString(markdown, "[$1](confluence://pageId/$3)")

	return markdown
}

// fixNestedListSpacing removes extra blank lines in nested lists recursively
func fixNestedListSpacing(markdown string) string {
	listMarker := `(?:[-*+]\s|\d+\.\s)`
	pattern := regexp.MustCompile(`(\n\s*` + listMarker + `[^\n]*)\n\s*\n(\s{2,}` + listMarker + `)`)
	result := pattern.ReplaceAllString(markdown, "$1\n$2")
	// Recursively fix until no more changes
	if result != markdown {
		return fixNestedListSpacing(result)
	}
	return result
}

// preprocessCDATA converts CDATA sections to preserve content through HTML parsing
func (c *Converter) preprocessCDATA(html string) string {
	// Replace CDATA sections with properly wrapped content
	cdataRegex := regexp.MustCompile(`<!\[CDATA\[([\s\S]*?)\]\]>`)
	return cdataRegex.ReplaceAllStringFunc(html, func(match string) string {
		// Extract content between CDATA markers
		if submatch := cdataRegex.FindStringSubmatch(match); len(submatch) > 1 {
			// Preserve whitespace by wrapping in <pre> and escape HTML special characters
			content := submatch[1]
			content = strings.ReplaceAll(content, "&", "&amp;")
			content = strings.ReplaceAll(content, "<", "&lt;")
			content = strings.ReplaceAll(content, ">", "&gt;")
			// Wrap in <pre> tag to preserve whitespace and indicate this is preformatted content
			return fmt.Sprintf("<pre data-cdata='true'>%s</pre>", content)
		}
		return match
	})
}
