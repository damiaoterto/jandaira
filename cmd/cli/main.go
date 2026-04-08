package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/damiaoterto/jandaira/internal/api"
	"github.com/damiaoterto/jandaira/internal/brain"
	"github.com/damiaoterto/jandaira/internal/config"
	"github.com/damiaoterto/jandaira/internal/i18n"
	"github.com/damiaoterto/jandaira/internal/queue"
	"github.com/damiaoterto/jandaira/internal/security"
	"github.com/damiaoterto/jandaira/internal/swarm"
	"github.com/damiaoterto/jandaira/internal/tool"
	"github.com/damiaoterto/jandaira/internal/ui"
)

func main() {
	serverMode := flag.Bool("server", false, "Iniciar em modo servidor HTTP/WS")
	port := flag.Int("port", 8080, "Porta do servidor da API")
	flag.Parse()

	ctx := context.Background()

	configPath := config.GetDefaultPath()
	cfg, err := config.Load(configPath)

	if err == nil && cfg.Language != "" {
		i18n.SetLanguage(cfg.Language)
	} else {
		i18n.Init()
	}

	if err == config.ErrConfigNotFound {
		if *serverMode {
			// Start API in setup mode
			fmt.Println("⚠️ Configuração não encontrada. Iniciando servidor no modo Setup via API...")
			// Create dummy Queen specifically to wait for setup or just start setup API.
			// Actually we will initialize the Server and it will intercept routes.
		} else {
			// Start CLI UI specific for Wizard
			fmt.Print("\033[H\033[2J") // Limpa o terminal
			p := tea.NewProgram(ui.NewWizardModel(configPath))
			if _, err := p.Run(); err != nil {
				fmt.Printf("Erro no assistente: %v\n", err)
				os.Exit(1)
			}
			// Attempt to load again after wizard
			cfg, err = config.Load(configPath)
			if err != nil {
				fmt.Println("⚠️ Configuração interrompida ou incompleta. Encerrando.")
				os.Exit(0)
			}
		}
	} else if err != nil {
		fmt.Printf("Erro ao carregar o arquivo de configuração: %v\n", err)
		os.Exit(1)
	}

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
		apiKey = "sk-mock-key-para-testes"
	}

	modelType := "gpt-5.3-mini"
	if cfg != nil {
		modelType = cfg.Model
	}

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
		srv := api.NewServer(newQuen, *port, configPath)
		if err := srv.Start(); err != nil {
			fmt.Printf(i18n.T("cli_api_init_error")+"\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Print("\033[H\033[2J")

	maxWorkers := 3
	if cfg != nil && cfg.MaxAgents > 0 {
		maxWorkers = cfg.MaxAgents
	}

	p := tea.NewProgram(
		ui.InitialModel(newQuen, swarmName, maxWorkers),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	newQuen.LogFunc = func(msg string) {
		p.Send(ui.StatusMsg(msg))
	}

	// INTERRUPÇÃO DO APICULTOR: quando RequiresApproval=true, a Rainha pausa
	// e envia um PermissionMsg para a UI que exibe o painel de aprovação.
	newQuen.AskPermissionFunc = func(toolName string, args string) {
		p.Send(ui.PermissionMsg{ToolName: toolName, Args: args})
	}

	if _, err := p.Run(); err != nil {
		fmt.Printf("Erro na interface do enxame: %v", err)
		os.Exit(1)
	}
}
