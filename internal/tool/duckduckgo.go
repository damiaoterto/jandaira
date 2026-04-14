package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/JoshuaDoes/duckduckgolang"
	"github.com/damiaoterto/jandaira/internal/i18n"
)

// WebSearchTool performs web searches using the DuckDuckGo Instant Answer API.
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

	appName := t.AppName
	if appName == "" {
		appName = "jandaira-swarm"
	}

	client := &duckduckgo.Client{AppName: appName}
	result, err := client.GetQueryResult(args.Query)
	if err != nil {
		return "", fmt.Errorf("erro ao buscar no DuckDuckGo: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(i18n.T("tool_websearch_header"), result.Query))

	hasContent := false

	if result.Answer != "" {
		sb.WriteString(i18n.T("tool_websearch_direct_answer"))
		fmt.Fprintf(&sb, "%s\n\n", result.Answer)
		hasContent = true
	}

	if result.Definition != "" {
		sb.WriteString(i18n.T("tool_websearch_definition"))
		fmt.Fprintf(&sb, "%s\n", result.Definition)
		if result.DefinitionSource != "" {
			sb.WriteString(fmt.Sprintf(i18n.T("tool_websearch_definition_source"), result.DefinitionSource, result.DefinitionURL))
		}
		sb.WriteString("\n")
		hasContent = true
	}

	if result.AbstractText != "" {
		sb.WriteString(i18n.T("tool_websearch_abstract"))
		fmt.Fprintf(&sb, "%s\n", result.AbstractText)
		if result.AbstractSource != "" {
			sb.WriteString(fmt.Sprintf(i18n.T("tool_websearch_abstract_source"), result.AbstractSource, result.AbstractURL))
		}
		sb.WriteString("\n")
		hasContent = true
	}

	if len(result.Results) > 0 {
		sb.WriteString(i18n.T("tool_websearch_results"))
		limit := len(result.Results)
		if limit > 5 {
			limit = 5
		}
		for _, r := range result.Results[:limit] {
			if r.Text != "" {
				fmt.Fprintf(&sb, "- **%s**\n  %s\n", r.FirstURL, r.Text)
			}
		}
		sb.WriteString("\n")
		hasContent = true
	}

	if len(result.RelatedTopics) > 0 {
		sb.WriteString(i18n.T("tool_websearch_related"))
		limit := len(result.RelatedTopics)
		if limit > 5 {
			limit = 5
		}
		for _, rt := range result.RelatedTopics[:limit] {
			if rt.Text != "" {
				fmt.Fprintf(&sb, "- %s\n", rt.Text)
				if rt.FirstURL != "" {
					fmt.Fprintf(&sb, "  URL: %s\n", rt.FirstURL)
				}
			}
		}
		sb.WriteString("\n")
		hasContent = true
	}

	if !hasContent {
		return fmt.Sprintf(i18n.T("tool_websearch_no_results"), args.Query), nil
	}

	return sb.String(), nil
}
