package confluence

import (
	"strings"
	"testing"

	"github.com/jackchuka/confluence-md/internal/confluence/model"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    model.PageURLInfo
		wantErr string
	}{
		{
			name:  "valid url",
			input: "https://example.atlassian.net/wiki/spaces/SPACE/pages/12345/Title",
			want: model.PageURLInfo{
				BaseURL:  "https://example.atlassian.net",
				SpaceKey: "SPACE",
				PageID:   "12345",
				Title:    "Title",
			},
		},
		{
			name:    "empty",
			input:   "",
			wantErr: "URL is empty",
		},
		{
			name:    "invalid",
			input:   "://bad url",
			wantErr: "invalid URL",
		},
		{
			name:    "missing id",
			input:   "https://example.atlassian.net/wiki/spaces/SPACE/pages//Title",
			wantErr: "could not extract page ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseURL(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected info: %#v want %#v", got, tt.want)
			}
		})
	}
}

func TestParseURLHandlesEncodedTitle(t *testing.T) {
	info, err := ParseURL("https://example.atlassian.net/wiki/spaces/SPACE/pages/12345/My%20Title")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Title != "My Title" {
		t.Fatalf("expected decoded title, got %s", info.Title)
	}
}
