package api

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/damiaoterto/jandaira/internal/model"
	"github.com/damiaoterto/jandaira/internal/service"
	"github.com/damiaoterto/jandaira/internal/swarm"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WsMessage is the JSON structure sent and received by the frontend via WebSocket.
//
// Outbound types (server → frontend):
//   - "status"           – generic progress message from the queen or a handler
//   - "log"              – raw log line emitted by the queen during a workflow
//   - "agent_created"    – the queen assembled a new specialist agent (AgentData is set)
//   - "agent_change"     – a new specialist agent has taken over (Agent field is set)
//   - "tool_start"       – an agent is about to execute a tool (Agent, Tool, Args are set)
//   - "approval_request" – the queen needs human approval before running a tool (ID, Tool, Args are set)
//   - "result"           – the workflow finished successfully (Message contains the final report)
//   - "error"            – the workflow failed or an unexpected condition occurred
//
// Inbound types (frontend → server):
//   - "approve" – the user approved or rejected a pending request (ID and Approved are set)
type WsMessage struct {
	Type      string       `json:"type"`
	ID        string       `json:"id,omitempty"` // approval request ID
	Message   string       `json:"message,omitempty"`
	Tool      string       `json:"tool,omitempty"`
	Args      string       `json:"args,omitempty"`
	Agent     string       `json:"agent,omitempty"`
	AgentData *model.Agent `json:"agent_data,omitempty"` // agent_created: full agent record
	Approved  bool         `json:"approved,omitempty"`   // inbound: true = approved, false = denied
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
	colmeiaService service.ColmeiaService
	skillService   service.SkillService

	// WebSocket client management
	clients   map[*websocket.Conn]bool
	clientsMu sync.Mutex

	// Approval management (ID -> validity)
	pendingApprovals   map[string]bool
	pendingApprovalsMu sync.Mutex
}

func NewServer(q *swarm.Queen, port int, cfgService service.ConfigService, sessionSvc service.SessionService, colmeiaSvc service.ColmeiaService, skillSvc service.SkillService) *Server {
	s := &Server{
		Queen:            q,
		Port:             port,
		configService:    cfgService,
		sessionService:   sessionSvc,
		colmeiaService:   colmeiaSvc,
		skillService:     skillSvc,
		clients:          make(map[*websocket.Conn]bool),
		pendingApprovals: make(map[string]bool),
	}

	q.LogFunc = func(msg string) {
		s.Broadcast(WsMessage{Type: "log", Message: msg})
	}

	q.AgentChangeFunc = func(agentName string) {
		s.Broadcast(WsMessage{Type: "agent_change", Agent: agentName})
	}

	q.ToolStartFunc = func(agentName string, toolName string, args string) {
		s.Broadcast(WsMessage{Type: "tool_start", Agent: agentName, Tool: toolName, Args: args})
	}

	q.AskPermissionFunc = func(toolName string, args string) {
		s.RequestApproval(toolName, args)
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

func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func (s *Server) Start() error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.Use(s.corsMiddleware())

	r.GET("/ws", s.handleWebSocket)

	api := r.Group("/api")
	api.Use(s.setupMiddleware())
	{
		api.POST("/setup", s.handleSetup)
		api.GET("/config", s.handleGetConfig)
		api.PUT("/config", s.handleUpdateConfig)
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
			sessions.POST("/:id/documents", s.handleUploadDocument)
		}

		// Skill routes (catálogo global de skills reutilizáveis)
		skills := api.Group("/skills")
		{
			skills.GET("", s.handleListSkills)
			skills.POST("", s.handleCreateSkill)
			skills.GET("/:id", s.handleGetSkill)
			skills.PUT("/:id", s.handleUpdateSkill)
			skills.DELETE("/:id", s.handleDeleteSkill)
		}

		// Colmeia routes (colmeias persistentes com agentes e histórico)
		colmeias := api.Group("/colmeias")
		{
			colmeias.GET("", s.handleListColmeias)
			colmeias.POST("", s.handleCreateColmeia)
			colmeias.GET("/:id", s.handleGetColmeia)
			colmeias.PUT("/:id", s.handleUpdateColmeia)
			colmeias.DELETE("/:id", s.handleDeleteColmeia)
			colmeias.POST("/:id/dispatch", s.handleColmeiaDispatch)
			colmeias.GET("/:id/historico", s.handleListHistoricoColmeia)
			colmeias.POST("/:id/documents", s.handleColmeiaUploadDocument)

			// Skills associadas à colmeia
			colmeias.GET("/:id/skills", s.handleListColmeiaSkills)
			colmeias.POST("/:id/skills", s.handleAttachSkillToColmeia)
			colmeias.DELETE("/:id/skills/:skillId", s.handleDetachSkillFromColmeia)

			agentes := colmeias.Group("/:id/agentes")
			{
				agentes.GET("", s.handleListAgentesColmeia)
				agentes.POST("", s.handleAddAgenteColmeia)
				agentes.GET("/:agentId", s.handleGetAgenteColmeia)
				agentes.PUT("/:agentId", s.handleUpdateAgenteColmeia)
				agentes.DELETE("/:agentId", s.handleRemoveAgenteColmeia)

				// Skills associadas ao agente
				agentes.GET("/:agentId/skills", s.handleListAgenteSkills)
				agentes.POST("/:agentId/skills", s.handleAttachSkillToAgente)
				agentes.DELETE("/:agentId/skills/:skillId", s.handleDetachSkillFromAgente)
			}
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
