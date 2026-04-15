package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/damiaoterto/jandaira/internal/i18n"
	"golang.org/x/net/html"
)

// WebSearchTool performs web searches by scraping DuckDuckGo HTML results.
type WebSearchTool struct {
	AppName string
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Description() string {
	return i18n.T("tool_websearch_description")
}

func (t *WebSearchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": i18n.T("tool_websearch_query_description"),
			},
		},
		"required": []string{"query"},
	}
}

type searchResult struct {
	Title   string
	URL     string
	Snippet string
}

func (t *WebSearchTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("erro ao ler argumentos JSON: %w", err)
	}

	if strings.TrimSpace(args.Query) == "" {
		return "", fmt.Errorf("o campo 'query' não pode ser vazio")
	}

	results, err := duckduckgoSearch(ctx, args.Query)
	if err != nil {
		return "", fmt.Errorf("erro ao buscar no DuckDuckGo: %w", err)
	}

	if len(results) == 0 {
		return fmt.Sprintf(i18n.T("tool_websearch_no_results"), args.Query), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(i18n.T("tool_websearch_header"), args.Query))
	sb.WriteString(i18n.T("tool_websearch_results"))

	limit := len(results)
	if limit > 8 {
		limit = 8
	}
	for _, r := range results[:limit] {
		fmt.Fprintf(&sb, "### %s\n%s\n%s\n\n", r.Title, r.URL, r.Snippet)
	}

	return sb.String(), nil
}

func duckduckgoSearch(ctx context.Context, query string) ([]searchResult, error) {
	endpoint := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DuckDuckGo returned status %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear HTML: %w", err)
	}

	return extractResults(doc), nil
}

func extractResults(doc *html.Node) []searchResult {
	var results []searchResult

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			if hasClass(n, "result__body") {
				r := parseResultNode(n)
				if r.Title != "" && r.URL != "" {
					results = append(results, r)
				}
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return results
}

func parseResultNode(n *html.Node) searchResult {
	var r searchResult

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case n.Data == "a" && hasClass(n, "result__a"):
				r.Title = strings.TrimSpace(textContent(n))
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						r.URL = resolveURL(attr.Val)
					}
				}
			case (n.Data == "a" && hasClass(n, "result__snippet")) ||
				(n.Data == "div" && hasClass(n, "result__snippet")):
				r.Snippet = strings.TrimSpace(textContent(n))
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)

	return r
}

func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			for _, c := range strings.Fields(attr.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(textContent(c))
	}
	return sb.String()
}

// resolveURL extracts the real URL from DuckDuckGo's redirect link.
func resolveURL(raw string) string {
	if strings.HasPrefix(raw, "//duckduckgo.com/l/?uddg=") {
		parsed, err := url.Parse("https:" + raw)
		if err == nil {
			if uddg := parsed.Query().Get("uddg"); uddg != "" {
				decoded, err := url.QueryUnescape(uddg)
				if err == nil {
					return decoded
				}
			}
		}
	}
	if strings.HasPrefix(raw, "http") {
		return raw
	}
	return raw
}
