package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jackchuka/confluence-md/internal/models"
)

// Downloader handles downloading images and attachments
type Downloader struct {
	httpClient *http.Client
	maxSize    int64
	email      string
	apiToken   string
}

// NewDownloader creates a new downloader with authentication
func NewDownloader(email, apiToken string) *Downloader {
	return &Downloader{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxSize:  int64(10 * 1024 * 1024),
		email:    email,
		apiToken: apiToken,
	}
}

// DownloadImages downloads all images referenced in a markdown document
func (d *Downloader) DownloadImages(doc *models.MarkdownDocument, outputDir string) error {
	if len(doc.Images) == 0 {
		return nil
	}

	// Create image directory
	imageDir := filepath.Join(outputDir, filepath.Dir(doc.Images[0].LocalPath))
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		return fmt.Errorf("failed to create image directory: %w", err)
	}

	// Download each image
	for i := range doc.Images {
		imageRef := &doc.Images[i]
		if err := d.downloadImage(imageRef, outputDir); err != nil {
			return fmt.Errorf("failed to download image %s: %w", imageRef.OriginalURL, err)
		}
	}

	return nil
}

// downloadImage downloads a single image
func (d *Downloader) downloadImage(imageRef *models.ImageRef, outputDir string) error {
	// Create HTTP request
	req, err := http.NewRequest("GET", imageRef.OriginalURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.SetBasicAuth(d.email, d.apiToken)

	// Make HTTP request
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Check content length
	if resp.ContentLength > d.maxSize {
		return fmt.Errorf("image too large: %d bytes (max %d)", resp.ContentLength, d.maxSize)
	}

	// Update image info
	imageRef.ContentType = resp.Header.Get("Content-Type")
	imageRef.Size = resp.ContentLength

	// Create file path
	filePath := filepath.Join(outputDir, imageRef.LocalPath)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	// Copy data with size limit
	_, err = io.CopyN(file, resp.Body, d.maxSize)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
