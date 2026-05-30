package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adammpkins/llama-terminal-completion/internal/fetch"
)

// Embedder produces embedding vectors for a batch of texts (index-aligned).
// *client.Client satisfies this via its Embeddings method.
type Embedder interface {
	Embeddings(input []string, model string) ([][]float32, error)
}

// textExtensions are treated as indexable text files when walking a directory.
var textExtensions = map[string]bool{
	".txt": true, ".md": true, ".markdown": true, ".go": true, ".py": true,
	".js": true, ".ts": true, ".json": true, ".yaml": true, ".yml": true,
	".toml": true, ".rst": true, ".html": true, ".css": true, ".sh": true,
	".rb": true, ".rs": true, ".java": true, ".c": true, ".h": true,
	".cpp": true, ".sql": true, ".csv": true,
}

// Source is a resolved document: its display name and raw text.
type Source struct {
	Name string
	Text string
}

// CollectSources resolves a path or URL into one or more text documents. A
// directory is walked recursively for known text file types.
func CollectSources(ctx context.Context, pathOrURL string) ([]Source, error) {
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		text, err := fetch.Fetch(ctx, pathOrURL)
		if err != nil {
			return nil, err
		}
		return []Source{{Name: pathOrURL, Text: text}}, nil
	}

	info, err := os.Stat(pathOrURL)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		data, err := os.ReadFile(pathOrURL)
		if err != nil {
			return nil, err
		}
		return []Source{{Name: pathOrURL, Text: string(data)}}, nil
	}

	var sources []Source
	err = filepath.WalkDir(pathOrURL, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip common noise directories.
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !textExtensions[strings.ToLower(filepath.Ext(p))] {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil // skip unreadable files
		}
		sources = append(sources, Source{Name: p, Text: string(data)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sources, nil
}

// Index chunks the sources, embeds them with the embedder, and adds the
// resulting records to the store. It returns the number of chunks added.
func Index(ctx context.Context, store *Store, embedder Embedder, model string, sources []Source) (int, error) {
	var texts []string
	var meta []Record
	for _, src := range sources {
		chunks := Chunk(src.Text, DefaultChunkSize, DefaultChunkOverlap)
		for i, ch := range chunks {
			texts = append(texts, ch)
			meta = append(meta, Record{
				ID:     fmt.Sprintf("%s#%d", src.Name, i),
				Source: src.Name,
				Text:   ch,
			})
		}
	}
	if len(texts) == 0 {
		return 0, nil
	}

	// Embed in batches to keep request sizes reasonable.
	const batch = 64
	for start := 0; start < len(texts); start += batch {
		end := min(start+batch, len(texts))
		vecs, err := embedder.Embeddings(texts[start:end], model)
		if err != nil {
			return 0, fmt.Errorf("embedding batch: %w", err)
		}
		if len(vecs) != end-start {
			return 0, fmt.Errorf("embedding count mismatch: got %d, want %d", len(vecs), end-start)
		}
		for i, v := range vecs {
			meta[start+i].Vector = v
		}
	}

	store.Add(meta...)
	return len(meta), nil
}
