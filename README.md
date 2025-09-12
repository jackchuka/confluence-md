# confluence-md

A CLI tool to convert Confluence pages to Markdown format with support for images and page trees.

## Features

- Convert single Confluence pages to Markdown
- Convert entire page trees with hierarchical structure
- Download and embed images from Confluence pages
- Support for Confluence Cloud with API authentication
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
confluence-md convert <page-url> --email your-email@example.com --api-token your-api-token
```

Example:

```bash
confluence-md convert https://example.atlassian.net/wiki/spaces/SPACE/pages/12345/Title \
  --email john.doe@company.com \
  --api-token your-api-token-here
```

### Convert a Page Tree

Convert an entire page hierarchy:

```bash
confluence-md tree <page-url> --email your-email@example.com --api-token your-api-token
```

### Options

- `--output, -o`: Output directory (default: current directory)
- `--download-images`: Download images from Confluence (default: true)
- `--email`: Your Confluence email address (required)
- `--api-token`: Your Confluence API token (required)

### Examples

```bash
# Convert to a specific directory
confluence-md convert <page-url> --email user@example.com --api-token token --output ./docs

# Convert without downloading images
confluence-md convert <page-url> --email user@example.com --api-token token --download-images=false

# Convert entire page tree
confluence-md tree <page-url> --email user@example.com --api-token token --output ./wiki
```

## Supported Confluence Elements

### Basic Elements

| Element       | Confluence Tag       | Conversion                                                  |
| ------------- | -------------------- | ----------------------------------------------------------- |
| **Images**    | `ac:image`           | Downloaded and converted to local markdown image references |
| **Emoticons** | `ac:emoticon`        | Converted to emoji shortnames or fallback text              |
| **Tables**    | Standard HTML tables | Full table support with proper markdown formatting          |
| **Lists**     | Standard HTML lists  | Nested lists with proper indentation                        |

### Macros (`ac:structured-macro`)

| Macro            | Status                 | Conversion                                                          |
| ---------------- | ---------------------- | ------------------------------------------------------------------- |
| **`info`**       | ✅ Fully Supported     | Converted to blockquote with ℹ️ Info prefix                         |
| **`warning`**    | ✅ Fully Supported     | Converted to blockquote with ⚠️ Warning prefix                      |
| **`note`**       | ✅ Fully Supported     | Converted to blockquote with 📝 Note prefix                         |
| **`tip`**        | ✅ Fully Supported     | Converted to blockquote with 💡 Tip prefix                          |
| **`code`**       | ✅ Fully Supported     | Converted to markdown code blocks with language syntax highlighting |
| **`expand`**     | ✅ Fully Supported     | Content extracted and rendered directly                             |
| **`toc`**        | ✅ Fully Supported     | Converted to `<!-- Table of Contents -->` comment                   |
| **`children`**   | ✅ Fully Supported     | Converted to `<!-- Child Pages -->` comment                         |
| **Other macros** | ⚠️ Partially Supported | Converted to `<!-- Unsupported macro: {name} -->` comments          |

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
