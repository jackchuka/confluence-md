package confluence

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackchuka/confluence-md/internal/confluence/model"
)

func TestClientServerDeploymentUsesBearerAndNoWikiPrefix(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"123","title":"Page","space":{"key":"SP"}}`))
	}))
	defer srv.Close()

	c := NewClient(Config{
		BaseURL:    srv.URL,
		Deployment: model.DeploymentServer,
		Token:      "pat-token",
	})

	page, err := c.GetPage("123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.ID != "123" {
		t.Fatalf("unexpected page id: %s", page.ID)
	}
	if gotPath != "/rest/api/content/123" {
		t.Errorf("path = %q, want /rest/api/content/123 (no /wiki prefix)", gotPath)
	}
	if gotAuth != "Bearer pat-token" {
		t.Errorf("Authorization = %q, want Bearer pat-token", gotAuth)
	}
}

func TestClientCloudDeploymentUsesBasicAndWikiPrefix(t *testing.T) {
	var gotPath string
	var gotUser, gotPass string
	var hadBasic bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotUser, gotPass, hadBasic = r.BasicAuth()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"123","title":"Page","space":{"key":"SP"}}`))
	}))
	defer srv.Close()

	c := NewClient(Config{
		BaseURL:     srv.URL,
		Deployment:  model.DeploymentCloud,
		ContextPath: "/wiki",
		Email:       "user@example.com",
		APIToken:    "cloud-token",
	})

	if _, err := c.GetPage("123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/wiki/rest/api/content/123" {
		t.Errorf("path = %q, want /wiki/rest/api/content/123", gotPath)
	}
	if !hadBasic || gotUser != "user@example.com" || gotPass != "cloud-token" {
		t.Errorf("basic auth = (%q,%q,%v), want (user@example.com, cloud-token, true)", gotUser, gotPass, hadBasic)
	}
}

func TestClientGetUserParamByDeployment(t *testing.T) {
	tests := []struct {
		name       string
		deployment model.Deployment
		wantParam  string
	}{
		{"cloud", model.DeploymentCloud, "accountId"},
		{"server", model.DeploymentServer, "key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQuery string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.RawQuery
				_, _ = w.Write([]byte(`{"displayName":"Jane"}`))
			}))
			defer srv.Close()

			c := NewClient(Config{BaseURL: srv.URL, Deployment: tt.deployment, Token: "x", Email: "e", APIToken: "t"})
			if _, err := c.GetUser("abc123"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(gotQuery, tt.wantParam+"=abc123") {
				t.Errorf("query = %q, want it to contain %s=abc123", gotQuery, tt.wantParam)
			}
		})
	}
}

func TestClientNormalizeDownloadLinkContextPath(t *testing.T) {
	tests := []struct {
		name        string
		contextPath string
		link        string
		want        string
	}{
		{"cloud", "/wiki", "/download/attachments/1/a.png", "https://host/wiki/download/attachments/1/a.png"},
		{"server no context", "", "/download/attachments/1/a.png", "https://host/download/attachments/1/a.png"},
		{"server context", "/confluence", "/download/attachments/1/a.png", "https://host/confluence/download/attachments/1/a.png"},
		{"already prefixed", "/wiki", "/wiki/download/attachments/1/a.png", "https://host/wiki/download/attachments/1/a.png"},
		{"absolute passthrough", "/wiki", "https://cdn.example.com/a.png", "https://cdn.example.com/a.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &client{siteURL: "https://host", contextPath: tt.contextPath}
			got, err := c.normalizeDownloadLink(tt.link)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("normalizeDownloadLink(%q) = %q, want %q", tt.link, got, tt.want)
			}
		})
	}
}
