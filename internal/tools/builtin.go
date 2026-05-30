package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Workspace bounds filesystem tools to a root directory unless AllowOutside is
// set. It is shared by the read/write/list tools.
type Workspace struct {
	Root         string
	AllowOutside bool
	// Shell is the shell used by the run_command tool (passed to shellrun).
	Shell string
	// MaxOutput caps the byte length of any single tool result fed back to the
	// model; 0 means use defaultMaxOutput.
	MaxOutput int
}

const defaultMaxOutput = 16 * 1024

// resolve turns a (possibly relative) path into an absolute one and enforces
// the workspace boundary.
func (w *Workspace) resolve(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(w.Root, path)
	}
	abs = filepath.Clean(abs)
	if !w.AllowOutside {
		rootAbs := filepath.Clean(w.Root)
		rel, err := filepath.Rel(rootAbs, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("path %q is outside the workspace (use --allow-outside to permit)", path)
		}
	}
	return abs, nil
}

func (w *Workspace) maxOutput() int {
	if w.MaxOutput > 0 {
		return w.MaxOutput
	}
	return defaultMaxOutput
}

// truncate caps s to the workspace's max output length, appending a marker.
func (w *Workspace) truncate(s string) string {
	max := w.maxOutput()
	if len(s) <= max {
		return s
	}
	return s[:max] + fmt.Sprintf("\n... [truncated %d bytes]", len(s)-max)
}

// DefaultRegistry builds a registry of the built-in tools bound to the given
// workspace. fetch is used by the web_fetch tool (injectable for tests).
func DefaultRegistry(ws *Workspace, fetch FetchFunc) *Registry {
	r := NewRegistry()
	r.Register(&readFileTool{ws: ws})
	r.Register(&listDirTool{ws: ws})
	r.Register(&writeFileTool{ws: ws})
	r.Register(&editFileTool{ws: ws})
	r.Register(&runCommandTool{ws: ws})
	r.Register(&webFetchTool{ws: ws, fetch: fetch})
	return r
}
