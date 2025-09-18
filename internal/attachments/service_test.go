package attachments

import (
	"errors"
	"testing"

	"github.com/jackchuka/confluence-md/internal/models"
)

type fakeDownloader struct {
	data string
	err  error
	last *models.ConfluenceAttachment
}

func (f *fakeDownloader) DownloadAttachmentContent(att *models.ConfluenceAttachment) ([]byte, error) {
	f.last = att
	if f.err != nil {
		return nil, f.err
	}
	return []byte(f.data), nil
}

func TestServiceResolveReturnsContent(t *testing.T) {
	fetcher := &fakeDownloader{data: "graph TD;"}
	service := NewService(fetcher)
	page := &models.ConfluencePage{
		Attachments: []models.ConfluenceAttachment{
			{Title: "diagram.mmd", Version: 1, MediaType: "text/plain"},
		},
	}

	content, err := service.Resolve(page, "diagram", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "graph TD;" {
		t.Fatalf("unexpected content: %q", content)
	}
	if fetcher.last.Title != "diagram.mmd" {
		t.Fatalf("unexpected attachment requested: %s", fetcher.last.Title)
	}
}

func TestServiceResolveChoosesBestAttachment(t *testing.T) {
	page := &models.ConfluencePage{
		Attachments: []models.ConfluenceAttachment{
			{Title: "diagram.png", Version: 5, MediaType: "image/png"},
			{Title: "diagram.mmd", Version: 4, MediaType: "text/plain"},
		},
	}

	attachment := selectAttachment(page.Attachments, "diagram", 0)
	if attachment == nil {
		t.Fatal("expected attachment")
	}
	if attachment.Title != "diagram.mmd" {
		t.Fatalf("expected text attachment, got %s", attachment.Title)
	}
}

func TestServiceResolveHonoursRevision(t *testing.T) {
	page := &models.ConfluencePage{
		Attachments: []models.ConfluenceAttachment{
			{Title: "diagram.mmd", Version: 1, MediaType: "text/plain"},
			{Title: "diagram.mmd", Version: 3, MediaType: "text/plain"},
		},
	}

	attachment := selectAttachment(page.Attachments, "diagram", 1)
	if attachment == nil || attachment.Version != 1 {
		t.Fatalf("expected version 1, got %#v", attachment)
	}

	attachmentLatest := selectAttachment(page.Attachments, "diagram", 0)
	if attachmentLatest == nil || attachmentLatest.Version != 3 {
		t.Fatalf("expected highest version, got %#v", attachmentLatest)
	}
}

func TestServiceResolveErrorsWhenMissing(t *testing.T) {
	service := NewService(&fakeDownloader{})
	page := &models.ConfluencePage{}

	if _, err := service.Resolve(page, "diagram", 0); err == nil {
		t.Fatal("expected error when attachment missing")
	}
}

func TestServicePropagatesDownloadError(t *testing.T) {
	fetcher := &fakeDownloader{err: errors.New("boom")}
	service := NewService(fetcher)
	page := &models.ConfluencePage{
		Attachments: []models.ConfluenceAttachment{
			{Title: "diagram.mmd", Version: 0, MediaType: "text/plain"},
		},
	}

	if _, err := service.Resolve(page, "diagram", 0); err == nil || err.Error() != "boom" {
		t.Fatalf("expected download error, got %v", err)
	}
}
