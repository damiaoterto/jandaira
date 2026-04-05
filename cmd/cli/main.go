package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/damiaoterto/jandaira/internal/brain"
	"github.com/damiaoterto/jandaira/internal/queue"
	"github.com/damiaoterto/jandaira/internal/swarm"
	"github.com/damiaoterto/jandaira/internal/tools"
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

	err = honeycomb.EnsureCollection(ctx, "enxame-alfa", 1536)
	if err != nil {
		fmt.Printf("Erro ao criar colecção: %v\n", err)
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	fmt.Println(apiKey)
	newBrain := brain.NewOpenAIBrain(apiKey, "gpt-4o-mini")

	groupQueue := queue.NewGroupQueue(3)
	queen := swarm.NewQueen(groupQueue, newBrain)

	queen.EquipTool(&tools.ListDirectoryTool{})
	queen.EquipTool(&tools.ReadFileTool{})

	queen.RegisterSwarm("enxame-alfa", swarm.Policy{
		MaxNectar:        5000, // Orçamento máximo de 5000 tokens para evitar surpresas
		Isolate:          true, // Exige execução dentro da Célula Wasm (Sandbox)
		RequiresApproval: true, // Requer aprovação HIL (Human-in-the-loop) para ferramentas perigosas
	})

	goal := "Analise o diretório actual, encontre ficheiros .go e faça um resumo de segurança."
	fmt.Printf("\n🚀 Despachando objectivo para as Operárias: '%s'\n\n", goal)

	err = queen.DispatchGoal(ctx, "enxame-alfa", goal)
	if err != nil {
		fmt.Printf("❌ A Rainha rejeitou o objectivo: %v\n", err)
		os.Exit(1)
	}

	groupQueue.Wait()

	fmt.Println("\n🍯 Missão concluída! O enxame regressou em segurança para o alvado.")
}
