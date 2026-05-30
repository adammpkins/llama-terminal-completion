package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildImageParts_URL(t *testing.T) {
	parts, err := buildImageParts([]string{"https://example.com/cat.png"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parts) != 1 || parts[0].Type != "image_url" || parts[0].ImageURL.URL != "https://example.com/cat.png" {
		t.Errorf("unexpected parts: %+v", parts)
	}
}

func TestBuildImageParts_LocalFileDataURI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pixel.png")
	if err := os.WriteFile(path, []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}
	parts, err := buildImageParts([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parts) != 1 || !strings.HasPrefix(parts[0].ImageURL.URL, "data:image/png;base64,") {
		t.Errorf("expected png data URI, got: %+v", parts)
	}
}

func TestBuildImageParts_UnsupportedType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(path, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := buildImageParts([]string{path}); err == nil {
		t.Error("expected error for unsupported image type")
	}
}
