package rag

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"sync"
)

// Embedder converts text to embedding vectors.
type Embedder interface {
	Embed(text string) ([]float64, error)
}

// VectorStore is the interface for storing and searching document chunks.
type VectorStore interface {
	Add(ctx context.Context, chunks []Chunk) error
	Search(ctx context.Context, queryVector []float64, topK int, kbID string) ([]SearchResult, error)
	DeleteByKB(ctx context.Context, kbID string) error
	Stats(ctx context.Context, kbID string) (map[string]interface{}, error)
}

// MemoryStore is an in-memory VectorStore implementation.
// It stores chunks in a slice and performs brute-force cosine similarity search.
type MemoryStore struct {
	mu     sync.RWMutex
	chunks []Chunk // all chunks across all KBs
}

// NewMemoryStore creates a new in-memory vector store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		chunks: make([]Chunk, 0),
	}
}

// Add stores chunks in the memory store.
func (s *MemoryStore) Add(_ context.Context, chunks []Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chunks = append(s.chunks, chunks...)
	return nil
}

// Search performs brute-force cosine similarity search.
// Returns topK results above zero score for the given KB.
func (s *MemoryStore) Search(_ context.Context, queryVector []float64, topK int, kbID string) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []SearchResult
	for _, c := range s.chunks {
		if kbID != "" && c.KBID != kbID {
			continue
		}
		if len(c.Embedding) == 0 {
			continue
		}
		score := cosineSimilarity(queryVector, c.Embedding)
		if score > 0 {
			results = append(results, SearchResult{Chunk: c, Score: score})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// DeleteByKB removes all chunks for a given knowledge base.
func (s *MemoryStore) DeleteByKB(_ context.Context, kbID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := make([]Chunk, 0, len(s.chunks))
	for _, c := range s.chunks {
		if c.KBID != kbID {
			filtered = append(filtered, c)
		}
	}
	s.chunks = filtered
	return nil
}

// Stats returns statistics for a knowledge base.
func (s *MemoryStore) Stats(_ context.Context, kbID string) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, c := range s.chunks {
		if c.KBID == kbID {
			count++
		}
	}
	return map[string]interface{}{
		"kb_id":      kbID,
		"chunk_count": count,
		"store_type": "memory",
	}, nil
}

// ChunkID generates a deterministic chunk ID from document ID and index.
func ChunkID(docID string, index int) string {
	h := md5.Sum([]byte(fmt.Sprintf("%s:%d", docID, index)))
	return hex.EncodeToString(h[:8])
}

// cosineSimilarity computes the cosine similarity between two vectors.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	normA = math.Sqrt(normA)
	normB = math.Sqrt(normB)
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (normA * normB)
}
