package converter

import (
	"strings"
	"testing"
	"time"

	"github.com/jackchuka/confluence-md/internal/models"
)

func TestConverterConvertPage(t *testing.T) {
	conv := NewConverter(nil, "images")

	page := &models.ConfluencePage{
		ID:       "123",
		Title:    "Sample Page",
		SpaceKey: "SPACE",
		Version:  1,
		Content: models.ConfluenceContent{
			Storage: models.ContentStorage{
				Value: "<p>Hello World</p><ac:image ri:filename=\"diagram.png\"></ac:image>",
			},
		},
		Metadata: models.ConfluenceMetadata{
			Labels: []models.Label{{Name: "Label"}},
		},
		CreatedBy: models.User{DisplayName: "Author"},
		UpdatedAt: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	page.Content.Storage.Representation = "storage"
	page.CreatedAt = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	page.UpdatedBy = models.User{DisplayName: "Editor"}

	tests := []struct {
		name    string
		page    *models.ConfluencePage
		wantErr string
	}{
		{
			name: "success",
			page: page,
		},
		{
			name:    "invalid page",
			page:    &models.ConfluencePage{Title: "Missing ID", Content: models.ConfluenceContent{Storage: models.ContentStorage{Value: "<p>content</p>"}}, SpaceKey: "SPACE"},
			wantErr: "page ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := conv.ConvertPage(tt.page, "https://example.atlassian.net")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				if doc != nil {
					t.Fatalf("expected nil doc, got %#v", doc)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if doc == nil {
				t.Fatal("expected document, got nil")
			}
			if !strings.Contains(doc.Content, "Hello World") {
				t.Fatalf("expected markdown content, got %q", doc.Content)
			}
			if len(doc.Images) != 1 {
				t.Fatalf("expected one image reference, got %d", len(doc.Images))
			}
			img := doc.Images[0]
			if img.FileName != "diagram.png" {
				t.Fatalf("unexpected image name %q", img.FileName)
			}
			wantURL := "https://example.atlassian.net/wiki/download/attachments/123/diagram.png"
			if img.OriginalURL != wantURL {
				t.Fatalf("unexpected original URL %q want %q", img.OriginalURL, wantURL)
			}
			if img.LocalPath != "images/diagram.png" {
				t.Fatalf("unexpected local path %q", img.LocalPath)
			}
		})
	}
}

func TestConverterPostprocessMarkdown(t *testing.T) {
	conv := NewConverter(nil, "images")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "collapse blank lines",
			input: "line1\n\n\nline2",
			want:  "line1\n\nline2",
		},
		{
			name:  "trim whitespace",
			input: "  content  \n\n",
			want:  "content",
		},
		{
			name:  "fix nested list spacing",
			input: "\n- item\n\n  - nested\n",
			want:  "- item\n  - nested",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := conv.postprocessMarkdown(tt.input)
			if got != tt.want {
				t.Fatalf("postprocessMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestConverterPreprocessCDATA(t *testing.T) {
	conv := NewConverter(nil, "images")
	input := "<![CDATA[<tag>&value]]>"
	got := conv.preprocessCDATA(input)
	if !strings.Contains(got, "<pre data-cdata='true'>") {
		t.Fatalf("expected pre block, got %q", got)
	}
	if strings.Contains(got, "<![CDATA[") {
		t.Fatalf("expected cdata markers removed, got %q", got)
	}
	if !strings.Contains(got, "&lt;tag&gt;") {
		t.Fatalf("expected html escaped content, got %q", got)
	}
}

func TestExtractorImageReferences(t *testing.T) {
	conv := NewConverter(nil, "assets")
	html := `<ac:image ri:filename="one.png"></ac:image><ac:image>missing</ac:image><ac:image ri:filename="two space.png"></ac:image>`

	images := conv.extractImageReferences(html, "123", "https://example.atlassian.net")
	if len(images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(images))
	}

	if images[0].OriginalURL != "https://example.atlassian.net/wiki/download/attachments/123/one.png" {
		t.Fatalf("unexpected original url: %q", images[0].OriginalURL)
	}
	if images[1].OriginalURL != "https://example.atlassian.net/wiki/download/attachments/123/two+space.png" {
		t.Fatalf("unexpected encoded url: %q", images[1].OriginalURL)
	}
	if images[1].LocalPath != "assets/two space.png" {
		t.Fatalf("unexpected local path: %q", images[1].LocalPath)
	}
}

func TestFixMarkdownLinks(t *testing.T) {
	input := "See [Page](/wiki/spaces/SPACE/pages/12345/Some-Page) for details"
	want := "See [Page](confluence://pageId/12345) for details"
	if got := fixMarkdownLinks(input); got != want {
		t.Fatalf("fixMarkdownLinks(%q) = %q, want %q", input, got, want)
	}
}

func TestFixNestedListSpacing(t *testing.T) {
	input := "\n- Item\n\n  - Nested\n\n    - Deep"
	want := "\n- Item\n  - Nested\n    - Deep"
	if got := fixNestedListSpacing(input); got != want {
		t.Fatalf("fixNestedListSpacing(%q) = %q, want %q", input, got, want)
	}
}
