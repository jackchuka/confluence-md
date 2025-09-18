package attachments

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jackchuka/confluence-md/internal/models"
)

// ContentDownloader defines the capability needed to fetch attachment content.
type ContentDownloader interface {
	DownloadAttachmentContent(attachment *models.ConfluenceAttachment) ([]byte, error)
}

// Resolver provides attachment content for macros such as mermaid.
type Resolver interface {
	Resolve(page *models.ConfluencePage, filename string, revision int) (string, error)
}

// Service implements Resolver using a Confluence content downloader.
type Service struct {
	downloader ContentDownloader
}

// NewService constructs a new attachment service.
func NewService(downloader ContentDownloader) *Service {
	return &Service{downloader: downloader}
}

// Resolve locates the best matching attachment on the given page and returns its content.
func (s *Service) Resolve(page *models.ConfluencePage, filename string, revision int) (string, error) {
	if s == nil || s.downloader == nil {
		return "", fmt.Errorf("attachment downloader is not configured")
	}

	if page == nil {
		return "", fmt.Errorf("page context not provided")
	}

	attachment := selectAttachment(page.Attachments, filename, revision)
	if attachment == nil {
		return "", fmt.Errorf("attachment %s not found", filename)
	}

	data, err := s.downloader.DownloadAttachmentContent(attachment)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func selectAttachment(attachments []models.ConfluenceAttachment, filename string, revision int) *models.ConfluenceAttachment {
	var candidates []*models.ConfluenceAttachment
	for i := range attachments {
		attachment := &attachments[i]
		if !matchesAttachmentFilename(attachment.Title, filename) {
			continue
		}

		if revision > 0 && attachment.Version > 0 && attachment.Version != revision {
			continue
		}

		candidates = append(candidates, attachment)
	}

	if len(candidates) == 0 {
		return nil
	}

	best := candidates[0]
	bestScore := attachmentPreferenceScore(best)
	for _, candidate := range candidates[1:] {
		score := attachmentPreferenceScore(candidate)
		if score > bestScore || (score == bestScore && candidate.Version > best.Version) {
			best = candidate
			bestScore = score
		}
	}

	return best
}

func matchesAttachmentFilename(attachmentTitle, filename string) bool {
	if attachmentTitle == "" || filename == "" {
		return false
	}

	if strings.EqualFold(attachmentTitle, filename) {
		return true
	}

	if !strings.Contains(filename, ".") {
		return strings.EqualFold(strings.TrimSuffix(attachmentTitle, filepath.Ext(attachmentTitle)), filename)
	}

	return false
}

func attachmentPreferenceScore(att *models.ConfluenceAttachment) int {
	if att == nil {
		return -1000
	}

	score := 0
	mediaType := strings.ToLower(att.MediaType)
	if strings.Contains(mediaType, "text") || strings.Contains(mediaType, "json") {
		score += 100
	}

	if strings.HasPrefix(mediaType, "image/") {
		score -= 100
	}

	ext := strings.ToLower(filepath.Ext(att.Title))
	switch ext {
	case ".mmd", ".mermaid", ".txt", ".md", ".json":
		score += 80
	case ".png", ".jpg", ".jpeg", ".gif", ".svg":
		score -= 50
	}

	return score
}
