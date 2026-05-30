// Package shellrun centralizes shell command execution, dangerous-command
// detection, and confirmation prompting. It is shared by the `cmd` CLI command
// and the agent's run_command tool so the danger heuristics live in one place
// and there is no import cycle between the cli and tools packages.
package shellrun

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// dangerousPatterns are regexes matched against a candidate command. Kept as a
// package var (compiled once) and intentionally conservative — false negatives
// are backstopped by the confirmation prompt.
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+-rf\s+/`),
	regexp.MustCompile(`rm\s+-rf\s+\*`),
	regexp.MustCompile(`>\s*/dev/sd`),
	regexp.MustCompile(`mkfs\.`),
	regexp.MustCompile(`dd\s+if=`),
	regexp.MustCompile(`:\(\)\{`),              // fork bomb
	regexp.MustCompile(`chmod\s+-R\s+777\s+/`), // recursive chmod on root
	regexp.MustCompile(`curl.*\|\s*bash`),      // pipe to bash
	regexp.MustCompile(`wget.*\|\s*bash`),
}

// IsDangerous reports whether a command matches a known dangerous pattern.
func IsDangerous(command string) bool {
	for _, p := range dangerousPatterns {
		if p.MatchString(command) {
			return true
		}
	}
	return false
}

// ResolveShell picks the shell to use: the configured value, else $SHELL,
// else /bin/sh.
func ResolveShell(configured string) string {
	if configured != "" {
		return configured
	}
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/sh"
}

// RunInteractive runs command through shell with the process's standard
// streams wired to os.Stdin/Stdout/Stderr — for foreground, interactive use.
func RunInteractive(command, shell string) error {
	cmd := exec.Command(ResolveShell(shell), "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("command exited with code %d", exitErr.ExitCode())
		}
		return fmt.Errorf("failed to run command: %w", err)
	}
	return nil
}

// RunCapture runs command through shell and returns its combined
// stdout+stderr — for programmatic/agent use where the output is fed back to
// the model. A non-zero exit is returned as an error with the captured output
// included so callers can surface it.
func RunCapture(ctx context.Context, command, shell string) (string, error) {
	cmd := exec.CommandContext(ctx, ResolveShell(shell), "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), fmt.Errorf("command exited with code %d", exitErr.ExitCode())
		}
		return string(out), fmt.Errorf("failed to run command: %w", err)
	}
	return string(out), nil
}

// Confirm writes prompt to out and reads a yes/no answer from in. It returns
// true only for "y"/"yes" (case-insensitive). A nil reader defaults to stdin.
func Confirm(prompt string, in io.Reader, out io.Writer) (bool, error) {
	if in == nil {
		in = os.Stdin
	}
	fmt.Fprint(out, prompt)
	reader := bufio.NewReader(in)
	response, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}
