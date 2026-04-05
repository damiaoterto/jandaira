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
			1. Seu único trabalho é escrever código limpo e seguro usando a ferramenta 'write_file'.
			2. VOCÊ NÃO TESTA CÓDIGO. Não tente usar ferramentas de execução.
			3. Leia o objetivo, escreva o arquivo, e retorne uma mensagem dizendo o nome do arquivo que você criou para que a próxima abelha possa testar.`,
		AllowedTools: []string{"write_file"},
	}

	auditora := swarm.Specialist{
		Name: "Auditora de Qualidade",
		SystemPrompt: `Você é a Auditora de Qualidade e Segurança da colmeia.
			REGRAS:
			1. Leia o relatório do trabalho anterior para descobrir qual arquivo foi criado.
			2. Use OBRIGATORIAMENTE a ferramenta 'execute_code' para testar o código na Sandbox.
			3. Após executar, analise a saída (stdout/stderr) e faça um relatório informando se o código funcionou corretamente.`,
		AllowedTools: []string{"execute_code", "read_file"},
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
