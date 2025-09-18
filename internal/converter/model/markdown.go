package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/jackchuka/confluence-md/internal/confluence/model"
)

// MarkdownDocument represents the output document structure
type MarkdownDocument struct {
	Frontmatter Frontmatter `yaml:",inline"`
	Content     string      `yaml:"-"`
	Images      []ImageRef  `yaml:"-"`
}

// Frontmatter represents YAML frontmatter for the Markdown document
type Frontmatter struct {
	Title      string         `yaml:"title"`
	Author     string         `yaml:"author"`
	Date       time.Time      `yaml:"date"`
	Labels     []string       `yaml:"labels,omitempty"`
	Confluence ConfluenceRef  `yaml:"confluence"`
	Custom     map[string]any `yaml:",inline,omitempty"`
}

// ConfluenceRef contains reference information back to the original Confluence page
type ConfluenceRef struct {
	PageID   string `yaml:"pageId"`
	SpaceKey string `yaml:"spaceKey"`
	Version  int    `yaml:"version"`
	URL      string `yaml:"url"`
}

// ImageRef represents a reference to a downloaded image
type ImageRef struct {
	OriginalURL string `json:"originalUrl"`
	LocalPath   string `json:"localPath"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

func (md *MarkdownDocument) WithFrontmatter() (string, error) {
	var builder strings.Builder

	// Write YAML frontmatter
	builder.WriteString("---\n")
	builder.WriteString(fmt.Sprintf("title: %q\n", md.Frontmatter.Title))
	builder.WriteString(fmt.Sprintf("author: %q\n", md.Frontmatter.Author))
	builder.WriteString(fmt.Sprintf("date: %q\n", md.Frontmatter.Date.Format(time.RFC3339)))

	if len(md.Frontmatter.Labels) > 0 {
		builder.WriteString("labels:\n")
		for _, label := range md.Frontmatter.Labels {
			builder.WriteString(fmt.Sprintf("  - %q\n", label))
		}
	}

	// Confluence reference
	builder.WriteString("confluence:\n")
	builder.WriteString(fmt.Sprintf("  pageId: %q\n", md.Frontmatter.Confluence.PageID))
	builder.WriteString(fmt.Sprintf("  spaceKey: %q\n", md.Frontmatter.Confluence.SpaceKey))
	builder.WriteString(fmt.Sprintf("  version: %d\n", md.Frontmatter.Confluence.Version))
	builder.WriteString(fmt.Sprintf("  url: %q\n", md.Frontmatter.Confluence.URL))

	// Custom fields
	for key, value := range md.Frontmatter.Custom {
		builder.WriteString(fmt.Sprintf("%s: %v\n", key, value))
	}

	builder.WriteString("---\n\n")

	// Write main content
	builder.WriteString(md.Content)

	return builder.String(), nil
}

// NewMarkdownDocument creates a new MarkdownDocument from a ConfluencePage
func NewMarkdownDocument(page *model.ConfluencePage, baseURL string) (*MarkdownDocument, error) {
	pageURL, err := page.GetURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate page URL: %w", err)
	}

	doc := &MarkdownDocument{
		Frontmatter: Frontmatter{
			Title:  page.Title,
			Author: page.CreatedBy.DisplayName,
			Date:   page.UpdatedAt,
			Labels: page.GetLabelNames(),
			Confluence: ConfluenceRef{
				PageID:   page.ID,
				SpaceKey: page.SpaceKey,
				Version:  page.Version,
				URL:      pageURL,
			},
		},
		Content: "", // Will be filled by converter
		Images:  []ImageRef{},
	}

	return doc, nil
}
