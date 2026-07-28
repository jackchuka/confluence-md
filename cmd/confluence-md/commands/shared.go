package commands

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gosimple/slug"
	"github.com/jackchuka/confluence-md/internal/confluence"
	confluenceModel "github.com/jackchuka/confluence-md/internal/confluence/model"
	"github.com/jackchuka/confluence-md/internal/converter"
)

// sanitizeFileName uses the mature gosimple/slug library for robust filename sanitization
func sanitizeFileName(name string) string {
	if name == "" {
		return "untitled"
	}

	sanitized := slug.MakeLang(name, "en")

	if sanitized == "" {
		return name
	}

	return sanitized
}

func buildOutputNamer(template string) (converter.OutputNamer, error) {
	if strings.TrimSpace(template) == "" {
		return nil, nil
	}

	namer, err := converter.NewTemplateOutputNamer(template)
	if err != nil {
		return nil, err
	}

	return namer, nil
}

// PageConversionResult represents the result of converting a single page
type PageConversionResult struct {
	OutputPath  string
	PageID      string
	Title       string
	ImagesCount int
	Success     bool
	Error       error
}

// convertSinglePage handles the full conversion pipeline for a single page
func convertSinglePage(client confluence.Client, page *confluenceModel.ConfluencePage, site confluenceModel.SiteInfo, opts PageOptions) *PageConversionResult {
	return convertSinglePageWithPath(client, page, site, "", opts)
}

// convertSinglePageWithPath handles conversion with a custom output path (for tree structure)
func convertSinglePageWithPath(client confluence.Client, page *confluenceModel.ConfluencePage, site confluenceModel.SiteInfo, outputPath string, opts PageOptions) *PageConversionResult {
	result := &PageConversionResult{
		PageID: page.ID,
		Title:  page.Title,
	}

	if outputPath == "" {
		fileName, err := converter.GenerateFileName(page, opts.OutputNamer)
		if err != nil {
			result.Error = fmt.Errorf("failed to generate output filename: %w", err)
			return result
		}
		outputPath = filepath.Join(opts.OutputDir, fileName)
	}
	result.OutputPath = outputPath

	// Create converter and convert page
	var options []converter.Option
	if opts.DownloadImages {
		options = append(options, converter.WithDownloadAttachments(opts.ImageFolder))
	}
	conv := converter.NewConverter(client, options...)
	doc, err := conv.ConvertPage(page, site, filepath.Dir(outputPath))
	if err != nil {
		result.Error = fmt.Errorf("failed to convert page: %w", err)
		return result
	}
	result.ImagesCount = len(doc.Images)

	if err := converter.SaveMarkdownDocument(doc, outputPath, opts.IncludeMetadata); err != nil {
		result.Error = fmt.Errorf("failed to save document: %w", err)
		return result
	}

	result.Success = true
	return result
}

// printConversionResult prints the result of a page conversion in a consistent format
func printConversionResult(result *PageConversionResult) {
	if result.Success {
		fmt.Printf("✅ Successfully converted page: %s\n", result.OutputPath)
		fmt.Printf("   Page ID: %s\n", result.PageID)
		fmt.Printf("   Title: %s\n", result.Title)
		if result.ImagesCount > 0 {
			fmt.Printf("   📥 Images downloaded: %d\n", result.ImagesCount)
		}
	} else {
		fmt.Printf("❌ Failed to convert page: %s\n", result.Title)
		if result.Error != nil {
			fmt.Printf("   Error: %v\n", result.Error)
		}
	}
	fmt.Println()
}

// routeMarkers are the path segments that separate the instance context path
// from a Confluence route. The context path is everything before the earliest
// marker (e.g. "/wiki" for Cloud, "" or "/confluence" for self-hosted).
var routeMarkers = []string{"/spaces/", "/display/", "/pages/"}

// resolveDeployment determines the deployment type from an explicit override or,
// failing that, from the host (Cloud instances live under *.atlassian.net).
func resolveDeployment(host, override string) (confluenceModel.Deployment, error) {
	switch strings.ToLower(strings.TrimSpace(override)) {
	case "cloud":
		return confluenceModel.DeploymentCloud, nil
	case "server", "datacenter", "dc", "self-hosted":
		return confluenceModel.DeploymentServer, nil
	case "":
		if strings.HasSuffix(strings.ToLower(host), ".atlassian.net") {
			return confluenceModel.DeploymentCloud, nil
		}
		return confluenceModel.DeploymentServer, nil
	default:
		return "", fmt.Errorf("unknown deployment type %q (expected 'cloud' or 'server')", override)
	}
}

// contextPathFromPath returns the path prefix in front of the first Confluence
// route marker, without a trailing slash.
func contextPathFromPath(path string) string {
	earliest := -1
	for _, marker := range routeMarkers {
		if idx := strings.Index(path, marker); idx != -1 && (earliest == -1 || idx < earliest) {
			earliest = idx
		}
	}
	if earliest <= 0 {
		return ""
	}
	return strings.TrimSuffix(path[:earliest], "/")
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func urlToPageInfo(pageURL, typeOverride string) (confluenceModel.PageURLInfo, error) {
	if pageURL == "" {
		return confluenceModel.PageURLInfo{}, fmt.Errorf("URL is empty")
	}

	u, err := url.Parse(pageURL)
	if err != nil {
		return confluenceModel.PageURLInfo{}, fmt.Errorf("invalid URL: %w", err)
	}

	deployment, err := resolveDeployment(u.Host, typeOverride)
	if err != nil {
		return confluenceModel.PageURLInfo{}, err
	}

	baseURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	contextPath := contextPathFromPath(u.Path)

	var pageID, spaceKey, title string

	// Prefer an explicit pageId query parameter (self-hosted viewpage.action links).
	if id := u.Query().Get("pageId"); isNumeric(id) {
		pageID = id
	}

	// Supported path forms:
	//   /wiki/spaces/SPACE/pages/12345/Title   (Cloud)
	//   /spaces/SPACE/pages/12345/Title         (self-hosted, modern)
	//   /display/SPACE/Page+Title               (self-hosted, pretty)
	parts := strings.Split(u.Path, "/")
	for i, part := range parts {
		switch part {
		case "spaces":
			if i+1 < len(parts) {
				spaceKey = parts[i+1]
			}
		case "display":
			if i+1 < len(parts) {
				spaceKey = parts[i+1]
			}
			if i+2 < len(parts) {
				// Confluence encodes spaces as '+' in pretty display URLs.
				title = strings.ReplaceAll(parts[i+2], "+", " ")
			}
		case "pages":
			if pageID == "" && i+1 < len(parts) && isNumeric(parts[i+1]) {
				pageID = parts[i+1]
			}
			if i+2 < len(parts) && parts[i+2] != "" {
				title = parts[i+2]
			}
		}
	}

	info := confluenceModel.PageURLInfo{
		BaseURL:     baseURL,
		PageID:      pageID,
		SpaceKey:    spaceKey,
		Title:       title,
		Deployment:  deployment,
		ContextPath: contextPath,
	}

	// A missing page ID is only recoverable for self-hosted pretty URLs, where
	// the caller can resolve it from the space key and title via the API.
	if pageID == "" {
		if deployment == confluenceModel.DeploymentServer && spaceKey != "" && title != "" {
			return info, nil
		}
		return confluenceModel.PageURLInfo{}, fmt.Errorf("could not extract page ID from URL")
	}

	return info, nil
}

// newClientForAuth validates authentication for the resolved deployment and
// builds a Confluence client.
func newClientForAuth(info confluenceModel.PageURLInfo, auth authOptions) (confluence.Client, error) {
	if auth.APIKey == "" {
		return nil, fmt.Errorf("an API token is required (use --api-token)")
	}

	cfg := confluence.Config{
		BaseURL:     info.BaseURL,
		Deployment:  info.Deployment,
		ContextPath: info.ContextPath,
	}

	if info.Deployment == confluenceModel.DeploymentServer {
		// Self-hosted uses a Personal Access Token via Bearer auth.
		cfg.Token = auth.APIKey
	} else {
		if auth.Email == "" {
			return nil, fmt.Errorf("an email is required for Confluence Cloud (use --email)")
		}
		cfg.Email = auth.Email
		cfg.APIToken = auth.APIKey
	}

	return confluence.NewClient(cfg), nil
}

// resolvePageID fills in a missing page ID by looking it up from the space key
// and title (used for self-hosted pretty URLs).
func resolvePageID(client confluence.Client, info *confluenceModel.PageURLInfo) error {
	if info.PageID != "" {
		return nil
	}

	id, err := client.FindPageID(info.SpaceKey, info.Title)
	if err != nil {
		return err
	}
	info.PageID = id
	return nil
}
