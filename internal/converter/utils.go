package converter

import (
	"html"
	"regexp"
	"strings"
)

// parseConfluenceImage extracts filename from Confluence ac:image elements
func parseConfluenceImage(html string) string {
	filenameRegex := regexp.MustCompile(`ri:filename="([^"]+)"`)
	matches := filenameRegex.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractLanguageParameter extracts the language from ac:parameter tags
func extractLanguageParameter(rawHTML string) string {
	langRegex := regexp.MustCompile(`<ac:parameter[^>]*ac:name="language"[^>]*>([^<]+)</ac:parameter>`)
	matches := langRegex.FindStringSubmatch(rawHTML)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractCodeContent extracts code from ac:plain-text-body, handling both CDATA and plain formats
func extractCodeContent(rawHTML string) string {
	// Extract content from ac:plain-text-body tag
	bodyRegex := regexp.MustCompile(`<ac:plain-text-body>([\s\S]*?)</ac:plain-text-body>`)
	matches := bodyRegex.FindStringSubmatch(rawHTML)
	if len(matches) < 2 {
		return ""
	}

	content := matches[1]

	// Decode HTML entities (goquery converts some characters)
	content = html.UnescapeString(content)

	// Remove CDATA markers if present (goquery converts <![CDATA[ to <!--[CDATA[)
	content = strings.TrimPrefix(content, "<!--[CDATA[")
	content = strings.TrimSuffix(content, "]]-->")

	// Also handle original CDATA format (in case it wasn't processed by goquery)
	content = strings.TrimPrefix(content, "<![CDATA[")
	content = strings.TrimSuffix(content, "]]>")

	return content
}
