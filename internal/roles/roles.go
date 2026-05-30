// Package roles manages named personas (system-prompt presets) and prompt
// templates. Roles live as YAML files under ~/.config/lt/roles/, with a small
// set of built-ins compiled in and overridable by same-named user files.
package roles

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Role is a named system-prompt preset with optional model/temperature
// overrides and a default tool set for the agent.
type Role struct {
	Name        string   `yaml:"name"`
	System      string   `yaml:"system"`
	Model       string   `yaml:"model,omitempty"`
	Temperature *float64 `yaml:"temperature,omitempty"`
	Tools       []string `yaml:"tools,omitempty"`
}

// Injectable filesystem hooks for testing.
var (
	configDirFunc = os.UserConfigDir
	homeDirFunc   = os.UserHomeDir
	mkdirAllFunc  = os.MkdirAll
	writeFileFunc = os.WriteFile
	readFileFunc  = os.ReadFile
	readDirFunc   = os.ReadDir
)

// builtins are compiled-in roles, available even with no config files.
var builtins = map[string]Role{
	"shell": {
		Name:   "shell",
		System: "You are a command-line expert. Answer concisely with correct, safe shell commands and brief explanations only when needed.",
	},
	"code-reviewer": {
		Name:   "code-reviewer",
		System: "You are a meticulous senior software engineer reviewing code. Identify correctness bugs, security issues, and clear simplifications. Be specific and reference exact lines. Prioritize high-signal findings.",
	},
	"commit-message": {
		Name:   "commit-message",
		System: "You write clear, conventional git commit messages. Output a concise subject line (<=72 chars) in imperative mood, then an optional body explaining the why. Output only the commit message.",
	},
}

// RolesDir returns the directory where role YAML files are stored.
func RolesDir() string {
	if dir, err := configDirFunc(); err == nil {
		return filepath.Join(dir, "lt", "roles")
	}
	if home, err := homeDirFunc(); err == nil {
		return filepath.Join(home, ".lt", "roles")
	}
	return filepath.Join(".", ".lt", "roles")
}

// Load returns the role with the given name. A user file takes precedence over
// a built-in of the same name.
func Load(name string) (*Role, error) {
	path := filepath.Join(RolesDir(), name+".yaml")
	data, err := readFileFunc(path)
	if err == nil {
		var r Role
		if err := yaml.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("parse role %s: %w", name, err)
		}
		if r.Name == "" {
			r.Name = name
		}
		return &r, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read role %s: %w", name, err)
	}
	if b, ok := builtins[name]; ok {
		r := b
		return &r, nil
	}
	return nil, fmt.Errorf("role %q not found", name)
}

// List returns all available roles (built-ins plus user files), de-duplicated
// by name with user files winning, sorted by name.
func List() ([]Role, error) {
	byName := make(map[string]Role)
	maps.Copy(byName, builtins)

	entries, err := readDirFunc(RolesDir())
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".yaml")
			if r, err := Load(name); err == nil {
				byName[name] = *r
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read roles dir: %w", err)
	}

	out := make([]Role, 0, len(byName))
	for _, r := range byName {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Save writes a role to a user YAML file.
func Save(r *Role) error {
	if r.Name == "" {
		return fmt.Errorf("role name is required")
	}
	dir := RolesDir()
	if err := mkdirAllFunc(dir, 0o755); err != nil {
		return fmt.Errorf("create roles dir: %w", err)
	}
	data, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal role: %w", err)
	}
	path := filepath.Join(dir, r.Name+".yaml")
	if err := writeFileFunc(path, data, 0o644); err != nil {
		return fmt.Errorf("write role: %w", err)
	}
	return nil
}
