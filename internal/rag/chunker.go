package rag

import (
	"fmt"
	"strings"
)

// Chunker splits documents into chunks.
type Chunker interface {
	Chunk(doc Document) ([]Chunk, error)
}

// RecursiveChunker splits text recursively using separators from coarse to fine.
// It attempts to keep chunks within ChunkSize while maintaining natural boundaries.
type RecursiveChunker struct {
	ChunkSize  int      // target chunk size in characters
	Overlap    int      // character overlap between chunks
	Separators []string // ordered from coarse to fine, e.g. ["\n\n", "\n", ". ", " "]
}

// DefaultSeparators are the default recursive chunking separators.
var DefaultSeparators = []string{"\n\n", "\n", ". ", " ", ""}

// NewRecursiveChunker creates a RecursiveChunker with sensible defaults.
func NewRecursiveChunker(chunkSize, overlap int) *RecursiveChunker {
	if chunkSize <= 0 {
		chunkSize = 800
	}
	if overlap <= 0 {
		overlap = 100
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 4
	}
	return &RecursiveChunker{
		ChunkSize:  chunkSize,
		Overlap:    overlap,
		Separators: DefaultSeparators,
	}
}

// Chunk splits a document into chunks using recursive splitting.
func (rc *RecursiveChunker) Chunk(doc Document) ([]Chunk, error) {
	if doc.Content == "" {
		return nil, fmt.Errorf("document has no content")
	}

	texts := rc.splitText(doc.Content, rc.Separators)
	chunks := make([]Chunk, 0, len(texts))
	for i, text := range texts {
		chunks = append(chunks, Chunk{
			ID:       fmt.Sprintf("%s-chunk-%04d", doc.ID, i+1),
			Content:  text,
			Metadata: copyMetadata(doc.Metadata),
			KBID:     doc.KBID,
			Index:    i + 1,
		})
	}
	return chunks, nil
}

// splitText recursively splits text using the given separators.
func (rc *RecursiveChunker) splitText(text string, separators []string) []string {
	if len(text) <= rc.ChunkSize {
		return []string{strings.TrimSpace(text)}
	}

	if len(separators) == 0 {
		return rc.splitBySize(text)
	}

	sep := separators[0]
	remainingSeparators := separators[1:]

	// Try to split using the current separator
	segments := strings.Split(text, sep)

	// If there are enough segments, merge them into chunks
	if len(segments) > 1 {
		return rc.mergeSegments(segments, sep, remainingSeparators)
	}

	// Fall through to the next separator
	return rc.splitText(text, remainingSeparators)
}

// mergeSegments groups segments into chunks respecting ChunkSize.
func (rc *RecursiveChunker) mergeSegments(segments []string, sep string, remainingSeparators []string) []string {
	var result []string
	var current strings.Builder

	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}

		// If a single segment is too long, split it recursively
		if len(seg) > rc.ChunkSize {
			// Flush current buffer first
			if current.Len() > 0 {
				result = append(result, strings.TrimSpace(current.String()))
				current.Reset()
			}
			subChunks := rc.splitText(seg, remainingSeparators)
			result = append(result, subChunks...)
			continue
		}

		// Check if adding this segment would exceed the chunk size
		if current.Len() > 0 && current.Len()+len(sep)+len(seg) > rc.ChunkSize {
			result = append(result, strings.TrimSpace(current.String()))
			// Keep overlap: copy the last part of the current chunk
			overlapText := rc.getOverlap(current.String())
			current.Reset()
			if overlapText != "" {
				current.WriteString(overlapText)
				current.WriteString(sep)
			}
		}

		if current.Len() > 0 {
			current.WriteString(sep)
		}
		current.WriteString(seg)
	}

	// Flush remaining
	if current.Len() > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}

	return result
}

// splitBySize is a fallback that splits text by character count.
func (rc *RecursiveChunker) splitBySize(text string) []string {
	var result []string
	runes := []rune(text)
	start := 0
	for start < len(runes) {
		end := start + rc.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[start:end])
		result = append(result, strings.TrimSpace(chunk))
		start = end - rc.Overlap
		if start >= len(runes) || start >= end {
			break
		}
	}
	return result
}

// getOverlap returns the last Overlap characters from text.
func (rc *RecursiveChunker) getOverlap(text string) string {
	runes := []rune(text)
	if len(runes) <= rc.Overlap {
		return ""
	}
	return string(runes[len(runes)-rc.Overlap:])
}

// copyMetadata makes a shallow copy of a metadata map.
func copyMetadata(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
