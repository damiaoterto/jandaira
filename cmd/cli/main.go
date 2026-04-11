package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

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
	"github.com/damiaoterto/jandaira/internal/ui"
)

func main() {
	serverMode := flag.Bool("server", false, "Iniciar em modo servidor HTTP/WS")
	port := flag.Int("port", 8080, "Porta do servidor da API")
	flag.Parse()

	ctx := context.Background()

	// ── Database ──────────────────────────────────────────────────────────────
	db, err := database.Open(config.GetDefaultPath())
	if err != nil {
		fmt.Printf("Erro ao abrir banco de dados: %v\n", err)
		os.Exit(1)
	}

	// ── Repository + Service ──────────────────────────────────────────────────
	cfgRepo := repository.NewConfigRepository(db)
	cfgService := service.NewConfigService(cfgRepo)

	sessionRepo := repository.NewSessionRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	sessionService := service.NewSessionService(sessionRepo, agentRepo)

	// ── Load config ───────────────────────────────────────────────────────────
	cfg, err := cfgService.Load()

	if err == nil && cfg.Language != "" {
		i18n.SetLanguage(cfg.Language)
	} else {
		i18n.Init()
	}

	if errors.Is(err, service.ErrNotConfigured) {
		if *serverMode {
			fmt.Println("⚠️ Configuração não encontrada. Iniciando servidor no modo Setup via API...")
		} else {
			fmt.Print("\033[H\033[2J")
			p := tea.NewProgram(ui.NewWizardModel(cfgService))
			if _, err := p.Run(); err != nil {
				fmt.Printf("Erro no assistente: %v\n", err)
				os.Exit(1)
			}
			cfg, err = cfgService.Load()
			if err != nil {
				fmt.Println("⚠️ Configuração interrompida ou incompleta. Encerrando.")
				os.Exit(0)
			}
		}
	} else if err != nil {
		fmt.Printf("Erro ao carregar configuração: %v\n", err)
		os.Exit(1)
	}

	// ── Brain (vector DB) ─────────────────────────────────────────────────────
	honeycomb, err := brain.NewLocalVectorDB("./.jandaira/memory.json")
	if err != nil {
		fmt.Printf("Erro ao inicializar o banco vetorial: %v\n", err)
		os.Exit(1)
	}

	swarmName := "enxame-alfa"
	if cfg != nil {
		swarmName = cfg.SwarmName
	}
	_ = honeycomb.EnsureCollection(ctx, swarmName, 1536)

	// ── API key ───────────────────────────────────────────────────────────────
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		repoDir := security.GetDefaultVaultDir()
		v, vErr := security.InitVault(repoDir)
		if vErr != nil {
			fmt.Printf("⚠️  Cofre indisponível: %v\n", vErr)
		} else {
			key, kErr := v.GetSecret("OPENAI_API_KEY")
			if kErr != nil {
				fmt.Printf("⚠️  Chave API não encontrada no cofre: %v\n", kErr)
			} else {
				apiKey = strings.TrimSpace(key)
			}
		}
	}

	if apiKey == "" {
		fmt.Println(i18n.T("warn_api_key_not_set"))
		apiKey = "sk-mock-key-para-testes"
	} else {
		os.Setenv("OPENAI_API_KEY", apiKey)
	}

	modelType := "gpt-4o-mini"
	if cfg != nil {
		modelType = cfg.Model
	}

	// ── Swarm ─────────────────────────────────────────────────────────────────
	openAIBrain := brain.NewOpenAIBrain(apiKey, modelType)
	groupQueue := queue.NewGroupQueue(3)
	newQuen := swarm.NewQueen(groupQueue, openAIBrain, honeycomb)

	newQuen.EquipTool(&tool.ListDirectoryTool{})
	newQuen.EquipTool(&tool.ReadFileTool{})
	newQuen.EquipTool(&tool.WriteFileTool{})
	newQuen.EquipTool(&tool.CreateDirectoryTool{})
	newQuen.EquipTool(&tool.ExecuteCodeTool{})

	if cfg != nil {
		newQuen.RegisterSwarm(swarmName, swarm.Policy{
			MaxNectar:        cfg.MaxNectar,
			Isolate:          cfg.Isolated,
			RequiresApproval: cfg.Supervised,
		})
	} else {
		newQuen.RegisterSwarm(swarmName, swarm.Policy{
			MaxNectar:        20000,
			Isolate:          true,
			RequiresApproval: true,
		})
	}

	if *serverMode {
		srv := api.NewServer(newQuen, *port, cfgService, sessionService)
		if err := srv.Start(); err != nil {
			fmt.Printf(i18n.T("cli_api_init_error")+"\n", err)
			os.Exit(1)
		}
		return
	}

	// ── CLI UI ────────────────────────────────────────────────────────────────
	fmt.Print("\033[H\033[2J")

	maxWorkers := 3
	if cfg != nil && cfg.MaxAgents > 0 {
		maxWorkers = cfg.MaxAgents
	}

	p := tea.NewProgram(
		ui.InitialModel(newQuen, swarmName, maxWorkers, sessionService),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	newQuen.LogFunc = func(msg string) {
		p.Send(ui.StatusMsg(msg))
	}
	newQuen.AskPermissionFunc = func(toolName string, args string) {
		p.Send(ui.PermissionMsg{ToolName: toolName, Args: args})
	}

	if _, err := p.Run(); err != nil {
		fmt.Printf("Erro na interface do enxame: %v", err)
		os.Exit(1)
	}
}
