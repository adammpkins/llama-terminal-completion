package main

import (
	"os"

	"github.com/adammpkins/llamaterm/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
