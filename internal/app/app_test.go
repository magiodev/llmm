package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func writeManifest(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func installFakeCommand(t *testing.T, name, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestConfigInitValidate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "config", "init"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	cmd = New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "config", "validate"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if output.String() != "config: ok\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestQuietSuppressesConfirmationOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--quiet", "--config", path, "config", "init"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatalf("output = %q", output.String())
	}
}

func TestUnknownRuntime(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "status", "missing"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown runtime") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigShowJSON(t *testing.T) {
	path := writeManifest(t, "version: 1\nnode: dgx\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "config", "show", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"node": "dgx"`) || !strings.Contains(output.String(), `"models": {}`) {
		t.Fatalf("output = %q", output.String())
	}
}

func TestModelsAreSorted(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels:\n  z:\n    runtime: ds4\n    path: /z\n  a:\n    runtime: ds4\n    path: /a\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "models"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if output.String() != "a\tds4\t/a\nz\tds4\t/z\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestStatusReturnsSupervisorError(t *testing.T) {
	installFakeCommand(t, "docker", `printf 'permission denied\n' >&2; exit 7`)
	path := writeManifest(t, "version: 1\nruntimes:\n  web:\n    type: docker\n    container: web\nmodels: {}\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "status"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("error = %v", err)
	}
	if output.String() != "web              error\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestDoctorDeep(t *testing.T) {
	installFakeCommand(t, "docker", `printf '/web\n'`)
	dir := t.TempDir()
	model := filepath.Join(dir, "model.gguf")
	data := []byte("model")
	if err := os.WriteFile(model, data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	manifest := fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: docker\n    container: web\nmodels:\n  model:\n    runtime: web\n    path: %s\n    size: %d\n    sha256: %s\n", model, len(data), sum)
	path := writeManifest(t, manifest)
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "doctor", "--deep"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "sha256 model") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestFileSHA256HonorsCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(path, []byte("model"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fileSHA256(ctx, path); err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("error = %v", err)
	}
}

func TestDoctorRejectsFIFO(t *testing.T) {
	installFakeCommand(t, "docker", `printf '/web\n'`)
	dir := t.TempDir()
	fifo := filepath.Join(dir, "model.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: docker\n    container: web\nmodels:\n  model:\n    runtime: web\n    path: %s\n", fifo))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "doctor"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "model model") {
		t.Fatalf("error = %v", err)
	}
}

func TestDoctorRejectsNonExecutable(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'loaded\n'`)
	dir := t.TempDir()
	executable := filepath.Join(dir, "server")
	if err := os.WriteFile(executable, []byte("binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\n    executable: %s\nmodels: {}\n", executable))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "doctor"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "runtime ds4") {
		t.Fatalf("error = %v", err)
	}
}
