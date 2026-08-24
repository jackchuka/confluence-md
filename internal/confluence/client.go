//go:generate go tool go.uber.org/mock/mockgen -source=$GOFILE -package=mock_$GOPACKAGE -destination=./mock/mock_$GOFILE
package confluence

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackchuka/confluence-md/internal/confluence/model"
	"github.com/jackchuka/confluence-md/internal/version"
)

type Client interface {
	GetPage(pageID string) (*model.ConfluencePage, error)
	GetChildPages(pageID string) ([]*model.ConfluencePage, error)
	DownloadAttachmentContent(attachment *model.ConfluenceAttachment) ([]byte, error)
	GetUser(accountID string) (*model.ConfluenceUser, error)
}

// client represents a Confluence API client
type client struct {
	baseURL    string
	email      string
	apiToken   string
	httpClient *http.Client
	userAgent  string
}

// NewClient creates a new Confluence API client
func NewClient(baseURL, email, apiToken string) Client {
	return &client{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		email:    email,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		userAgent: fmt.Sprintf("ConfluenceMd/%s", version.Short()),
	}
}

// GetPage retrieves a Confluence page by ID
func (c *client) GetPage(pageID string) (*model.ConfluencePage, error) {
	// Build URL with expansions to get all needed data
	endpoint := fmt.Sprintf("/wiki/rest/api/content/%s", pageID)
	params := url.Values{
		"expand": []string{
			"body.storage,metadata.labels,version,space,history,children.attachment",
		},
	}

	fullURL := c.baseURL + endpoint + "?" + params.Encode()

	resp, err := c.makeRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get page %s: %w", pageID, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp, fmt.Sprintf("get page %s", pageID))
	}

	var apiPage model.ConfluenceAPIPage
	if err := json.NewDecoder(resp.Body).Decode(&apiPage); err != nil {
		return nil, fmt.Errorf("failed to decode page response: %w", err)
	}

	// Convert API response to our model
	page := model.ConvertAPIPageToModel(&apiPage)

	return page, nil
}

const defaultChildPageLimit = 100

// maxErrorBodyChars caps how much of an unrecognized error body is echoed back.
const maxErrorBodyChars = 256

// GetChildPages retrieves all child pages for a given page ID
func (c *client) GetChildPages(pageID string) ([]*model.ConfluencePage, error) {
	endpoint := fmt.Sprintf("/wiki/rest/api/content/%s/child/page", pageID)
	params := url.Values{
		"expand": []string{"body.storage,metadata.labels,version,space,history"},
		"limit":  []string{strconv.Itoa(defaultChildPageLimit)},
	}

	var childPages []*model.ConfluencePage
	start := 0

	for {
		params.Set("start", strconv.Itoa(start))
		fullURL := c.baseURL + endpoint + "?" + params.Encode()

		resp, err := c.makeRequest("GET", fullURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get child pages for %s: %w", pageID, err)
		}

		if resp.StatusCode != http.StatusOK {
			err := c.handleErrorResponse(resp, fmt.Sprintf("get child pages for %s", pageID))
			_ = resp.Body.Close()
			return nil, err
		}

		var searchResult model.ConfluenceSearchResult
		if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to decode child pages response: %w", err)
		}
		_ = resp.Body.Close()

		for _, apiPage := range searchResult.Results {
			page := model.ConvertAPIPageToModel(&apiPage)
			childPages = append(childPages, page)
		}

		count := len(searchResult.Results)
		if count == 0 {
			break
		}

		limit := searchResult.Limit
		if limit <= 0 {
			limit = defaultChildPageLimit
		}

		if count < limit {
			break
		}

		start += limit
	}

	return childPages, nil
}

// makeRequest makes an HTTP request with authentication
func (c *client) makeRequest(method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication
	req.SetBasicAuth(c.email, c.apiToken)

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}

// DownloadAttachmentContent downloads attachment binary content
func (c *client) DownloadAttachmentContent(attachment *model.ConfluenceAttachment) ([]byte, error) {
	if attachment == nil {
		return nil, fmt.Errorf("attachment is nil")
	}

	if attachment.DownloadLink == "" {
		return nil, fmt.Errorf("attachment %s has no download link", attachment.Title)
	}

	downloadURL, err := c.normalizeDownloadLink(attachment.DownloadLink)
	if err != nil {
		return nil, err
	}

	urls := []string{downloadURL}
	// Some Confluence Cloud sites reject API-token auth on the legacy
	// /wiki/download/ media path (responding 401 with www-authenticate: OAuth).
	// The v1 REST attachment endpoint honors token auth, so try it as a fallback.
	// A v2-form download link already normalizes to that same REST URL, so drop
	// the duplicate instead of sending an identical request twice.
	if fallbackURL, ok := c.attachmentRESTDownloadURL(attachment); ok && fallbackURL != downloadURL {
		urls = append(urls, fallbackURL)
	}

	var lastErr error
	for _, u := range urls {
		data, err := c.fetchAttachmentFrom(u, attachment.Title)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}

	return nil, lastErr
}

// fetchAttachmentFrom performs one download attempt, closing the response body
// on every path so a failed attempt does not strand the connection.
func (c *client) fetchAttachmentFrom(downloadURL, title string) ([]byte, error) {
	resp, err := c.fetchBinary(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download attachment %s: %w", title, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp, fmt.Sprintf("download attachment %s", title))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read attachment content: %w", err)
	}

	return data, nil
}

// fetchBinary issues an authenticated GET for raw attachment bytes.
func (c *client) fetchBinary(downloadURL string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.email, c.apiToken)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", c.userAgent)

	return c.httpClient.Do(req)
}

// attachmentRESTDownloadURL builds the v1 REST download URL for an attachment,
// which accepts API-token Basic auth where the legacy /wiki/download/ path may not.
func (c *client) attachmentRESTDownloadURL(attachment *model.ConfluenceAttachment) (string, bool) {
	if attachment.ID == "" {
		return "", false
	}

	pageID, ok := pageIDFromDownloadLink(attachment.DownloadLink)
	if !ok {
		return "", false
	}

	return fmt.Sprintf("%s/wiki/rest/api/content/%s/child/attachment/%s/download",
		c.baseURL, pageID, attachment.ID), true
}

// pageIDFromDownloadLink extracts the parent page ID from a download link, in
// either the legacy /download/attachments/{pageID}/{filename}?... form or the
// /rest/api/content/{pageID}/child/attachment/{attachmentID}/download form
// returned by the v2 attachments API.
func pageIDFromDownloadLink(link string) (string, bool) {
	for _, sep := range []string{"/attachments/", "/content/"} {
		_, rest, found := strings.Cut(link, sep)
		if !found {
			continue
		}

		pageID, _, found := strings.Cut(rest, "/")
		if !found || pageID == "" {
			continue
		}

		return pageID, true
	}

	return "", false
}

func (c *client) normalizeDownloadLink(link string) (string, error) {
	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		return link, nil
	}

	if !strings.HasPrefix(link, "/") {
		link = "/" + link
	}

	// Attachment download links are relative to the Confluence context path, not
	// to the site root. Both the legacy /download/... media path and the
	// /rest/api/... form returned by the v2 attachments API need /wiki prefixed;
	// without it the request lands outside Confluence and 404s.
	if !strings.HasPrefix(link, "/wiki/") {
		link = "/wiki" + link
	}

	if strings.Contains(link, " ") {
		link = strings.ReplaceAll(link, " ", "%20")
	}

	full := c.baseURL + link
	parsed, err := url.Parse(full)
	if err != nil {
		return "", fmt.Errorf("invalid attachment url %s: %w", full, err)
	}
	return parsed.String(), nil
}

// GetUser retrieves user information by account ID
func (c *client) GetUser(accountID string) (*model.ConfluenceUser, error) {
	endpoint := fmt.Sprintf("/wiki/rest/api/user?accountId=%s", url.QueryEscape(accountID))
	fullURL := c.baseURL + endpoint

	resp, err := c.makeRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user %s: %w", accountID, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp, fmt.Sprintf("get user %s", accountID))
	}

	var user model.ConfluenceUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user response: %w", err)
	}

	return &user, nil
}

// handleErrorResponse handles error responses from the API
func (c *client) handleErrorResponse(resp *http.Response, operation string) error {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to %s: HTTP %d", operation, resp.StatusCode)
	}

	// Try to parse error response. A body in an unmodelled shape still
	// unmarshals cleanly with every field zero, so require a non-empty message
	// before trusting it — otherwise the error reads "failed to X: " and says
	// nothing at all.
	var errorResp model.ConfluenceErrorResponse
	if err := json.Unmarshal(bodyBytes, &errorResp); err == nil {
		if msg := errorResp.Describe(); msg != "" {
			return fmt.Errorf("failed to %s: HTTP %d - %s", operation, resp.StatusCode, msg)
		}
	}

	// Fallback to HTTP status
	return fmt.Errorf("failed to %s: HTTP %d - %s", operation, resp.StatusCode, summarizeErrorBody(bodyBytes))
}

// summarizeErrorBody trims an unmodelled error body down to something printable.
// A 401 on the media path answers with a full HTML login page, and image
// failures are reported one line each.
func summarizeErrorBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) <= maxErrorBodyChars {
		return text
	}

	return text[:maxErrorBodyChars] + "... (truncated)"
}
