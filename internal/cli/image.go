package cli

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adammpkins/llama-terminal-completion/internal/client"
)

// imagePaths is the value of ask's --image flag (repeatable; paths or URLs).
var imagePaths []string

// imageMIME maps a file extension to its image MIME type.
var imageMIME = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
}

// buildImageParts converts the given image references into ContentPart values.
// http(s) references pass through as URLs; local files are read and encoded as
// data URIs.
func buildImageParts(refs []string) ([]client.ContentPart, error) {
	parts := make([]client.ContentPart, 0, len(refs))
	for _, ref := range refs {
		if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
			parts = append(parts, client.ContentPart{
				Type:     "image_url",
				ImageURL: &client.ImageURL{URL: ref},
			})
			continue
		}
		data, err := os.ReadFile(ref)
		if err != nil {
			return nil, fmt.Errorf("read image %s: %w", ref, err)
		}
		mime := imageMIME[strings.ToLower(filepath.Ext(ref))]
		if mime == "" {
			return nil, fmt.Errorf("unsupported image type for %s (png, jpg, gif, webp)", ref)
		}
		uri := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
		parts = append(parts, client.ContentPart{
			Type:     "image_url",
			ImageURL: &client.ImageURL{URL: uri},
		})
	}
	return parts, nil
}
