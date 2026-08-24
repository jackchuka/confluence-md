package converter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/jackchuka/confluence-md/internal/confluence"
	confluenceModel "github.com/jackchuka/confluence-md/internal/confluence/model"
	"github.com/jackchuka/confluence-md/internal/converter/model"
	"github.com/jackchuka/confluence-md/internal/converter/plugin"
	"github.com/jackchuka/confluence-md/internal/converter/plugin/attachments"
)

const maxImageSizeBytes = 10 * 1024 * 1024
const maxFileSizeBytes = 100 * 1024 * 1024

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
func NewConverter(client confluence.Client, opts ...Option) *Converter {
	c := &Converter{}

	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}

	var resolver attachments.Resolver
	if client != nil {
		resolver = attachments.NewService(client)
		if c.imageFolder != "" {
			c.attachments = resolver
		}
		// Use the client-aware plugin constructor for user resolution
		c.plugin = plugin.NewConfluencePluginWithClient(client, resolver, c.imageFolder)
	} else {
		// Use the basic plugin constructor when no client available
		c.plugin = plugin.NewConfluencePlugin(resolver, c.imageFolder)
	}
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

// ConvertHTML converts raw HTML string to Markdown
func (c *Converter) ConvertHTML(html string) (string, error) {
	return c.convertHtml(html)
}

// ConvertPage converts a Confluence page to Markdown
func (c *Converter) ConvertPage(
	page *confluenceModel.ConfluencePage,
	site confluenceModel.SiteInfo,
	outputDir string,
) (*model.MarkdownDocument, error) {
	if err := page.Validate(); err != nil {
		return nil, fmt.Errorf("invalid page: %w", err)
	}
	c.plugin.SetSite(site)
	c.plugin.SetCurrentPage(page)

	// Create markdown document
	doc, err := model.NewMarkdownDocument(page, site)
	if err != nil {
		return nil, fmt.Errorf("failed to create markdown document: %w", err)
	}

	htmlContent := page.Content.Storage.Value

	markdown, err := c.convertHtml(htmlContent)
	if err != nil {
		return nil, fmt.Errorf("failed to convert HTML to Markdown: %w", err)
	}
	doc.Content = markdown
	// Extract image and file references for downloading
	doc.Images = c.extractImageReferences(htmlContent, doc.Frontmatter.Confluence.PageID, site)
	doc.Files = c.extractFileReferences(htmlContent, doc.Frontmatter.Confluence.PageID, site)

	if c.attachments != nil {
		if err := c.downloadImages(doc, page, outputDir); err != nil {
			return nil, fmt.Errorf("failed to download images: %w", err)
		}
		if err := c.downloadFiles(doc, page, outputDir); err != nil {
			return nil, fmt.Errorf("failed to download files: %w", err)
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
	return c.downloadRefs(doc.Images, page, outputDir, maxImageSizeBytes, "image")
}

// downloadFiles fetches non-image attachments (view-file macros) and writes them to disk.
func (c *Converter) downloadFiles(doc *model.MarkdownDocument, page *confluenceModel.ConfluencePage, outputDir string) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}
	if len(doc.Files) == 0 {
		return nil
	}
	if page == nil {
		return fmt.Errorf("page context is required to download files")
	}
	return c.downloadRefs(doc.Files, page, outputDir, maxFileSizeBytes, "file")
}

// downloadRefs downloads a set of attachment references into the image folder,
// enforcing a per-item size cap (maxSize <= 0 disables the cap). kind labels
// the item in log and error messages ("image" or "file").
func (c *Converter) downloadRefs(refs []model.ImageRef, page *confluenceModel.ConfluencePage, outputDir string, maxSize int64, kind string) error {
	for i := range refs {
		ref := &refs[i]
		attachment, data, err := c.attachments.DownloadAttachment(page, ref.FileName, 0)
		if err != nil {
			// A single missing/undownloadable attachment must not abort the
			// whole page (common for stale references to deleted files); warn
			// and keep the markdown reference pointing at the expected path.
			fmt.Printf("⚠️  Warning: skipping %s %s: %v\n", kind, ref.FileName, err)
			continue
		}

		if maxSize > 0 && attachment.FileSize > maxSize {
			fmt.Printf("⚠️  Warning: skipping %s %s: too large (%d bytes, max %d)\n", kind, ref.FileName, attachment.FileSize, maxSize)
			continue
		}

		ref.ContentType = attachment.MediaType
		ref.Size = attachment.FileSize

		filePath := filepath.Join(outputDir, c.imageFolder, ref.FileName)
		fmt.Printf("Downloading %s: %s to %s\n", kind, ref.FileName, filePath)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", kind, err)
		}

		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s %s: %w", kind, ref.FileName, err)
		}
	}

	return nil
}
