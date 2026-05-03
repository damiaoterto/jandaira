package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	firecrawl "github.com/firecrawl/firecrawl/apps/go-sdk"
	"github.com/firecrawl/firecrawl/apps/go-sdk/option"

	"github.com/damiaoterto/jandaira/internal/security"
)

const firecrawlVaultKey = "firecrawl_api_key"

// FirecrawlTool uses the Firecrawl API to scrape, crawl, search, and map websites.
type FirecrawlTool struct {
	Vault *security.Vault
}

func (t *FirecrawlTool) Name() string { return "firecrawl" }

func (t *FirecrawlTool) Description() string {
	return `Web scraping and crawling tool powered by Firecrawl. Supported actions:
- scrape: Extract markdown content from a single URL
- crawl: Recursively crawl a website, returning all pages as markdown
- search: Search the web and return result summaries
- map: Discover all links on a website`
}

func (t *FirecrawlTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"scrape", "crawl", "search", "map"},
				"description": "Action: scrape (single URL), crawl (full site), search (web query), map (discover links)",
			},
			"url": map[string]interface{}{
				"type":        "string",
				"description": "Target URL (required for scrape, crawl, map)",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query (required for search action)",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Max pages to crawl or results to return (default: 10)",
			},
		},
		"required": []string{"action"},
	}
}

type firecrawlArgs struct {
	Action string `json:"action"`
	URL    string `json:"url"`
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
}

func (t *FirecrawlTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	if t.Vault == nil {
		return "", fmt.Errorf("firecrawl not configured: vault unavailable")
	}

	var args firecrawlArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	apiKey, err := t.Vault.GetSecret(firecrawlVaultKey)
	if err != nil {
		return "", fmt.Errorf("firecrawl API key not set: configure via POST /api/tools/preconfigured/firecrawl")
	}

	client, err := firecrawl.NewClient(option.WithAPIKey(apiKey))
	if err != nil {
		return "", fmt.Errorf("failed to create firecrawl client: %w", err)
	}

	if args.Limit <= 0 {
		args.Limit = 10
	}

	switch args.Action {
	case "scrape":
		return t.scrape(ctx, client, args)
	case "crawl":
		return t.crawl(ctx, client, args)
	case "search":
		return t.search(ctx, client, args)
	case "map":
		return t.mapSite(ctx, client, args)
	default:
		return "", fmt.Errorf("unknown action %q: must be scrape, crawl, search, or map", args.Action)
	}
}

func (t *FirecrawlTool) scrape(ctx context.Context, client *firecrawl.Client, args firecrawlArgs) (string, error) {
	if args.URL == "" {
		return "", fmt.Errorf("url is required for scrape action")
	}
	doc, err := client.Scrape(ctx, args.URL, &firecrawl.ScrapeOptions{
		Formats:         []string{"markdown"},
		OnlyMainContent: firecrawl.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("scrape %s: %w", args.URL, err)
	}
	if doc.Markdown == "" {
		return fmt.Sprintf("no markdown content extracted from %s", args.URL), nil
	}
	return doc.Markdown, nil
}

func (t *FirecrawlTool) crawl(ctx context.Context, client *firecrawl.Client, args firecrawlArgs) (string, error) {
	if args.URL == "" {
		return "", fmt.Errorf("url is required for crawl action")
	}
	job, err := client.Crawl(ctx, args.URL, &firecrawl.CrawlOptions{
		Limit: firecrawl.Int(args.Limit),
		ScrapeOptions: &firecrawl.ScrapeOptions{
			Formats:         []string{"markdown"},
			OnlyMainContent: firecrawl.Bool(true),
		},
	})
	if err != nil {
		return "", fmt.Errorf("crawl %s: %w", args.URL, err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Crawled %d/%d pages from %s (status: %s)\n\n", job.Completed, job.Total, args.URL, job.Status)
	for _, page := range job.Data {
		if src, ok := page.Metadata["sourceURL"].(string); ok {
			fmt.Fprintf(&sb, "## %s\n\n", src)
		}
		if page.Markdown != "" {
			sb.WriteString(page.Markdown)
			sb.WriteString("\n\n---\n\n")
		}
	}
	return sb.String(), nil
}

func (t *FirecrawlTool) search(ctx context.Context, client *firecrawl.Client, args firecrawlArgs) (string, error) {
	if args.Query == "" {
		return "", fmt.Errorf("query is required for search action")
	}
	results, err := client.Search(ctx, args.Query, &firecrawl.SearchOptions{
		Limit: firecrawl.Int(args.Limit),
	})
	if err != nil {
		return "", fmt.Errorf("search %q: %w", args.Query, err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Search results for: %s\n\n", args.Query)
	for _, r := range results.Web {
		title, _ := r["title"].(string)
		url, _ := r["url"].(string)
		description, _ := r["description"].(string)
		fmt.Fprintf(&sb, "### %s\n%s\n%s\n\n", title, url, description)
	}
	return sb.String(), nil
}

func (t *FirecrawlTool) mapSite(ctx context.Context, client *firecrawl.Client, args firecrawlArgs) (string, error) {
	if args.URL == "" {
		return "", fmt.Errorf("url is required for map action")
	}
	mapData, err := client.Map(ctx, args.URL, &firecrawl.MapOptions{
		Limit: firecrawl.Int(args.Limit),
	})
	if err != nil {
		return "", fmt.Errorf("map %s: %w", args.URL, err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d links on %s\n\n", len(mapData.Links), args.URL)
	for _, link := range mapData.Links {
		if link.Title != "" {
			fmt.Fprintf(&sb, "- [%s](%s)\n", link.Title, link.URL)
		} else {
			fmt.Fprintf(&sb, "- %s\n", link.URL)
		}
	}
	return sb.String(), nil
}
