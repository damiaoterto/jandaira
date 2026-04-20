package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/damiaoterto/jandaira/internal/brain"
	"github.com/damiaoterto/jandaira/internal/service"
	"github.com/gin-gonic/gin"
)

const maxUploadSize = 32 << 20 // 32 MB

// workspaceDir is the root directory where extracted document texts are written
// so that the read_file tool can access them by name.
const workspaceDir = "workspace/sessions"

// handleUploadDocument receives a document file, extracts its text, embeds each
// chunk, stores the vectors in ChromaDB, and writes the extracted text to disk
// so that the read_file tool can access it during mission execution.
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
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found."})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)
	if err := c.Request.ParseMultipartForm(maxUploadSize); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large or invalid form (limit: 32 MB)."})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Field 'file' is required (multipart/form-data)."})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		log.Printf("ERROR handleUploadDocument open: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open uploaded file."})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		log.Printf("ERROR handleUploadDocument read: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read uploaded file."})
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
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "No text found in document."})
		return
	}

	chunks := brain.ChunkText(text)
	if len(chunks) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Failed to segment document text."})
		return
	}

	// Write extracted text to disk so read_file can access it during mission.
	workspacePath, diskErr := saveTextToDisk(sessionID, fileHeader.Filename, text)
	if diskErr != nil {
		log.Printf("WARN handleUploadDocument saveTextToDisk: %v", diskErr)
		// Non-fatal: ChromaDB indexing still proceeds.
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	stored, embeddingUnavailable := storeChunksInVectorDB(ctx, s, chunks, collection, sessionID, fileHeader.Filename, workspacePath, "session_id")

	if embeddingUnavailable {
		s.Broadcast(WsMessage{
			Type:    "status",
			Message: fmt.Sprintf("📄 Document '%s' saved to workspace (embedding not supported by current provider): %s", fileHeader.Filename, workspacePath),
		})
		s.saveDocumentRecord(fileHeader.Filename, workspacePath, "", "session_id", sessionID, 0)
		c.JSON(http.StatusCreated, gin.H{
			"message":        "Document saved to workspace. Embedding not supported by current provider (use OpenAI for semantic search).",
			"filename":       fileHeader.Filename,
			"workspace_path": workspacePath,
			"chunks":         0,
			"collection":     "",
			"session_id":     sessionID,
		})
		return
	}

	if stored == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No chunks saved to vector database. Check the logs."})
		return
	}

	s.saveDocumentRecord(fileHeader.Filename, workspacePath, collection, "session_id", sessionID, stored)

	s.Broadcast(WsMessage{
		Type:    "status",
		Message: fmt.Sprintf("📄 Document '%s' indexed in memory: %d/%d chunks saved. Workspace: %s", fileHeader.Filename, stored, len(chunks), workspacePath),
	})

	c.JSON(http.StatusCreated, gin.H{
		"message":        "Document indexed successfully.",
		"filename":       fileHeader.Filename,
		"workspace_path": workspacePath,
		"chunks":         stored,
		"collection":     collection,
		"session_id":     sessionID,
	})
}

// handleColmeiaUploadDocument indexes a document into the colmeia's scoped vector
// memory in Qdrant (collection = colmeiaID). Falls back to disk-only when the
// active LLM provider does not support embeddings (e.g. Anthropic).
//
//	POST /api/colmeias/:id/documents
//	Content-Type: multipart/form-data
//	Form field:   file  (required)
func (s *Server) handleColmeiaUploadDocument(c *gin.Context) {
	colmeiaID := c.Param("id")

	if _, err := s.colmeiaService.GetColmeia(colmeiaID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Hive not found."})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)
	if err := c.Request.ParseMultipartForm(maxUploadSize); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large or invalid form (limit: 32 MB)."})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Field 'file' is required (multipart/form-data)."})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		log.Printf("ERROR handleColmeiaUploadDocument open: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open uploaded file."})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		log.Printf("ERROR handleColmeiaUploadDocument read: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read uploaded file."})
		return
	}

	text, err := brain.ExtractText(fileHeader.Filename, data)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if len(text) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "No text found in document."})
		return
	}

	chunks := brain.ChunkText(text)
	if len(chunks) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Failed to segment document text."})
		return
	}

	workspacePath, diskErr := saveColmeiaTextToDisk(colmeiaID, fileHeader.Filename, text)
	if diskErr != nil {
		log.Printf("WARN handleColmeiaUploadDocument saveTextToDisk: %v", diskErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Collection scoped to the colmeia so its semantic memory is isolated.
	collection := "colmeia-" + sanitizeID(colmeiaID)

	stored, embeddingUnavailable := storeChunksInVectorDB(ctx, s, chunks, collection, colmeiaID, fileHeader.Filename, workspacePath, "colmeia_id")

	if embeddingUnavailable {
		s.Broadcast(WsMessage{
			Type:    "status",
			Message: fmt.Sprintf("📄 Document '%s' saved to hive workspace (embedding not supported by current provider): %s", fileHeader.Filename, workspacePath),
		})
		s.saveDocumentRecord(fileHeader.Filename, workspacePath, "", "colmeia_id", colmeiaID, 0)
		c.JSON(http.StatusCreated, gin.H{
			"message":        "Document saved to workspace. Embedding not supported by current provider (use OpenAI for semantic search).",
			"filename":       fileHeader.Filename,
			"workspace_path": workspacePath,
			"chunks":         0,
			"collection":     "",
			"colmeia_id":     colmeiaID,
		})
		return
	}

	if stored == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No chunks saved to vector database. Check the logs."})
		return
	}

	s.saveDocumentRecord(fileHeader.Filename, workspacePath, collection, "colmeia_id", colmeiaID, stored)

	s.Broadcast(WsMessage{
		Type:    "status",
		Message: fmt.Sprintf("📄 Document '%s' indexed in hive memory: %d/%d chunks saved. Workspace: %s", fileHeader.Filename, stored, len(chunks), workspacePath),
	})

	c.JSON(http.StatusCreated, gin.H{
		"message":        "Document indexed successfully in hive.",
		"filename":       fileHeader.Filename,
		"workspace_path": workspacePath,
		"chunks":         stored,
		"collection":     collection,
		"colmeia_id":     colmeiaID,
	})
}

// handleListSessionDocuments lists all documents uploaded to a session.
//
//	GET /api/sessions/:id/documents
func (s *Server) handleListSessionDocuments(c *gin.Context) {
	sessionID := c.Param("id")
	if _, err := s.sessionService.GetSession(sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found."})
		return
	}
	docs, err := s.documentService.ListByScope("session_id", sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list documents."})
		return
	}
	c.JSON(http.StatusOK, docs)
}

// handleListColmeiaDocuments lists all documents uploaded to a colmeia.
//
//	GET /api/colmeias/:id/documents
func (s *Server) handleListColmeiaDocuments(c *gin.Context) {
	colmeiaID := c.Param("id")
	if _, err := s.colmeiaService.GetColmeia(colmeiaID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Hive not found."})
		return
	}
	docs, err := s.documentService.ListByScope("colmeia_id", colmeiaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list documents."})
		return
	}
	c.JSON(http.StatusOK, docs)
}

// handleDeleteDocument deletes a document record from SQLite, its chunks from
// Qdrant (when a collection is recorded), and the workspace file from disk.
//
//	DELETE /api/sessions/:id/documents/:docId
//	DELETE /api/colmeias/:id/documents/:docId
func (s *Server) handleDeleteDocument(c *gin.Context) {
	docID := c.Param("docId")

	doc, err := s.documentService.GetDocument(docID)
	if err != nil {
		if errors.Is(err, service.ErrDocumentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Document not found."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch document."})
		return
	}

	// Delete chunks from Qdrant when they were indexed.
	if doc.Collection != "" && s.Queen.Honeycomb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		filter := map[string]string{
			"filename": doc.Filename,
			doc.ScopeKey: doc.ScopeVal,
		}
		if qdErr := s.Queen.Honeycomb.DeleteByFilter(ctx, doc.Collection, filter); qdErr != nil {
			log.Printf("WARN handleDeleteDocument Qdrant: %v", qdErr)
			// Non-fatal: proceed with SQLite and disk cleanup.
		}
	}

	// Remove workspace file from disk.
	if doc.WorkspacePath != "" {
		if rmErr := os.Remove(doc.WorkspacePath); rmErr != nil && !os.IsNotExist(rmErr) {
			log.Printf("WARN handleDeleteDocument os.Remove: %v", rmErr)
		}
	}

	// Remove SQLite record.
	if err := s.documentService.Delete(docID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete document."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Document deleted successfully."})
}

// saveDocumentRecord persists a Document record to SQLite. Non-fatal on error.
func (s *Server) saveDocumentRecord(filename, workspacePath, collection, scopeKey, scopeVal string, chunks int) {
	if _, err := s.documentService.Create(filename, workspacePath, collection, scopeKey, scopeVal, chunks); err != nil {
		log.Printf("WARN saveDocumentRecord: %v", err)
	}
}

// storeChunksInVectorDB embeds each chunk and upserts it into Qdrant under the
// given collection. The collection is created lazily on the first successful
// embed so the correct vector dimension is used. scopeKey/scopeVal are added
// to every chunk's metadata (e.g. "session_id"/"abc" or "colmeia_id"/"xyz").
//
// Returns (stored int, embeddingUnavailable bool). embeddingUnavailable is true
// when the active provider does not support embeddings (e.g. Anthropic); in
// that case stored is always 0 and callers should degrade gracefully.
func storeChunksInVectorDB(
	ctx context.Context,
	s *Server,
	chunks []string,
	collection, scopeVal, filename, workspacePath, scopeKey string,
) (stored int, embeddingUnavailable bool) {
	if s.Queen.Honeycomb == nil {
		return 0, false
	}

	collectionReady := false

	for i, chunk := range chunks {
		vector, err := s.Queen.Brain.Embed(ctx, chunk)
		if err != nil {
			if i == 0 {
				log.Printf("WARN storeChunksInVectorDB embed unavailable: %v", err)
				return 0, true
			}
			log.Printf("WARN storeChunksInVectorDB embed chunk %d: %v", i, err)
			continue
		}

		if !collectionReady {
			if ensErr := s.Queen.Honeycomb.EnsureCollection(ctx, collection, len(vector)); ensErr != nil {
				log.Printf("ERROR storeChunksInVectorDB EnsureCollection: %v", ensErr)
				return stored, false
			}
			collectionReady = true
		}

		docID := fmt.Sprintf("doc-%s-%s-%d-%d", scopeVal, sanitizeID(filename), i, time.Now().UnixNano())
		metadata := map[string]string{
			"content":        toValidUTF8(chunk),
			"type":           "document_chunk",
			"filename":       toValidUTF8(filename),
			scopeKey:         toValidUTF8(scopeVal),
			"workspace_path": toValidUTF8(workspacePath),
			"chunk":          fmt.Sprintf("%d", i),
			"total":          fmt.Sprintf("%d", len(chunks)),
		}

		if storeErr := s.Queen.Honeycomb.Store(ctx, collection, docID, vector, metadata); storeErr != nil {
			log.Printf("WARN storeChunksInVectorDB store chunk %d: %v", i, storeErr)
			continue
		}
		stored++
	}

	return stored, false
}

// toValidUTF8 returns s with any invalid UTF-8 sequences replaced by the
// Unicode replacement character so Qdrant does not panic on non-UTF-8 input
// (e.g. Latin-1 encoded CSV/PDF files).
func toValidUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "\uFFFD")
}

// saveColmeiaTextToDisk writes extracted text to workspace/colmeias/{colmeiaID}/{stem}.txt.
func saveColmeiaTextToDisk(colmeiaID, originalFilename, text string) (string, error) {
	const colmeiaWorkspaceDir = "workspace/colmeias"
	dir := filepath.Join(colmeiaWorkspaceDir, colmeiaID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("falha ao criar diretório workspace: %w", err)
	}

	base := strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))
	filename := sanitizeID(base) + ".txt"
	fullPath := filepath.Join(dir, filename)

	if err := os.WriteFile(fullPath, []byte(text), 0644); err != nil {
		return "", fmt.Errorf("falha ao escrever arquivo no workspace: %w", err)
	}

	return filepath.Join(colmeiaWorkspaceDir, colmeiaID, filename), nil
}

// saveTextToDisk writes the extracted text to workspace/sessions/{sessionID}/{stem}.txt
// so that the read_file tool can read it by path during mission execution.
// Returns the relative path that agents should use with read_file.
func saveTextToDisk(sessionID, originalFilename, text string) (string, error) {
	dir := filepath.Join(workspaceDir, sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("falha ao criar diretório workspace: %w", err)
	}

	// Strip the original extension and always save as .txt so agents
	// know exactly what extension to use with read_file.
	base := strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))
	filename := sanitizeID(base) + ".txt"
	fullPath := filepath.Join(dir, filename)

	if err := os.WriteFile(fullPath, []byte(text), 0644); err != nil {
		return "", fmt.Errorf("falha ao escrever arquivo no workspace: %w", err)
	}

	// Return the relative path that read_file resolves from CWD.
	return filepath.Join(workspaceDir, sessionID, filename), nil
}

// sanitizeID replaces characters not safe in a ChromaDB document ID or filename.
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
