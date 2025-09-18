package converter

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/PuerkitoBio/goquery"
	"github.com/jackchuka/confluence-md/internal/attachments"
	"github.com/jackchuka/confluence-md/internal/models"
	"golang.org/x/net/html"
)

type confluencePlugin struct {
	imageFolder        string
	attachmentResolver attachments.Resolver
	currentPage        *models.ConfluencePage
}

// NewConfluencePlugin creates a new plugin for Confluence elements
func NewConfluencePlugin(resolver attachments.Resolver, imageFolder string) *confluencePlugin {
	return &confluencePlugin{
		imageFolder:        imageFolder,
		attachmentResolver: resolver,
	}
}

// SetAttachmentResolver configures how attachment content is resolved
func (p *confluencePlugin) SetAttachmentResolver(res attachments.Resolver) {
	p.attachmentResolver = res
}

// SetCurrentPage records which page is currently being converted
func (p *confluencePlugin) SetCurrentPage(page *models.ConfluencePage) {
	p.currentPage = page
}

// Name returns the plugin name
func (p *confluencePlugin) Name() string {
	return "confluence"
}

// Init initializes the plugin
func (p *confluencePlugin) Init(conv *converter.Converter) error {
	// Register handlers for Confluence elements
	conv.Register.RendererFor("ac:image", converter.TagTypeInline, p.handleImage, converter.PriorityStandard)
	conv.Register.RendererFor("ac:emoticon", converter.TagTypeInline, p.handleEmoticon, converter.PriorityStandard)
	conv.Register.RendererFor("ac:structured-macro", converter.TagTypeBlock, p.handleMacro, converter.PriorityStandard)

	// Register custom table handler with higher priority to override default
	conv.Register.RendererFor("table", converter.TagTypeBlock, p.handleTable, converter.PriorityEarly)

	return nil
}

// cellHasComplexContent checks if a single cell contains complex elements
func (p *confluencePlugin) cellHasComplexContent(cell *html.Node) bool {
	blockElementCount := 0

	for child := cell.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode {
			switch child.Data {
			case "ul", "ol", "div", "blockquote", "pre", "table":
				// These elements are always considered complex
				return true
			case "p", "h1", "h2", "h3", "h4", "h5", "h6":
				blockElementCount++
				// If we have more than one block element, it's complex
				if blockElementCount > 1 {
					return true
				}
				// Check if this block element contains br tags
				if p.containsBrTags(child) {
					return true
				}
			case "br":
				// Any br tag at cell level indicates complex formatting
				return true
			}
		}
	}

	return false
}

// containsBrTags checks if a node contains any br tags
func (p *confluencePlugin) containsBrTags(n *html.Node) bool {
	if n == nil {
		return false
	}

	// Check current node
	if n.Type == html.ElementNode && n.Data == "br" {
		return true
	}

	// Check children recursively
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if p.containsBrTags(child) {
			return true
		}
	}

	return false
}

// getCellHTMLContent extracts the raw HTML content from a cell, preserving complex structures
func (p *confluencePlugin) getCellHTMLContent(cell *html.Node) string {
	var result strings.Builder

	for child := cell.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.ElementNode:
			// Render the child element as HTML
			var buf strings.Builder
			_ = html.Render(&buf, child)
			result.WriteString(buf.String())
			// Add space between elements but not newline to keep it in one table cell
			if child.NextSibling != nil {
				result.WriteString(" ")
			}
		case html.TextNode:
			text := strings.TrimSpace(child.Data)
			if text != "" {
				result.WriteString(text)
				if child.NextSibling != nil {
					result.WriteString(" ")
				}
			}
		}
	}

	// Remove newlines to keep content in one table cell
	content := result.String()
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.ReplaceAll(content, "\r", "")
	// Clean up multiple spaces
	content = strings.Join(strings.Fields(content), " ")

	return content
}

// handleTable converts HTML tables to markdown tables, preserving HTML content for complex cells
func (p *confluencePlugin) handleTable(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	// Extract table data
	var rows [][]string
	var isHeaderRow []bool

	// Find tbody
	var tbody *html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "tbody" {
			tbody = c
			break
		}
	}

	if tbody == nil {
		return converter.RenderTryNext // Let default handler try
	}

	// Process rows
	for tr := tbody.FirstChild; tr != nil; tr = tr.NextSibling {
		if tr.Type != html.ElementNode || tr.Data != "tr" {
			continue
		}

		var row []string
		isHeader := false

		for cell := tr.FirstChild; cell != nil; cell = cell.NextSibling {
			if cell.Type != html.ElementNode {
				continue
			}

			if cell.Data == "th" {
				isHeader = true
			}

			if cell.Data == "td" || cell.Data == "th" {
				var cellContent string

				if p.cellHasComplexContent(cell) {
					// For complex cells, preserve the HTML content
					cellContent = p.getCellHTMLContent(cell)
				} else {
					// For simple cells, convert to markdown
					var buf strings.Builder
					if cell.FirstChild != nil {
						ctx.RenderNodes(ctx, &buf, cell.FirstChild)
					}
					cellContent = strings.TrimSpace(buf.String())
				}

				// Handle empty cells
				if cellContent == "" || cellContent == "&nbsp;" {
					cellContent = " "
				}

				row = append(row, cellContent)
			}
		}

		if len(row) > 0 {
			rows = append(rows, row)
			isHeaderRow = append(isHeaderRow, isHeader)
		}
	}

	if len(rows) == 0 {
		return converter.RenderTryNext
	}

	// Determine max columns
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}

	// Pad rows to have same number of columns
	for i := range rows {
		for len(rows[i]) < maxCols {
			rows[i] = append(rows[i], " ")
		}
	}

	// Write table
	for i, row := range rows {
		_, _ = w.WriteString("| ")
		for j, cell := range row {
			_, _ = w.WriteString(cell)
			if j < len(row)-1 {
				_, _ = w.WriteString(" | ")
			}
		}
		_, _ = w.WriteString(" |\n")

		// Add separator after header row
		if i == 0 && isHeaderRow[0] {
			_, _ = w.WriteString("|")
			for j := 0; j < maxCols; j++ {
				_, _ = w.WriteString("---|")
			}
			_, _ = w.WriteString("\n")
		}
	}

	_, _ = w.WriteString("\n")
	return converter.RenderSuccess
}

// handleImage converts Confluence images to markdown
func (p *confluencePlugin) handleImage(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	// Extract filename from ri:filename attribute
	filename := ""
	for _, attr := range n.Attr {
		if attr.Key == "ri:filename" {
			filename = attr.Val
			break
		}
	}

	if filename == "" {
		var buf strings.Builder
		_ = html.Render(&buf, n)
		filename = parseConfluenceImage(buf.String())
	}

	if filename == "" {
		_, _ = w.WriteString("<!-- Image attachment not found -->")
		return converter.RenderSuccess
	}

	// Build local path for the image
	localPath := p.imageFolder + "/" + filename

	_, _ = fmt.Fprintf(w, "![%s](%s)", filename, url.PathEscape(localPath))

	return converter.RenderSuccess
}

func (p *confluencePlugin) handleEmoticon(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	for _, attr := range n.Attr {
		if attr.Key == "ac:emoji-fallback" && attr.Val != "" {
			_, _ = w.WriteString(attr.Val + " ")
			return converter.RenderTryNext
		}
	}

	for _, attr := range n.Attr {
		if attr.Key == "ac:emoji-shortname" && attr.Val != "" {
			_, _ = w.WriteString(attr.Val + " ")
			return converter.RenderTryNext
		}
	}

	for _, attr := range n.Attr {
		if attr.Key == "ac:name" && attr.Val != "" {
			_, _ = fmt.Fprintf(w, ":%s:", attr.Val)
			return converter.RenderTryNext
		}
	}

	_, _ = w.WriteString(":emoji: ")
	return converter.RenderTryNext
}

func (p *confluencePlugin) handleMacro(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	macroName := ""
	for _, attr := range n.Attr {
		if attr.Key == "ac:name" {
			macroName = attr.Val
			break
		}
	}

	if macroName == "" {
		macroName = "unknown"
	}

	tryNext := false

	// Handle different macro types
	var result string
	switch macroName {
	case "info":
		result = p.handleBlockquoteMacro(ctx, n, "ℹ️", "Info")
	case "warning":
		result = p.handleBlockquoteMacro(ctx, n, "⚠️", "Warning")
	case "note":
		result = p.handleBlockquoteMacro(ctx, n, "📝", "Note")
	case "tip":
		result = p.handleBlockquoteMacro(ctx, n, "💡", "Tip")
	case "code":
		result = p.handleCodeMacro(n)
	case "mermaid-cloud":
		result = p.handleMermaidMacro(n)
	case "expand":
		result = p.handleExpandMacro(ctx, n)
	case "toc":
		result, tryNext = p.handleTocMacro(n)
	case "children":
		result = "<!-- Child Pages -->"
	default:
		result = fmt.Sprintf("<!-- Unsupported macro: %s -->", macroName)
	}

	_, _ = w.WriteString(result)
	if tryNext {
		return converter.RenderTryNext
	}
	return converter.RenderSuccess
}

func (p *confluencePlugin) handleBlockquoteMacro(ctx converter.Context, n *html.Node, emoji, label string) string {
	content := p.convertNestedHTML(ctx, n)
	prefix := fmt.Sprintf("%s **%s:**", emoji, label)

	if content == "" {
		return "> " + prefix
	}

	// Handle multi-line content for blockquotes
	lines := strings.Split(content, "\n")
	if len(lines) > 1 {
		result := "> " + prefix + "\n"
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				result += "> " + line + "\n"
			} else {
				result += ">\n"
			}
		}
		return strings.TrimRight(result, "\n")
	}
	return fmt.Sprintf("> %s %s", prefix, content)
}

// handleCodeMacro converts code macros to code blocks
func (p *confluencePlugin) handleCodeMacro(n *html.Node) string {
	// Convert node to goquery selection for compatibility with existing logic
	var buf strings.Builder
	_ = html.Render(&buf, n)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(buf.String()))
	if err != nil {
		return fmt.Sprintf("<!-- Error rendering macro: %s -->", err.Error())
	}
	selection := doc.Selection
	rawHTML, _ := selection.Html()
	language := extractLanguageParameter(rawHTML)

	code := extractPlainTextBodyContent(selection, rawHTML)
	if code == "" {
		code = extractCodeContent(rawHTML)
	}

	if language != "" {
		return fmt.Sprintf("```%s\n%s\n```\n", language, code)
	}
	return fmt.Sprintf("```\n%s\n```\n", code)
}

func (p *confluencePlugin) handleMermaidMacro(n *html.Node) string {
	var buf strings.Builder
	_ = html.Render(&buf, n)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(buf.String()))
	if err != nil {
		return fmt.Sprintf("<!-- Error rendering macro: %s -->", err.Error())
	}
	selection := doc.Selection

	filename := extractMacroParameter(selection, "filename")
	revisionStr := extractMacroParameter(selection, "revision")
	revision := 0
	if revisionStr != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(revisionStr)); err == nil {
			revision = parsed
		}
	}

	if filename == "" {
		return "<!-- Mermaid macro missing filename -->"
	}
	if p.attachmentResolver == nil {
		return fmt.Sprintf("<!-- Mermaid attachment %s unavailable -->", filename)
	}
	if p.currentPage == nil {
		return fmt.Sprintf("<!-- Mermaid attachment %s unavailable -->", filename)
	}
	diagram, err := p.attachmentResolver.Resolve(p.currentPage, filename, revision)
	if err != nil {
		return fmt.Sprintf("<!-- Failed to load mermaid %s: %v -->", filename, err)
	}
	diagram = strings.TrimSpace(diagram)
	if diagram == "" {
		return "<!-- Empty mermaid macro -->"
	}
	return fmt.Sprintf("```mermaid\n%s\n```\n", diagram)
}

func (p *confluencePlugin) handleTocMacro(n *html.Node) (string, bool) {
	result := "<!-- Table of Contents -->"

	// For TOC: check if it has parameter children or is self-closing
	hasParameters := false
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == "ac:parameter" {
			hasParameters = true
			break
		}
	}

	if !hasParameters {
		// Self-closing or no parameters, continue processing siblings
		return result, true
	}

	// Container tag with parameters - don't use tryNext to avoid parameter leakage
	return result, false
}

func (p *confluencePlugin) handleExpandMacro(ctx converter.Context, n *html.Node) string {
	// Extract content from rich-text-body using recursive conversion
	content := p.convertNestedHTML(ctx, n)

	// Just return the content directly without wrapper - content is already rendered
	if content != "" {
		return content + "\n\n"
	}

	return ""
}

// convertNestedHTML recursively converts HTML content within macro nodes
func (p *confluencePlugin) convertNestedHTML(ctx converter.Context, n *html.Node) string {
	// Find ac:rich-text-body node
	richTextBody := p.findRichTextBodyNode(n)
	if richTextBody == nil {
		return ""
	}

	// Convert only the direct children of rich-text-body that belong to this macro
	var buf strings.Builder

	// Process each direct child of the rich-text-body individually
	for child := richTextBody.FirstChild; child != nil; child = child.NextSibling {
		// Skip whitespace-only text nodes
		if child.Type == html.TextNode {
			text := strings.TrimSpace(child.Data)
			if text != "" {
				_, _ = buf.WriteString(text)
			}
			continue
		}

		// Process element nodes
		if child.Type == html.ElementNode {
			// Skip empty <p/> elements used as terminators
			if child.Data == "p" && child.FirstChild == nil {
				continue
			}
			ctx.RenderNodes(ctx, &buf, child)
		}
	}

	return strings.TrimSpace(buf.String())
}

// findRichTextBodyNode recursively finds ac:rich-text-body node
func (p *confluencePlugin) findRichTextBodyNode(n *html.Node) *html.Node {
	if n == nil {
		return nil
	}

	// Check if current node is ac:rich-text-body
	if n.Type == html.ElementNode && n.Data == "ac:rich-text-body" {
		return n
	}

	// Recursively search children
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := p.findRichTextBodyNode(child); found != nil {
			return found
		}
	}

	return nil
}

func extractPlainTextBodyContent(selection *goquery.Selection, rawHTML string) string {
	plainTextBody := selection.Find("ac\\:plain-text-body").First()
	if plainTextBody.Length() == 0 {
		return extractCodeContent(rawHTML)
	}

	preTag := plainTextBody.Find("pre[data-cdata='true']").First()
	if preTag.Length() > 0 {
		content := preTag.Text()

		content = strings.ReplaceAll(content, "&lt;", "<")
		content = strings.ReplaceAll(content, "&gt;", ">")
		content = strings.ReplaceAll(content, "&amp;", "&")

		return strings.TrimSpace(content)
	}

	return extractCodeContent(rawHTML)
}

func extractMacroParameter(selection *goquery.Selection, name string) string {
	param := selection.Find(fmt.Sprintf("ac\\:parameter[ac\\:name='%s']", name)).First()
	if param.Length() == 0 {
		return ""
	}
	return strings.TrimSpace(param.Text())
}
