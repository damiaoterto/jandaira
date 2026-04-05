package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/damiaoterto/jandaira/internal/brain"
	"github.com/damiaoterto/jandaira/internal/queue"
	"github.com/damiaoterto/jandaira/internal/swarm"
	"github.com/damiaoterto/jandaira/internal/tool"
	"github.com/damiaoterto/jandaira/internal/ui"
)

func main() {
	ctx := context.Background()

	honeycomb, err := brain.NewLocalVectorDB("./.jandaira/data")
	if err != nil {
		fmt.Printf("Erro ao inicializar o banco vetorial: %v\n", err)
		os.Exit(1)
	}
	_ = honeycomb.EnsureCollection(ctx, "enxame-alfa", 1536)

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("⚠️ Aviso: OPENAI_API_KEY não definida.")
		apiKey = "sk-mock-key-para-testes"
	}

	brain := brain.NewOpenAIBrain(apiKey, "gpt-4o-mini")
	groupQueue := queue.NewGroupQueue(3)
	newQuen := swarm.NewQueen(groupQueue, brain, honeycomb)

	newQuen.EquipTool(&tool.ListDirectoryTool{})
	newQuen.EquipTool(&tool.ReadFileTool{})
	newQuen.EquipTool(&tool.WriteFileTool{})
	newQuen.EquipTool(&tool.ExecuteCodeTool{})

	newQuen.RegisterSwarm("enxame-alfa", swarm.Policy{
		MaxNectar:        20000,
		Isolate:          true,
		RequiresApproval: true,
	})

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

	fmt.Print("\033[H\033[2J")

	p := tea.NewProgram(ui.InitialModel(newQuen, workflow))

	newQuen.LogFunc = func(msg string) {
		p.Send(ui.StatusMsg(msg))
	}

	if _, err := p.Run(); err != nil {
		fmt.Printf("Erro na interface do enxame: %v", err)
		os.Exit(1)
	}
}
