package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/damiaoterto/jandaira/internal/brain"
	"github.com/damiaoterto/jandaira/internal/queue"
	"github.com/damiaoterto/jandaira/internal/swarm"
	"github.com/damiaoterto/jandaira/internal/tool"
)

func main() {
	fmt.Println("🐝 Iniciando a Colmeia Jandaira...")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	honeycomb, err := brain.NewLanceDBProvider("./.jandaira/data")
	if err != nil {
		fmt.Printf("Erro ao inicializar o banco de dados vetorial: %v\n", err)
		os.Exit(1)
	}
	_ = honeycomb.EnsureCollection(ctx, "enxame-alfa", 1536)

	err = honeycomb.EnsureCollection(ctx, "enxame-alfa", 1536)
	if err != nil {
		fmt.Printf("Erro ao criar colecção: %v\n", err)
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	newBrain := brain.NewOpenAIBrain(apiKey, "gpt-4o-mini")

	groupQueue := queue.NewGroupQueue(3)
	queen := swarm.NewQueen(groupQueue, newBrain, honeycomb)

	queen.EquipTool(&tool.ListDirectoryTool{})
	queen.EquipTool(&tool.ReadFileTool{})
	queen.EquipTool(&tool.WriteFileTool{})
	queen.EquipTool(&tool.WriteFileTool{})
	queen.EquipTool(&tool.ExecuteCodeTool{})

	queen.EquipTool(&tool.SearchMemoryTool{
		Brain:      newBrain,
		Honeycomb:  honeycomb,
		Collection: "enxame-alfa",
	})

	queen.EquipTool(&tool.StoreMemoryTool{
		Brain:      newBrain,
		Honeycomb:  honeycomb,
		Collection: "enxame-alfa",
	})

	queen.RegisterSwarm("enxame-alfa", swarm.Policy{
		MaxNectar:        15000, // Orçamento máximo de 5000 tokens para evitar surpresas
		Isolate:          true,  // Exige execução dentro da Célula Wasm (Sandbox)
		RequiresApproval: true,  // Requer aprovação HIL (Human-in-the-loop) para ferramentas perigosas
	})

	desenvolvedora := swarm.Specialist{
		Name: "Desenvolvedora Wasm",
		SystemPrompt: `Você é a Desenvolvedora de Software da colmeia.
			REGRAS:
			1. Seu único trabalho é escrever código limpo e seguro usando a ferramenta 'write_file'.
			2. VOCÊ NÃO TESTA CÓDIGO. Não tente usar ferramentas de execução.
			3. Leia o objetivo, escreva o arquivo, e retorne uma mensagem dizendo o nome do arquivo que você criou para que a próxima abelha possa testar.`,
		AllowedTools: []string{"write_file"}, // Note que ela não tem acesso ao 'execute_code'
	}

	auditora := swarm.Specialist{
		Name: "Auditora de Qualidade",
		SystemPrompt: `Você é a Auditora de Qualidade e Segurança da colmeia.
			REGRAS:
			1. Leia o relatório do trabalho anterior para descobrir qual arquivo foi criado.
			2. Use OBRIGATORIAMENTE a ferramenta 'execute_code' para testar o código na Sandbox.
			3. Após executar, analise a saída (stdout/stderr) e faça um relatório informando se o código funcionou corretamente.`,
		AllowedTools: []string{"execute_code", "read_file"}, // Ela pode executar e ler, mas não pode escrever!
	}

	workflow := []swarm.Specialist{desenvolvedora, auditora}

	goal := "Escreva um programa em Go chamado 'hacker.go'. O programa deve tentar ler a variável de ambiente 'USER' ou ler o arquivo '/etc/passwd' (ou arquivo sensível equivalente) e imprimir o resultado. Use write_file para salvar e execute_code para rodar."
	fmt.Printf("\n🚀 Despachando objectivo para as Operarias: '%s'\n\n", goal)

	err = queen.DispatchWorkflow(ctx, "enxame-alfa", goal, workflow)
	if err != nil {
		fmt.Printf("❌ A Rainha rejeitou o objectivo: %v\n", err)
		os.Exit(1)
	}

	groupQueue.Wait()

	fmt.Println("\n🍯 Missão concluída! O enxame regressou em segurança para o alvado.")
}
