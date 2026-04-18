package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/damiaoterto/jandaira/internal/api"
	"github.com/damiaoterto/jandaira/internal/brain"
	"github.com/damiaoterto/jandaira/internal/config"
	"github.com/damiaoterto/jandaira/internal/database"
	"github.com/damiaoterto/jandaira/internal/i18n"
	"github.com/damiaoterto/jandaira/internal/queue"
	"github.com/damiaoterto/jandaira/internal/repository"
	"github.com/damiaoterto/jandaira/internal/security"
	"github.com/damiaoterto/jandaira/internal/service"
	"github.com/damiaoterto/jandaira/internal/swarm"
	"github.com/damiaoterto/jandaira/internal/tool"
)

func main() {
	port := flag.Int("port", 8080, "Port for Webserver")
	flag.Parse()

	ctx := context.Background()

	db, err := database.Open(config.GetDefaultPath())
	if err != nil {
		fmt.Printf("Erro ao abrir banco de dados: %v\n", err)
		os.Exit(1)
	}

	cfgRepo := repository.NewConfigRepository(db)
	cfgService := service.NewConfigService(cfgRepo)

	sessionRepo := repository.NewSessionRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	sessionService := service.NewSessionService(sessionRepo, agentRepo)

	colmeiaRepo := repository.NewColmeiaRepository(db)
	agenteColmeiaRepo := repository.NewAgenteColmeiaRepository(db)
	historicoRepo := repository.NewHistoricoDespachoRepository(db)
	colmeiaService := service.NewColmeiaService(colmeiaRepo, agenteColmeiaRepo, historicoRepo)

	skillRepo := repository.NewSkillRepository(db)
	skillService := service.NewSkillService(skillRepo)

	cfg, err := cfgService.Load()
	if err != nil && !errors.Is(err, service.ErrNotConfigured) {
		fmt.Printf("Erro ao carregar configuração: %v\n", err)
		os.Exit(1)
	}

	if cfg != nil && cfg.Language != "" {
		i18n.SetLanguage(cfg.Language)
	} else {
		i18n.Init()
	}

	swarmName := "enxame-alfa"
	if cfg != nil && cfg.SwarmName != "" {
		swarmName = cfg.SwarmName
	}

	chromaURL := os.Getenv("CHROMA_URL")
	if chromaURL == "" {
		chromaURL = "http://localhost:8000"
	}
	honeycomb, err := brain.NewChromaHoneycomb(ctx, chromaURL)
	if err != nil {
		fmt.Printf("Error initializing ChromaDB: %v\n", err)
		os.Exit(1)
	}
	_ = honeycomb.EnsureCollection(ctx, swarmName, 1536)

	graphPath := filepath.Join(filepath.Dir(config.GetDefaultPath()), "knowledge_graph.json")
	knowledgeGraph, err := brain.NewLocalKnowledgeGraph(graphPath)
	if err != nil {
		fmt.Printf("Error initializing knowledge graph: %v\n", err)
		os.Exit(1)
	}

	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		repoDir := security.GetDefaultVaultDir()
		if v, err := security.InitVault(repoDir); err == nil {
			if key, err := v.GetSecret("OPENAI_API_KEY"); err == nil {
				apiKey = strings.TrimSpace(key)
			}
		}
	}
	if apiKey != "" {
		os.Setenv("OPENAI_API_KEY", apiKey)
	} else {
		fmt.Println(i18n.T("warn_api_key_not_set"))
		apiKey = "sk-mock-key-for-testing"
	}

	modelType := "gpt-4o-mini"
	if cfg != nil && cfg.Model != "" {
		modelType = cfg.Model
	}

	openAIBrain := brain.NewOpenAIBrain(apiKey, modelType)
	groupQueue := queue.NewGroupQueue(3)
	queen := swarm.NewQueen(groupQueue, openAIBrain, honeycomb)
	queen.Graph = knowledgeGraph

	queen.EquipTool(&tool.ListDirectoryTool{})
	queen.EquipTool(&tool.ReadFileTool{})
	queen.EquipTool(&tool.CreateDirectoryTool{})
	queen.EquipTool(&tool.ExecuteCodeTool{})
	queen.EquipTool(&tool.SearchMemoryTool{Brain: openAIBrain, Honeycomb: honeycomb, Collection: swarmName})
	queen.EquipTool(&tool.StoreMemoryTool{Brain: openAIBrain, Honeycomb: honeycomb, Collection: swarmName})
	queen.EquipTool(&tool.WebSearchTool{})

	if cfg != nil {
		queen.RegisterSwarm(swarmName, swarm.Policy{
			MaxNectar:        cfg.MaxNectar,
			Isolate:          cfg.Isolated,
			RequiresApproval: cfg.Supervised,
		})
	} else {
		queen.RegisterSwarm(swarmName, swarm.Policy{
			MaxNectar:        20000,
			Isolate:          true,
			RequiresApproval: true,
		})
	}

	server := api.NewServer(queen, *port, cfgService, sessionService, colmeiaService, skillService)

	queen.LogFunc = func(msg string) {
		server.Broadcast(api.WsMessage{Type: "status", Message: msg})
	}
	queen.AskPermissionFunc = func(toolName string, args string) {
		server.RequestApproval(toolName, args)
	}

	if err := server.Start(); err != nil {
		fmt.Printf("Fatal server error: %v\n", err)
		os.Exit(1)
	}
}
