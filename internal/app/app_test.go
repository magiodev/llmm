package app

import (
	"bytes"
	"context"
	"errors"
	"hash"
	"os"
	"path/filepath"
	"strings"
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
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "status", "missing"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown runtime") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestConfigShowJSON(t *testing.T) {
	path := writeManifest(t, "version: 1\nnode: dgx\nruntimes:\n  example:\n    type: systemd\n    service: example.service\n")
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
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels:\n  z:\n    runtime: example\n    path: /z\n  a:\n    runtime: example\n    path: /a\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "models"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if output.String() != "a\texample\t/a\nz\texample\t/z\n" {
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
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "config", "show", "--format", "toml"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("error = %v", err)
	}
}
func TestConfigInitRefusesOverwrite(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "config", "init"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v", err)
	}
}
func TestConfigInitForce(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "config", "init", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}
func TestQuietValidate(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
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
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\n  web:\n    type: systemd\n    service: web.service\nmodels: {}\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "status"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "example") || !strings.Contains(output.String(), "web") {
		t.Fatalf("output = %q", output.String())
	}
}
func TestStatusSingleRuntime(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'active\n'`)
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "status", "example"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "example") || !strings.Contains(output.String(), "active") {
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
func TestActionSystemdSuccess(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'active\n'`)
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "start", "example"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "example: active") {
		t.Fatalf("output = %q", output.String())
	}
}
func TestActionUnknownRuntime(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "start", "missing"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown runtime") {
		t.Fatalf("error = %v", err)
	}
}
func TestActionMissingConfig(t *testing.T) {
	cmd := New("test")
	cmd.SetArgs([]string{"--config", "/nonexistent.yaml", "start", "example"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected read error")
	}
}
func TestActionFails(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'permission denied\n' >&2; exit 7`)
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "start", "example"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("error = %v", err)
	}
}
func TestActionStatusFails(t *testing.T) {
	installFakeCommand(t, "systemctl", `case "$*" in *is-active*) printf 'boom\n' >&2; exit 3;; *) printf 'ok\n';; esac`)
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "start", "example"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}
func TestActionQuiet(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'active\n'`)
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--quiet", "--config", path, "start", "example"})
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
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
	t.Setenv("LLMM_CONFIG", path)
	cmd2 := New("test")
	start2 := findCommand(cmd2, "start")
	names, directive := start2.ValidArgsFunction(start2, nil, "")
	if len(names) != 1 || names[0] != "example" || directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("names = %v directive = %d", names, directive)
	}
}

type failWriter struct{}

func (failWriter) Write(p []byte) (int, error) { return 0, errors.New("boom") }
func TestConfigShowWriteError(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
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
