package converter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackchuka/confluence-md/internal/converter/model"
)

// SaveMarkdownDocument writes the markdown document to disk with optional frontmatter.
func SaveMarkdownDocument(doc *model.MarkdownDocument, outputPath string, withFrontmatter bool) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	content := doc.Content
	if withFrontmatter {
		rendered, err := doc.WithFrontmatter()
		if err != nil {
			return fmt.Errorf("failed to convert document to markdown: %w", err)
		}
		content = rendered
		doc.Content = rendered
	}

	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write markdown file: %w", err)
	}

	return nil
}
