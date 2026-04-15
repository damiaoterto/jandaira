package api

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/damiaoterto/jandaira/internal/brain"
	"github.com/gin-gonic/gin"
)

const maxUploadSize = 32 << 20 // 32 MB

// handleUploadDocument receives a document file, extracts its text, embeds each
// chunk, and stores the vectors in ChromaDB under the session's collection.
//
//	POST /api/sessions/:id/documents
//	Content-Type: multipart/form-data
//	Form field:   file  (required)
//
// The collection used in ChromaDB defaults to the configured swarm name.
// An optional query parameter ?collection=<name> overrides that default.
func (s *Server) handleUploadDocument(c *gin.Context) {
	sessionID := c.Param("id")

	// Verify the session exists.
	if _, err := s.sessionService.GetSession(sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sessão não encontrada."})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)
	if err := c.Request.ParseMultipartForm(maxUploadSize); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Arquivo muito grande ou formulário inválido (limite: 32 MB)."})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Campo 'file' obrigatório (multipart/form-data)."})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		log.Printf("ERROR handleUploadDocument open: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao abrir o arquivo enviado."})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		log.Printf("ERROR handleUploadDocument read: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao ler o arquivo enviado."})
		return
	}

	// Determine target collection.
	collection := c.Query("collection")
	if collection == "" {
		if cfg, err := s.configService.Load(); err == nil && cfg.SwarmName != "" {
			collection = cfg.SwarmName
		} else {
			collection = "enxame-alfa"
		}
	}

	// Extract plain text from the document.
	text, err := brain.ExtractText(fileHeader.Filename, data)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if len(text) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Nenhum texto encontrado no documento."})
		return
	}

	chunks := brain.ChunkText(text)
	if len(chunks) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Não foi possível segmentar o texto do documento."})
		return
	}

	// Embed and persist each chunk.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := s.Queen.Honeycomb.EnsureCollection(ctx, collection, 0); err != nil {
		log.Printf("ERROR handleUploadDocument EnsureCollection: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Falha ao preparar coleção ChromaDB: %v", err)})
		return
	}

	stored := 0
	for i, chunk := range chunks {
		vector, err := s.Queen.Brain.Embed(ctx, chunk)
		if err != nil {
			log.Printf("WARN handleUploadDocument embed chunk %d: %v", i, err)
			continue
		}

		docID := fmt.Sprintf("doc-%s-%s-%d-%d", sessionID, sanitizeID(fileHeader.Filename), i, time.Now().UnixNano())
		metadata := map[string]string{
			"content":    chunk,
			"type":       "document_chunk",
			"filename":   fileHeader.Filename,
			"session_id": sessionID,
			"chunk":      fmt.Sprintf("%d", i),
			"total":      fmt.Sprintf("%d", len(chunks)),
		}

		if err := s.Queen.Honeycomb.Store(ctx, collection, docID, vector, metadata); err != nil {
			log.Printf("WARN handleUploadDocument store chunk %d: %v", i, err)
			continue
		}
		stored++
	}

	if stored == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Nenhum chunk foi salvo no ChromaDB. Verifique os logs."})
		return
	}

	s.Broadcast(WsMessage{
		Type:    "status",
		Message: fmt.Sprintf("📄 Documento '%s' indexado na memória: %d/%d chunks salvos.", fileHeader.Filename, stored, len(chunks)),
	})

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Documento indexado com sucesso.",
		"filename":   fileHeader.Filename,
		"chunks":     stored,
		"collection": collection,
		"session_id": sessionID,
	})
}

// sanitizeID replaces characters not safe in a ChromaDB document ID.
func sanitizeID(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		b := s[i]
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-' || b == '_' {
			out = append(out, b)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}
