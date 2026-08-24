package converter

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	confluenceModel "github.com/jackchuka/confluence-md/internal/confluence/model"
	"github.com/jackchuka/confluence-md/internal/converter/model"
	"github.com/jackchuka/confluence-md/internal/converter/plugin"
)

// convertHtml converts raw Confluence HTML into Markdown text.
func (c *Converter) convertHtml(html string) (string, error) {
	processedHTML := c.preprocessCDATA(html)

	md, err := c.mdConverter.ConvertString(processedHTML)
	if err != nil {
		fmt.Printf("Conversion error: %v\n", err)
	}

	return c.postprocessMarkdown(md), nil
}

// postprocessMarkdown normalizes whitespace and link formatting in Markdown output.
func (c *Converter) postprocessMarkdown(markdown string) string {
	markdown = regexp.MustCompile(`\n{3,}`).ReplaceAllString(markdown, "\n\n")
	markdown = fixNestedListSpacing(markdown)
	markdown = fixMarkdownLinks(markdown)

	return strings.TrimSpace(markdown)
}

// extractImageReferences finds image attachments referenced in the Confluence HTML.
func (c *Converter) extractImageReferences(html, pageID string, site confluenceModel.SiteInfo) []model.ImageRef {
	var imageRefs []model.ImageRef

	acImageRegex := regexp.MustCompile(`<ac:image[^>]*>[\s\S]*?</ac:image>`)
	matches := acImageRegex.FindAllString(html, -1)

	root := strings.TrimSuffix(site.BaseURL, "/") + site.ContextPath

	for _, imageHTML := range matches {
		fileName := plugin.ParseConfluenceImage(imageHTML)
		if fileName == "" {
			continue
		}

		encodedFilename := url.QueryEscape(fileName)
		actualURL := fmt.Sprintf("%s/download/attachments/%s/%s",
			root, pageID, encodedFilename)

		imageRefs = append(imageRefs, model.ImageRef{
			OriginalURL: actualURL,
			FileName:    fileName,
		})
	}

	return imageRefs
}

// fileMacroRegex matches Confluence file-view macros that embed an attachment.
var fileMacroRegex = regexp.MustCompile(
	`<ac:structured-macro[^>]*ac:name="(?:view-file|viewpdf|viewdoc|viewxls|viewppt|multimedia)"[\s\S]*?</ac:structured-macro>`)

var riFilenameRegex = regexp.MustCompile(`ri:filename="([^"]+)"`)

// extractFileReferences finds non-image attachments embedded via file-view
// macros (view-file, viewpdf, …) so they can be downloaded alongside images.
func (c *Converter) extractFileReferences(html, pageID string, site confluenceModel.SiteInfo) []model.ImageRef {
	var fileRefs []model.ImageRef
	seen := map[string]bool{}

	root := strings.TrimSuffix(site.BaseURL, "/") + site.ContextPath

	for _, macroHTML := range fileMacroRegex.FindAllString(html, -1) {
		m := riFilenameRegex.FindStringSubmatch(macroHTML)
		if len(m) < 2 || m[1] == "" || seen[m[1]] {
			continue
		}
		fileName := m[1]
		seen[fileName] = true

		actualURL := fmt.Sprintf("%s/download/attachments/%s/%s",
			root, pageID, url.QueryEscape(fileName))

		fileRefs = append(fileRefs, model.ImageRef{
			OriginalURL: actualURL,
			FileName:    fileName,
		})
	}

	return fileRefs
}

// fixMarkdownLinks converts Confluence-specific links into internal references.
func fixMarkdownLinks(markdown string) string {
	// Cloud (/wiki/spaces/...) and self-hosted (/spaces/...) modern page links.
	spacesLinkRegex := regexp.MustCompile(`\[([^\]]+)\]\((?:/wiki)?/spaces/[^/]+/pages/(\d+)/[^)]+\)`)
	markdown = spacesLinkRegex.ReplaceAllString(markdown, "[$1](confluence://pageId/$2)")

	// Self-hosted legacy links: /pages/viewpage.action?pageId=12345
	viewpageLinkRegex := regexp.MustCompile(`\[([^\]]+)\]\([^)]*?/pages/viewpage\.action\?pageId=(\d+)[^)]*\)`)
	markdown = viewpageLinkRegex.ReplaceAllString(markdown, "[$1](confluence://pageId/$2)")

	return markdown
}

// fixNestedListSpacing removes extraneous blank lines in nested lists.
func fixNestedListSpacing(markdown string) string {
	listMarker := `(?:[-*+]\s|\d+\.\s)`
	pattern := regexp.MustCompile(`(\n\s*` + listMarker + `[^\n]*)\n\s*\n(\s{2,}` + listMarker + `)`)
	result := pattern.ReplaceAllString(markdown, "$1\n$2")
	if result != markdown {
		return fixNestedListSpacing(result)
	}
	return result
}

// preprocessCDATA preserves content inside CDATA nodes prior to HTML parsing.
func (c *Converter) preprocessCDATA(html string) string {
	cdataRegex := regexp.MustCompile(`<!\[CDATA\[([\s\S]*?)\]\]>`)
	return cdataRegex.ReplaceAllStringFunc(html, func(match string) string {
		if submatch := cdataRegex.FindStringSubmatch(match); len(submatch) > 1 {
			content := submatch[1]
			content = strings.ReplaceAll(content, "&", "&amp;")
			content = strings.ReplaceAll(content, "<", "&lt;")
			content = strings.ReplaceAll(content, ">", "&gt;")
			return fmt.Sprintf("<pre data-cdata='true'>%s</pre>", content)
		}
		return match
	})
}
