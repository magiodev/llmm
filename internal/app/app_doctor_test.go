package app

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

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
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\n    executable: %s\nmodels: {}\n", executable))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "doctor"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "runtime example") {
		t.Fatalf("error = %v", err)
	}
}
func TestDoctorMissingConfig(t *testing.T) {
	cmd := New("test")
	cmd.SetArgs([]string{"--config", "/nonexistent.yaml", "doctor"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected read error")
	}
}
func TestDoctorSystemdServiceMissing(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'not-found\n'; exit 4`)
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "doctor"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "service example") {
		t.Fatalf("error = %v", err)
	}
}
func TestDoctorDockerError(t *testing.T) {
	installFakeCommand(t, "docker", `printf 'no such container\n' >&2; exit 1`)
	path := writeManifest(t, "version: 1\nruntimes:\n  web:\n    type: docker\n    container: web\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "doctor"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "docker web") {
		t.Fatalf("error = %v", err)
	}
}
func TestDoctorDockerErrorNoOutput(t *testing.T) {
	installFakeCommand(t, "docker", `exit 1`)
	path := writeManifest(t, "version: 1\nruntimes:\n  web:\n    type: docker\n    container: web\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "doctor"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "docker web") {
		t.Fatalf("error = %v", err)
	}
}
func TestDoctorExecutableOK(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'loaded\n'`)
	dir := t.TempDir()
	executable := filepath.Join(dir, "server")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\n    executable: %s\nmodels: {}\n", executable))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "doctor"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}
func TestDoctorModelNoSize(t *testing.T) {
	installFakeCommand(t, "docker", `printf '/web\n'`)
	dir := t.TempDir()
	model := filepath.Join(dir, "m.gguf")
	if err := os.WriteFile(model, []byte("m"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: docker\n    container: web\nmodels:\n  m:\n    runtime: web\n    path: %s\n", model))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "doctor"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}
func TestDoctorDeepMismatch(t *testing.T) {
	installFakeCommand(t, "docker", `printf '/web\n'`)
	dir := t.TempDir()
	model := filepath.Join(dir, "m.gguf")
	if err := os.WriteFile(model, []byte("model"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: docker\n    container: web\nmodels:\n  m:\n    runtime: web\n    path: %s\n    size: %d\n    sha256: %s\n", model, len("model"), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "doctor", "--deep"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("error = %v", err)
	}
}
func TestDoctorDeepOpenError(t *testing.T) {
	old := openFile
	openFile = func(string) (*os.File, error) { return nil, errors.New("boom") }
	defer func() { openFile = old }()
	installFakeCommand(t, "docker", `printf '/web\n'`)
	dir := t.TempDir()
	model := filepath.Join(dir, "m.gguf")
	if err := os.WriteFile(model, []byte("model"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: docker\n    container: web\nmodels:\n  m:\n    runtime: web\n    path: %s\n    size: %d\n    sha256: %s\n", model, len("model"), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "doctor", "--deep"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("error = %v", err)
	}
}
func TestDoctorArtifactOK(t *testing.T) {
	installFakeCommand(t, "docker", `printf '/web\n'`)
	dir := t.TempDir()
	primary := filepath.Join(dir, "primary.gguf")
	if err := os.WriteFile(primary, []byte("primary"), 0o600); err != nil {
		t.Fatal(err)
	}
	tokenizer := filepath.Join(dir, "tokenizer.json")
	if err := os.WriteFile(tokenizer, []byte("tok"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: docker\n    container: web\nmodels:\n  m:\n    runtime: web\n    path: %s\n    artifacts:\n      - path: %s\n", primary, tokenizer))
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "doctor"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v (output %q)", err, output.String())
	}
	if !strings.Contains(output.String(), "artifact m") {
		t.Fatalf("output = %q", output.String())
	}
}
func TestDoctorArtifactMissing(t *testing.T) {
	installFakeCommand(t, "docker", `printf '/web\n'`)
	path := writeManifest(t, "version: 1\nruntimes:\n  web:\n    type: docker\n    container: web\nmodels:\n  m:\n    runtime: web\n    path: /primary.gguf\n    artifacts:\n      - path: /nope.json\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "doctor"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "artifact m") {
		t.Fatalf("error = %v", err)
	}
}
func TestDoctorArtifactSizeMismatch(t *testing.T) {
	installFakeCommand(t, "docker", `printf '/web\n'`)
	dir := t.TempDir()
	artifact := filepath.Join(dir, "a.json")
	if err := os.WriteFile(artifact, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: docker\n    container: web\nmodels:\n  m:\n    runtime: web\n    path: /primary.gguf\n    artifacts:\n      - path: %s\n        size: 999\n", artifact))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "doctor", "--deep"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "artifact m") {
		t.Fatalf("error = %v", err)
	}
}
func TestDoctorArtifactDeepNoSHA(t *testing.T) {
	installFakeCommand(t, "docker", `printf '/web\n'`)
	dir := t.TempDir()
	primary := filepath.Join(dir, "primary.gguf")
	if err := os.WriteFile(primary, []byte("primary"), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(dir, "a.json")
	if err := os.WriteFile(artifact, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: docker\n    container: web\nmodels:\n  m:\n    runtime: web\n    path: %s\n    artifacts:\n      - path: %s\n        size: 3\n", primary, artifact))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "doctor", "--deep"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}
}
func TestDoctorArtifactDeepOK(t *testing.T) {
	installFakeCommand(t, "docker", `printf '/web\n'`)
	dir := t.TempDir()
	primary := filepath.Join(dir, "primary.gguf")
	if err := os.WriteFile(primary, []byte("primary"), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(dir, "a.json")
	data := []byte("abc")
	if err := os.WriteFile(artifact, data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: docker\n    container: web\nmodels:\n  m:\n    runtime: web\n    path: %s\n    artifacts:\n      - path: %s\n        size: %d\n        sha256: %s\n", primary, artifact, len(data), sum))
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "doctor", "--deep"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v (output %q)", err, output.String())
	}
	if !strings.Contains(output.String(), "sha256 m artifact 0") {
		t.Fatalf("output = %q", output.String())
	}
}
func TestDoctorArtifactDeepMismatch(t *testing.T) {
	installFakeCommand(t, "docker", `printf '/web\n'`)
	dir := t.TempDir()
	artifact := filepath.Join(dir, "a.json")
	if err := os.WriteFile(artifact, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: docker\n    container: web\nmodels:\n  m:\n    runtime: web\n    path: /primary.gguf\n    artifacts:\n      - path: %s\n        size: 3\n        sha256: %s\n", artifact, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "doctor", "--deep"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "sha256 m artifact 0") {
		t.Fatalf("error = %v", err)
	}
}
func TestDoctorArtifactDeepOpenError(t *testing.T) {
	old := openFile
	openFile = func(string) (*os.File, error) { return nil, errors.New("boom") }
	defer func() { openFile = old }()
	installFakeCommand(t, "docker", `printf '/web\n'`)
	dir := t.TempDir()
	artifact := filepath.Join(dir, "a.json")
	if err := os.WriteFile(artifact, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: docker\n    container: web\nmodels:\n  m:\n    runtime: web\n    path: /primary.gguf\n    artifacts:\n      - path: %s\n        size: 3\n        sha256: %s\n", artifact, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "doctor", "--deep"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "sha256 m artifact 0") {
		t.Fatalf("error = %v", err)
	}
}
