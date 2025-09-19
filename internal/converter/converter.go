package converter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/jackchuka/confluence-md/internal/confluence/client"
	confluenceModel "github.com/jackchuka/confluence-md/internal/confluence/model"
	"github.com/jackchuka/confluence-md/internal/converter/model"
	"github.com/jackchuka/confluence-md/internal/converter/plugin"
	"github.com/jackchuka/confluence-md/internal/converter/plugin/attachments"
)

const maxImageSizeBytes = 10 * 1024 * 1024

// Converter handles HTML to Markdown conversion
type Converter struct {
	mdConverter *converter.Converter
	plugin      *plugin.ConfluencePlugin
	attachments attachments.Resolver

	// options
	imageFolder string
}

type Option func(*Converter)

func WithDownloadAttachments(imageFolder string) Option {
	return func(c *Converter) {
		c.imageFolder = imageFolder
	}
}

// NewConverter creates a new HTML to Markdown converter
func NewConverter(client *client.Client, opts ...Option) *Converter {
	c := &Converter{}

	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}

	resolver := attachments.NewService(client)
	if c.imageFolder != "" {
		c.attachments = resolver
	}
	c.plugin = plugin.NewConfluencePlugin(resolver, c.imageFolder)
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
			// official table plugin doesn't handle complex cells well
			// table.NewTablePlugin(),
			c.plugin,
		),
	)
	c.mdConverter = conv

	return c
}

// ConvertPage converts a Confluence page to Markdown
func (c *Converter) ConvertPage(
	page *confluenceModel.ConfluencePage,
	baseURL string,
	outputDir string,
) (*model.MarkdownDocument, error) {
	if err := page.Validate(); err != nil {
		return nil, fmt.Errorf("invalid page: %w", err)
	}
	c.plugin.SetCurrentPage(page)

	// Create markdown document
	doc, err := model.NewMarkdownDocument(page, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create markdown document: %w", err)
	}

	htmlContent := page.Content.Storage.Value

	markdown, err := c.convertHtml(htmlContent)
	if err != nil {
		return nil, fmt.Errorf("failed to convert HTML to Markdown: %w", err)
	}
	doc.Content = markdown
	// Extract image references for downloading
	imageRefs := c.extractImageReferences(htmlContent, doc.Frontmatter.Confluence.PageID, baseURL)
	doc.Images = imageRefs

	if c.attachments != nil {
		if err := c.downloadImages(doc, page, outputDir); err != nil {
			return nil, fmt.Errorf("failed to download images: %w", err)
		}
	}

	return doc, nil
}

// downloadImages fetches referenced images via the attachment service and writes them to disk.
func (c *Converter) downloadImages(doc *model.MarkdownDocument, page *confluenceModel.ConfluencePage, outputDir string) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}

	if len(doc.Images) == 0 {
		return nil
	}

	if page == nil {
		return fmt.Errorf("page context is required to download images")
	}

	for i := range doc.Images {
		imageRef := &doc.Images[i]
		attachment, data, err := c.attachments.DownloadAttachment(page, imageRef.FileName, 0)
		if err != nil {
			return fmt.Errorf("failed to download image %s: %w", imageRef.FileName, err)
		}

		if attachment.FileSize > maxImageSizeBytes {
			return fmt.Errorf("image %s too large: %d bytes (max %d)", imageRef.FileName, attachment.FileSize, maxImageSizeBytes)
		}

		imageRef.ContentType = attachment.MediaType
		imageRef.Size = attachment.FileSize

		filePath := filepath.Join(outputDir, c.imageFolder, imageRef.FileName)
		fmt.Println("Downloading image:", imageRef.FileName, "to", filePath)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return fmt.Errorf("failed to create image directory: %w", err)
		}

		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return fmt.Errorf("failed to write image %s: %w", imageRef.FileName, err)
		}
	}

	return nil
}
