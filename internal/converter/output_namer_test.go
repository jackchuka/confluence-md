package converter

import (
	"testing"
	"time"

	confluenceModel "github.com/jackchuka/confluence-md/internal/confluence/model"
)

func TestGenerateFileName_Default(t *testing.T) {
	page := &confluenceModel.ConfluencePage{Title: "Sample Page"}

	name, err := GenerateFileName(page, nil)
	if err != nil {
		t.Fatalf("GenerateFileName returned error: %v", err)
	}
	if name != "sample-page.md" {
		t.Fatalf("expected sample-page.md, got %q", name)
	}
}

func TestGenerateFileName_Template(t *testing.T) {
	template := "{{ .Page.UpdatedAt.Format \"2006-01-02\" }}-{{ .SlugTitle }}"
	namer, err := NewTemplateOutputNamer(template)
	if err != nil {
		t.Fatalf("NewTemplateOutputNamer returned error: %v", err)
	}

	page := &confluenceModel.ConfluencePage{
		Title:     "Release Notes",
		UpdatedAt: time.Date(2024, 9, 12, 10, 0, 0, 0, time.UTC),
	}

	name, err := GenerateFileName(page, namer)
	if err != nil {
		t.Fatalf("GenerateFileName returned error: %v", err)
	}

	if name != "2024-09-12-release-notes.md" {
		t.Fatalf("expected 2024-09-12-release-notes.md, got %q", name)
	}
}

func TestGenerateFileName_TemplateAddsExtension(t *testing.T) {
	namer, err := NewTemplateOutputNamer("{{ .SlugTitle }}")
	if err != nil {
		t.Fatalf("NewTemplateOutputNamer returned error: %v", err)
	}

	page := &confluenceModel.ConfluencePage{Title: "Docs"}

	name, err := GenerateFileName(page, namer)
	if err != nil {
		t.Fatalf("GenerateFileName returned error: %v", err)
	}

	if name != "docs.md" {
		t.Fatalf("expected docs.md, got %q", name)
	}
}

func TestGenerateFileName_TemplateLabelNames(t *testing.T) {
	page := &confluenceModel.ConfluencePage{Title: "Docs"}
	page.Metadata.Labels = []confluenceModel.Label{{Name: "howto"}, {Name: "api"}}

	tests := map[string]string{
		`{{ range .LabelNames }}{{ . }}-{{ end }}{{ .SlugTitle }}`: "howto-api-docs.md",
		`{{ join .LabelNames "_" }}-{{ .SlugTitle }}`:              "howto_api-docs.md",
	}
	for tmpl, want := range tests {
		namer, err := NewTemplateOutputNamer(tmpl)
		if err != nil {
			t.Fatalf("NewTemplateOutputNamer(%q) returned error: %v", tmpl, err)
		}
		name, err := GenerateFileName(page, namer)
		if err != nil {
			t.Fatalf("GenerateFileName(%q) returned error: %v", tmpl, err)
		}
		if name != want {
			t.Fatalf("template %q: expected %q, got %q", tmpl, want, name)
		}
	}
}
