// Package rag implements a pure-Go, CGO-free retrieval store: documents are
// chunked, embedded via the OpenAI-compatible embeddings endpoint, persisted as
// JSON, and searched with an in-memory cosine-similarity scan. This is sized
// for personal-scale corpora (hundreds to low thousands of chunks); for larger
// sets an approximate-nearest-neighbor index would be the next step.
package rag

import "strings"

// Default chunking parameters (in runes).
const (
	DefaultChunkSize    = 800
	DefaultChunkOverlap = 100
)

// Chunk splits text into overlapping windows of approximately size runes. It
// prefers to break on paragraph or sentence boundaries near the window edge to
// avoid splitting mid-word where possible.
func Chunk(text string, size, overlap int) []string {
	if size <= 0 {
		size = DefaultChunkSize
	}
	if overlap < 0 || overlap >= size {
		overlap = DefaultChunkOverlap
	}
	runes := []rune(text)
	if len(runes) <= size {
		t := strings.TrimSpace(text)
		if t == "" {
			return nil
		}
		return []string{t}
	}

	var chunks []string
	start := 0
	for start < len(runes) {
		end := min(start+size, len(runes))
		// Try to end on a natural boundary within the last 20% of the window.
		if end < len(runes) {
			if b := lastBoundary(runes, start+(size*4/5), end); b > start {
				end = b
			}
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
		start = max(end-overlap, 0)
	}
	return chunks
}

// lastBoundary returns the index just after the last sentence/paragraph break
// in runes[lo:hi], or -1 if none.
func lastBoundary(runes []rune, lo, hi int) int {
	for i := hi - 1; i > lo; i-- {
		switch runes[i] {
		case '\n', '.', '!', '?':
			return i + 1
		}
	}
	return -1
}
