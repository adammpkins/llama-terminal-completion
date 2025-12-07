package main

import (
	"os"

	"github.com/adammpkins/llamaterm/internal/cli"
)

// run is the main entry point, separated for testability
func run() int {
	if err := cli.Execute(); err != nil {
		return 1
	}
	return 0
}

func main() {
	os.Exit(run())
}
