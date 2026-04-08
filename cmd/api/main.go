package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/damiaoterto/jandaira/internal/api"
	"github.com/damiaoterto/jandaira/internal/brain"
	"github.com/damiaoterto/jandaira/internal/config"
	"github.com/damiaoterto/jandaira/internal/i18n"
	"github.com/damiaoterto/jandaira/internal/queue"
	"github.com/damiaoterto/jandaira/internal/security"
	"github.com/damiaoterto/jandaira/internal/swarm"
	"github.com/damiaoterto/jandaira/internal/tool"
)

func main() {
	port := flag.Int("port", 8080, "Port for Webserver")
	flag.Parse()

	ctx := context.Background()

	configPath := config.GetDefaultPath()
	cfg, _ := config.Load(configPath)

	if cfg != nil && cfg.Language != "" {
		i18n.SetLanguage(cfg.Language)
	} else {
		i18n.Init()
	}

	swarmName := "enxame-alfa"
	if cfg != nil && cfg.SwarmName != "" {
		swarmName = cfg.SwarmName
	}

	honeycomb, err := brain.NewLocalVectorDB("./.jandaira/memory.json")
	if err != nil {
		fmt.Printf("Error initializing local vector database: %v\n", err)
		os.Exit(1)
	}
	_ = honeycomb.EnsureCollection(ctx, swarmName, 1536)

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		repoDir := security.GetDefaultVaultDir()
		if v, err := security.InitVault(repoDir); err == nil {
			if key, err := v.GetSecret("OPENAI_API_KEY"); err == nil {
				apiKey = key
			}
		}
	}
	if apiKey == "" {
		fmt.Println(i18n.T("warn_api_key_not_set"))
		apiKey = "sk-mock-key-for-testing"
	}

	modelType := "gpt-5.4-nano"
	if cfg != nil && cfg.Model != "" {
		modelType = cfg.Model
	}

	brain := brain.NewOpenAIBrain(apiKey, modelType)
	groupQueue := queue.NewGroupQueue(3)
	queen := swarm.NewQueen(groupQueue, brain, honeycomb)

	queen.EquipTool(&tool.ListDirectoryTool{})
	queen.EquipTool(&tool.ReadFileTool{})
	queen.EquipTool(&tool.WriteFileTool{})
	queen.EquipTool(&tool.CreateDirectoryTool{})
	queen.EquipTool(&tool.ExecuteCodeTool{})

	queen.EquipTool(&tool.SearchMemoryTool{
		Brain:      brain,
		Honeycomb:  honeycomb,
		Collection: swarmName,
	})

	queen.EquipTool(&tool.StoreMemoryTool{
		Brain:      brain,
		Honeycomb:  honeycomb,
		Collection: swarmName,
	})

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

	server := api.NewServer(queen, *port, configPath)

	// LogFunc: forwards all Queen logs to connected WebSocket clients
	queen.LogFunc = func(msg string) {
		server.Broadcast(api.WsMessage{Type: "status", Message: msg})
	}

	// AskPermissionFunc: delegates to RequestApproval so each request gets a unique ID.
	// AgentChangeFunc and ToolStartFunc are wired automatically inside NewServer.
	queen.AskPermissionFunc = func(toolName string, args string) {
		server.RequestApproval(toolName, args)
	}

	if err := server.Start(); err != nil {
		fmt.Printf("Fatal server error: %v\n", err)
		os.Exit(1)
	}
}
