// Package rag implements knowledge base integration (RAG) for the AI Gateway.
// It intercepts chat requests, retrieves relevant document chunks from a
// vector store, and injects them as context into the LLM prompt.
package rag

// Config defines the top-level RAG configuration.
type Config struct {
	Enabled          bool            `yaml:"enabled" json:"enabled"`
	TopK             int             `yaml:"top_k" json:"top_k"`                             // chunks to retrieve (default 5)
	ScoreThreshold   float64         `yaml:"score_threshold" json:"score_threshold"`         // minimum similarity (default 0.75)
	MaxContextTokens int             `yaml:"max_context_tokens" json:"max_context_tokens"`   // max tokens for context (default 2000)
	EmbeddingProvider string         `yaml:"embedding_provider" json:"embedding_provider"`   // upstream name for embedding
	EmbeddingModel   string          `yaml:"embedding_model" json:"embedding_model"`         // model name (default text-embedding-3-small)
	SystemPrompt     string          `yaml:"system_prompt" json:"system_prompt"`             // static system prompt
	RouteOverrides   []RouteOverride `yaml:"route_overrides" json:"route_overrides"`
	KnowledgeBases   []KnowledgeBase `yaml:"knowledge_bases" json:"knowledge_bases"`
}

// RouteOverride allows per-route RAG parameter overrides.
type RouteOverride struct {
	RouteMatch     string  `yaml:"route_match" json:"route_match"`         // glob pattern
	TopK           int     `yaml:"top_k" json:"top_k"`
	ScoreThreshold float64 `yaml:"score_threshold" json:"score_threshold"`
	Enabled        bool    `yaml:"enabled" json:"enabled"`
}

// KnowledgeBase defines a knowledge base configuration.
type KnowledgeBase struct {
	ID            string `yaml:"id" json:"id"`
	Name          string `yaml:"name" json:"name"`
	ChunkSize     int    `yaml:"chunk_size" json:"chunk_size"`         // default 800
	ChunkOverlap  int    `yaml:"chunk_overlap" json:"chunk_overlap"`   // default 100
}
