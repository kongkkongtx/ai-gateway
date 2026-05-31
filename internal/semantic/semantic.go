package semantic

import (
	"math"
)

// SemanticRouterConfig defines semantic routing rules.
// Each category maps a set of example queries to a target upstream.
type SemanticRouterConfig struct {
	Enabled    bool       `yaml:"enabled"`
	Provider   string     `yaml:"provider"` // Upstream name to use for embeddings
	Threshold  float64    `yaml:"threshold"` // Minimum similarity (0.0-1.0)
	Categories []Category `yaml:"categories"`
}

type Category struct {
	Name     string   `yaml:"name"`
	Examples []string `yaml:"examples"`
	Target   string   `yaml:"target"`   // Target upstream or model pattern
	Priority int      `yaml:"priority"`
}

// SemanticCacheConfig defines semantic cache settings.
type SemanticCacheConfig struct {
	Enabled    bool    `yaml:"enabled"`
	Threshold  float64 `yaml:"threshold"` // Similarity threshold (0.0-1.0)
	TTL        string  `yaml:"ttl"`      // Cache TTL (e.g. "10m", "1h")
	MaxEntries int     `yaml:"max_entries"`
}

// CosineSimilarity computes the cosine similarity between two vectors.
func CosineSimilarity(a, b []float64) float64 {
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

// Vector represents an embedding vector with optional metadata.
type Vector struct {
	Values []float64
	Text   string
	Label  string
}
