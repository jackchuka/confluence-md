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

	"github.com/jackchuka/confluence-md/internal/models"
	"github.com/jackchuka/confluence-md/internal/version"
)

// Client represents a Confluence API client
type Client struct {
	baseURL    string
	email      string
	apiToken   string
	httpClient *http.Client
	userAgent  string
}

// NewClient creates a new Confluence API client
func NewClient(baseURL, email, apiToken string) *Client {
	return &Client{
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
func (c *Client) GetPage(pageID string) (*models.ConfluencePage, error) {
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

	var apiPage ConfluenceAPIPage
	if err := json.NewDecoder(resp.Body).Decode(&apiPage); err != nil {
		return nil, fmt.Errorf("failed to decode page response: %w", err)
	}

	// Convert API response to our model
	page := convertAPIPageToModel(&apiPage)

	return page, nil
}

const defaultChildPageLimit = 100

// GetChildPages retrieves all child pages for a given page ID
func (c *Client) GetChildPages(pageID string) ([]*models.ConfluencePage, error) {
	endpoint := fmt.Sprintf("/wiki/rest/api/content/%s/child/page", pageID)
	params := url.Values{
		"expand": []string{"body.storage,metadata.labels,version,space,history"},
		"limit":  []string{strconv.Itoa(defaultChildPageLimit)},
	}

	var childPages []*models.ConfluencePage
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

		var searchResult ConfluenceSearchResult
		if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to decode child pages response: %w", err)
		}
		_ = resp.Body.Close()

		for _, apiPage := range searchResult.Results {
			page := convertAPIPageToModel(&apiPage)
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
func (c *Client) makeRequest(method, url string, body io.Reader) (*http.Response, error) {
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

// handleErrorResponse handles error responses from the API
func (c *Client) handleErrorResponse(resp *http.Response, operation string) error {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to %s: HTTP %d", operation, resp.StatusCode)
	}

	// Try to parse error response
	var errorResp ConfluenceErrorResponse
	if err := json.Unmarshal(bodyBytes, &errorResp); err == nil {
		return fmt.Errorf("failed to %s: %s", operation, errorResp.Message)
	}

	// Fallback to HTTP status
	return fmt.Errorf("failed to %s: HTTP %d - %s", operation, resp.StatusCode, string(bodyBytes))
}

// ConfluenceAPIPage represents the API response structure for a page
type ConfluenceAPIPage struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Title  string `json:"title"`
	Body   struct {
		Storage struct {
			Value          string `json:"value"`
			Representation string `json:"representation"`
		} `json:"storage"`
	} `json:"body"`
	Version struct {
		Number int       `json:"number"`
		When   time.Time `json:"when"`
		By     struct {
			Type        string `json:"type"`
			AccountID   string `json:"accountId"`
			DisplayName string `json:"displayName"`
			Email       string `json:"email"`
		} `json:"by"`
	} `json:"version"`
	Space struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"space"`
	History struct {
		CreatedDate time.Time `json:"createdDate"`
		CreatedBy   struct {
			Type        string `json:"type"`
			AccountID   string `json:"accountId"`
			DisplayName string `json:"displayName"`
			Email       string `json:"email"`
		} `json:"createdBy"`
	} `json:"history"`
	Metadata struct {
		Labels struct {
			Results []struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Prefix string `json:"prefix"`
			} `json:"results"`
		} `json:"labels"`
	} `json:"metadata"`
}

// ConfluenceSearchResult represents the API response for search queries
type ConfluenceSearchResult struct {
	Results []ConfluenceAPIPage `json:"results"`
	Start   int                 `json:"start"`
	Limit   int                 `json:"limit"`
	Size    int                 `json:"size"`
}

// ConfluenceErrorResponse represents an error response from the API
type ConfluenceErrorResponse struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Reason     string `json:"reason"`
}

// convertAPIPageToModel converts the API response to our domain model
func convertAPIPageToModel(apiPage *ConfluenceAPIPage) *models.ConfluencePage {
	// Convert labels
	var labels []models.Label
	for _, apiLabel := range apiPage.Metadata.Labels.Results {
		labels = append(labels, models.Label{
			ID:   apiLabel.ID,
			Name: apiLabel.Name,
		})
	}

	return &models.ConfluencePage{
		ID:       apiPage.ID,
		Title:    apiPage.Title,
		SpaceKey: apiPage.Space.Key,
		Version:  apiPage.Version.Number,
		Content: models.ConfluenceContent{
			Storage: models.ContentStorage{
				Value:          apiPage.Body.Storage.Value,
				Representation: apiPage.Body.Storage.Representation,
			},
		},
		Metadata: models.ConfluenceMetadata{
			Labels:     labels,
			Properties: make(map[string]string), // TODO: Extract properties if needed
		},
		Attachments: []models.ConfluenceAttachment{}, // TODO: Get attachments in separate call if needed
		CreatedAt:   apiPage.History.CreatedDate,
		UpdatedAt:   apiPage.Version.When,
		CreatedBy: models.User{
			AccountID:   apiPage.History.CreatedBy.AccountID,
			DisplayName: apiPage.History.CreatedBy.DisplayName,
			Email:       apiPage.History.CreatedBy.Email,
		},
		UpdatedBy: models.User{
			AccountID:   apiPage.Version.By.AccountID,
			DisplayName: apiPage.Version.By.DisplayName,
			Email:       apiPage.Version.By.Email,
		},
	}
}
