package converter

import (
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gosimple/slug"
	confluenceModel "github.com/jackchuka/confluence-md/internal/confluence/model"
)

// OutputNamer generates a filename for a converted Confluence page.
type OutputNamer interface {
	FileName(page *confluenceModel.ConfluencePage) (string, error)
}

type outputNamerFunc func(*confluenceModel.ConfluencePage) (string, error)

func (f outputNamerFunc) FileName(page *confluenceModel.ConfluencePage) (string, error) {
	return f(page)
}

// DefaultOutputNamer returns the built-in filename generator.
func DefaultOutputNamer() OutputNamer {
	return outputNamerFunc(defaultFileName)
}

// GenerateFileName resolves the filename for a page using the provided namer or the default.
func GenerateFileName(page *confluenceModel.ConfluencePage, namer OutputNamer) (string, error) {
	if page == nil {
		return "", fmt.Errorf("page cannot be nil")
	}

	if namer == nil {
		namer = DefaultOutputNamer()
	}

	name, err := namer.FileName(page)
	if err != nil {
		return "", err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("generated filename is empty")
	}

	// Normalise to a base filename to avoid introducing directory traversal.
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")

	if name == "." || name == ".." {
		return "", fmt.Errorf("generated filename %q is invalid", name)
	}

	if filepath.Ext(name) == "" {
		name += ".md"
	}

	return name, nil
}

func defaultFileName(page *confluenceModel.ConfluencePage) (string, error) {
	title := strings.TrimSpace(page.Title)
	slugified := slug.MakeLang(title, "en")
	if slugified == "" {
		slugified = "untitled"
	}
	return slugified + ".md", nil
}

var templateFuncMap = template.FuncMap{
	"slug": func(value string) string {
		return slug.MakeLang(value, "en")
	},
}

// TemplateOutputNamer renders filenames from a text/template string.
type TemplateOutputNamer struct {
	tmpl *template.Template
}

// NewTemplateOutputNamer creates a template-driven output namer.
func NewTemplateOutputNamer(tmpl string) (OutputNamer, error) {
	if strings.TrimSpace(tmpl) == "" {
		return nil, fmt.Errorf("template cannot be empty")
	}

	parsed, err := template.New("output_name").Funcs(templateFuncMap).Parse(tmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse output name template: %w", err)
	}

	return &TemplateOutputNamer{tmpl: parsed}, nil
}

func (n *TemplateOutputNamer) FileName(page *confluenceModel.ConfluencePage) (string, error) {
	if page == nil {
		return "", fmt.Errorf("page cannot be nil")
	}

	data := outputTemplateData{
		Page:      page,
		SlugTitle: slug.MakeLang(strings.TrimSpace(page.Title), "en"),
	}

	var builder strings.Builder
	if err := n.tmpl.Execute(&builder, data); err != nil {
		return "", fmt.Errorf("failed to execute output name template: %w", err)
	}

	return builder.String(), nil
}

type outputTemplateData struct {
	Page      *confluenceModel.ConfluencePage
	SlugTitle string
}
