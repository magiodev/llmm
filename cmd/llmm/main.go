package main

import (
	"fmt"
	"os"

	"github.com/magiodev/llmm/internal/app"
)

var version = "dev"

func main() {
	os.Exit(run(version))
}

// run executes the CLI and returns an exit code. It is separate from main so
// the failure path is unit-testable without os.Exit terminating the process.
func run(version string) int {
	if err := app.New(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
