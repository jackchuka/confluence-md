package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackchuka/confluence-md/internal/confluence"
	confluenceModel "github.com/jackchuka/confluence-md/internal/confluence/model"
	"github.com/jackchuka/confluence-md/internal/converter"
	"github.com/spf13/cobra"
)

// TreeOptions contains all options for the tree command
type TreeOptions struct {
	authOptions
	commonOptions

	OutputNamer converter.OutputNamer

	// Processing options
	MaxDepth int      // -1 for unlimited, default: 3
	Parallel int      // Concurrent fetches, default: 3
	Exclude  []string // Glob patterns to exclude

	// Output options
	DryRun bool // Preview without converting
}

var treeOpts TreeOptions

// treeCmd represents the tree command for recursive page conversion
var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Convert a Confluence page tree recursively",
	Long: `Convert a Confluence page and all its child pages recursively.
	
This command fetches a page and all its descendants up to a specified depth,
converting them to Markdown while preserving the hierarchy structure.

Examples:
  # Convert a page tree using URL
  confluence-md tree https://example.atlassian.net/wiki/spaces/SPACE/pages/12345/Title

  confluence-md tree https://example.atlassian.net/wiki/spaces/SPACE/pages/12345/Title --depth 2

  # Preview what would be converted
  confluence-md tree https://example.atlassian.net/wiki/spaces/SPACE/pages/12345/Title --dry-run`,
	RunE: runTreeCommand,
}

func init() {
	rootCmd.AddCommand(treeCmd)

	treeOpts.authOptions.InitFlags(treeCmd)
	treeOpts.commonOptions.InitFlags(treeCmd)

	// Required flags
	_ = treeCmd.MarkFlagRequired("api-token")
	_ = treeCmd.MarkFlagRequired("email")

	// Processing flags
	treeCmd.Flags().IntVar(&treeOpts.MaxDepth, "depth", -1, "Maximum depth to traverse (-1 for unlimited)")
	treeCmd.Flags().IntVar(&treeOpts.Parallel, "parallel", 3, "Number of parallel page fetches")
	treeCmd.Flags().StringSliceVar(&treeOpts.Exclude, "exclude", []string{}, "Glob patterns to exclude pages")

	// Output flags
	treeCmd.Flags().BoolVar(&treeOpts.DryRun, "dry-run", false, "Preview without converting")
}

func runTreeCommand(_ *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing required argument: page URL")
	}
	pageURL := args[0]

	pageInfo, err := urlToPageInfo(pageURL)
	if err != nil {
		return fmt.Errorf("invalid Confluence URL: %w", err)
	}

	// Validate input options
	if err := validateTreeOptions(); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	namer, err := buildOutputNamer(treeOpts.OutputNameTemplate)
	if err != nil {
		return fmt.Errorf("invalid output name template: %w", err)
	}
	treeOpts.OutputNamer = namer

	client := confluence.NewClient(pageInfo.BaseURL, treeOpts.Email, treeOpts.APIKey)

	if treeOpts.DryRun {
		fmt.Println("🔍 Dry run mode - analyzing page tree...")
		return performDryRun(client, pageInfo.PageID, &treeOpts)
	}

	return performTreeConversion(client, pageInfo.BaseURL, pageInfo.PageID, &treeOpts)
}

func validateTreeOptions() error {
	// Validate depth
	if treeOpts.MaxDepth < -1 {
		return fmt.Errorf("depth must be -1 (unlimited) or greater, got: %d", treeOpts.MaxDepth)
	}

	// Validate parallel
	if treeOpts.Parallel < 1 {
		return fmt.Errorf("parallel must be at least 1, got: %d", treeOpts.Parallel)
	}

	return nil
}

func performDryRun(client confluence.Client, rootPageID string, opts *TreeOptions) error {
	fmt.Println("\n📊 Page tree structure:")

	// Fetch and display tree structure
	tree, err := fetchPageTree(client, rootPageID, opts.MaxDepth, 0, opts.Exclude)
	if err != nil {
		return fmt.Errorf("failed to fetch page tree: %w", err)
	}

	// Display tree
	displayTree(tree, 0)

	// Show statistics
	stats := calculateTreeStats(tree)
	fmt.Printf("\n📈 Statistics:\n")
	fmt.Printf("  Total pages: %d\n", stats.TotalPages)
	fmt.Printf("  Max depth: %d\n", stats.MaxDepth)
	fmt.Printf("  Total size: ~%d KB\n", stats.EstimatedSize/1024)

	return nil
}

func performTreeConversion(client confluence.Client, baseURL, rootPageID string, opts *TreeOptions) error {
	// Create output directory
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Fetch page tree
	tree, err := fetchPageTree(client, rootPageID, opts.MaxDepth, 0, opts.Exclude)
	if err != nil {
		return fmt.Errorf("failed to fetch page tree: %w", err)
	}

	// Convert tree recursively using shared pipeline
	results := &ConversionResults{}
	err = convertPageTree(client, tree, opts.OutputDir, baseURL, opts, results)

	// Display results
	fmt.Printf("✅ Conversion complete!\n")
	fmt.Printf("  Successful: %d pages\n", results.Success)
	if results.Failed > 0 {
		fmt.Printf("  Failed: %d pages\n", results.Failed)
		fmt.Printf("  See error details above\n")
	}
	fmt.Printf("  Output: %s\n", opts.OutputDir)

	if err != nil {
		return fmt.Errorf("conversion completed with errors")
	}

	return nil
}

// PageNode represents a page in the tree structure
type PageNode struct {
	ID       string
	Title    string
	Level    int
	Parent   *PageNode // Reference to parent node
	Path     []string  // Full hierarchical path from root to this page
	Children []*PageNode
	Error    error
}

// TreeStats holds statistics about the page tree
type TreeStats struct {
	TotalPages    int
	MaxDepth      int
	EstimatedSize int
}

// ConversionResults tracks conversion progress
type ConversionResults struct {
	Success int
	Failed  int
	Errors  []error
}

func fetchPageTree(client confluence.Client, pageID string, maxDepth int, currentDepth int, excludePatterns []string) (*PageNode, error) {
	return fetchPageTreeWithParent(client, pageID, maxDepth, currentDepth, excludePatterns, nil, []string{})
}

func fetchPageTreeWithParent(client confluence.Client, pageID string, maxDepth int, currentDepth int, excludePatterns []string, parent *PageNode, parentPath []string) (*PageNode, error) {
	// Check depth limit
	if maxDepth != -1 && currentDepth > maxDepth {
		return nil, nil
	}

	// Fetch page details
	page, err := client.GetPage(pageID)
	if err != nil {
		return &PageNode{
			ID:     pageID,
			Title:  "Error loading page",
			Level:  currentDepth,
			Parent: parent,
			Path:   append(parentPath, "Error loading page"),
			Error:  err,
		}, nil
	}

	// Check exclusion patterns
	if shouldExclude(page.Title, excludePatterns) {
		return nil, nil
	}

	// Build path for current node
	currentPath := append(parentPath, page.Title)

	node := &PageNode{
		ID:     pageID,
		Title:  page.Title,
		Level:  currentDepth,
		Parent: parent,
		Path:   currentPath,
	}

	// Fetch children if within depth limit
	if maxDepth == -1 || currentDepth < maxDepth {
		children, err := client.GetChildPages(pageID)
		if err != nil {
			// Log error but continue
			fmt.Printf("⚠️  Warning: Failed to fetch children for %s: %v\n", page.Title, err)
		} else {
			for _, child := range children {
				childNode, err := fetchPageTreeWithParent(client, child.ID, maxDepth, currentDepth+1, excludePatterns, node, currentPath)
				if err != nil {
					fmt.Printf("⚠️  Warning: Failed to process child %s: %v\n", child.Title, err)
					continue
				}
				if childNode != nil {
					node.Children = append(node.Children, childNode)
				}
			}
		}
	}

	return node, nil
}

func shouldExclude(title string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, _ := filepath.Match(pattern, title)
		if matched {
			return true
		}
	}
	return false
}

func displayTree(node *PageNode, indent int) {
	if node == nil {
		return
	}

	prefix := strings.Repeat("  ", indent)
	if indent > 0 {
		prefix = strings.Repeat("  ", indent-1) + "├─ "
	}

	if node.Error != nil {
		fmt.Printf("%s%s (Error: %v)\n", prefix, node.Title, node.Error)
	} else {
		fmt.Printf("%s%s\n", prefix, node.Title)
	}

	for _, child := range node.Children {
		displayTree(child, indent+1)
	}
}

func calculateTreeStats(node *PageNode) *TreeStats {
	if node == nil {
		return &TreeStats{}
	}

	stats := &TreeStats{
		TotalPages:    1,
		MaxDepth:      node.Level,
		EstimatedSize: len(node.Title) * 100, // Rough estimate
	}

	for _, child := range node.Children {
		childStats := calculateTreeStats(child)
		stats.TotalPages += childStats.TotalPages
		if childStats.MaxDepth > stats.MaxDepth {
			stats.MaxDepth = childStats.MaxDepth
		}
		stats.EstimatedSize += childStats.EstimatedSize
	}

	return stats
}

func convertPageTree(client confluence.Client, node *PageNode, outputDir string, baseURL string, opts *TreeOptions, results *ConversionResults) error {
	if node == nil {
		return nil
	}

	// Convert current page
	fmt.Printf("📄 Converting: %s\n", node.Title)

	page, err := client.GetPage(node.ID)
	if err != nil {
		fmt.Printf("  ❌ Failed to fetch: %v\n", err)
		results.Failed++
		results.Errors = append(results.Errors, err)

		return nil
	}
	// Generate hierarchical output path
	outputPath, err := getOutputPath(node, page, outputDir, opts.OutputNamer)
	if err != nil {
		fmt.Printf("  ❌ Failed to resolve output path: %v\n", err)
		results.Failed++
		results.Errors = append(results.Errors, err)
		return nil
	}

	// Create options for tree conversion (inherit from tree options)
	conversionOpts := PageOptions{
		authOptions:   authOptions{Email: opts.Email, APIKey: opts.APIKey},
		commonOptions: opts.commonOptions,
		OutputNamer:   opts.OutputNamer,
	}

	// Use shared conversion pipeline with custom path
	result := convertSinglePageWithPath(client, page, baseURL, outputPath, conversionOpts)

	// Use shared result display
	printConversionResult(result)

	if result.Success {
		results.Success++
	} else {
		results.Failed++
		results.Errors = append(results.Errors, result.Error)
	}

	// Convert children
	for _, child := range node.Children {
		if err := convertPageTree(client, child, outputDir, baseURL, opts, results); err != nil {
			return err
		}
	}

	return nil
}

func getOutputPath(node *PageNode, page *confluenceModel.ConfluencePage, baseDir string, namer converter.OutputNamer) (string, error) {
	path := baseDir

	if len(node.Path) > 1 {
		dirPath := node.Path[:len(node.Path)-1]
		for _, pathElement := range dirPath {
			path = filepath.Join(path, sanitizeFileName(pathElement))
		}
		if err := os.MkdirAll(path, 0755); err != nil {
			return "", fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	fileName, err := converter.GenerateFileName(page, namer)
	if err != nil {
		return "", err
	}

	return filepath.Join(path, fileName), nil
}
