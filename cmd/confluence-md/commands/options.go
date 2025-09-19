package commands

import (
	"github.com/spf13/cobra"
)

type authOptions struct {
	APIKey string
	Email  string
}

func (a *authOptions) InitFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&a.APIKey, "api-token", "t", "", "Confluence API token (required)")
	cmd.Flags().StringVarP(&a.Email, "email", "e", "", "Confluence user email (default: extracted from URL)")
}

type commonOptions struct {
	DownloadImages     bool
	ImageFolder        string
	IncludeMetadata    bool
	OutputDir          string
	OutputNameTemplate string
}

func (c *commonOptions) InitFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&c.DownloadImages, "download-images", true, "Download images locally")
	cmd.Flags().StringVar(&c.ImageFolder, "image-folder", "assets", "Folder for downloaded images")
	cmd.Flags().BoolVar(&c.IncludeMetadata, "include-metadata", true, "Include YAML frontmatter")
	cmd.Flags().StringVarP(&c.OutputDir, "output", "o", "./output", "Output directory")
	cmd.Flags().StringVar(&c.OutputNameTemplate, "output-name-template", "", "Go template for output filename; available data: {{ .Page.* }}, {{ .SlugTitle }}, {{ .LabelNames }}")
}
