package models

import (
	"strings"
	"testing"
	"time"
)

func validPage() *ConfluencePage {
	return &ConfluencePage{
		ID:       "123",
		Title:    "Sample",
		SpaceKey: "SPACE",
		Version:  1,
		Content: ConfluenceContent{
			Storage: ContentStorage{Value: "<p>content</p>"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestConfluencePageValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ConfluencePage)
		wantErr string
	}{
		{
			name:    "valid",
			mutate:  func(*ConfluencePage) {},
			wantErr: "",
		},
		{
			name: "missing id",
			mutate: func(p *ConfluencePage) {
				p.ID = ""
			},
			wantErr: "page ID cannot be empty",
		},
		{
			name: "missing title",
			mutate: func(p *ConfluencePage) {
				p.Title = ""
			},
			wantErr: "page title cannot be empty",
		},
		{
			name: "missing content",
			mutate: func(p *ConfluencePage) {
				p.Content.Storage.Value = ""
			},
			wantErr: "page content cannot be empty",
		},
		{
			name: "missing space key",
			mutate: func(p *ConfluencePage) {
				p.SpaceKey = ""
			},
			wantErr: "space key cannot be empty",
		},
		{
			name: "invalid attachment",
			mutate: func(p *ConfluencePage) {
				p.Attachments = []ConfluenceAttachment{{}}
			},
			wantErr: "invalid attachment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := validPage()
			tt.mutate(page)
			err := page.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestConfluencePageGetURL(t *testing.T) {
	page := validPage()
	url, err := page.GetURL("https://example.atlassian.net")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://example.atlassian.net/wiki/spaces/SPACE/pages/123/Sample"
	if url != want {
		t.Fatalf("unexpected url: %s want %s", url, want)
	}
}

func TestConfluencePageGetURLInvalidBase(t *testing.T) {
	page := validPage()
	if _, err := page.GetURL("://bad"); err == nil {
		t.Fatal("expected error for invalid base url")
	}
}

func TestGetLabelNames(t *testing.T) {
	page := validPage()
	page.Metadata.Labels = []Label{{Name: "one"}, {Name: "two"}}
	got := page.GetLabelNames()
	if len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("unexpected labels: %#v", got)
	}
}

func TestConfluenceAttachmentValidate(t *testing.T) {
	attachment := ConfluenceAttachment{
		ID:           "1",
		Title:        "file",
		MediaType:    "image/png",
		FileSize:     10,
		DownloadLink: "https://example/file",
	}
	if err := attachment.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(*ConfluenceAttachment)
		wantErr string
	}{
		{
			name:    "missing id",
			mutate:  func(a *ConfluenceAttachment) { a.ID = "" },
			wantErr: "attachment ID cannot be empty",
		},
		{
			name:    "missing title",
			mutate:  func(a *ConfluenceAttachment) { a.Title = "" },
			wantErr: "attachment title cannot be empty",
		},
		{
			name:    "missing media type",
			mutate:  func(a *ConfluenceAttachment) { a.MediaType = "" },
			wantErr: "attachment media type cannot be empty",
		},
		{
			name:    "invalid size",
			mutate:  func(a *ConfluenceAttachment) { a.FileSize = 0 },
			wantErr: "attachment file size must be greater than 0",
		},
		{
			name:    "missing link",
			mutate:  func(a *ConfluenceAttachment) { a.DownloadLink = "" },
			wantErr: "attachment download link cannot be empty",
		},
		{
			name:    "invalid link",
			mutate:  func(a *ConfluenceAttachment) { a.DownloadLink = "::" },
			wantErr: "invalid download link",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := attachment
			tt.mutate(&a)
			err := a.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
