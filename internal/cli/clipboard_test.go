package cli

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestGetClipboardCommandDarwin(t *testing.T) {
	// Save and restore original
	origGetOS := getOS
	defer func() { getOS = origGetOS }()

	getOS = func() string { return "darwin" }

	cmd, err := getClipboardCommand()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cmd == nil {
		t.Fatal("Expected command, got nil")
	}
	if cmd.Path == "" && cmd.Args[0] != "pbcopy" {
		t.Errorf("Expected pbcopy command")
	}
}

func TestGetClipboardCommandLinuxXclip(t *testing.T) {
	origGetOS := getOS
	origLookPath := lookPath
	defer func() {
		getOS = origGetOS
		lookPath = origLookPath
	}()

	getOS = func() string { return "linux" }
	lookPath = func(file string) (string, error) {
		if file == "xclip" {
			return "/usr/bin/xclip", nil
		}
		return "", fmt.Errorf("not found")
	}

	cmd, err := getClipboardCommand()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cmd == nil {
		t.Fatal("Expected command, got nil")
	}
}

func TestGetClipboardCommandLinuxXsel(t *testing.T) {
	origGetOS := getOS
	origLookPath := lookPath
	defer func() {
		getOS = origGetOS
		lookPath = origLookPath
	}()

	getOS = func() string { return "linux" }
	lookPath = func(file string) (string, error) {
		if file == "xsel" {
			return "/usr/bin/xsel", nil
		}
		return "", fmt.Errorf("not found")
	}

	cmd, err := getClipboardCommand()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cmd == nil {
		t.Fatal("Expected command, got nil")
	}
}

func TestGetClipboardCommandLinuxNoTool(t *testing.T) {
	origGetOS := getOS
	origLookPath := lookPath
	defer func() {
		getOS = origGetOS
		lookPath = origLookPath
	}()

	getOS = func() string { return "linux" }
	lookPath = func(file string) (string, error) {
		return "", fmt.Errorf("not found")
	}

	_, err := getClipboardCommand()
	if err == nil {
		t.Fatal("Expected error when no clipboard tool found")
	}
	if err.Error() != "no clipboard tool found (install xclip or xsel)" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestGetClipboardCommandWindows(t *testing.T) {
	origGetOS := getOS
	defer func() { getOS = origGetOS }()

	getOS = func() string { return "windows" }

	cmd, err := getClipboardCommand()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cmd == nil {
		t.Fatal("Expected command, got nil")
	}
}

func TestGetClipboardCommandUnsupported(t *testing.T) {
	origGetOS := getOS
	defer func() { getOS = origGetOS }()

	getOS = func() string { return "freebsd" }

	_, err := getClipboardCommand()
	if err == nil {
		t.Fatal("Expected error for unsupported platform")
	}
	if err.Error() != "unsupported platform: freebsd" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestCopyToClipboardWithMock(t *testing.T) {
	origRunCmd := runClipboardCmd
	origGetOS := getOS
	defer func() {
		runClipboardCmd = origRunCmd
		getOS = origGetOS
	}()

	getOS = func() string { return "darwin" }
	runClipboardCmd = func(cmd *exec.Cmd) error {
		return nil // Success
	}

	err := copyToClipboard("test content")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestCopyToClipboardError(t *testing.T) {
	origRunCmd := runClipboardCmd
	origGetOS := getOS
	defer func() {
		runClipboardCmd = origRunCmd
		getOS = origGetOS
	}()

	getOS = func() string { return "darwin" }
	runClipboardCmd = func(cmd *exec.Cmd) error {
		return fmt.Errorf("clipboard failed")
	}

	err := copyToClipboard("test content")
	if err == nil {
		t.Fatal("Expected error")
	}
	if err.Error() != "clipboard failed" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestCopyToClipboardUnsupportedOS(t *testing.T) {
	origGetOS := getOS
	defer func() { getOS = origGetOS }()

	getOS = func() string { return "plan9" }

	err := copyToClipboard("test")
	if err == nil {
		t.Fatal("Expected error for unsupported platform")
	}
}
