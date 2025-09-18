package models

import (
	"fmt"
	"net/url"
	"time"
)

// ConfluencePage represents a page fetched from Confluence API
type ConfluencePage struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	SpaceKey    string                 `json:"spaceKey"`
	Version     int                    `json:"version"`
	Content     ConfluenceContent      `json:"body"`
	Metadata    ConfluenceMetadata     `json:"metadata"`
	Attachments []ConfluenceAttachment `json:"attachments"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
	CreatedBy   User                   `json:"createdBy"`
	UpdatedBy   User                   `json:"updatedBy"`
}

// ConfluenceContent represents the content structure from Confluence
type ConfluenceContent struct {
	Storage ContentStorage `json:"storage"`
}

// ContentStorage represents the storage format of Confluence content
type ContentStorage struct {
	Value          string `json:"value"`          // HTML content
	Representation string `json:"representation"` // Always "storage"
}

// ConfluenceMetadata contains page metadata from Confluence
type ConfluenceMetadata struct {
	Labels     []Label           `json:"labels"`
	Properties map[string]string `json:"properties"`
}

// Label represents a Confluence page label
type Label struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ConfluenceAttachment represents a file attachment on a Confluence page
type ConfluenceAttachment struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	MediaType    string `json:"mediaType"`
	FileSize     int64  `json:"fileSize"`
	DownloadLink string `json:"downloadLink"`
	Version      int    `json:"version"`
}

// User represents a Confluence user
type User struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email,omitempty"`
}

// Validate validates the ConfluencePage model
func (cp *ConfluencePage) Validate() error {
	if cp.ID == "" {
		return fmt.Errorf("page ID cannot be empty")
	}

	if cp.Title == "" {
		return fmt.Errorf("page title cannot be empty")
	}

	if cp.Content.Storage.Value == "" {
		return fmt.Errorf("page content cannot be empty")
	}

	if cp.SpaceKey == "" {
		return fmt.Errorf("space key cannot be empty")
	}

	// Validate attachments
	for i, attachment := range cp.Attachments {
		if err := attachment.Validate(); err != nil {
			return fmt.Errorf("invalid attachment at index %d: %w", i, err)
		}
	}

	return nil
}

// GetURL constructs the Confluence page URL
func (cp *ConfluencePage) GetURL(baseURL string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	pageURL := fmt.Sprintf("%s/wiki/spaces/%s/pages/%s/%s",
		base.String(), cp.SpaceKey, cp.ID, url.PathEscape(cp.Title))

	return pageURL, nil
}

// GetLabelNames returns a slice of label names
func (cp *ConfluencePage) GetLabelNames() []string {
	names := make([]string, len(cp.Metadata.Labels))
	for i, label := range cp.Metadata.Labels {
		names[i] = label.Name
	}
	return names
}

// Validate validates the ConfluenceAttachment
func (ca *ConfluenceAttachment) Validate() error {
	if ca.ID == "" {
		return fmt.Errorf("attachment ID cannot be empty")
	}

	if ca.Title == "" {
		return fmt.Errorf("attachment title cannot be empty")
	}

	if ca.MediaType == "" {
		return fmt.Errorf("attachment media type cannot be empty")
	}

	if ca.FileSize <= 0 {
		return fmt.Errorf("attachment file size must be greater than 0")
	}

	if ca.DownloadLink == "" {
		return fmt.Errorf("attachment download link cannot be empty")
	}

	// Validate download link is a valid URL
	if _, err := url.Parse(ca.DownloadLink); err != nil {
		return fmt.Errorf("invalid download link: %w", err)
	}

	return nil
}

// PageURLInfo contains information extracted from a Confluence page URL
type PageURLInfo struct {
	BaseURL  string
	SpaceKey string
	PageID   string
	Title    string
}
