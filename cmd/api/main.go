package main

import (
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
	"github.com/damiaoterto/jandaira/internal/provider"
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

	documentRepo := repository.NewDocumentRepository(db)
	documentService := service.NewDocumentService(documentRepo)

	webhookRepo := repository.NewWebhookRepository(db)
	webhookService := service.NewWebhookService(webhookRepo)

	outboundWebhookRepo := repository.NewOutboundWebhookRepository(db)
	outboundWebhookService := service.NewOutboundWebhookService(outboundWebhookRepo)
	outboundWebhookService.Start(3)

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

	vectorDBDir := filepath.Join(filepath.Dir(config.GetDefaultPath()), "vectordb")
	honeycomb, err := brain.NewVectorEngine(vectorDBDir)
	if err != nil {
		fmt.Printf("Error initializing vector engine: %v\n", err)
		os.Exit(1)
	}

	graphPath := filepath.Join(filepath.Dir(config.GetDefaultPath()), "knowledge_graph.json")
	knowledgeGraph, err := brain.NewLocalKnowledgeGraph(graphPath)
	if err != nil {
		fmt.Printf("Error initializing knowledge graph: %v\n", err)
		os.Exit(1)
	}

	providerName := "openai"
	if cfg != nil && cfg.Provider != "" {
		providerName = strings.ToLower(cfg.Provider)
	}

	repoDir := security.GetDefaultVaultDir()
	vault, _ := security.InitVault(repoDir)

	maxTokensFn := func() int {
		c, err := cfgService.Load()
		if err != nil || c.MaxNectar == 0 {
			return 0
		}
		return c.MaxNectar
	}

	configuredModel := ""
	if cfg != nil {
		configuredModel = cfg.Model
	}

	activeBrain, embedBrain, err := provider.BuildBrains(providerName, configuredModel, vault, maxTokensFn)
	if err != nil {
		fmt.Printf("Error initializing %s brain: %v\n", providerName, err)
		os.Exit(1)
	}

	groupQueue := queue.NewGroupQueue(3)
	queen := swarm.NewQueen(groupQueue, activeBrain, honeycomb)
	queen.Graph = knowledgeGraph

	queen.EquipTool(&tool.ListDirectoryTool{})
	queen.EquipTool(&tool.ReadFileTool{})
	queen.EquipTool(&tool.ExecuteCodeTool{})
	queen.EquipTool(&tool.SearchMemoryTool{Brain: embedBrain, Honeycomb: honeycomb, Collection: swarmName})
	queen.EquipTool(&tool.StoreMemoryTool{Brain: embedBrain, Honeycomb: honeycomb, Collection: swarmName})
	queen.EquipTool(&tool.WebSearchTool{})
	queen.EquipTool(&tool.FirecrawlTool{Vault: vault})

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

	server := api.NewServer(queen, *port, cfgService, sessionService, colmeiaService, skillService, documentService, webhookService, outboundWebhookService)

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
