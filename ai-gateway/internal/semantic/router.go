package semantic

import (
	"fmt"
	"log/slog"
	"math"
	"sort"
	"sync"

	"github.com/yushi/ai-gateway/internal/provider/openai"
)

// Router performs semantic routing based on embedding similarity.
// It pre-computes category embeddings at startup and matches incoming
// queries against them using cosine similarity.
type Router struct {
	cfg       SemanticRouterConfig
	categories []CategoryVector
	embedder  Embedder
	mu        sync.RWMutex
	logger    *slog.Logger
}

// CategoryVector holds a category with its pre-computed embedding.
type CategoryVector struct {
	Category
	Embedding []float64
}

// Embedder computes text embeddings using a provider's embedding API.
type Embedder interface {
	Embed(text string) (*openai.EmbeddingResponse, error)
}

// NewRouter creates a semantic router. Pass nil for embedder to disable.
func NewRouter(cfg SemanticRouterConfig, embedder Embedder, logger *slog.Logger) *Router {
	return &Router{
		cfg:      cfg,
		embedder: embedder,
		logger:   logger,
	}
}

// Initialize pre-computes embeddings for all category examples.
func (r *Router) Initialize() error {
	if !r.cfg.Enabled || r.embedder == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.categories = nil
	for _, cat := range r.cfg.Categories {
		var allEmbeds [][]float64
		for _, example := range cat.Examples {
			resp, err := r.embedder.Embed(example)
			if err != nil {
				r.logger.Warn("failed to compute embedding for category example",
					"category", cat.Name, "error", err)
				continue
			}
			if len(resp.Data) > 0 {
				allEmbeds = append(allEmbeds, resp.Data[0].Embedding)
			}
		}
		if len(allEmbeds) == 0 {
			r.logger.Warn("no embeddings computed for category", "category", cat.Name)
			continue
		}
		// Average the embeddings of all examples for this category
		avgEmbed := averageVectors(allEmbeds)
		r.categories = append(r.categories, CategoryVector{
			Category:  cat,
			Embedding: avgEmbed,
		})
		r.logger.Info("semantic category initialized",
			"category", cat.Name,
			"examples", len(cat.Examples),
			"target", cat.Target)
	}

	// Sort by priority (higher first)
	sort.Slice(r.categories, func(i, j int) bool {
		return r.categories[i].Priority > r.categories[j].Priority
	})

	r.logger.Info("semantic router initialized",
		"categories", len(r.categories))
	return nil
}

// Match finds the best-matching category for the given query text.
// Returns the matched category and similarity score, or nil if below threshold.
func (r *Router) Match(query string) (*CategoryVector, float64, error) {
	if !r.cfg.Enabled || r.embedder == nil || len(r.categories) == 0 {
		return nil, 0, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	resp, err := r.embedder.Embed(query)
	if err != nil {
		return nil, 0, fmt.Errorf("compute query embedding: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, 0, fmt.Errorf("no embedding returned for query")
	}

	queryEmbed := resp.Data[0].Embedding
	var best *CategoryVector
	bestScore := r.cfg.Threshold

	for i := range r.categories {
		score := CosineSimilarity(queryEmbed, r.categories[i].Embedding)
		if score > bestScore {
			bestScore = score
			best = &r.categories[i]
		}
	}

	if best != nil {
		r.logger.Debug("semantic route matched",
			"category", best.Name,
			"target", best.Target,
			"score", math.Round(bestScore*1000)/1000)
	}

	return best, bestScore, nil
}

// Reload updates the router configuration and re-computes embeddings.
func (r *Router) Reload(cfg SemanticRouterConfig) {
	r.mu.Lock()
	r.cfg = cfg
	r.mu.Unlock()
	// Re-initialize will be called externally
}

func averageVectors(vectors [][]float64) []float64 {
	if len(vectors) == 0 {
		return nil
	}
	n := len(vectors)
	dim := len(vectors[0])
	result := make([]float64, dim)
	for _, v := range vectors {
		for i := range v {
			result[i] += v[i]
		}
	}
	for i := range result {
		result[i] /= float64(n)
	}
	return result
}
