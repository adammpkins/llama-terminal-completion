package roles

import (
	"os"
	"path/filepath"
	"testing"
)

// useTempDir points the roles/templates dirs at a temp directory for the test.
func useTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig := configDirFunc
	configDirFunc = func() (string, error) { return dir, nil }
	t.Cleanup(func() { configDirFunc = orig })
	return filepath.Join(dir, "lt", "roles")
}

func TestLoadBuiltin(t *testing.T) {
	useTempDir(t)
	r, err := Load("shell")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Name != "shell" || r.System == "" {
		t.Errorf("unexpected builtin role: %+v", r)
	}
}

func TestLoadUnknown(t *testing.T) {
	useTempDir(t)
	if _, err := Load("does-not-exist"); err == nil {
		t.Error("expected error for unknown role")
	}
}

func TestSaveAndLoadUserRole(t *testing.T) {
	useTempDir(t)
	temp := 0.3
	if err := Save(&Role{Name: "pirate", System: "Arr.", Model: "gpt-4o", Temperature: &temp}); err != nil {
		t.Fatalf("save error: %v", err)
	}
	r, err := Load("pirate")
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if r.System != "Arr." || r.Model != "gpt-4o" || r.Temperature == nil || *r.Temperature != 0.3 {
		t.Errorf("round-trip mismatch: %+v", r)
	}
}

func TestUserRoleOverridesBuiltin(t *testing.T) {
	useTempDir(t)
	if err := Save(&Role{Name: "shell", System: "custom shell prompt"}); err != nil {
		t.Fatalf("save error: %v", err)
	}
	r, err := Load("shell")
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if r.System != "custom shell prompt" {
		t.Errorf("user role did not override builtin: %q", r.System)
	}
}

func TestListMergesBuiltinsAndUser(t *testing.T) {
	useTempDir(t)
	if err := Save(&Role{Name: "zzz-custom", System: "x"}); err != nil {
		t.Fatalf("save error: %v", err)
	}
	rs, err := List()
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	names := map[string]bool{}
	for _, r := range rs {
		names[r.Name] = true
	}
	for _, want := range []string{"shell", "code-reviewer", "commit-message", "zzz-custom"} {
		if !names[want] {
			t.Errorf("List missing %q; got %v", want, names)
		}
	}
}

func TestRender(t *testing.T) {
	out, err := Render("Hello {{.input}} in {{.cwd}}", map[string]string{"input": "world"})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if out[:11] != "Hello world" {
		t.Errorf("unexpected render: %q", out)
	}
	// Missing key renders empty rather than erroring.
	out, err = Render("a={{.missing}}b", nil)
	if err != nil || out != "a=b" {
		t.Errorf("missing key handling: %q err=%v", out, err)
	}
}

func TestLoadTemplate(t *testing.T) {
	dir := t.TempDir()
	orig := configDirFunc
	configDirFunc = func() (string, error) { return dir, nil }
	defer func() { configDirFunc = orig }()

	tdir := filepath.Join(dir, "lt", "templates")
	if err := os.MkdirAll(tdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tdir, "greet.tmpl"), []byte("Hi {{.input}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadTemplate("greet")
	if err != nil {
		t.Fatalf("load template error: %v", err)
	}
	if s != "Hi {{.input}}" {
		t.Errorf("unexpected template content: %q", s)
	}
}
