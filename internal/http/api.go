package http

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/damiaoterto/jandaira/internal/swarm"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WsMessage struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
	Tool    string `json:"tool,omitempty"`
	Args    string `json:"args,omitempty"`
	Agent   string `json:"agent,omitempty"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	Queen    *swarm.Queen
	Workflow []swarm.Specialist
	Port     int

	clients   map[*websocket.Conn]bool
	clientsMu sync.Mutex
}

func NewServer(q *swarm.Queen, w []swarm.Specialist, port int) *Server {
	s := &Server{
		Queen:    q,
		Workflow: w,
		Port:     port,
		clients:  make(map[*websocket.Conn]bool),
	}

	// Dispara evento no exato momento que um agente assume o controle
	q.AgentChangeFunc = func(agentName string) {
		s.Broadcast(WsMessage{
			Type:  "agent_change",
			Agent: agentName,
		})
	}

	// Dispara evento no exato momento que uma ferramenta inicia execução
	q.ToolStartFunc = func(agentName string, toolName string, args string) {
		s.Broadcast(WsMessage{
			Type:  "tool_start",
			Agent: agentName,
			Tool:  toolName,
			Args:  args,
		})
	}

	return s
}

func (s *Server) Broadcast(msg WsMessage) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	for client := range s.clients {
		err := client.WriteJSON(msg)
		if err != nil {
			client.Close()
			delete(s.clients, client)
		}
	}
}

func (s *Server) Start() error {
	// Modo release para não poluir o terminal com logs do Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Rota do WebSocket
	r.GET("/ws", s.handleWebSocket)

	// Rotas REST
	api := r.Group("/api")
	{
		api.POST("/dispatch", s.handleDispatch)
		api.POST("/approve", s.handleApproval)

		// NOVAS ROTAS DE METADADOS PARA A UI
		api.GET("/tools", s.handleListTools)
		api.GET("/agents", s.handleListAgents)
	}

	fmt.Printf("🌐 Servidor da Jandaira (Gin + WebSockets) voando na porta %d...\n", s.Port)
	return r.Run(fmt.Sprintf(":%d", s.Port))
}

func (s *Server) handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Printf("Falha no upgrade para WebSocket: %v\n", err)
		return
	}

	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	go func() {
		defer func() {
			s.clientsMu.Lock()
			delete(s.clients, conn)
			s.clientsMu.Unlock()
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}

func (s *Server) handleDispatch(c *gin.Context) {
	var req struct {
		Goal    string `json:"goal" binding:"required"`
		GroupID string `json:"group_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Objetivo (goal) é obrigatório."})
		return
	}

	if req.GroupID == "" {
		req.GroupID = "enxame-alfa"
	}

	// Como a execução pode demorar, fazemos isso em uma goroutine
	// e liberamos o POST HTTP imediatamente. O frontend acompanha pelo WS.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		s.Broadcast(WsMessage{Type: "status", Message: "🚀 A Rainha recebeu o objetivo e está iniciando o enxame..."})

		resultChan, errChan := s.Queen.DispatchWorkflow(ctx, req.GroupID, req.Goal, s.Workflow)

		select {
		case res := <-resultChan:
			s.Broadcast(WsMessage{Type: "result", Message: res})
		case err := <-errChan:
			s.Broadcast(WsMessage{Type: "error", Message: err.Error()})
		case <-ctx.Done():
			s.Broadcast(WsMessage{Type: "error", Message: "Timeout da missão atingido."})
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{"message": "Missão despachada para o enxame. Acompanhe via WebSocket."})
}

func (s *Server) handleApproval(c *gin.Context) {
	var req struct {
		Approved bool `json:"approved"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O campo boolean 'approved' é obrigatório."})
		return
	}

	// Envia a resposta do Apicultor para a Rainha desbloquear o fluxo
	s.Queen.ApprovalChan <- req.Approved

	msg := "Ação bloqueada pelo Apicultor."
	if req.Approved {
		msg = "Ação autorizada pelo Apicultor. Retomando fluxo..."
	}
	s.Broadcast(WsMessage{Type: "status", Message: "👨‍🌾 " + msg})

	c.JSON(http.StatusOK, gin.H{"message": "Resposta do Apicultor registrada com sucesso."})
}

func (s *Server) handleListTools(c *gin.Context) {
	toolsList := make([]map[string]interface{}, 0)

	// Acessando as ferramentas diretamente do mapa da Rainha
	for name, tool := range s.Queen.Tools {
		toolsList = append(toolsList, map[string]interface{}{
			"name":        name,
			"description": tool.Description(),
			"parameters":  tool.Parameters(),
		})
	}

	c.JSON(http.StatusOK, gin.H{"tools": toolsList})
}

func (s *Server) handleListAgents(c *gin.Context) {
	agentsList := make([]map[string]interface{}, 0)

	for _, agent := range s.Workflow {
		agentsList = append(agentsList, map[string]interface{}{
			"name":          agent.Name,
			"system_prompt": agent.SystemPrompt,
			"allowed_tools": agent.AllowedTools,
		})
	}

	c.JSON(http.StatusOK, gin.H{"agents": agentsList})
}
