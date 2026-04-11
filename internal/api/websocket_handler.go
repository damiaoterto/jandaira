package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// handleWebSocket upgrades the connection and registers the client for broadcasts.
// Incoming "approve" messages are forwarded to the Queen's approval channel.
//
//	GET /ws
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
