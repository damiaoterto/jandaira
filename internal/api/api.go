package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/damiaoterto/jandaira/internal/config"
	"github.com/damiaoterto/jandaira/internal/security"
	"github.com/damiaoterto/jandaira/internal/swarm"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WsMessage is the JSON structure sent and received by the frontend via WebSocket.
type WsMessage struct {
	Type     string `json:"type"`               // "status", "approval_request", "approve", "result", "error", "agent_change", "tool_start"
	ID       string `json:"id,omitempty"`       // unique ID of the approval request
	Message  string `json:"message,omitempty"`
	Tool     string `json:"tool,omitempty"`
	Args     string `json:"args,omitempty"`
	Agent    string `json:"agent,omitempty"`
	Approved bool   `json:"approved,omitempty"` // used by the frontend to respond (true/false)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	Queen      *swarm.Queen
	Port       int
	configPath string

	// WebSocket client management
	clients   map[*websocket.Conn]bool
	clientsMu sync.Mutex

	// Approval management (ID -> validity)
	pendingApprovals   map[string]bool
	pendingApprovalsMu sync.Mutex
}

func NewServer(q *swarm.Queen, port int, configPath string) *Server {
	s := &Server{
		Queen:            q,
		Port:             port,
		configPath:       configPath,
		clients:          make(map[*websocket.Conn]bool),
		pendingApprovals: make(map[string]bool),
	}

	// Fires an event whenever an agent takes control of the pipeline
	q.AgentChangeFunc = func(agentName string) {
		s.Broadcast(WsMessage{Type: "agent_change", Agent: agentName})
	}

	// Fires an event at the exact moment a tool begins execution
	q.ToolStartFunc = func(agentName string, toolName string, args string) {
		s.Broadcast(WsMessage{Type: "tool_start", Agent: agentName, Tool: toolName, Args: args})
	}

	return s
}

// RequestApproval generates a unique ID, registers the pending request, and broadcasts it to the UI via WebSocket.
func (s *Server) RequestApproval(toolName string, args string) {
	id := fmt.Sprintf("req-%d", time.Now().UnixNano())

	s.pendingApprovalsMu.Lock()
	s.pendingApprovals[id] = true
	s.pendingApprovalsMu.Unlock()

	s.Broadcast(WsMessage{
		Type: "approval_request",
		ID:   id,
		Tool: toolName,
		Args: args,
	})
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
	// Release mode to avoid polluting the terminal with Gin logs
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// WebSocket route
	r.GET("/ws", s.handleWebSocket)

	// REST routes
	api := r.Group("/api")
	api.Use(s.setupMiddleware())
	{
		api.POST("/setup", s.handleSetup)
		api.POST("/dispatch", s.handleDispatch)
		api.GET("/tools", s.handleListTools)
		api.GET("/agents", s.handleListAgents)
	}

	fmt.Printf("🌐 Jandaira server (Gin + WebSockets) listening on port %d...\n", s.Port)
	return r.Run(fmt.Sprintf(":%d", s.Port))
}

func (s *Server) setupMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Bypass setup for setup
		if c.Request.URL.Path == "/api/setup" {
			c.Next()
			return
		}

		_, err := config.Load(s.configPath)
		if err == config.ErrConfigNotFound {
			c.AbortWithStatusJSON(http.StatusPreconditionRequired, gin.H{
				"error": "A colmeia ainda não foi configurada. Execute POST em /api/setup primeiro.",
			})
			return
		}
		c.Next()
	}
}

func (s *Server) handleSetup(c *gin.Context) {
	_, err := config.Load(s.configPath)
	if err == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "A colmeia já foi configurada. Não é possível alterar a estrutura principal pelo setup base."})
		return
	}

	var rawReq struct {
		config.Config
		APIKey string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parâmetros inválidos para configuração."})
		return
	}

	req := rawReq.Config

	// Definir defaults se necessário
	if req.MaxNectar == 0 {
		req.MaxNectar = 20000
	}
	if req.Model == "" {
		req.Model = "gpt-4o-mini"
	}
	if req.SwarmName == "" {
		req.SwarmName = "enxame-alfa"
	}

	if err := config.Save(s.configPath, &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao gravar configuração no disco."})
		return
	}

	if rawReq.APIKey != "" {
		repoDir := security.GetDefaultVaultDir()
		if v, err := security.InitVault(repoDir); err == nil {
			_ = v.SaveSecret("OPENAI_API_KEY", rawReq.APIKey)
		}
	}

	// Regista a política na Queen que estava aguardando
	s.Queen.RegisterSwarm(req.SwarmName, swarm.Policy{
		MaxNectar:        req.MaxNectar,
		Isolate:          req.Isolated,
		RequiresApproval: req.Supervised,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Configuração salva com sucesso! O ecossistema está pronto e as operárias acordaram."})
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

	// Listen for messages from the frontend (including approval responses)
	go func() {
		defer func() {
			s.clientsMu.Lock()
			delete(s.clients, conn)
			s.clientsMu.Unlock()
			conn.Close()
		}()
		for {
			var incoming WsMessage
			if err := conn.ReadJSON(&incoming); err != nil {
				break // connection closed or error
			}

			// Frontend responded to an approval request
			if incoming.Type == "approve" && incoming.ID != "" {
				s.pendingApprovalsMu.Lock()
				valid := s.pendingApprovals[incoming.ID]
				if valid {
					delete(s.pendingApprovals, incoming.ID) // prevent double processing
				}
				s.pendingApprovalsMu.Unlock()

				if valid {
					s.Queen.ApprovalChan <- incoming.Approved
					statusMsg := "Action blocked by the Beekeeper."
					if incoming.Approved {
						statusMsg = "Action authorized by the Beekeeper. Resuming workflow..."
					}
					s.Broadcast(WsMessage{Type: "status", Message: "👨\u200d🌾 " + statusMsg})
				} else {
					s.Broadcast(WsMessage{Type: "error", Message: "Invalid or already processed approval ID."})
				}
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Field 'goal' is required."})
		return
	}

	if req.GroupID == "" {
		if cfg, err := config.Load(s.configPath); err == nil {
			req.GroupID = cfg.SwarmName
		} else {
			req.GroupID = "enxame-alfa"
		}
	}

	// Long-running task: offload to a goroutine and return HTTP immediately.
	// The frontend follows progress via WebSocket.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		s.Broadcast(WsMessage{Type: "status", Message: "🚀 Queen received the goal and is starting the swarm..."})

		maxWorkers := 3
		if cfg, err := config.Load(s.configPath); err == nil && cfg.MaxAgents > 0 {
			maxWorkers = cfg.MaxAgents
		}
		
		dynamicPipeline, err := s.Queen.AssembleSwarm(ctx, req.Goal, maxWorkers)
		if err != nil {
			s.Broadcast(WsMessage{Type: "error", Message: fmt.Sprintf("Error planning the swarm: %v", err)})
			return
		}

		resultChan, errChan := s.Queen.DispatchWorkflow(ctx, req.GroupID, req.Goal, dynamicPipeline)

		select {
		case res := <-resultChan:
			s.Broadcast(WsMessage{Type: "result", Message: res})
		case err := <-errChan:
			s.Broadcast(WsMessage{Type: "error", Message: err.Error()})
		case <-ctx.Done():
			s.Broadcast(WsMessage{Type: "error", Message: "Mission timeout reached."})
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{"message": "Mission dispatched to the swarm. Follow progress via WebSocket."})
}



func (s *Server) handleListTools(c *gin.Context) {
	toolsList := make([]map[string]interface{}, 0)

	// Read tools directly from the Queen's tool map
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
	// Since agents are assembled dynamically per request, we don't have a static list anymore.
	c.JSON(http.StatusOK, gin.H{"agents": []map[string]interface{}{}})
}
