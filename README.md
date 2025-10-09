# confluence-md

[![Test](https://github.com/jackchuka/confluence-md/workflows/Test/badge.svg)](https://github.com/jackchuka/confluence-md/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/jackchuka/confluence-md)](https://goreportcard.com/report/github.com/jackchuka/confluence-md)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A CLI tool to convert Confluence pages to Markdown format with a single command. Supports images, tables, lists, and various macros (**yes, even mermaid diagrams!**).

## Features

- Convert single Confluence pages to Markdown
- Convert entire page trees with hierarchical structure
- Download and embed images from Confluence pages
- Support for Confluence Cloud with API authentication
- Enhanced support for Confluence-specific elements (user references, status badges, time elements)
- Clean, readable Markdown output
- Cross-platform support (Linux, macOS, Windows)

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/jackchuka/confluence-md/releases).

### From Source

```bash
go install github.com/jackchuka/confluence-md/cmd/confluence-md@latest
```

## Usage

### Authentication

You'll need:

- Your Confluence email address
- A Confluence API token ([create one here](https://id.atlassian.com/manage-profile/security/api-tokens))

### Convert a Single Page

```bash
confluence-md page <page-url> --email your-email@example.com --api-token your-api-token
```

Example:

```bash
confluence-md page https://example.atlassian.net/wiki/spaces/SPACE/pages/12345/Title \
  --email john.doe@company.com \
  --api-token your-api-token-here
```

### Convert a Page Tree

Convert an entire page hierarchy:

```bash
confluence-md tree <page-url> --email your-email@example.com --api-token your-api-token
```

### Convert HTML Files

Convert Confluence HTML directly without API access (useful for testing or working with exported HTML):

```bash
# Convert from file
confluence-md html page.html -o output.md

# Convert from stdin
cat page.html | confluence-md html -o output.md

# Output to stdout
confluence-md html page.html
```

### Common Options

- `--email, -e`: Your Confluence email address (**required**)
- `--api-token, -t`: Your Confluence API token (**required**)
- `--output, -o`: Output directory (default: current directory)
- `--output-name-template`: Go template for the markdown filename (see below)
- `--download-images`: Download images from Confluence (default: true)
- `--image-folder`: Folder to save images (default: `assets`)
- `--include-metadata`: Include page metadata in the Markdown front matter (default: true)

### Examples

```bash
# Convert to a specific directory
confluence-md page <page-url> --email user@example.com --api-token token --output ./docs

# Prefix filenames with the last updated date (YYYY-MM-DD-title.md)
confluence-md page <page-url> \
  --email user@example.com \
  --api-token token \
  --output-name-template "{{ .Page.UpdatedAt.Format \"2006-01-02\" }}-{{ .SlugTitle }}"

# Convert without downloading images
confluence-md page <page-url> --email user@example.com --api-token token --download-images=false

# Convert entire page tree
confluence-md tree <page-url> --email user@example.com --api-token token --output ./wiki
```

### Output name templates

The `--output-name-template` flag accepts a Go text/template string. Templates can reference:

- `{{ .Page }}` – the full Confluence page object (e.g. `{{ .Page.UpdatedAt.Format "2006-01-02" }}`)
  - `{{ .Page.Title }}` – the original page title
  - `{{ .Page.ID }}` – the Confluence page ID
  - `{{ .Page.SpaceKey }}` – the Confluence space key
  - see ConfluencePage struct for more fields
- `{{ .SlugTitle }}` – the default slugified title (e.g. `sample-page`)

Additionally, you can use the following helper functions:

- `{{ slug <string> }}` – slugifies a string (e.g. `Sample Page` → `sample-page`)

If the rendered filename omits an extension, `.md` is appended automatically.

## Supported Confluence Elements

### Basic Elements

| Element             | Confluence Tag             | Conversion                                                              |
| ------------------- | -------------------------- | ----------------------------------------------------------------------- |
| **Images**          | `ac:image`                 | Downloaded and converted to local markdown image references             |
| **Emoticons**       | `ac:emoticon`              | Converted to emoji fallback or shortnames                               |
| **Tables**          | Standard HTML tables       | Full table support with proper markdown formatting                      |
| **Lists**           | Standard HTML lists        | Nested lists with proper indentation                                    |
| **User Links**      | `ac:link` + `ri:user`      | Converted to `@DisplayName` (or `@user(account-id)` if name not cached) |
| **Time Elements**   | `<time>`                   | Datetime attribute extracted and displayed                              |
| **Inline Comments** | `ac:inline-comment-marker` | Text preserved with comment reference                                   |
| **Placeholders**    | `ac:placeholder`           | Converted to HTML comments                                              |

### Macros (`ac:structured-macro`)

| Macro               | Status                      | Conversion                                                          |
| ------------------- | --------------------------- | ------------------------------------------------------------------- |
| **`info`**          | ✅ Fully Supported          | Converted to blockquote with ℹ️ Info prefix                         |
| **`warning`**       | ✅ Fully Supported          | Converted to blockquote with ⚠️ Warning prefix                      |
| **`note`**          | ✅ Fully Supported          | Converted to blockquote with 📝 Note prefix                         |
| **`tip`**           | ✅ Fully Supported          | Converted to blockquote with 💡 Tip prefix                          |
| **`code`**          | ✅ Fully Supported          | Converted to markdown code blocks with language syntax highlighting |
| **`mermaid-cloud`** | ✅ Fully Supported          | Converted to mermaid code blocks                                    |
| **`expand`**        | ✅ Fully Supported          | Content extracted and rendered directly                             |
| **`details`**       | ✅ Fully Supported          | Content extracted and rendered directly                             |
| **`status`**        | ✅ Fully Supported          | Converted to emoji badges (🔴 **S1**, 🟡, 🟢, 🔵, ⚪)               |
| **`toc`**           | ⚠️ Partially Supported      | Converted to `<!-- Table of Contents -->` comment                   |
| **`children`**      | ⚠️ Partially Supported      | Converted to `<!-- Child Pages -->` comment                         |
| **Other macros**    | Plan to support per request | Converted to `<!-- Unsupported macro: {name} -->` comments          |

### User Name Resolution

User references (`@user`) are automatically resolved to display names when converting pages via the `page` or `tree` commands

**Note:** When using the `html` command (without Confluence API access), user names cannot be resolved and will always display as `@user(account-id)`.

## Output Structure

The tool creates:

- Markdown files (.md) for each page
- An `assets/` directory containing downloaded images
- Hierarchical directory structure for page trees

## Development

### Prerequisites

- Go 1.24.4 or later

### Building

```bash
git clone https://github.com/jackchuka/confluence-md.git
cd confluence-md
go build -o confluence-md cmd/confluence-md/main.go
```

### Testing

```bash
go test ./...
```

### Linting

```bash
golangci-lint run
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Run tests and linting
6. Submit a pull request

## Support

For issues and feature requests, please use the [GitHub issue tracker](https://github.com/jackchuka/confluence-md/issues).
