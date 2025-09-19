package plugin

import (
	"fmt"
	"strings"
	"testing"

	htmldom "golang.org/x/net/html"

	convpkg "github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/jackchuka/confluence-md/internal/confluence/model"
)

func TestCellHasComplexContent(t *testing.T) {
	plugin := &ConfluencePlugin{}

	tests := []struct {
		name string
		html string
		want bool
	}{
		{
			name: "simple paragraph",
			html: `<table><tbody><tr><td><p>Content</p></td></tr></tbody></table>`,
			want: false,
		},
		{
			name: "multiple paragraphs",
			html: `<table><tbody><tr><td><p>First</p><p>Second</p></td></tr></tbody></table>`,
			want: true,
		},
		{
			name: "contains list",
			html: `<table><tbody><tr><td><ul><li>Item</li></ul></td></tr></tbody></table>`,
			want: true,
		},
		{
			name: "contains br",
			html: `<table><tbody><tr><td>Line<br/>Break</td></tr></tbody></table>`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell := findNode(t, tt.html, "td")
			got := plugin.cellHasComplexContent(cell)
			if got != tt.want {
				t.Fatalf("cellHasComplexContent(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestContainsBrTags(t *testing.T) {
	plugin := &ConfluencePlugin{}
	node := findNode(t, `<p>Line<br/>Break</p>`, "p")
	if !plugin.containsBrTags(node) {
		t.Fatalf("expected br detection")
	}
	if plugin.containsBrTags(findNode(t, `<p>No break</p>`, "p")) {
		t.Fatalf("unexpected br detection")
	}
}

func TestGetCellHTMLContent(t *testing.T) {
	plugin := &ConfluencePlugin{}
	cell := findNode(t, `<table><tbody><tr><td><p>Text</p><a href="/link">Link</a></td></tr></tbody></table>`, "td")
	got := plugin.getCellHTMLContent(cell)
	if !strings.Contains(got, "<p>Text</p>") || !strings.Contains(got, "<a href=\"/link\">Link</a>") {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestHandleImage(t *testing.T) {
	plugin := &ConfluencePlugin{imageFolder: "images"}
	node := findNode(t, `<ac:image ri:filename="diagram.png"></ac:image>`, "ac:image")
	var out strings.Builder
	status := plugin.handleImage(nil, &out, node)
	if status != convpkg.RenderSuccess {
		t.Fatalf("expected render success, got %v", status)
	}
	if out.String() != "![diagram.png](images%2Fdiagram.png)" {
		t.Fatalf("unexpected markdown: %q", out.String())
	}
}

func TestHandleEmoticon(t *testing.T) {
	plugin := &ConfluencePlugin{}
	node := findNode(t, `<ac:emoticon ac:emoji-fallback="😊"></ac:emoticon>`, "ac:emoticon")
	var out strings.Builder
	status := plugin.handleEmoticon(nil, &out, node)
	if status != convpkg.RenderTryNext {
		t.Fatalf("expected try next, got %v", status)
	}
	if out.String() != "😊 " {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestHandleTocMacro(t *testing.T) {
	plugin := &ConfluencePlugin{}
	node := findNode(t, `<ac:structured-macro ac:name="toc" />`, "ac:structured-macro")
	result, tryNext := plugin.handleTocMacro(node)
	if result != "<!-- Table of Contents -->" || !tryNext {
		t.Fatalf("unexpected result %q tryNext %v", result, tryNext)
	}

	nodeWithParams := findNode(t, `<ac:structured-macro ac:name="toc"><ac:parameter ac:name="maxLevel">3</ac:parameter></ac:structured-macro>`, "ac:structured-macro")
	result, tryNext = plugin.handleTocMacro(nodeWithParams)
	if tryNext {
		t.Fatalf("expected tryNext false when parameters present")
	}
	if result != "<!-- Table of Contents -->" {
		t.Fatalf("unexpected result %q", result)
	}
}

func TestHandleCodeMacro(t *testing.T) {
	plugin := &ConfluencePlugin{}
	node := findNode(t, `<ac:structured-macro ac:name="code"><ac:parameter ac:name="language">go</ac:parameter><ac:plain-text-body><!--[CDATA[fmt.Println(&quot;ok&quot;)]]></ac:plain-text-body></ac:structured-macro>`, "ac:structured-macro")
	result := plugin.handleCodeMacro(node)
	expected := "```go\nfmt.Println(\"ok\")\n```\n"
	if result != expected {
		t.Fatalf("unexpected code block: %q", result)
	}
}

func TestHandleMermaidCloudMacro(t *testing.T) {
	plugin := &ConfluencePlugin{
		attachmentResolver: &stubResolver{expectedPageID: "123", expectedFilename: "diagram", expectedRevision: 2, content: "graph TD;\nA-->B;"},
	}
	plugin.SetCurrentPage(&model.ConfluencePage{ID: "123"})
	node := findNode(t, `<ac:structured-macro ac:name="mermaid-cloud"><ac:parameter ac:name="filename">diagram</ac:parameter><ac:parameter ac:name="revision">2</ac:parameter></ac:structured-macro>`, "ac:structured-macro")
	result := plugin.handleMermaidMacro(node)
	expected := "```mermaid\ngraph TD;\nA-->B;\n```\n"
	if result != expected {
		t.Fatalf("unexpected mermaid cloud block: %q", result)
	}
}

func TestHandleMermaidCloudMacroMissingResolver(t *testing.T) {
	plugin := &ConfluencePlugin{}
	plugin.SetCurrentPage(&model.ConfluencePage{ID: "123"})
	node := findNode(t, `<ac:structured-macro ac:name="mermaid-cloud"><ac:parameter ac:name="filename">diagram</ac:parameter></ac:structured-macro>`, "ac:structured-macro")
	result := plugin.handleMermaidMacro(node)
	if !strings.Contains(result, "Mermaid attachment diagram unavailable") {
		t.Fatalf("expected unavailable message, got %q", result)
	}
}

type stubResolver struct {
	expectedPageID   string
	expectedFilename string
	expectedRevision int
	content          string
}

func (s *stubResolver) Resolve(page *model.ConfluencePage, filename string, revision int) (string, error) {
	if page == nil || page.ID != s.expectedPageID {
		return "", fmt.Errorf("unexpected page %v", page)
	}
	if filename != s.expectedFilename || revision != s.expectedRevision {
		return "", fmt.Errorf("unexpected inputs %s %d", filename, revision)
	}
	return s.content, nil
}

func (s *stubResolver) DownloadAttachment(page *model.ConfluencePage, filename string, revision int) (*model.ConfluenceAttachment, []byte, error) {
	if page == nil || page.ID != s.expectedPageID {
		return nil, nil, fmt.Errorf("unexpected page %v", page)
	}
	if filename != s.expectedFilename || revision != s.expectedRevision {
		return nil, nil, fmt.Errorf("unexpected inputs %s %d", filename, revision)
	}
	attachment := &model.ConfluenceAttachment{
		ID: "att-1",
	}
	return attachment, []byte(s.content), nil
}

func findNode(t *testing.T, markup, tag string) *htmldom.Node {
	t.Helper()
	node, err := htmldom.Parse(strings.NewReader(markup))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	found := searchNode(node, tag)
	if found == nil {
		t.Fatalf("failed to find tag %s in markup %s", tag, markup)
	}
	return found
}

func searchNode(n *htmldom.Node, tag string) *htmldom.Node {
	if n.Type == htmldom.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := searchNode(c, tag); found != nil {
			return found
		}
	}
	return nil
}
