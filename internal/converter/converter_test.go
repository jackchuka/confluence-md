package converter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mock_confluence "github.com/jackchuka/confluence-md/internal/confluence/mock"
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
			site := confModel.SiteInfo{BaseURL: "https://example.atlassian.net", Deployment: confModel.DeploymentCloud, ContextPath: "/wiki"}
			doc, err := conv.ConvertPage(tt.page, site, ".")
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

func TestConverterConvertPageLinks(t *testing.T) {
	conv := NewConverter(nil)

	page := &confModel.ConfluencePage{
		ID:       "100",
		Title:    "Source Page",
		SpaceKey: "MKT",
		Version:  1,
		Content: confModel.ConfluenceContent{
			Storage: confModel.ContentStorage{
				// 1) title-only page link (same space, no body) — used to vanish
				// 2) page link with an explicit link body
				// 3) cross-space page link
				Value: `<p>viz <ac:link><ri:page ri:content-title="UC0075 - Notifikace" /></ac:link>.</p>` +
					`<p>detail <ac:link><ri:page ri:content-title="EN0012 - Administrátor" /><ac:link-body>Administrátor</ac:link-body></ac:link>.</p>` +
					`<p>jinde <ac:link><ri:page ri:content-title="Portál EDC" ri:space-key="OPS" /></ac:link>.</p>`,
			},
		},
	}
	page.Content.Storage.Representation = "storage"

	site := confModel.SiteInfo{BaseURL: "https://dory.eon.cz", Deployment: confModel.DeploymentServer, ContextPath: ""}
	doc, err := conv.ConvertPage(page, site, ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wants := []string{
		// title-only link resolves to text + pretty display URL (no longer "viz .")
		"[UC0075 - Notifikace](https://dory.eon.cz/display/MKT/UC0075+-+Notifikace)",
		// explicit body wins as the link text; falls back to current page's space
		"[Administrátor](https://dory.eon.cz/display/MKT/EN0012+-+Administr%C3%A1tor)",
		// explicit space key is honored
		"[Portál EDC](https://dory.eon.cz/display/OPS/Port%C3%A1l+EDC)",
	}
	for _, want := range wants {
		if !strings.Contains(doc.Content, want) {
			t.Errorf("expected markdown to contain %q\n got: %s", want, doc.Content)
		}
	}
	if strings.Contains(doc.Content, "viz .") {
		t.Errorf("page link was dropped (found \"viz .\"): %s", doc.Content)
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

func TestConverterViewFileMacro(t *testing.T) {
	pdf := []byte("%PDF-1.7 fake bytes")
	attachment := &confModel.ConfluenceAttachment{Title: "Plna_moc.pdf", MediaType: "application/pdf", FileSize: int64(len(pdf))}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockResolver := mock_attachments.NewMockResolver(ctrl)
	mockResolver.EXPECT().DownloadAttachment(gomock.Any(), "Plna_moc.pdf", 0).Return(attachment, pdf, nil)

	conv := NewConverter(nil, WithDownloadAttachments("assets"))
	conv.attachments = mockResolver

	page := &confModel.ConfluencePage{
		ID:       "100",
		Title:    "TMPL001 v01 - Plná Moc",
		SpaceKey: "MKT",
		Version:  1,
		Content: confModel.ConfluenceContent{
			Storage: confModel.ContentStorage{
				Value: `<p>Soubor:</p><ac:structured-macro ac:name="view-file" ac:schema-version="1">` +
					`<ac:parameter ac:name="name"><ri:attachment ri:filename="Plna_moc.pdf" /></ac:parameter>` +
					`</ac:structured-macro>`,
			},
		},
		Attachments: []confModel.ConfluenceAttachment{{
			ID:           "att1",
			Title:        "Plna_moc.pdf",
			MediaType:    "application/pdf",
			FileSize:     int64(len(pdf)),
			DownloadLink: "/download/attachments/100/Plna_moc.pdf",
		}},
	}
	page.Content.Storage.Representation = "storage"

	site := confModel.SiteInfo{BaseURL: "https://dory.eon.cz", Deployment: confModel.DeploymentServer}
	tmpDir := t.TempDir()
	doc, err := conv.ConvertPage(page, site, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The macro must become a markdown link to the local file (not an "Unsupported macro" comment).
	wantLink := "[Plna_moc.pdf](assets%2FPlna_moc.pdf)"
	if !strings.Contains(doc.Content, wantLink) {
		t.Errorf("expected link %q, got: %s", wantLink, doc.Content)
	}
	if strings.Contains(doc.Content, "Unsupported macro") {
		t.Errorf("view-file macro left unsupported: %s", doc.Content)
	}

	// The attachment must have been downloaded to disk.
	got, err := os.ReadFile(filepath.Join(tmpDir, "assets", "Plna_moc.pdf"))
	if err != nil {
		t.Fatalf("expected downloaded file: %v", err)
	}
	if string(got) != string(pdf) {
		t.Errorf("unexpected file content: %q", string(got))
	}
}

func TestConverterContentByLabelAndJiraMacros(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock_confluence.NewMockClient(ctrl)
	// user cache lookups during SetCurrentPage may call GetUser; allow any.
	mockClient.EXPECT().GetUser(gomock.Any()).Return(nil, fmt.Errorf("n/a")).AnyTimes()
	mockClient.EXPECT().
		SearchByCQL(`label = "ep0006" and ancestor = "194514542"`, 100).
		Return([]*confModel.ConfluencePage{
			{ID: "1", Title: "US0010 - Import CSV", SpaceKey: "MKT"},
			{ID: "2", Title: "US0011 - Kontrola dat", SpaceKey: "MKT"},
		}, nil)

	conv := NewConverter(mockClient)

	page := &confModel.ConfluencePage{
		ID:       "100",
		Title:    "EP0006 - Manuální import",
		SpaceKey: "MKT",
		Version:  1,
		Content: confModel.ConfluenceContent{
			Storage: confModel.ContentStorage{
				Value: `<p><ac:structured-macro ac:name="jira"><ac:parameter ac:name="key">MAR-42</ac:parameter></ac:structured-macro></p>` +
					`<h1>Související User Stories</h1>` +
					`<ac:structured-macro ac:name="contentbylabel"><ac:parameter ac:name="cql">label = "ep0006" and ancestor = "194514542"</ac:parameter></ac:structured-macro>`,
			},
		},
	}
	page.Content.Storage.Representation = "storage"

	site := confModel.SiteInfo{BaseURL: "https://dory.eon.cz", Deployment: confModel.DeploymentServer}
	doc, err := conv.ConvertPage(page, site, ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wants := []string{
		"MAR-42",
		"- [US0010 - Import CSV](https://dory.eon.cz/display/MKT/US0010+-+Import+CSV)",
		"- [US0011 - Kontrola dat](https://dory.eon.cz/display/MKT/US0011+-+Kontrola+dat)",
	}
	for _, want := range wants {
		if !strings.Contains(doc.Content, want) {
			t.Errorf("expected %q in output\n got: %s", want, doc.Content)
		}
	}
	if strings.Contains(doc.Content, "Unsupported macro") {
		t.Errorf("a macro was left unsupported: %s", doc.Content)
	}
}

func TestConverterChildrenMacro(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock_confluence.NewMockClient(ctrl)
	mockClient.EXPECT().GetUser(gomock.Any()).Return(nil, fmt.Errorf("n/a")).AnyTimes()
	mockClient.EXPECT().GetChildPages("100").Return([]*confModel.ConfluencePage{
		{ID: "2", Title: "Portál EDC", SpaceKey: "MKT"},
		{ID: "3", Title: "Dodavatel MamaAI", SpaceKey: "MKT"},
	}, nil)

	conv := NewConverter(mockClient)

	page := &confModel.ConfluencePage{
		ID:       "100",
		Title:    "Externí komponenty systému",
		SpaceKey: "MKT",
		Version:  1,
		Content: confModel.ConfluenceContent{
			Storage: confModel.ContentStorage{
				Value: `<ac:structured-macro ac:name="children"></ac:structured-macro><p>WIP architektura</p>`,
			},
		},
	}
	page.Content.Storage.Representation = "storage"

	site := confModel.SiteInfo{BaseURL: "https://dory.eon.cz", Deployment: confModel.DeploymentServer}
	doc, err := conv.ConvertPage(page, site, ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wants := []string{
		"<!-- Child Pages -->",
		"- [Portál EDC](https://dory.eon.cz/display/MKT/Port%C3%A1l+EDC)",
		"- [Dodavatel MamaAI](https://dory.eon.cz/display/MKT/Dodavatel+MamaAI)",
		"<!-- /Child Pages -->",
		"WIP architektura",
	}
	for _, want := range wants {
		if !strings.Contains(doc.Content, want) {
			t.Errorf("expected %q in output\n got: %s", want, doc.Content)
		}
	}
}

func TestConverterMarkdownMacro(t *testing.T) {
	conv := NewConverter(nil)

	page := &confModel.ConfluencePage{
		ID:       "100",
		Title:    "Navržené řešení",
		SpaceKey: "MKT",
		Version:  1,
		Content: confModel.ConfluenceContent{
			Storage: confModel.ContentStorage{
				Value: `<p>Úvod.</p>` +
					`<ac:structured-macro ac:name="markdown" ac:schema-version="1">` +
					`<ac:plain-text-body><![CDATA[## Marginální skóre

Vzorec: ` + "`score = a & b`" + ` kde ` + "`a < b`" + `

- první bod
- druhý bod

| Sloupec | Hodnota |
|---|---|
| x | 1 |
]]></ac:plain-text-body></ac:structured-macro>`,
			},
		},
	}
	page.Content.Storage.Representation = "storage"

	site := confModel.SiteInfo{BaseURL: "https://dory.eon.cz", Deployment: confModel.DeploymentServer}
	doc, err := conv.ConvertPage(page, site, ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The macro body is already Markdown: it must survive verbatim, not be
	// fenced as code, escaped, or dropped.
	wants := []string{
		"## Marginální skóre",
		"- první bod",
		"| Sloupec | Hodnota |",
		"`score = a & b`", // entities decoded back
		"`a < b`",
	}
	for _, want := range wants {
		if !strings.Contains(doc.Content, want) {
			t.Errorf("expected %q in output\n got: %s", want, doc.Content)
		}
	}
	if strings.Contains(doc.Content, "Unsupported macro") {
		t.Errorf("markdown macro left unsupported: %s", doc.Content)
	}
	if strings.Contains(doc.Content, "```") {
		t.Errorf("markdown body must not be fenced as code: %s", doc.Content)
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
