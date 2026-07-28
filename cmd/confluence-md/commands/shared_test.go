package commands

import (
	"testing"

	confluenceModel "github.com/jackchuka/confluence-md/internal/confluence/model"
)

func TestUrlToPageInfo(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		typeOverride string
		wantErr      bool
		wantBase     string
		wantContext  string
		wantPageID   string
		wantSpace    string
		wantTitle    string
		wantDeploy   confluenceModel.Deployment
	}{
		{
			name:        "cloud standard",
			url:         "https://example.atlassian.net/wiki/spaces/SPACE/pages/12345/Some+Title",
			wantBase:    "https://example.atlassian.net",
			wantContext: "/wiki",
			wantPageID:  "12345",
			wantSpace:   "SPACE",
			wantDeploy:  confluenceModel.DeploymentCloud,
		},
		{
			name:        "server modern spaces url",
			url:         "https://wiki.example.com/spaces/SPACE/pages/999/Title",
			wantBase:    "https://wiki.example.com",
			wantContext: "",
			wantPageID:  "999",
			wantSpace:   "SPACE",
			wantDeploy:  confluenceModel.DeploymentServer,
		},
		{
			name:        "server viewpage action",
			url:         "https://wiki.example.com/pages/viewpage.action?pageId=456",
			wantBase:    "https://wiki.example.com",
			wantContext: "",
			wantPageID:  "456",
			wantDeploy:  confluenceModel.DeploymentServer,
		},
		{
			name:        "server pretty display url (page id resolved later)",
			url:         "https://wiki.example.com/display/SPACE/Page+Title",
			wantBase:    "https://wiki.example.com",
			wantContext: "",
			wantPageID:  "",
			wantSpace:   "SPACE",
			wantTitle:   "Page Title",
			wantDeploy:  confluenceModel.DeploymentServer,
		},
		{
			name:        "server with context path",
			url:         "https://example.com/confluence/display/SPACE/Title",
			wantBase:    "https://example.com",
			wantContext: "/confluence",
			wantSpace:   "SPACE",
			wantTitle:   "Title",
			wantDeploy:  confluenceModel.DeploymentServer,
		},
		{
			name:         "override cloud host as server",
			url:          "https://example.atlassian.net/pages/viewpage.action?pageId=1",
			typeOverride: "server",
			wantBase:     "https://example.atlassian.net",
			wantPageID:   "1",
			wantDeploy:   confluenceModel.DeploymentServer,
		},
		{
			name:    "cloud missing page id errors",
			url:     "https://example.atlassian.net/wiki/spaces/SPACE/overview",
			wantErr: true,
		},
		{
			name:         "unknown type errors",
			url:          "https://wiki.example.com/pages/viewpage.action?pageId=1",
			typeOverride: "bogus",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := urlToPageInfo(tt.url, tt.typeOverride)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none (info=%+v)", info)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info.BaseURL != tt.wantBase {
				t.Errorf("BaseURL = %q, want %q", info.BaseURL, tt.wantBase)
			}
			if info.ContextPath != tt.wantContext {
				t.Errorf("ContextPath = %q, want %q", info.ContextPath, tt.wantContext)
			}
			if info.PageID != tt.wantPageID {
				t.Errorf("PageID = %q, want %q", info.PageID, tt.wantPageID)
			}
			if info.SpaceKey != tt.wantSpace {
				t.Errorf("SpaceKey = %q, want %q", info.SpaceKey, tt.wantSpace)
			}
			if tt.wantTitle != "" && info.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", info.Title, tt.wantTitle)
			}
			if info.Deployment != tt.wantDeploy {
				t.Errorf("Deployment = %q, want %q", info.Deployment, tt.wantDeploy)
			}
		})
	}
}

func TestResolveDeployment(t *testing.T) {
	tests := []struct {
		host     string
		override string
		want     confluenceModel.Deployment
		wantErr  bool
	}{
		{host: "example.atlassian.net", want: confluenceModel.DeploymentCloud},
		{host: "wiki.example.com", want: confluenceModel.DeploymentServer},
		{host: "wiki.example.com", override: "cloud", want: confluenceModel.DeploymentCloud},
		{host: "example.atlassian.net", override: "server", want: confluenceModel.DeploymentServer},
		{host: "example.atlassian.net", override: "self-hosted", want: confluenceModel.DeploymentServer},
		{host: "wiki.example.com", override: "nonsense", wantErr: true},
	}

	for _, tt := range tests {
		got, err := resolveDeployment(tt.host, tt.override)
		if tt.wantErr {
			if err == nil {
				t.Errorf("resolveDeployment(%q, %q): expected error", tt.host, tt.override)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveDeployment(%q, %q): unexpected error %v", tt.host, tt.override, err)
			continue
		}
		if got != tt.want {
			t.Errorf("resolveDeployment(%q, %q) = %q, want %q", tt.host, tt.override, got, tt.want)
		}
	}
}
