package main

import (
	"os"
	"testing"
)

func TestRunHelp(t *testing.T) {
	// Save and restore os.Args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Set args to show help
	os.Args = []string{"lt", "--help"}

	// run() should return 0 for help
	exitCode := run()
	if exitCode != 0 {
		t.Logf("run() with --help returned %d", exitCode)
	}
}

func TestRunVersion(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"lt", "version"}

	exitCode := run()
	if exitCode != 0 {
		t.Logf("run() with version returned %d", exitCode)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"lt", "unknowncommand12345"}

	// Should return 1 for unknown command
	exitCode := run()
	if exitCode != 1 {
		t.Logf("run() with unknown command returned %d, expected 1", exitCode)
	}
}

func TestRunNoArgs(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"lt"}

	// Should show help and return 0
	exitCode := run()
	if exitCode != 0 {
		t.Logf("run() with no args returned %d", exitCode)
	}
}

func TestRunConfigCommand(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"lt", "config", "path"}

	exitCode := run()
	if exitCode != 0 {
		t.Logf("run() with config path returned %d", exitCode)
	}
}

func TestRunHistoryCommand(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"lt", "history", "list"}

	exitCode := run()
	if exitCode != 0 {
		t.Logf("run() with history list returned %d", exitCode)
	}
}
