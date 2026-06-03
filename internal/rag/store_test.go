package rag

import (
	"context"
	"testing"
)

func TestRecursiveChunkerBasic(t *testing.T) {
	chunker := NewRecursiveChunker(50, 5)
	doc := Document{
		ID:      "doc-1",
		Content: "This is a long document that should be split into multiple chunks for testing purposes. Each chunk should contain roughly fifty characters of text.",
		KBID:    "kb-1",
	}
	chunks, err := chunker.Chunk(doc)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
	// Check all chunks have content
	for i, c := range chunks {
		if c.Content == "" {
			t.Fatalf("chunk %d has empty content", i)
		}
		if c.KBID != "kb-1" {
			t.Fatalf("chunk %d has wrong KBID", i)
		}
	}
}

func TestRecursiveChunkerEmptyDoc(t *testing.T) {
	chunker := NewRecursiveChunker(100, 10)
	_, err := chunker.Chunk(Document{ID: "empty", Content: "", KBID: "kb-1"})
	if err == nil {
		t.Fatal("expected error for empty document")
	}
}

func TestRecursiveChunkerSmallDoc(t *testing.T) {
	chunker := NewRecursiveChunker(1000, 10)
	doc := Document{ID: "small", Content: "Short text.", KBID: "kb-1"}
	chunks, err := chunker.Chunk(doc)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for small doc, got %d", len(chunks))
	}
}

func TestRecursiveChunkerNaturalBoundaries(t *testing.T) {
	chunker := NewRecursiveChunker(30, 5)
	doc := Document{
		ID: "para",
		Content: "First paragraph. This is some text.\n\nSecond paragraph. More text here.\n\nThird paragraph. Final bit of text.",
		KBID: "kb-1",
	}
	chunks, err := chunker.Chunk(doc)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
}

func TestMemoryStoreAddAndSearch(t *testing.T) {
	store := NewMemoryStore()
	chunks := []Chunk{
		{ID: "c1", Content: "The sky is blue on a clear day.", Embedding: []float64{1, 0, 0}, KBID: "kb-1"},
		{ID: "c2", Content: "The ocean is deep and blue.", Embedding: []float64{0.9, 0.1, 0}, KBID: "kb-1"},
		{ID: "c3", Content: "Python is a programming language.", Embedding: []float64{0, 0, 1}, KBID: "kb-1"},
	}
	if err := store.Add(context.Background(), chunks); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Search for "something about sky" (vector similar to c1 and c2)
	query := []float64{0.95, 0.05, 0}
	results, err := store.Search(context.Background(), query, 2, "kb-1")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Score < results[1].Score {
		t.Fatal("results should be sorted by score descending")
	}
}

func TestMemoryStoreSearchByKB(t *testing.T) {
	store := NewMemoryStore()
	store.Add(context.Background(), []Chunk{
		{ID: "c1", Content: "weather data", Embedding: []float64{1, 0}, KBID: "kb-weather"},
		{ID: "c2", Content: "code snippet", Embedding: []float64{0, 1}, KBID: "kb-code"},
	})

	// Search only kb-weather
	results, _ := store.Search(context.Background(), []float64{1, 0}, 10, "kb-weather")
	if len(results) != 1 {
		t.Fatalf("expected 1 result from kb-weather, got %d", len(results))
	}
	if results[0].Chunk.ID != "c1" {
		t.Fatalf("expected c1, got %s", results[0].Chunk.ID)
	}
}

func TestMemoryStoreDeleteByKB(t *testing.T) {
	store := NewMemoryStore()
	store.Add(context.Background(), []Chunk{
		{ID: "c1", Content: "a", Embedding: []float64{1, 0}, KBID: "kb-1"},
		{ID: "c2", Content: "b", Embedding: []float64{0, 1}, KBID: "kb-2"},
	})
	if err := store.DeleteByKB(context.Background(), "kb-1"); err != nil {
		t.Fatalf("DeleteByKB failed: %v", err)
	}
	stats, _ := store.Stats(context.Background(), "kb-1")
	if stats["chunk_count"].(int) != 0 {
		t.Fatalf("expected 0 chunks after deletion, got %d", stats["chunk_count"])
	}
}

func TestMemoryStoreStats(t *testing.T) {
	store := NewMemoryStore()
	store.Add(context.Background(), []Chunk{
		{ID: "c1", Content: "a", Embedding: []float64{1, 0}, KBID: "kb-1"},
		{ID: "c2", Content: "b", Embedding: []float64{1, 0}, KBID: "kb-1"},
	})
	stats, _ := store.Stats(context.Background(), "kb-1")
	if stats["chunk_count"] != 2 {
		t.Fatalf("expected 2 chunks, got %d", stats["chunk_count"])
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{1, 0, 0}
	if s := cosineSimilarity(a, b); s != 1.0 {
		t.Fatalf("expected 1.0 for identical vectors, got %f", s)
	}
	a = []float64{1, 0, 0}
	b = []float64{0, 1, 0}
	if s := cosineSimilarity(a, b); s != 0.0 {
		t.Fatalf("expected 0.0 for orthogonal vectors, got %f", s)
	}
	a = []float64{1, 0}
	b = []float64{1, 0, 0} // different lengths
	if s := cosineSimilarity(a, b); s != 0.0 {
		t.Fatalf("expected 0.0 for different lengths, got %f", s)
	}
}

func TestChunkID(t *testing.T) {
	id1 := ChunkID("doc-1", 1)
	id2 := ChunkID("doc-1", 2)
	id3 := ChunkID("doc-2", 1)
	if id1 == id2 {
		t.Fatal("different indices should produce different IDs")
	}
	if id1 == id3 {
		t.Fatal("different docs should produce different IDs")
	}
	if len(id1) != 16 {
		t.Fatalf("expected 16 char hex, got %d: %s", len(id1), id1)
	}
}
