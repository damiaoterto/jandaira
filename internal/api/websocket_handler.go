package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// handleWebSocket upgrades the HTTP connection to WebSocket, registers the
// client so it receives all broadcasts, then starts reading messages from it.
//
//	GET /ws
func (s *Server) handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade failed: %v\n", err)
		return
	}

	s.addClient(conn)
	go s.readClientMessages(conn)
}

// addClient registers a connection in the broadcast pool.
func (s *Server) addClient(conn *websocket.Conn) {
	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()
}

// removeClient unregisters a connection and closes it.
func (s *Server) removeClient(conn *websocket.Conn) {
	s.clientsMu.Lock()
	delete(s.clients, conn)
	s.clientsMu.Unlock()
	conn.Close()
}

// readClientMessages loops until the connection is closed, dispatching each
// incoming message to handleClientMessage.
func (s *Server) readClientMessages(conn *websocket.Conn) {
	defer s.removeClient(conn)

	for {
		var msg WsMessage
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}
		s.handleClientMessage(msg)
	}
}

// handleClientMessage processes a single message received from the frontend.
// Currently only the "approve" type is handled; unknown types are silently ignored.
func (s *Server) handleClientMessage(msg WsMessage) {
	if msg.Type != "approve" || msg.ID == "" {
		return
	}

	s.pendingApprovalsMu.Lock()
	valid := s.pendingApprovals[msg.ID]
	if valid {
		delete(s.pendingApprovals, msg.ID)
	}
	s.pendingApprovalsMu.Unlock()

	if !valid {
		s.Broadcast(WsMessage{Type: "error", Message: "Invalid or already processed approval ID."})
		return
	}

	s.Queen.ApprovalChan <- msg.Approved

	statusMsg := "Action blocked by the Beekeeper."
	if msg.Approved {
		statusMsg = "Action authorized by the Beekeeper. Resuming workflow..."
	}
	s.Broadcast(WsMessage{Type: "status", Message: "👨\u200d🌾 " + statusMsg})
}
