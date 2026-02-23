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
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

func (md *MarkdownDocument) WithFrontmatter() (string, error) {
	var builder strings.Builder

	// Write YAML frontmatter
	builder.WriteString("---\n")
	fmt.Fprintf(&builder, "title: %q\n", md.Frontmatter.Title)
	fmt.Fprintf(&builder, "author: %q\n", md.Frontmatter.Author)
	fmt.Fprintf(&builder, "date: %q\n", md.Frontmatter.Date.Format(time.RFC3339))

	if len(md.Frontmatter.Labels) > 0 {
		builder.WriteString("labels:\n")
		for _, label := range md.Frontmatter.Labels {
			fmt.Fprintf(&builder, "  - %q\n", label)
		}
	}

	// Confluence reference
	builder.WriteString("confluence:\n")
	fmt.Fprintf(&builder, "  pageId: %q\n", md.Frontmatter.Confluence.PageID)
	fmt.Fprintf(&builder, "  spaceKey: %q\n", md.Frontmatter.Confluence.SpaceKey)
	fmt.Fprintf(&builder, "  version: %d\n", md.Frontmatter.Confluence.Version)
	fmt.Fprintf(&builder, "  url: %q\n", md.Frontmatter.Confluence.URL)

	// Custom fields
	for key, value := range md.Frontmatter.Custom {
		fmt.Fprintf(&builder, "%s: %v\n", key, value)
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
