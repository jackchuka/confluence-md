package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackchuka/confluence-md/internal/confluence/model"
	"github.com/jackchuka/confluence-md/internal/version"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestNewClient(t *testing.T) {
	originalVersion := version.Version
	version.Version = "v1.2.3"
	t.Cleanup(func() {
		version.Version = originalVersion
	})

	tests := []struct {
		name          string
		baseURL       string
		wantBaseURL   string
		wantUserAgent string
	}{
		{
			name:          "trims trailing slash",
			baseURL:       "https://example.atlassian.net/",
			wantBaseURL:   "https://example.atlassian.net",
			wantUserAgent: "ConfluenceMd/v1.2.3",
		},
		{
			name:          "no trailing slash",
			baseURL:       "https://example.atlassian.net",
			wantBaseURL:   "https://example.atlassian.net",
			wantUserAgent: "ConfluenceMd/v1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(tt.baseURL, "user", "token")
			if c.baseURL != tt.wantBaseURL {
				t.Fatalf("baseURL = %s, want %s", c.baseURL, tt.wantBaseURL)
			}
			if c.userAgent != tt.wantUserAgent {
				t.Fatalf("userAgent = %s, want %s", c.userAgent, tt.wantUserAgent)
			}
			if got := c.httpClient.Timeout; got != 60*time.Second {
				t.Fatalf("timeout = %s, want %s", got, 60*time.Second)
			}
		})
	}
}

func TestClient_GetPage(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	apiPage := ConfluenceAPIPage{
		ID:    "123",
		Title: "Sample",
	}
	apiPage.Type = "page"
	apiPage.Status = "current"
	apiPage.Body.Storage.Value = "<p>Hello</p>"
	apiPage.Body.Storage.Representation = "storage"
	apiPage.Version.Number = 7
	apiPage.Version.When = now
	apiPage.Version.By.AccountID = "abc"
	apiPage.Version.By.DisplayName = "Updater"
	apiPage.Version.By.Email = "updater@example.com"
	apiPage.Space.Key = "SPACE"
	apiPage.Space.Name = "Space"
	apiPage.History.CreatedDate = now.Add(-time.Hour)
	apiPage.History.CreatedBy.AccountID = "def"
	apiPage.History.CreatedBy.DisplayName = "Creator"
	apiPage.History.CreatedBy.Email = "creator@example.com"
	apiPage.Metadata.Labels.Results = []struct {
		ID     string "json:\"id\""
		Name   string "json:\"name\""
		Prefix string "json:\"prefix\""
	}{{ID: "1", Name: "important"}}
	apiPage.Children.Attachment.Results = append(apiPage.Children.Attachment.Results, struct {
		ID      string "json:\"id\""
		Title   string "json:\"title\""
		Version struct {
			Number int "json:\"number\""
		} "json:\"version\""
		Extensions struct {
			MediaType string "json:\"mediaType\""
			FileSize  int64  "json:\"fileSize\""
		} "json:\"extensions\""
		Links struct {
			Download string "json:\"download\""
		} "json:\"_links\""
	}{
		ID:    "att-1",
		Title: "diagram.mmd",
		Version: struct {
			Number int "json:\"number\""
		}{Number: 2},
		Extensions: struct {
			MediaType string "json:\"mediaType\""
			FileSize  int64  "json:\"fileSize\""
		}{MediaType: "text/plain", FileSize: 42},
		Links: struct {
			Download string "json:\"download\""
		}{Download: "/download/attachments/123/diagram.mmd"},
	})

	transportSuccess := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			return nil, fmt.Errorf("unexpected method %s", r.Method)
		}
		if user, token, ok := r.BasicAuth(); !ok || user != "user" || token != "token" {
			return nil, fmt.Errorf("unexpected auth %s %s", user, token)
		}
		if got := r.URL.Query().Get("expand"); !strings.Contains(got, "body.storage") {
			return nil, fmt.Errorf("missing expand parameter: %s", got)
		}
		return jsonResponse(t, http.StatusOK, apiPage), nil
	})

	transportError := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(t, http.StatusNotFound, ConfluenceErrorResponse{Message: "page not found"}), nil
	})

	tests := []struct {
		name      string
		transport roundTripFunc
		wantErr   string
	}{
		{
			name:      "success",
			transport: transportSuccess,
		},
		{
			name:      "api error",
			transport: transportError,
			wantErr:   "page not found",
		},
		{
			name: "request failure",
			transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return nil, errors.New("network down")
			}),
			wantErr: "network down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := New("https://example.atlassian.net", "user", "token")
			client.httpClient = &http.Client{Transport: tt.transport}

			page, err := client.GetPage("123")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				if page != nil {
					t.Fatalf("expected nil page, got %#v", page)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if page == nil {
				t.Fatal("expected page, got nil")
			}
			if page.ID != "123" || page.Title != "Sample" {
				t.Fatalf("unexpected page: %#v", page)
			}
			if len(page.Metadata.Labels) != 1 || page.Metadata.Labels[0].Name != "important" {
				t.Fatalf("unexpected labels: %#v", page.Metadata.Labels)
			}
			if len(page.Attachments) != 1 {
				t.Fatalf("expected 1 attachment, got %d", len(page.Attachments))
			}
			if page.Attachments[0].Title != "diagram.mmd" || page.Attachments[0].Version != 2 {
				t.Fatalf("unexpected attachment: %#v", page.Attachments[0])
			}
		})
	}
}

func TestClientDownloadAttachmentContent(t *testing.T) {
	attachment := &model.ConfluenceAttachment{
		Title:        "diagram.mmd",
		DownloadLink: "/download/attachments/123/diagram.mmd",
	}

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://example.atlassian.net/wiki/download/attachments/123/diagram.mmd" {
			return nil, fmt.Errorf("unexpected url %s", r.URL.String())
		}
		if user, token, ok := r.BasicAuth(); !ok || user != "user" || token != "token" {
			return nil, fmt.Errorf("unexpected auth %s %s", user, token)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("graph TD;")),
		}, nil
	})

	client := New("https://example.atlassian.net", "user", "token")
	client.httpClient = &http.Client{Transport: transport}

	data, err := client.DownloadAttachmentContent(attachment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "graph TD;" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestNormalizeDownloadLink(t *testing.T) {
	client := New("https://example.atlassian.net", "user", "token")

	tests := []struct {
		input string
		want  string
	}{
		{
			input: "https://other/download/file",
			want:  "https://other/download/file",
		},
		{
			input: "/wiki/download/attachments/123/file.txt",
			want:  "https://example.atlassian.net/wiki/download/attachments/123/file.txt",
		},
		{
			input: "/download/attachments/123/file.txt",
			want:  "https://example.atlassian.net/wiki/download/attachments/123/file.txt",
		},
		{
			input: "download/attachments/123/file with space.txt",
			want:  "https://example.atlassian.net/wiki/download/attachments/123/file%20with%20space.txt",
		},
	}

	for _, tt := range tests {
		got, err := client.normalizeDownloadLink(tt.input)
		if err != nil {
			t.Fatalf("normalizeDownloadLink(%q) unexpected error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("normalizeDownloadLink(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestClient_GetChildPages(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	transportPagination := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if user, token, ok := r.BasicAuth(); !ok || user != "user" || token != "token" {
			return nil, fmt.Errorf("unexpected auth %s %s", user, token)
		}
		start := r.URL.Query().Get("start")
		if start == "" {
			start = "0"
		}
		switch start {
		case "0":
			resp := ConfluenceSearchResult{
				Results: []ConfluenceAPIPage{
					{
						ID:    "1",
						Title: "Child 1",
						Space: struct {
							Key  string "json:\"key\""
							Name string "json:\"name\""
						}{Key: "SPACE"},
						Version: struct {
							Number int       "json:\"number\""
							When   time.Time "json:\"when\""
							By     struct {
								Type        string "json:\"type\""
								AccountID   string "json:\"accountId\""
								DisplayName string "json:\"displayName\""
								Email       string "json:\"email\""
							} "json:\"by\""
						}{Number: 2, When: now},
						Body: struct {
							Storage struct {
								Value          string "json:\"value\""
								Representation string "json:\"representation\""
							} "json:\"storage\""
						}{Storage: struct {
							Value          string "json:\"value\""
							Representation string "json:\"representation\""
						}{Value: "<p>First</p>", Representation: "storage"}},
						History: struct {
							CreatedDate time.Time "json:\"createdDate\""
							CreatedBy   struct {
								Type        string "json:\"type\""
								AccountID   string "json:\"accountId\""
								DisplayName string "json:\"displayName\""
								Email       string "json:\"email\""
							} "json:\"createdBy\""
						}{CreatedDate: now.Add(-time.Hour)},
					},
					{
						ID:    "2",
						Title: "Child 2",
					},
				},
				Limit: 2,
			}
			return jsonResponse(t, http.StatusOK, resp), nil
		case "2":
			resp := ConfluenceSearchResult{
				Results: []ConfluenceAPIPage{{ID: "3", Title: "Child 3"}},
				Limit:   2,
			}
			return jsonResponse(t, http.StatusOK, resp), nil
		default:
			return jsonResponse(t, http.StatusOK, ConfluenceSearchResult{}), nil
		}
	})

	transportError := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(t, http.StatusBadGateway, ConfluenceErrorResponse{Message: "bad gateway"}), nil
	})

	tests := []struct {
		name      string
		transport roundTripFunc
		wantLen   int
		wantErr   string
	}{
		{
			name:      "pagination",
			transport: transportPagination,
			wantLen:   3,
		},
		{
			name:      "api error",
			transport: transportError,
			wantErr:   "bad gateway",
		},
		{
			name: "request failure",
			transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return nil, errors.New("boom")
			}),
			wantErr: "boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := New("https://example.atlassian.net", "user", "token")
			client.httpClient = &http.Client{Transport: tt.transport}

			pages, err := client.GetChildPages("parent")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				if pages != nil {
					t.Fatalf("expected nil pages, got %#v", pages)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(pages) != tt.wantLen {
				t.Fatalf("expected %d pages, got %d", tt.wantLen, len(pages))
			}
		})
	}
}

func jsonResponse(t *testing.T, status int, payload any) *http.Response {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(data)),
	}
}

func TestHandleErrorResponse(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantText string
	}{
		{
			name:     "json body",
			status:   http.StatusNotFound,
			body:     `{"message":"missing"}`,
			wantText: "missing",
		},
		{
			name:     "fallback",
			status:   http.StatusBadGateway,
			body:     "upstream failure",
			wantText: "HTTP 502 - upstream failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New("https://example", "user", "token")
			resp := &http.Response{
				StatusCode: tt.status,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
				Status:     http.StatusText(tt.status),
			}

			err := c.handleErrorResponse(resp, "op")
			if err == nil || !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("expected error containing %q, got %v", tt.wantText, err)
			}
		})
	}
}

func TestConvertAPIPageToModel(t *testing.T) {
	created := time.Date(2024, 1, 1, 1, 1, 1, 0, time.UTC)
	updated := created.Add(time.Hour)

	api := &ConfluenceAPIPage{}
	api.ID = "123"
	api.Title = "Title"
	api.Space.Key = "SPACE"
	api.Version.Number = 5
	api.Version.When = updated
	api.Version.By.AccountID = "acc2"
	api.Version.By.DisplayName = "Updater"
	api.Version.By.Email = "updater@example.com"
	api.Body.Storage.Value = "<p>content</p>"
	api.Body.Storage.Representation = "storage"
	api.Metadata.Labels.Results = []struct {
		ID     string "json:\"id\""
		Name   string "json:\"name\""
		Prefix string "json:\"prefix\""
	}{{ID: "1", Name: "label"}}
	api.History.CreatedDate = created
	api.History.CreatedBy.AccountID = "acc1"
	api.History.CreatedBy.DisplayName = "Creator"
	api.History.CreatedBy.Email = "creator@example.com"

	model := convertAPIPageToModel(api)

	if model.ID != "123" || model.Title != "Title" {
		t.Fatalf("unexpected model: %#v", model)
	}
	if model.Metadata.Labels[0].Name != "label" {
		t.Fatalf("expected label name, got %#v", model.Metadata.Labels)
	}
	if model.CreatedAt != created || model.UpdatedAt != updated {
		t.Fatalf("unexpected timestamps: created %s updated %s", model.CreatedAt, model.UpdatedAt)
	}
	if model.CreatedBy.DisplayName != "Creator" || model.UpdatedBy.DisplayName != "Updater" {
		t.Fatalf("unexpected users: %#v %#v", model.CreatedBy, model.UpdatedBy)
	}
}

func TestClient_makeRequestSetsHeaders(t *testing.T) {
	client := New("https://example", "user", "token")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, "ConfluenceMd/") {
			t.Fatalf("unexpected user agent: %s", ua)
		}
		if accept := r.Header.Get("Accept"); accept != "application/json" {
			t.Fatalf("unexpected accept header: %s", accept)
		}
		if ct := r.Header.Get("Content-Type"); ct != "" {
			t.Fatalf("content-type should be empty, got %s", ct)
		}
		if user, token, ok := r.BasicAuth(); !ok || user != "user" || token != "token" {
			t.Fatalf("unexpected auth: %s %s", user, token)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("{}")),
		}, nil
	})}

	_, err := client.makeRequest(http.MethodGet, "https://example/resource", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify content-type header is set when body is present
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected JSON content type, got %s", ct)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("{}")),
		}, nil
	})}

	_, err = client.makeRequest(http.MethodPost, "https://example/resource", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleErrorResponseReadFailure(t *testing.T) {
	client := New("https://example", "user", "token")
	brokenBody := io.NopCloser(io.MultiReader(&failingReader{}))
	resp := &http.Response{
		StatusCode: http.StatusTeapot,
		Body:       brokenBody,
	}

	err := client.handleErrorResponse(resp, "op")
	if err == nil || !strings.Contains(err.Error(), "HTTP 418") {
		t.Fatalf("expected fallback error, got %v", err)
	}
}

type failingReader struct{}

func (f *failingReader) Read(p []byte) (int, error) {
	return 0, errors.New("read error")
}
