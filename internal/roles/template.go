package roles

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"text/template"
)

// TemplatesDir returns the directory where prompt templates are stored.
func TemplatesDir() string {
	if dir, err := configDirFunc(); err == nil {
		return filepath.Join(dir, "lt", "templates")
	}
	if home, err := homeDirFunc(); err == nil {
		return filepath.Join(home, ".lt", "templates")
	}
	return filepath.Join(".", ".lt", "templates")
}

// Render applies Go text/template substitution to s with the given variables.
// Built-in variables (input, stdin, cwd) are merged with caller-supplied vars,
// which take precedence. Missing keys render as empty rather than erroring.
func Render(s string, vars map[string]string) (string, error) {
	data := map[string]string{}
	if cwd, err := os.Getwd(); err == nil {
		data["cwd"] = cwd
	}
	maps.Copy(data, vars)

	tmpl, err := template.New("prompt").Option("missingkey=zero").Parse(s)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}
	return buf.String(), nil
}

// LoadTemplate reads a named template file from TemplatesDir.
func LoadTemplate(name string) (string, error) {
	path := filepath.Join(TemplatesDir(), name+".tmpl")
	data, err := readFileFunc(path)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", name, err)
	}
	return string(data), nil
}
