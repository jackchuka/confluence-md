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
	DownloadAttachment(page *model.ConfluencePage, filename string, revision int) (*model.ConfluenceAttachment, []byte, error)
}

// Service implements Resolver using a Confluence content downloader.
type Service struct {
	client *client.Client
}

// NewService constructs a new attachment service.
func NewService(client *client.Client) *Service {
	return &Service{client: client}
}

// Resolve locates the best matching attachment on the given page and returns its content.
func (s *Service) Resolve(page *model.ConfluencePage, filename string, revision int) (string, error) {
	if s == nil {
		return "", fmt.Errorf("attachment downloader is not configured")
	}

	if s.client == nil {
		return "", fmt.Errorf("attachment downloader client is not configured")
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

// DownloadAttachment retrieves attachment bytes for the given filename and optional revision.
func (s *Service) DownloadAttachment(page *model.ConfluencePage, filename string, revision int) (*model.ConfluenceAttachment, []byte, error) {
	if s == nil {
		return nil, nil, fmt.Errorf("attachment downloader is not configured")
	}

	if s.client == nil {
		return nil, nil, fmt.Errorf("attachment downloader client is not configured")
	}

	if page == nil {
		return nil, nil, fmt.Errorf("page context not provided")
	}

	attachment := selectAttachment(page.Attachments, filename, revision)
	if attachment == nil {
		return nil, nil, fmt.Errorf("attachment %s not found", filename)
	}

	data, err := s.client.DownloadAttachmentContent(attachment)
	if err != nil {
		return nil, nil, err
	}

	return attachment, data, nil
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
