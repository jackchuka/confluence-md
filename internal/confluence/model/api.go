package model

import (
	"time"
)

// ConfluenceAPIPage represents the API response structure for a page
type ConfluenceAPIPage struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Title  string `json:"title"`
	Body   struct {
		Storage struct {
			Value          string `json:"value"`
			Representation string `json:"representation"`
		} `json:"storage"`
	} `json:"body"`
	Version struct {
		Number int       `json:"number"`
		When   time.Time `json:"when"`
		By     struct {
			Type        string `json:"type"`
			AccountID   string `json:"accountId"`
			DisplayName string `json:"displayName"`
			Email       string `json:"email"`
		} `json:"by"`
	} `json:"version"`
	Space struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"space"`
	History struct {
		CreatedDate time.Time `json:"createdDate"`
		CreatedBy   struct {
			Type        string `json:"type"`
			AccountID   string `json:"accountId"`
			DisplayName string `json:"displayName"`
			Email       string `json:"email"`
		} `json:"createdBy"`
	} `json:"history"`
	Metadata struct {
		Labels struct {
			Results []struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Prefix string `json:"prefix"`
			} `json:"results"`
		} `json:"labels"`
	} `json:"metadata"`
	Children struct {
		Attachment struct {
			Results []struct {
				ID      string `json:"id"`
				Title   string `json:"title"`
				Version struct {
					Number int `json:"number"`
				} `json:"version"`
				Extensions struct {
					MediaType string `json:"mediaType"`
					FileSize  int64  `json:"fileSize"`
				} `json:"extensions"`
				Links struct {
					Download string `json:"download"`
				} `json:"_links"`
			} `json:"results"`
		} `json:"attachment"`
	} `json:"children"`
}

// ConfluenceSearchResult represents the API response for search queries
type ConfluenceSearchResult struct {
	Results []ConfluenceAPIPage `json:"results"`
	Start   int                 `json:"start"`
	Limit   int                 `json:"limit"`
	Size    int                 `json:"size"`
}

// ConfluenceErrorResponse represents an error response from the API
type ConfluenceErrorResponse struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Reason     string `json:"reason"`
}

// ConfluenceUser represents a Confluence user from the API
type ConfluenceUser struct {
	Type        string `json:"type"`
	AccountID   string `json:"accountId"`
	AccountType string `json:"accountType"`
	Email       string `json:"email"`
	PublicName  string `json:"publicName"`
	DisplayName string `json:"displayName"`
}

// ConvertAPIPageToModel converts the API response to our domain model
func ConvertAPIPageToModel(apiPage *ConfluenceAPIPage) *ConfluencePage {
	// Convert labels
	var labels []Label
	for _, apiLabel := range apiPage.Metadata.Labels.Results {
		labels = append(labels, Label{
			ID:   apiLabel.ID,
			Name: apiLabel.Name,
		})
	}

	var attachments []ConfluenceAttachment
	for _, att := range apiPage.Children.Attachment.Results {
		attachments = append(attachments, ConfluenceAttachment{
			ID:           att.ID,
			Title:        att.Title,
			MediaType:    att.Extensions.MediaType,
			FileSize:     att.Extensions.FileSize,
			DownloadLink: att.Links.Download,
			Version:      att.Version.Number,
		})
	}

	return &ConfluencePage{
		ID:       apiPage.ID,
		Title:    apiPage.Title,
		SpaceKey: apiPage.Space.Key,
		Version:  apiPage.Version.Number,
		Content: ConfluenceContent{
			Storage: ContentStorage{
				Value:          apiPage.Body.Storage.Value,
				Representation: apiPage.Body.Storage.Representation,
			},
		},
		Metadata: ConfluenceMetadata{
			Labels:     labels,
			Properties: make(map[string]string), // TODO: Extract properties if needed
		},
		Attachments: attachments,
		CreatedAt:   apiPage.History.CreatedDate,
		UpdatedAt:   apiPage.Version.When,
		CreatedBy: User{
			AccountID:   apiPage.History.CreatedBy.AccountID,
			DisplayName: apiPage.History.CreatedBy.DisplayName,
			Email:       apiPage.History.CreatedBy.Email,
		},
		UpdatedBy: User{
			AccountID:   apiPage.Version.By.AccountID,
			DisplayName: apiPage.Version.By.DisplayName,
			Email:       apiPage.Version.By.Email,
		},
	}
}
