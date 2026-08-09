package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/spf13/cobra"
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

func findCommand(root *cobra.Command, prefix string) *cobra.Command {
	for _, c := range root.Commands() {
		if strings.HasPrefix(c.Use, prefix) {
			return c
		}
	}
	return nil
}

func TestConfigValidateRejects(t *testing.T) {
	path := writeManifest(t, "version: 99\nruntimes: {}\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "config", "validate"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestConfigShowMissing(t *testing.T) {
	cmd := New("test")
	cmd.SetArgs([]string{"--config", "/nonexistent.yaml", "config", "show"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected read error")
	}
}

func TestConfigShowBadFormat(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "config", "show", "--format", "toml"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("error = %v", err)
	}
}

func TestConfigInitRefusesOverwrite(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "config", "init"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v", err)
	}
}

func TestConfigInitForce(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "config", "init", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestQuietValidate(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels: {}\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--quiet", "--config", path, "config", "validate"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatalf("output = %q", output.String())
	}
}

func TestStatusMissingConfig(t *testing.T) {
	cmd := New("test")
	cmd.SetArgs([]string{"--config", "/nonexistent.yaml", "status"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected read error")
	}
}

func TestStatusSuccess(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'active\n'`)
	path := writeManifest(t, "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\n  web:\n    type: systemd\n    service: web.service\nmodels: {}\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "status"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "ds4") || !strings.Contains(output.String(), "web") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestStatusSingleRuntime(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'active\n'`)
	path := writeManifest(t, "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels: {}\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "status", "ds4"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "ds4") || !strings.Contains(output.String(), "active") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestModelsMissingConfig(t *testing.T) {
	cmd := New("test")
	cmd.SetArgs([]string{"--config", "/nonexistent.yaml", "models"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected read error")
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
	path := writeManifest(t, "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "doctor"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "service ds4") {
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
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\n    executable: %s\nmodels: {}\n", executable))
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

func TestActionSystemdSuccess(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'active\n'`)
	path := writeManifest(t, "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels: {}\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "start", "ds4"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "ds4: active") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestActionUnknownRuntime(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "start", "missing"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown runtime") {
		t.Fatalf("error = %v", err)
	}
}

func TestActionMissingConfig(t *testing.T) {
	cmd := New("test")
	cmd.SetArgs([]string{"--config", "/nonexistent.yaml", "start", "ds4"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected read error")
	}
}

func TestActionFails(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'permission denied\n' >&2; exit 7`)
	path := writeManifest(t, "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "start", "ds4"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("error = %v", err)
	}
}

func TestActionStatusFails(t *testing.T) {
	installFakeCommand(t, "systemctl", `case "$*" in *is-active*) printf 'boom\n' >&2; exit 3;; *) printf 'ok\n';; esac`)
	path := writeManifest(t, "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "start", "ds4"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}

func TestActionQuiet(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'active\n'`)
	path := writeManifest(t, "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels: {}\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--quiet", "--config", path, "start", "ds4"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatalf("output = %q", output.String())
	}
}

func TestActionCompletion(t *testing.T) {
	t.Setenv("LLMM_CONFIG", "/nonexistent.yaml")
	cmd := New("test")
	start := findCommand(cmd, "start")
	if start == nil {
		t.Fatal("no start command")
	}
	if _, directive := start.ValidArgsFunction(start, nil, ""); directive != cobra.ShellCompDirectiveError {
		t.Fatalf("directive = %d", directive)
	}
	path := writeManifest(t, "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels: {}\n")
	t.Setenv("LLMM_CONFIG", path)
	cmd2 := New("test")
	start2 := findCommand(cmd2, "start")
	names, directive := start2.ValidArgsFunction(start2, nil, "")
	if len(names) != 1 || names[0] != "ds4" || directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("names = %v directive = %d", names, directive)
	}
}

type failWriter struct{}

func (failWriter) Write(p []byte) (int, error) { return 0, errors.New("boom") }

func TestConfigShowWriteError(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetOut(failWriter{})
	cmd.SetArgs([]string{"--config", path, "config", "show"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}

func TestFileSHA256OpenError(t *testing.T) {
	if _, err := fileSHA256(context.Background(), "/nonexistent"); err == nil {
		t.Fatal("expected open error")
	}
}

type failHash struct{}

func (failHash) Write(p []byte) (int, error) { return 0, errors.New("boom") }
func (failHash) Sum(b []byte) []byte         { return nil }
func (failHash) Reset()                      {}
func (failHash) Size() int                   { return 0 }
func (failHash) BlockSize() int              { return 0 }

func TestFileSHA256HashWriteError(t *testing.T) {
	old := newHash
	newHash = func() hash.Hash { return failHash{} }
	defer func() { newHash = old }()
	path := filepath.Join(t.TempDir(), "m.gguf")
	if err := os.WriteFile(path, []byte("m"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := fileSHA256(context.Background(), path); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}

func TestFileSHA256ReadError(t *testing.T) {
	old := readFile
	readFile = func(*os.File, []byte) (int, error) { return 0, errors.New("boom") }
	defer func() { readFile = old }()
	path := filepath.Join(t.TempDir(), "m.gguf")
	if err := os.WriteFile(path, []byte("m"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := fileSHA256(context.Background(), path); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}
