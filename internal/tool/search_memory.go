package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/damiaoterto/jandaira/internal/brain"
)

type SearchMemoryTool struct {
	Brain      brain.Brain
	Honeycomb  brain.Honeycomb
	Collection string
}

type StoreMemoryTool struct {
	Brain      brain.Brain
	Honeycomb  brain.Honeycomb
	Collection string
}

func (t *SearchMemoryTool) Name() string { return "search_memory" }

func (t *SearchMemoryTool) Description() string {
	return "Busca informações na memória de longo prazo do enxame (banco de dados vetorial) usando semântica. Útil para lembrar de auditorias passadas ou regras do sistema."
}

func (t *SearchMemoryTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "A pergunta ou conceito que você quer buscar na memória.",
			},
		},
		"required": []string{"query"},
	}
}

func (t *SearchMemoryTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("erro ao ler argumentos JSON: %w", err)
	}

	vector, err := t.Brain.Embed(ctx, args.Query)
	if err != nil {
		return "", fmt.Errorf("erro ao gerar embedding: %w", err)
	}

	results, err := t.Honeycomb.Search(ctx, t.Collection, vector, 3)
	if err != nil {
		return "", fmt.Errorf("erro na busca vetorial: %w", err)
	}

	if len(results) == 0 {
		return "Nenhuma memória encontrada sobre este assunto.", nil
	}

	var response strings.Builder
	response.WriteString("Memórias encontradas:\n")
	for _, res := range results {
		fmt.Fprintf(&response, "- ID: %s | Relevância: %.2f | Conteúdo: %s\n", res.ID, res.Score, res.Metadata["content"])
	}

	return response.String(), nil
}

func (t *StoreMemoryTool) Name() string { return "store_memory" }

func (t *StoreMemoryTool) Description() string {
	return "Salva uma informação importante na memória de longo prazo para que o enxame possa lembrar no futuro."
}

func (t *StoreMemoryTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "A informação, código ou resumo que deve ser memorizado.",
			},
		},
		"required": []string{"content"},
	}
}

func (t *StoreMemoryTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("erro ao ler argumentos JSON: %w", err)
	}

	vector, err := t.Brain.Embed(ctx, args.Content)
	if err != nil {
		return "", fmt.Errorf("erro ao gerar embedding: %w", err)
	}

	docID := fmt.Sprintf("mem-%d", time.Now().UnixMilli())
	err = t.Honeycomb.Store(ctx, t.Collection, docID, vector, map[string]string{
		"content": args.Content,
		"type":    "agent_note",
	})

	if err != nil {
		return "", fmt.Errorf("erro ao salvar no LanceDB: %w", err)
	}

	return fmt.Sprintf("Sucesso! A informação foi guardada na memória do enxame com o ID %s.", docID), nil
}
