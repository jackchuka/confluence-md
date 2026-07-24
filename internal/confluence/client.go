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
	GetUser(userID string) (*model.ConfluenceUser, error)
	FindPageID(spaceKey, title string) (string, error)
}

// Config holds the connection settings for a Confluence instance.
type Config struct {
	// BaseURL is the scheme and host, e.g. "https://wiki.example.com".
	BaseURL string
	// Deployment selects the API dialect (cloud or server).
	Deployment model.Deployment
	// ContextPath is the path prefix in front of the REST API, without a
	// trailing slash ("/wiki" for Cloud, "" or e.g. "/confluence" for server).
	ContextPath string

	// Email and APIToken are used for Cloud HTTP Basic authentication.
	Email    string
	APIToken string

	// Token is a Personal Access Token used for self-hosted Bearer
	// authentication.
	Token string
}

// client represents a Confluence API client
type client struct {
	siteURL     string // scheme://host, no trailing slash
	contextPath string // "/wiki" for Cloud, "" or "/confluence" for server
	apiBase     string // siteURL + contextPath + "/rest/api"
	deployment  model.Deployment
	email       string
	apiToken    string
	token       string
	httpClient  *http.Client
	userAgent   string
}

// NewClient creates a new Confluence API client
func NewClient(cfg Config) Client {
	deployment := cfg.Deployment
	if deployment == "" {
		deployment = model.DeploymentCloud
	}

	siteURL := strings.TrimSuffix(cfg.BaseURL, "/")
	contextPath := strings.TrimSuffix(cfg.ContextPath, "/")

	return &client{
		siteURL:     siteURL,
		contextPath: contextPath,
		apiBase:     siteURL + contextPath + "/rest/api",
		deployment:  deployment,
		email:       cfg.Email,
		apiToken:    cfg.APIToken,
		token:       cfg.Token,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		userAgent: fmt.Sprintf("ConfluenceMd/%s", version.Short()),
	}
}

// setAuth applies the appropriate authentication scheme for the deployment.
func (c *client) setAuth(req *http.Request) {
	if c.deployment == model.DeploymentServer {
		// Self-hosted uses Personal Access Tokens via Bearer auth.
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		return
	}
	// Cloud uses HTTP Basic auth with email + API token.
	req.SetBasicAuth(c.email, c.apiToken)
}

// GetPage retrieves a Confluence page by ID
func (c *client) GetPage(pageID string) (*model.ConfluencePage, error) {
	// Build URL with expansions to get all needed data
	endpoint := fmt.Sprintf("/content/%s", pageID)
	params := url.Values{
		"expand": []string{
			"body.storage,metadata.labels,version,space,history,children.attachment",
		},
	}

	fullURL := c.apiBase + endpoint + "?" + params.Encode()

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

// GetChildPages retrieves all child pages for a given page ID
func (c *client) GetChildPages(pageID string) ([]*model.ConfluencePage, error) {
	endpoint := fmt.Sprintf("/content/%s/child/page", pageID)
	params := url.Values{
		"expand": []string{"body.storage,metadata.labels,version,space,history"},
		"limit":  []string{strconv.Itoa(defaultChildPageLimit)},
	}

	var childPages []*model.ConfluencePage
	start := 0

	for {
		params.Set("start", strconv.Itoa(start))
		fullURL := c.apiBase + endpoint + "?" + params.Encode()

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
	c.setAuth(req)

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
	if fallbackURL, ok := c.attachmentRESTDownloadURL(attachment); ok {
		urls = append(urls, fallbackURL)
	}

	var lastResp *http.Response
	for _, u := range urls {
		resp, err := c.fetchBinary(u)
		if err != nil {
			return nil, fmt.Errorf("failed to download attachment %s: %w", attachment.Title, err)
		}

		if resp.StatusCode == http.StatusOK {
			defer func() {
				_ = resp.Body.Close()
			}()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read attachment content: %w", err)
			}
			return data, nil
		}

		if lastResp != nil {
			_ = lastResp.Body.Close()
		}
		lastResp = resp
	}

	defer func() {
		_ = lastResp.Body.Close()
	}()
	return nil, c.handleErrorResponse(lastResp, fmt.Sprintf("download attachment %s", attachment.Title))
}

// fetchBinary issues an authenticated GET for raw attachment bytes.
func (c *client) fetchBinary(downloadURL string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setAuth(req)
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

	return fmt.Sprintf("%s/content/%s/child/attachment/%s/download",
		c.apiBase, pageID, attachment.ID), true
}

// pageIDFromDownloadLink extracts the parent page ID from a download link of the
// form /download/attachments/{pageID}/{filename}?...
func pageIDFromDownloadLink(link string) (string, bool) {
	_, rest, found := strings.Cut(link, "/attachments/")
	if !found {
		return "", false
	}

	pageID, _, found := strings.Cut(rest, "/")
	if !found || pageID == "" {
		return "", false
	}

	return pageID, true
}

func (c *client) normalizeDownloadLink(link string) (string, error) {
	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		return link, nil
	}

	if !strings.HasPrefix(link, "/") {
		link = "/" + link
	}

	// Download links are relative to the instance context path ("/wiki" for
	// Cloud, "" or e.g. "/confluence" for self-hosted). Prefix it unless the
	// link already carries it.
	if c.contextPath != "" && !strings.HasPrefix(link, c.contextPath+"/") {
		link = c.contextPath + link
	}

	if strings.Contains(link, " ") {
		link = strings.ReplaceAll(link, " ", "%20")
	}

	full := c.siteURL + link
	parsed, err := url.Parse(full)
	if err != nil {
		return "", fmt.Errorf("invalid attachment url %s: %w", full, err)
	}
	return parsed.String(), nil
}

// GetUser retrieves user information by identifier. On Cloud the identifier is
// an account ID; on self-hosted instances it is a user key.
func (c *client) GetUser(userID string) (*model.ConfluenceUser, error) {
	param := "accountId"
	if c.deployment == model.DeploymentServer {
		param = "key"
	}

	query := url.Values{param: []string{userID}}
	fullURL := c.apiBase + "/user?" + query.Encode()

	resp, err := c.makeRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user %s: %w", userID, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp, fmt.Sprintf("get user %s", userID))
	}

	var user model.ConfluenceUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user response: %w", err)
	}

	return &user, nil
}

// FindPageID resolves a page ID from its space key and title. This is primarily
// used for self-hosted "pretty" URLs (e.g. /display/SPACE/Page+Title) that do
// not embed the numeric page ID.
func (c *client) FindPageID(spaceKey, title string) (string, error) {
	if spaceKey == "" || title == "" {
		return "", fmt.Errorf("both space key and title are required to look up a page ID")
	}

	query := url.Values{
		"spaceKey": []string{spaceKey},
		"title":    []string{title},
		"limit":    []string{"1"},
	}
	fullURL := c.apiBase + "/content?" + query.Encode()

	resp, err := c.makeRequest("GET", fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to look up page %q in space %q: %w", title, spaceKey, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", c.handleErrorResponse(resp, fmt.Sprintf("look up page %q in space %q", title, spaceKey))
	}

	var searchResult model.ConfluenceSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return "", fmt.Errorf("failed to decode page lookup response: %w", err)
	}

	if len(searchResult.Results) == 0 || searchResult.Results[0].ID == "" {
		return "", fmt.Errorf("no page titled %q found in space %q", title, spaceKey)
	}

	return searchResult.Results[0].ID, nil
}

// handleErrorResponse handles error responses from the API
func (c *client) handleErrorResponse(resp *http.Response, operation string) error {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to %s: HTTP %d", operation, resp.StatusCode)
	}

	// Try to parse error response
	var errorResp model.ConfluenceErrorResponse
	if err := json.Unmarshal(bodyBytes, &errorResp); err == nil {
		return fmt.Errorf("failed to %s: %s", operation, errorResp.Message)
	}

	// Fallback to HTTP status
	return fmt.Errorf("failed to %s: HTTP %d - %s", operation, resp.StatusCode, string(bodyBytes))
}
