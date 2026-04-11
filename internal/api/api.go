package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/damiaoterto/jandaira/internal/model"
	"github.com/damiaoterto/jandaira/internal/security"
	"github.com/damiaoterto/jandaira/internal/service"
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
	Queen          *swarm.Queen
	Port           int
	configService  service.ConfigService
	sessionService service.SessionService

	// WebSocket client management
	clients   map[*websocket.Conn]bool
	clientsMu sync.Mutex

	// Approval management (ID -> validity)
	pendingApprovals   map[string]bool
	pendingApprovalsMu sync.Mutex
}

func NewServer(q *swarm.Queen, port int, cfgService service.ConfigService, sessionSvc service.SessionService) *Server {
	s := &Server{
		Queen:            q,
		Port:             port,
		configService:    cfgService,
		sessionService:   sessionSvc,
		clients:          make(map[*websocket.Conn]bool),
		pendingApprovals: make(map[string]bool),
	}

	q.AgentChangeFunc = func(agentName string) {
		s.Broadcast(WsMessage{Type: "agent_change", Agent: agentName})
	}

	q.ToolStartFunc = func(agentName string, toolName string, args string) {
		s.Broadcast(WsMessage{Type: "tool_start", Agent: agentName, Tool: toolName, Args: args})
	}

	return s
}

// RequestApproval generates a unique ID, registers the pending request, and
// broadcasts it to the UI via WebSocket.
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
		if err := client.WriteJSON(msg); err != nil {
			client.Close()
			delete(s.clients, client)
		}
	}
}

func (s *Server) Start() error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.GET("/ws", s.handleWebSocket)

	api := r.Group("/api")
	api.Use(s.setupMiddleware())
	{
		api.POST("/setup", s.handleSetup)
		api.POST("/dispatch", s.handleDispatch)
		api.GET("/tools", s.handleListTools)
		api.GET("/agents", s.handleListAgents)

		// Session routes
		sessions := api.Group("/sessions")
		{
			sessions.GET("", s.handleListSessions)
			sessions.POST("", s.handleCreateSession)
			sessions.GET("/:id", s.handleGetSession)
			sessions.DELETE("/:id", s.handleDeleteSession)
			sessions.POST("/:id/dispatch", s.handleSessionDispatch)
			sessions.GET("/:id/agents", s.handleListSessionAgents)
		}
	}

	fmt.Printf("🌐 Jandaira server (Gin + WebSockets) listening on port %d...\n", s.Port)
	return r.Run(fmt.Sprintf(":%d", s.Port))
}

// setupMiddleware blocks all routes except /api/setup while the app is not configured.
func (s *Server) setupMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/api/setup" {
			c.Next()
			return
		}

		configured, err := s.configService.IsConfigured()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Erro ao verificar configuração.",
			})
			return
		}
		if !configured {
			c.AbortWithStatusJSON(http.StatusPreconditionRequired, gin.H{
				"error": "A colmeia ainda não foi configurada. Execute POST em /api/setup primeiro.",
			})
			return
		}

		c.Next()
	}
}

func (s *Server) handleSetup(c *gin.Context) {
	configured, err := s.configService.IsConfigured()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao verificar configuração."})
		return
	}
	if configured {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "A colmeia já foi configurada. Não é possível alterar a estrutura principal pelo setup base.",
		})
		return
	}

	var rawReq struct {
		model.AppConfig
		APIKey string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parâmetros inválidos para configuração."})
		return
	}

	cfg := rawReq.AppConfig

	if cfg.MaxNectar == 0 {
		cfg.MaxNectar = 20000
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}
	if cfg.SwarmName == "" {
		cfg.SwarmName = "enxame-alfa"
	}

	if err := s.configService.Save(&cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao gravar configuração no banco de dados."})
		return
	}

	if rawReq.APIKey != "" {
		repoDir := security.GetDefaultVaultDir()
		if v, err := security.InitVault(repoDir); err == nil {
			_ = v.SaveSecret("OPENAI_API_KEY", rawReq.APIKey)
		}
	}

	s.Queen.RegisterSwarm(cfg.SwarmName, swarm.Policy{
		MaxNectar:        cfg.MaxNectar,
		Isolate:          cfg.Isolated,
		RequiresApproval: cfg.Supervised,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Configuração salva com sucesso! O ecossistema está pronto e as operárias acordaram.",
	})
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
			var incoming WsMessage
			if err := conn.ReadJSON(&incoming); err != nil {
				break
			}

			if incoming.Type == "approve" && incoming.ID != "" {
				s.pendingApprovalsMu.Lock()
				valid := s.pendingApprovals[incoming.ID]
				if valid {
					delete(s.pendingApprovals, incoming.ID)
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
		if cfg, err := s.configService.Load(); err == nil {
			req.GroupID = cfg.SwarmName
		} else {
			req.GroupID = "enxame-alfa"
		}
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		s.Broadcast(WsMessage{Type: "status", Message: "🚀 Queen received the goal and is starting the swarm..."})

		maxWorkers := 3
		if cfg, err := s.configService.Load(); err == nil && cfg.MaxAgents > 0 {
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
	c.JSON(http.StatusOK, gin.H{"agents": []map[string]interface{}{}})
}

