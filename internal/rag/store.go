package rag

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
)

// Record is a single embedded chunk in the store.
type Record struct {
	ID     string    `json:"id"`
	Source string    `json:"source"`
	Text   string    `json:"text"`
	Vector []float32 `json:"vector"`
}

// Store is an in-memory collection of records persisted to a JSON file.
type Store struct {
	Records []Record `json:"records"`
	path    string
}

// Injectable filesystem hooks for testing.
var (
	mkdirAllFunc  = os.MkdirAll
	writeFileFunc = os.WriteFile
	readFileFunc  = os.ReadFile
)

// Open loads the store at path, or returns an empty store if the file does not
// exist yet.
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := readFileFunc(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read store: %w", err)
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parse store: %w", err)
	}
	s.path = path
	return s, nil
}

// Add appends records to the store.
func (s *Store) Add(records ...Record) {
	s.Records = append(s.Records, records...)
}

// Save persists the store to its backing file.
func (s *Store) Save() error {
	if err := mkdirAllFunc(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create store dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal store: %w", err)
	}
	if err := writeFileFunc(s.path, data, 0o644); err != nil {
		return fmt.Errorf("write store: %w", err)
	}
	return nil
}

// Result is a scored search hit.
type Result struct {
	Record Record
	Score  float64
}

// Search returns the top-k records by cosine similarity to query.
func (s *Store) Search(query []float32, k int) []Result {
	results := make([]Result, 0, len(s.Records))
	for _, r := range s.Records {
		results = append(results, Result{Record: r, Score: Cosine(query, r.Vector)})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if k > 0 && k < len(results) {
		results = results[:k]
	}
	return results
}

// Cosine returns the cosine similarity of two vectors, or 0 if either is empty
// or zero-magnitude or the dimensions differ.
func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
