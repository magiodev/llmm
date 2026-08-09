package main

import (
	"io"
	"os"
	"testing"
)

func TestRunSuccess(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"llmm"}
	if code := run("test"); code != 0 {
		t.Fatalf("code = %d", code)
	}
}

func TestRunError(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"llmm", "--config", "/nonexistent.yaml", "config", "validate"}
	if code := run("test"); code != 1 {
		t.Fatalf("code = %d", code)
	}
}

func TestRunErrorPrintsToStderr(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"llmm", "--config", "/nonexistent.yaml", "config", "validate"}
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	code := run("test")
	os.Stderr = old
	w.Close()
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("code = %d", code)
	}
	if len(output) == 0 {
		t.Fatal("expected stderr output")
	}
}
