package rag

import (
	"context"
	"math"
	"path/filepath"
	"testing"
)

func TestCosine(t *testing.T) {
	if got := Cosine([]float32{1, 0}, []float32{1, 0}); math.Abs(got-1) > 1e-9 {
		t.Errorf("identical vectors cosine = %v, want 1", got)
	}
	if got := Cosine([]float32{1, 0}, []float32{0, 1}); math.Abs(got) > 1e-9 {
		t.Errorf("orthogonal cosine = %v, want 0", got)
	}
	if got := Cosine([]float32{1, 2}, []float32{1}); got != 0 {
		t.Errorf("dimension mismatch should be 0, got %v", got)
	}
	if got := Cosine(nil, nil); got != 0 {
		t.Errorf("empty cosine = %v, want 0", got)
	}
}

func TestChunk(t *testing.T) {
	if c := Chunk("short text", 800, 100); len(c) != 1 || c[0] != "short text" {
		t.Errorf("short text should be one chunk, got %v", c)
	}
	long := ""
	for i := 0; i < 50; i++ {
		long += "This is sentence number. "
	}
	chunks := Chunk(long, 100, 20)
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if len([]rune(c)) > 120 { // size + a little slack for boundary search
			t.Errorf("chunk too large: %d runes", len([]rune(c)))
		}
	}
}

func TestStoreSaveSearch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "idx", "store.json")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	store.Add(
		Record{ID: "1", Source: "a", Text: "alpha", Vector: []float32{1, 0, 0}},
		Record{ID: "2", Source: "b", Text: "beta", Vector: []float32{0, 1, 0}},
		Record{ID: "3", Source: "c", Text: "gamma", Vector: []float32{0.9, 0.1, 0}},
	)
	if err := store.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Reopen and search for something close to the first vector.
	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if len(reopened.Records) != 3 {
		t.Fatalf("expected 3 records after reload, got %d", len(reopened.Records))
	}
	results := reopened.Search([]float32{1, 0, 0}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Record.ID != "1" {
		t.Errorf("nearest should be record 1, got %s", results[0].Record.ID)
	}
}

// fakeEmbedder returns a deterministic vector per input for testing Index.
type fakeEmbedder struct{ calls int }

func (f *fakeEmbedder) Embeddings(input []string, model string) ([][]float32, error) {
	f.calls++
	out := make([][]float32, len(input))
	for i, s := range input {
		out[i] = []float32{float32(len(s)), 1, 0}
	}
	return out, nil
}

func TestIndex(t *testing.T) {
	store, _ := Open(filepath.Join(t.TempDir(), "store.json"))
	emb := &fakeEmbedder{}
	sources := []Source{{Name: "doc1", Text: "hello world this is a document"}}
	n, err := Index(context.Background(), store, emb, "test-model", sources)
	if err != nil {
		t.Fatalf("index error: %v", err)
	}
	if n == 0 || n != len(store.Records) {
		t.Errorf("indexed %d but store has %d", n, len(store.Records))
	}
	for _, r := range store.Records {
		if len(r.Vector) != 3 {
			t.Errorf("record missing vector: %+v", r)
		}
	}
}
