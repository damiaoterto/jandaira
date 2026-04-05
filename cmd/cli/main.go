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
	"github.com/damiaoterto/jandaira/internal/queue"
	"github.com/damiaoterto/jandaira/internal/swarm"
	"github.com/damiaoterto/jandaira/internal/tool"
	"github.com/damiaoterto/jandaira/internal/ui"
)

func main() {
	serverMode := flag.Bool("server", false, "Iniciar em modo servidor HTTP/WS")
	port := flag.Int("port", 8080, "Porta do servidor da API")
	flag.Parse()

	ctx := context.Background()

	// ── Config Load/Setup ──────────────────────────────────────────────────
	configPath := config.GetDefaultPath()
	cfg, err := config.Load(configPath)

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

	// ── Initialize Database and Environment ────────────────────────────────
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
		fmt.Println("⚠️ Aviso: OPENAI_API_KEY não definida.")
		apiKey = "sk-mock-key-para-testes"
	}

	modelType := "gpt-4o-mini"
	if cfg != nil {
		modelType = cfg.Model
	}

	// ── Initialize the Hive Core ───────────────────────────────────────────
	openAIBrain := brain.NewOpenAIBrain(apiKey, modelType)
	groupQueue := queue.NewGroupQueue(3)
	newQuen := swarm.NewQueen(groupQueue, openAIBrain, honeycomb)

	newQuen.EquipTool(&tool.ListDirectoryTool{})
	newQuen.EquipTool(&tool.ReadFileTool{})
	newQuen.EquipTool(&tool.WriteFileTool{})
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

	desenvolvedora := swarm.Specialist{
		Name: "Desenvolvedora Wasm",
		SystemPrompt: `Você é a Desenvolvedora de Software da colmeia.
			REGRAS:
			1. Seu principal trabalho é escrever código limpo e seguro usando 'write_file'.
			2. Se o usuário pedir para revisar ou modificar arquivos existentes, USE OBRIGATORIAMENTE a ferramenta 'read_file' para ler o conteúdo real do disco antes de responder.
			3. VOCÊ NÃO TESTA CÓDIGO. Não tente usar ferramentas de execução.
			4. Leia o objetivo, faça suas tarefas (leitura/escrita), e retorne uma mensagem sumário detalhando o que foi feito.`,
		AllowedTools: []string{"write_file", "read_file", "list_directory"},
	}

	auditora := swarm.Specialist{
		Name: "Auditora de Qualidade",
		SystemPrompt: `Você é a Auditora de Qualidade e Segurança da colmeia.
			REGRAS:
			1. A sua função é ler, inspecionar e testar o código trabalhado na fase anterior.
			2. USE OBRIGATORIAMENTE a ferramenta 'read_file' para extrair o código-fonte real dos ficheiros mencionados. NUNCA diga que não consegue ler arquivos; use a ferramenta para isso.
			3. Se for adequado, use a ferramenta 'execute_code' para testar o código na Sandbox e ler a sua saída.
			4. Faça um relatório de qualidade e segurança informando claramente os problemas detetados no código que leu.`,
		AllowedTools: []string{"execute_code", "read_file", "list_directory"},
	}

	workflow := []swarm.Specialist{desenvolvedora, auditora}

	// ── Start Server or TUI ──────────────────────────────────────────────
	if *serverMode {
		srv := api.NewServer(newQuen, workflow, *port, configPath)
		if err := srv.Start(); err != nil {
			fmt.Printf("Erro no servidor da api: %v", err)
			os.Exit(1)
		}
		return
	}

	fmt.Print("\033[H\033[2J")

	p := tea.NewProgram(ui.InitialModel(newQuen, workflow, swarmName))

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
