package confluence

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/jackchuka/confluence-md/internal/confluence/model"
)

// ParseURL extracts base URL and page ID from a Confluence URL
func ParseURL(pageURL string) (model.PageURLInfo, error) {
	// Example URL: https://example.atlassian.net/wiki/spaces/SPACE/pages/12345/Title
	// We need to extract:
	// - baseURL: https://example.atlassian.net
	// - spaceKey: SPACE
	// - pageID: 12345
	// - title: Title

	if pageURL == "" {
		return model.PageURLInfo{}, fmt.Errorf("URL is empty")
	}

	// Parse the URL
	u, err := url.Parse(pageURL)
	if err != nil {
		return model.PageURLInfo{}, fmt.Errorf("invalid URL: %w", err)
	}

	// Extract base URL
	baseURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	var pageID string
	var spaceKey string
	var title string

	// Extract page ID from path
	// Path format: /wiki/spaces/SPACE/pages/12345/Title
	parts := strings.Split(u.Path, "/")
	for i, part := range parts {
		if part == "spaces" && i+1 < len(parts) {
			spaceKey = parts[i+1]
		}
		if part == "pages" && i+1 < len(parts) {
			pageID = parts[i+1]
		}
		if i == len(parts)-1 {
			title = part
		}
	}

	if pageID == "" {
		return model.PageURLInfo{}, fmt.Errorf("could not extract page ID from URL")
	}

	return model.PageURLInfo{
		BaseURL:  baseURL,
		PageID:   pageID,
		SpaceKey: spaceKey,
		Title:    title,
	}, nil
}
