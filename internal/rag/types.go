package rag

// Document represents a source document to be chunked and indexed.
type Document struct {
	ID       string            `json:"id"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"` // source, title, page, etc.
	KBID     string            `json:"kb_id"`
}

// Chunk represents a single chunk of a document with its embedding.
type Chunk struct {
	ID        string            `json:"id"`
	Content   string            `json:"content"`
	Embedding []float64         `json:"embedding,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	KBID      string            `json:"kb_id"`
	Index     int               `json:"index"`
}

// SearchResult represents a single search result with similarity score.
type SearchResult struct {
	Chunk Chunk   `json:"chunk"`
	Score float64 `json:"score"`
}
