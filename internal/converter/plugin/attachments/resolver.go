package attachments

import (
	"fmt"
	"strings"

	"github.com/jackchuka/confluence-md/internal/confluence/client"
	"github.com/jackchuka/confluence-md/internal/confluence/model"
)

// Resolver provides attachment content for macros such as mermaid.
type Resolver interface {
	Resolve(page *model.ConfluencePage, filename string, revision int) (string, error)
}

// service implements Resolver using a Confluence content downloader.
type service struct {
	client *client.Client
}

// NewService constructs a new attachment service.
func NewService(client *client.Client) *service {
	return &service{client: client}
}

// Resolve locates the best matching attachment on the given page and returns its content.
func (s *service) Resolve(page *model.ConfluencePage, filename string, revision int) (string, error) {
	if s == nil {
		return "", fmt.Errorf("attachment downloader is not configured")
	}

	if page == nil {
		return "", fmt.Errorf("page context not provided")
	}

	attachment := selectAttachment(page.Attachments, filename, revision)
	if attachment == nil {
		return "", fmt.Errorf("attachment %s not found", filename)
	}

	data, err := s.client.DownloadAttachmentContent(attachment)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func selectAttachment(attachments []model.ConfluenceAttachment, filename string, revision int) *model.ConfluenceAttachment {
	for i := range attachments {
		attachment := &attachments[i]
		if strings.EqualFold(attachment.Title, filename) {
			return attachment
		}
	}

	return nil
}
