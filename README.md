<div align="center">

# confluence-md

**Convert Confluence pages to clean Markdown with one command.**

Point it at a page URL to get Markdown with front matter and local images,<br>
or convert a whole page tree into a matching folder hierarchy.

[![Test](https://github.com/jackchuka/confluence-md/actions/workflows/test.yml/badge.svg)](https://github.com/jackchuka/confluence-md/actions/workflows/test.yml)
[![Release](https://img.shields.io/github/v/release/jackchuka/confluence-md?sort=semver)](https://github.com/jackchuka/confluence-md/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

<img src="assets/demo.gif" alt="confluence-md converting Confluence storage-format HTML with status, info, code, task list, table and warning elements into Markdown" width="800">

[Installation](#installation) · [Quick Start](#quick-start) · [Commands](#commands) · [Supported Elements](#supported-confluence-elements)

</div>

---

Confluence stores pages in its own XHTML dialect full of `ac:structured-macro`, `ri:user` and `ac:image` tags. Copy-pasting or generic HTML-to-Markdown converters drop or mangle most of it. **confluence-md** understands that format: code macros become fenced code blocks, info and warning panels become blockquotes, status lozenges become badges, user mentions resolve to display names, and attached images are downloaded next to the Markdown file. This works well for migrating docs into a repository or feeding Confluence content to other tools.

## Features

- **Single pages or whole trees**: `page` converts one page; `tree` walks its descendants and mirrors the hierarchy as directories
- **Images included**: attachments are downloaded into an `assets/` folder and linked with relative paths
- **Confluence macros**: code, info/warning/note/tip panels, status, expand, details, and Mermaid diagrams (`mermaid-cloud`)
- **User mentions**: resolved to `@Display Name` via the Confluence API
- **Front matter**: title, author, date, labels, and page ID/space/version/URL in YAML
- **Custom filenames**: name output files with a Go template, e.g. prefix them with the last-updated date
- **Tree controls**: limit depth, exclude pages by title glob, fetch in parallel, or preview with `--dry-run`
- **Offline mode**: `html` converts storage-format HTML from a file or stdin, no API access needed
- **Single binary**: Linux, macOS, and Windows

## Installation

### Homebrew

```bash
brew install jackchuka/tap/confluence-md
```

### Go

```bash
go install github.com/jackchuka/confluence-md/cmd/confluence-md@latest
```

### Binary

Download a prebuilt binary from the [releases page](https://github.com/jackchuka/confluence-md/releases).

## Quick Start

1. **Create an API token** at [id.atlassian.com](https://id.atlassian.com/manage-profile/security/api-tokens). Use it with the email address of your Confluence Cloud account.

2. **Convert a page** by its URL:

   ```bash
   confluence-md page https://example.atlassian.net/wiki/spaces/DOCS/pages/12345/Getting+Started \
     --email you@example.com --api-token "$CONFLUENCE_API_TOKEN"
   ```

   This writes `./output/getting-started.md` and its images to `./output/assets/`.

3. **Convert a page and all its children**, previewing first:

   ```bash
   confluence-md tree <page-url> --email you@example.com --api-token "$CONFLUENCE_API_TOKEN" --dry-run
   confluence-md tree <page-url> --email you@example.com --api-token "$CONFLUENCE_API_TOKEN" --output ./wiki
   ```

## Commands

| Command                     | Description                                                      |
| --------------------------- | ---------------------------------------------------------------- |
| `confluence-md page <url>`  | Convert a single page                                            |
| `confluence-md tree <url>`  | Convert a page and its descendants into a directory hierarchy    |
| `confluence-md html [file]` | Convert storage-format HTML from a file or stdin (no API needed) |
| `confluence-md version`     | Print the version                                                |

### `page` and `tree` options

| Flag                     | Description                                     | Default           |
| ------------------------ | ----------------------------------------------- | ----------------- |
| `-e`, `--email`          | Confluence account email (**required**)         |                   |
| `-t`, `--api-token`      | Confluence API token (**required**)             |                   |
| `-o`, `--output`         | Output directory                                | `./output`        |
| `--output-name-template` | Go template for the output filename (see below) | slug of the title |
| `--download-images`      | Download images locally                         | `true`            |
| `--image-folder`         | Folder for downloaded images                    | `assets`          |
| `--include-metadata`     | Write YAML front matter                         | `true`            |

### `tree` options

| Flag         | Description                                       | Default |
| ------------ | ------------------------------------------------- | ------- |
| `--depth`    | Maximum depth to traverse (`-1` for unlimited)    | `-1`    |
| `--exclude`  | Glob patterns matched against page titles to skip |         |
| `--parallel` | Number of parallel page fetches                   | `3`     |
| `--dry-run`  | Print the page tree and stats without converting  | `false` |

### `html` options

| Flag             | Description                      | Default  |
| ---------------- | -------------------------------- | -------- |
| `-o`, `--output` | Output file                      | stdout   |
| `--image-folder` | Folder path used for image links | `assets` |

```bash
confluence-md html page.html -o page.md   # from a file
cat page.html | confluence-md html        # from stdin to stdout
```

Without API access, `html` cannot resolve user names (they render as `@user(account-id)`) or fetch Mermaid attachments.

### Examples

```bash
# Prefix filenames with the last-updated date: 2025-01-31-getting-started.md
confluence-md page <page-url> -e you@example.com -t "$CONFLUENCE_API_TOKEN" \
  --output-name-template '{{ .Page.UpdatedAt.Format "2006-01-02" }}-{{ .SlugTitle }}'

# Skip images
confluence-md page <page-url> -e you@example.com -t "$CONFLUENCE_API_TOKEN" --download-images=false

# Two levels deep, skipping archived pages
confluence-md tree <page-url> -e you@example.com -t "$CONFLUENCE_API_TOKEN" --depth 2 --exclude 'Archive*'
```

## Output Name Templates

`--output-name-template` takes a Go [`text/template`](https://pkg.go.dev/text/template) string with this data:

- `{{ .Page }}`: the full page object, e.g. `{{ .Page.Title }}`, `{{ .Page.ID }}`, `{{ .Page.SpaceKey }}`, `{{ .Page.UpdatedAt.Format "2006-01-02" }}` (see `ConfluencePage` in [`internal/confluence/model`](internal/confluence/model/page.go))
- `{{ .SlugTitle }}`: the slugified title, e.g. `getting-started`
- `{{ .LabelNames }}`: the page's label names as a list, e.g. `{{ join .LabelNames "-" }}`

The helpers `{{ slug <string> }}` (slugify any string) and `{{ join <list> <separator> }}` are available. If the result has no extension, `.md` is appended.

## Output Structure

```text
output/
├── getting-started.md
├── assets/                 # downloaded images
└── getting-started/        # tree: children go in a folder named after the parent
    ├── installation.md
    └── configuration.md
```

## Supported Confluence Elements

### Basic Elements

| Element             | Confluence Tag             | Conversion                                                            |
| ------------------- | -------------------------- | --------------------------------------------------------------------- |
| **Images**          | `ac:image`                 | Downloaded and linked as local Markdown images                        |
| **Emoticons**       | `ac:emoticon`              | Emoji fallback or shortname                                           |
| **Tables**          | HTML tables                | Markdown tables                                                       |
| **Lists**           | HTML lists                 | Nested lists with proper indentation                                  |
| **Task Lists**      | `ac:task-list`             | `- [x]` / `- [ ]` task items, nested tasks indented                   |
| **User Links**      | `ac:link` + `ri:user`      | `@DisplayName` (or `@user(account-id)` if the name can't be resolved) |
| **Time Elements**   | `<time>`                   | The `datetime` attribute as text                                      |
| **Inline Comments** | `ac:inline-comment-marker` | Text preserved, plus a `<!-- comment-ref: id -->` comment             |
| **Placeholders**    | `ac:placeholder`           | HTML comments                                                         |

### Macros (`ac:structured-macro`)

| Macro                            | Support | Conversion                                                                 |
| -------------------------------- | ------- | -------------------------------------------------------------------------- |
| `info`, `warning`, `note`, `tip` | Full    | Blockquote with an `Info`/`Warning`/`Note`/`Tip` label                     |
| `code`                           | Full    | Fenced code block with language                                            |
| `mermaid-cloud`                  | Full    | `mermaid` code block, loaded from the page attachment                      |
| `expand`, `details`              | Full    | Content rendered inline                                                    |
| `status`                         | Full    | Colored badge, e.g. 🟢 **Done**                                            |
| `toc`                            | Partial | `<!-- Table of Contents -->` comment                                       |
| `children`                       | Partial | `<!-- Child Pages -->` comment                                             |
| Others                           | No      | `<!-- Unsupported macro: {name} -->` comment; open an issue to request one |

## Development

```bash
go test ./...
go build -o confluence-md ./cmd/confluence-md
golangci-lint run
```

The demo GIF is generated with [VHS](https://github.com/charmbracelet/vhs): `vhs assets/demo.tape`.

## Contributing

Contributions are welcome. For larger changes, please open an [issue](https://github.com/jackchuka/confluence-md/issues) first to discuss what you would like to change.

## License

[MIT](LICENSE)
