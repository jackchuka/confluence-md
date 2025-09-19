package converter

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

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
func (c *Converter) extractImageReferences(html, pageID, baseURL string) []model.ImageRef {
	var imageRefs []model.ImageRef

	acImageRegex := regexp.MustCompile(`<ac:image[^>]*>[\s\S]*?</ac:image>`)
	matches := acImageRegex.FindAllString(html, -1)

	for _, imageHTML := range matches {
		fileName := plugin.ParseConfluenceImage(imageHTML)
		if fileName == "" {
			continue
		}

		encodedFilename := url.QueryEscape(fileName)
		actualURL := fmt.Sprintf("%s/wiki/download/attachments/%s/%s",
			strings.TrimSuffix(baseURL, "/"), pageID, encodedFilename)

		imageRefs = append(imageRefs, model.ImageRef{
			OriginalURL: actualURL,
			FileName:    fileName,
		})
	}

	return imageRefs
}

// fixMarkdownLinks converts Confluence-specific links into internal references.
func fixMarkdownLinks(markdown string) string {
	confLinkRegex := regexp.MustCompile(`\[([^\]]+)\]\(/wiki/spaces/([^/]+)/pages/(\d+)/[^)]+\)`)
	return confLinkRegex.ReplaceAllString(markdown, "[$1](confluence://pageId/$3)")
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
