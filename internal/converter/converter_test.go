package converter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	confModel "github.com/jackchuka/confluence-md/internal/confluence/model"
	convModel "github.com/jackchuka/confluence-md/internal/converter/model"
	mock_attachments "github.com/jackchuka/confluence-md/internal/converter/plugin/attachments/mock"
	gomock "go.uber.org/mock/gomock"
)

func TestConverterConvertPage(t *testing.T) {
	conv := NewConverter(nil)

	page := &confModel.ConfluencePage{
		ID:       "123",
		Title:    "Sample Page",
		SpaceKey: "SPACE",
		Version:  1,
		Content: confModel.ConfluenceContent{
			Storage: confModel.ContentStorage{
				Value: "<p>Hello World</p><ac:image ri:filename=\"diagram.png\"></ac:image>",
			},
		},
		Metadata: confModel.ConfluenceMetadata{
			Labels: []confModel.Label{{Name: "Label"}},
		},
		CreatedBy: confModel.User{DisplayName: "Author"},
		UpdatedAt: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	page.Content.Storage.Representation = "storage"
	page.CreatedAt = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	page.UpdatedBy = confModel.User{DisplayName: "Editor"}

	tests := []struct {
		name    string
		page    *confModel.ConfluencePage
		wantErr string
	}{
		{
			name: "success",
			page: page,
		},
		{
			name:    "invalid page",
			page:    &confModel.ConfluencePage{Title: "Missing ID", Content: confModel.ConfluenceContent{Storage: confModel.ContentStorage{Value: "<p>content</p>"}}, SpaceKey: "SPACE"},
			wantErr: "page ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := conv.ConvertPage(tt.page, "https://example.atlassian.net", ".")
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
		})
	}
}

func TestConverterDownloadImages(t *testing.T) {
	data := []byte("image-bytes")
	attachment := &confModel.ConfluenceAttachment{Title: "diagram.png", MediaType: "image/png", FileSize: int64(len(data))}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockResolver := mock_attachments.NewMockResolver(ctrl)
	mockResolver.EXPECT().DownloadAttachment(gomock.Any(), "diagram.png", 0).Return(attachment, data, nil)

	conv := &Converter{
		imageFolder: "images",
		attachments: mockResolver,
	}

	doc := &convModel.MarkdownDocument{
		Images: []convModel.ImageRef{{
			FileName: "diagram.png",
		}},
	}

	page := &confModel.ConfluencePage{
		Attachments: []confModel.ConfluenceAttachment{{Title: "diagram.png"}},
	}

	tmpDir := t.TempDir()

	if err := conv.downloadImages(doc, page, tmpDir); err != nil {
		t.Fatalf("DownloadImages returned error: %v", err)
	}

	imagePath := filepath.Join(tmpDir, "images", "diagram.png")
	got, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatalf("failed to read downloaded image: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("unexpected image content: %q", string(got))
	}
	if doc.Images[0].ContentType != "image/png" {
		t.Fatalf("expected content type image/png, got %q", doc.Images[0].ContentType)
	}
	if doc.Images[0].Size != int64(len(data)) {
		t.Fatalf("expected size %d, got %d", len(data), doc.Images[0].Size)
	}
}

func TestSaveMarkdownDocument(t *testing.T) {
	tmpDir := t.TempDir()
	doc := &convModel.MarkdownDocument{
		Content: "body",
		Frontmatter: convModel.Frontmatter{
			Title:  "Title",
			Author: "Author",
			Date:   time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
			Confluence: convModel.ConfluenceRef{
				PageID:   "123",
				SpaceKey: "SPACE",
				Version:  1,
				URL:      "https://example.atlassian.net/wiki/pages/123",
			},
		},
	}

	plainPath := filepath.Join(tmpDir, "doc.md")
	if err := SaveMarkdownDocument(doc, plainPath, false); err != nil {
		t.Fatalf("SaveMarkdownDocument returned error: %v", err)
	}

	plainContent, err := os.ReadFile(plainPath)
	if err != nil {
		t.Fatalf("failed to read markdown file: %v", err)
	}
	if string(plainContent) != "body" {
		t.Fatalf("unexpected markdown content: %q", string(plainContent))
	}

	// Reset content and save with frontmatter
	doc.Content = "body"
	frontPath := filepath.Join(tmpDir, "doc-with-frontmatter.md")
	if err := SaveMarkdownDocument(doc, frontPath, true); err != nil {
		t.Fatalf("SaveMarkdownDocument with frontmatter returned error: %v", err)
	}

	frontContent, err := os.ReadFile(frontPath)
	if err != nil {
		t.Fatalf("failed to read frontmatter file: %v", err)
	}
	frontStr := string(frontContent)
	if !strings.HasPrefix(frontStr, "---\n") {
		t.Fatalf("expected frontmatter prefix, got %q", frontStr)
	}
	if !strings.Contains(frontStr, "title: \"Title\"") {
		t.Fatalf("expected title in frontmatter, got %q", frontStr)
	}
	if doc.Content != frontStr {
		t.Fatalf("expected document content updated with frontmatter")
	}
}

func TestConverterPostprocessMarkdown(t *testing.T) {
	conv := NewConverter(nil)

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
	conv := NewConverter(nil, nil)
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
