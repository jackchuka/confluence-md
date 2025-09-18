package downloader

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackchuka/confluence-md/internal/converter/model"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestDownloadImages(t *testing.T) {
	data := bytes.Repeat([]byte("a"), 8)

	doc := &model.MarkdownDocument{
		Images: []model.ImageRef{{
			OriginalURL: "https://example.com/image.png",
			LocalPath:   "assets/image.png",
			FileName:    "image.png",
		}},
	}

	d := NewDownloader("user@example.com", "token")
	d.maxSize = int64(len(data))
	d.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			user, token, ok := r.BasicAuth()
			if !ok || user != "user@example.com" || token != "token" {
				t.Fatalf("unexpected auth: %s %s", user, token)
			}
			if r.URL.String() != "https://example.com/image.png" {
				t.Fatalf("unexpected url: %s", r.URL)
			}
			body := io.NopCloser(bytes.NewReader(data))
			return &http.Response{
				StatusCode:    http.StatusOK,
				Body:          body,
				Header:        http.Header{"Content-Type": []string{"image/png"}},
				ContentLength: int64(len(data)),
			}, nil
		}),
	}

	dir := t.TempDir()
	if err := d.DownloadImages(doc, dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	filePath := filepath.Join(dir, "assets", "image.png")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if !bytes.Equal(content, data) {
		t.Fatalf("unexpected file contents: %q", content)
	}
	if doc.Images[0].ContentType != "image/png" {
		t.Fatalf("expected content type saved")
	}
}

func TestDownloadImagesHandlesErrors(t *testing.T) {
	tests := []struct {
		name      string
		transport roundTripFunc
		wantErr   string
	}{
		{
			name: "http error",
			transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader("boom")),
					Status:     "500 Internal Server Error",
				}, nil
			}),
			wantErr: "HTTP 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &model.MarkdownDocument{
				Images: []model.ImageRef{{
					OriginalURL: "https://example.com/image.png",
					LocalPath:   "assets/image.png",
					FileName:    "image.png",
				}},
			}

			d := NewDownloader("user", "token")
			d.httpClient = &http.Client{Transport: tt.transport}
			d.maxSize = 8

			err := d.DownloadImages(doc, t.TempDir())
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestDownloadImagesSkipsWhenNoImages(t *testing.T) {
	d := NewDownloader("user", "token")
	if err := d.DownloadImages(&model.MarkdownDocument{}, t.TempDir()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
