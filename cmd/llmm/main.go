package main

import (
	"fmt"
	"os"

	"github.com/magiodev/llmm/internal/app"
)

var version = "dev"

func main() {
	if err := app.New(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
