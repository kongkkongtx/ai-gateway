package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kongkkongtx/ai-gateway/internal/config"
	"github.com/kongkkongtx/ai-gateway/internal/rag"
)

func (g *Gateway) handleListKnowledgeBases(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, g.cfg.RAG.KnowledgeBases)
}

func (g *Gateway) handleCreateKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	var kb rag.KnowledgeBase
	if err := json.NewDecoder(r.Body).Decode(&kb); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	if kb.Name == "" {
		writeError(w, http.StatusBadRequest, "Knowledge base name is required")
		return
	}
	if kb.ID == "" {
		kb.ID = fmt.Sprintf("kb-%d", time.Now().UnixNano())
	}
	if kb.ChunkSize <= 0 {
		kb.ChunkSize = 800
	}
	if kb.ChunkOverlap <= 0 {
		kb.ChunkOverlap = 100
	}

	// Check for duplicates
	for _, existing := range g.cfg.RAG.KnowledgeBases {
		if existing.Name == kb.Name {
			writeError(w, http.StatusBadRequest, "Knowledge base already exists: "+kb.Name)
			return
		}
	}

	g.cfg.RAG.KnowledgeBases = append(g.cfg.RAG.KnowledgeBases, kb)
	if _, err := config.SaveVersioned(g.cfg, g.configPath, "Knowledge base created: "+kb.Name); err != nil {
		g.logger.Warn("failed to persist RAG config", "error", err)
	}
	g.logger.Info("knowledge base created", "id", kb.ID, "name", kb.Name)
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok", "id": kb.ID})
}

func (g *Gateway) handleGetKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	for _, kb := range g.cfg.RAG.KnowledgeBases {
		if kb.ID == id {
			writeJSON(w, http.StatusOK, kb)
			return
		}
	}
	writeError(w, http.StatusNotFound, "Knowledge base not found")
}

func (g *Gateway) handleDeleteKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	kbs := g.cfg.RAG.KnowledgeBases
	for i, kb := range kbs {
		if kb.ID == id {
			g.cfg.RAG.KnowledgeBases = append(kbs[:i], kbs[i+1:]...)

			// Remove chunks from store
			if g.ragStore != nil {
				if err := g.ragStore.DeleteByKB(r.Context(), id); err != nil {
					g.logger.Warn("failed to delete KB chunks", "id", id, "error", err)
				}
			}

			if _, err := config.SaveVersioned(g.cfg, g.configPath, "Knowledge base deleted: "+id); err != nil {
				g.logger.Warn("failed to persist RAG config", "error", err)
			}
			g.logger.Info("knowledge base deleted", "id", id)
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
	}
	writeError(w, http.StatusNotFound, "Knowledge base not found")
}

type ingestRequest struct {
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

func (g *Gateway) handleIngestDocument(w http.ResponseWriter, r *http.Request) {
	if g.ragStore == nil {
		writeError(w, http.StatusBadRequest, "RAG engine is not initialized")
		return
	}

	id := chi.URLParam(r, "id")

	// Verify KB exists
	var found bool
	for _, kb := range g.cfg.RAG.KnowledgeBases {
		if kb.ID == id {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "Knowledge base not found")
		return
	}

	var req ingestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "Document content is required")
		return
	}

	// Find chunking parameters
	var chunkSize, chunkOverlap int
	for _, kb := range g.cfg.RAG.KnowledgeBases {
		if kb.ID == id {
			chunkSize = kb.ChunkSize
			chunkOverlap = kb.ChunkOverlap
			break
		}
	}

	// Chunk the document
	chunker := rag.NewRecursiveChunker(chunkSize, chunkOverlap)
	doc := rag.Document{
		ID:       fmt.Sprintf("doc-%d", time.Now().UnixNano()),
		Content:  req.Content,
		Metadata: req.Metadata,
		KBID:     id,
	}

	chunks, err := chunker.Chunk(doc)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Document chunking failed: "+err.Error())
		return
	}

	// Generate IDs and embed if embedder is available
	for i := range chunks {
		chunks[i].ID = rag.ChunkID(doc.ID, i+1)
		if g.ragEmbedder != nil {
			vec, err := g.ragEmbedder.Embed(chunks[i].Content)
			if err != nil {
				g.logger.Warn("rag: chunk embedding failed", "error", err)
			} else {
				chunks[i].Embedding = vec
			}
		}
	}

	if err := g.ragStore.Add(r.Context(), chunks); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to store chunks: "+err.Error())
		return
	}

	g.logger.Info("document ingested",
		"kb", id,
		"chunks", len(chunks),
		"size", len(req.Content),
	)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":   "ok",
		"chunks":   len(chunks),
		"doc_id":   doc.ID,
	})
}

type ingestBatchRequest struct {
	Documents []ingestRequest `json:"documents"`
}

func (g *Gateway) handleIngestBatch(w http.ResponseWriter, r *http.Request) {
	if g.ragStore == nil {
		writeError(w, http.StatusBadRequest, "RAG engine is not initialized")
		return
	}

	id := chi.URLParam(r, "id")
	var req ingestBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	if len(req.Documents) == 0 {
		writeError(w, http.StatusBadRequest, "At least one document is required")
		return
	}

	totalChunks := 0
	for _, docReq := range req.Documents {
		if docReq.Content == "" {
			continue
		}
		// Reuse single ingest logic
		chunker := rag.NewRecursiveChunker(800, 100)
		doc := rag.Document{
			ID:       fmt.Sprintf("doc-%d", time.Now().UnixNano()),
			Content:  docReq.Content,
			Metadata: docReq.Metadata,
			KBID:     id,
		}
		chunks, _ := chunker.Chunk(doc)
		for i := range chunks {
			chunks[i].ID = rag.ChunkID(doc.ID, i+1)
			if g.ragEmbedder != nil {
				vec, _ := g.ragEmbedder.Embed(chunks[i].Content)
				chunks[i].Embedding = vec
			}
		}
		g.ragStore.Add(r.Context(), chunks)
		totalChunks += len(chunks)
	}

	g.logger.Info("batch ingest completed", "kb", id, "chunks", totalChunks)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status": "ok",
		"chunks": totalChunks,
	})
}

func (g *Gateway) handleKBStats(w http.ResponseWriter, r *http.Request) {
	if g.ragStore == nil {
		writeError(w, http.StatusNotFound, "RAG engine not initialized")
		return
	}
	id := chi.URLParam(r, "id")
	stats, err := g.ragStore.Stats(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (g *Gateway) handleDeleteChunk(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "Individual chunk deletion not yet implemented")
}
