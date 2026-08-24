package confluence

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackchuka/confluence-md/internal/confluence/model"
)

func TestNormalizeDownloadLink(t *testing.T) {
	c := &client{baseURL: "https://example.atlassian.net"}

	tests := []struct {
		name string
		link string
		want string
	}{
		{
			// The v2 attachments API returns this form. It is relative to the
			// Confluence context path, so it needs /wiki just as much as the
			// legacy media path does.
			name: "rest api link gets the wiki context path",
			link: "/rest/api/content/123/child/attachment/att456/download",
			want: "https://example.atlassian.net/wiki/rest/api/content/123/child/attachment/att456/download",
		},
		{
			name: "legacy download link gets the wiki context path",
			link: "/download/attachments/123/image.png?version=1",
			want: "https://example.atlassian.net/wiki/download/attachments/123/image.png?version=1",
		},
		{
			name: "link already carrying the context path is left alone",
			link: "/wiki/download/attachments/123/image.png",
			want: "https://example.atlassian.net/wiki/download/attachments/123/image.png",
		},
		{
			name: "relative link is rooted before prefixing",
			link: "download/attachments/123/image.png",
			want: "https://example.atlassian.net/wiki/download/attachments/123/image.png",
		},
		{
			name: "absolute link is passed through untouched",
			link: "https://cdn.example.com/image.png",
			want: "https://cdn.example.com/image.png",
		},
		{
			name: "spaces are escaped",
			link: "/download/attachments/123/my image.png",
			want: "https://example.atlassian.net/wiki/download/attachments/123/my%20image.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.normalizeDownloadLink(tt.link)
			if err != nil {
				t.Fatalf("normalizeDownloadLink(%q) returned error: %v", tt.link, err)
			}
			if got != tt.want {
				t.Errorf("normalizeDownloadLink(%q)\n got: %s\nwant: %s", tt.link, got, tt.want)
			}
		})
	}
}

func TestPageIDFromDownloadLink(t *testing.T) {
	tests := []struct {
		name   string
		link   string
		want   string
		wantOK bool
	}{
		{
			name:   "v2 rest api form",
			link:   "/rest/api/content/98765/child/attachment/att111/download",
			want:   "98765",
			wantOK: true,
		},
		{
			name:   "legacy media form",
			link:   "/download/attachments/12345/image.png?version=1",
			want:   "12345",
			wantOK: true,
		},
		{
			name:   "unrecognised form",
			link:   "/some/other/path",
			want:   "",
			wantOK: false,
		},
		{
			name:   "missing trailing segment",
			link:   "/download/attachments/12345",
			want:   "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := pageIDFromDownloadLink(tt.link)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("pageIDFromDownloadLink(%q) = (%q, %v), want (%q, %v)",
					tt.link, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestDownloadAttachmentContentUsesWikiContextPath(t *testing.T) {
	var gotPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		if !strings.HasPrefix(r.URL.Path, "/wiki/") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"type":"about:blank","title":"Not Found","status":404,"detail":"No endpoint"}`))
			return
		}
		_, _ = w.Write([]byte("PNGDATA"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@example.com", "token")
	data, err := c.DownloadAttachmentContent(&model.ConfluenceAttachment{
		ID:           "att456",
		Title:        "image.png",
		DownloadLink: "/rest/api/content/123/child/attachment/att456/download",
	})
	if err != nil {
		t.Fatalf("DownloadAttachmentContent returned error: %v", err)
	}
	if string(data) != "PNGDATA" {
		t.Errorf("content = %q, want %q", data, "PNGDATA")
	}
	if len(gotPaths) != 1 || gotPaths[0] != "/wiki/rest/api/content/123/child/attachment/att456/download" {
		t.Errorf("requested paths = %v, want a single /wiki-prefixed request", gotPaths)
	}
}

func TestDownloadAttachmentContentSurfacesErrorDetail(t *testing.T) {
	// Every candidate URL 404s, so the caller must be told why. Before the
	// error-shape fix this produced "failed to download attachment X: " with
	// nothing after the colon.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"status":404,"code":"NOT_FOUND","title":"Not Found"}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@example.com", "token")
	_, err := c.DownloadAttachmentContent(&model.ConfluenceAttachment{
		ID:           "att456",
		Title:        "image.png",
		DownloadLink: "/rest/api/content/123/child/attachment/att456/download",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.HasSuffix(err.Error(), ": ") {
		t.Errorf("error message is empty after the colon: %q", err)
	}
	for _, want := range []string{"404", "Not Found"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestDownloadAttachmentContentFallsBackToRESTPath(t *testing.T) {
	// Some sites answer the legacy media path with 401 even for a valid API
	// token. The REST endpoint is tried next and its content is what we keep.
	var gotPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		if strings.HasPrefix(r.URL.Path, "/wiki/download/") {
			w.Header().Set("WWW-Authenticate", "OAuth")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("<html><body>Log in</body></html>"))
			return
		}
		_, _ = w.Write([]byte("PNGDATA"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@example.com", "token")
	data, err := c.DownloadAttachmentContent(&model.ConfluenceAttachment{
		ID:           "att456",
		Title:        "image.png",
		DownloadLink: "/download/attachments/123/image.png",
	})
	if err != nil {
		t.Fatalf("DownloadAttachmentContent returned error: %v", err)
	}
	if string(data) != "PNGDATA" {
		t.Errorf("content = %q, want %q", data, "PNGDATA")
	}
	if len(gotPaths) != 2 {
		t.Fatalf("requested paths = %v, want the legacy path then the REST path", gotPaths)
	}
	if gotPaths[1] != "/wiki/rest/api/content/123/child/attachment/att456/download" {
		t.Errorf("fallback path = %q", gotPaths[1])
	}
}

func TestDownloadAttachmentContentSkipsDuplicateFallback(t *testing.T) {
	// A v2-form download link normalizes to the same URL the REST fallback
	// builds. Retrying it verbatim can never succeed, so it must not be sent.
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"status":404,"code":"NOT_FOUND","title":"Not Found"}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@example.com", "token")
	_, err := c.DownloadAttachmentContent(&model.ConfluenceAttachment{
		ID:           "att456",
		Title:        "image.png",
		DownloadLink: "/rest/api/content/123/child/attachment/att456/download",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if requests != 1 {
		t.Errorf("server received %d requests, want 1", requests)
	}
}

func TestHandleErrorResponseTruncatesUnrecognizedBody(t *testing.T) {
	// A 401 on the media path answers with a full HTML login page, and every
	// failed image prints its own error line.
	body := strings.Repeat("<div>login</div>", 200)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@example.com", "token")
	_, err := c.DownloadAttachmentContent(&model.ConfluenceAttachment{
		ID:           "att456",
		Title:        "image.png",
		DownloadLink: "/rest/api/content/123/child/attachment/att456/download",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if len(err.Error()) > maxErrorBodyChars*2 {
		t.Errorf("error message is %d chars, want the body truncated", len(err.Error()))
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("error %q does not mark the body as truncated", err)
	}
}
