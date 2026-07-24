package model

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Deployment identifies which flavor of Confluence an instance is.
type Deployment string

const (
	// DeploymentCloud is Atlassian-hosted Confluence Cloud (*.atlassian.net).
	DeploymentCloud Deployment = "cloud"
	// DeploymentServer is a self-hosted Confluence Server / Data Center instance.
	DeploymentServer Deployment = "server"
)

// SiteInfo captures everything needed to address a specific Confluence instance,
// independent of any single page.
type SiteInfo struct {
	// BaseURL is the scheme and host only, e.g. "https://wiki.example.com".
	BaseURL string
	// Deployment is the flavor of the instance (cloud or server).
	Deployment Deployment
	// ContextPath is the path prefix in front of Confluence routes and the REST
	// API, without a trailing slash. It is "/wiki" for Cloud and typically ""
	// (or e.g. "/confluence") for self-hosted instances.
	ContextPath string
}

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

// User represents a Confluence user.
//
// Cloud identifies users by AccountID, whereas self-hosted (Server/Data Center)
// identifies them by UserKey and/or Username.
type User struct {
	AccountID   string `json:"accountId"`
	UserKey     string `json:"userKey,omitempty"`
	Username    string `json:"username,omitempty"`
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

// GetURL constructs the Confluence page URL for the given site.
//
// Cloud pages live under the "/wiki" context at /spaces/.../pages/ID, while
// self-hosted (Server/Data Center) pages are addressed via the version-agnostic
// /pages/viewpage.action?pageId=ID endpoint under the instance context path.
func (cp *ConfluencePage) GetURL(site SiteInfo) (string, error) {
	base, err := url.Parse(site.BaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	root := strings.TrimSuffix(base.String(), "/") + site.ContextPath

	if site.Deployment == DeploymentServer {
		return fmt.Sprintf("%s/pages/viewpage.action?pageId=%s", root, cp.ID), nil
	}

	return fmt.Sprintf("%s/spaces/%s/pages/%s/%s",
		root, cp.SpaceKey, cp.ID, url.PathEscape(cp.Title)), nil
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
	BaseURL     string
	SpaceKey    string
	PageID      string
	Title       string
	Deployment  Deployment
	ContextPath string
}

// Site returns the SiteInfo described by this URL.
func (p PageURLInfo) Site() SiteInfo {
	return SiteInfo{
		BaseURL:     p.BaseURL,
		Deployment:  p.Deployment,
		ContextPath: p.ContextPath,
	}
}
