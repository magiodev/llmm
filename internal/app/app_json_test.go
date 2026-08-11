package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusJSON(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'active\n'`)
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "status", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v (output %q)", err, output.String())
	}
	var entries []map[string]any
	if err := json.Unmarshal(output.Bytes(), &entries); err != nil {
		t.Fatalf("unmarshal = %v (output %q)", err, output.String())
	}
	if len(entries) != 1 || entries[0]["name"] != "example" || entries[0]["type"] != "systemd" || entries[0]["state"] != "active" {
		t.Fatalf("entries = %#v", entries)
	}
}
func TestStatusJSONError(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'boom\n' >&2; exit 3`)
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "status", "--format", "json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error")
	}
	var entries []map[string]any
	if err := json.Unmarshal(output.Bytes(), &entries); err != nil {
		t.Fatalf("unmarshal = %v (output %q)", err, output.String())
	}
	if len(entries) != 1 || entries[0]["state"] != "error" {
		t.Fatalf("entries = %#v", entries)
	}
}
func TestModelsJSON(t *testing.T) {
	path := writeManifest(t, "version: 1\ndefault_model: flash\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels:\n  flash:\n    runtime: example\n    path: /flash.gguf\n    context: 4096\n    output: 2048\n    artifacts:\n      - path: /tok.json\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "models", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v (output %q)", err, output.String())
	}
	var infos []map[string]any
	if err := json.Unmarshal(output.Bytes(), &infos); err != nil {
		t.Fatalf("unmarshal = %v (output %q)", err, output.String())
	}
	if len(infos) != 1 || infos[0]["name"] != "flash" || infos[0]["runtime"] != "example" || infos[0]["path"] != "/flash.gguf" || infos[0]["default"] != true || infos[0]["context"] != float64(4096) || infos[0]["output"] != float64(2048) {
		t.Fatalf("infos = %#v", infos)
	}
	artifacts, ok := infos[0]["artifacts"].([]any)
	if !ok || len(artifacts) != 1 {
		t.Fatalf("artifacts = %#v", infos[0]["artifacts"])
	}
}
func TestDoctorJSON(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'loaded\n'`)
	dir := t.TempDir()
	model := filepath.Join(dir, "m.gguf")
	if err := os.WriteFile(model, []byte("model"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels:\n  m:\n    runtime: example\n    path: %s\n", model))
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "doctor", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v (output %q)", err, output.String())
	}
	var result map[string]any
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal = %v (output %q)", err, output.String())
	}
	if result["success"] != true {
		t.Fatalf("success = %#v", result["success"])
	}
	checks, ok := result["checks"].([]any)
	if !ok || len(checks) == 0 {
		t.Fatalf("checks = %#v", result["checks"])
	}
}
func TestDoctorJSONFailure(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'loaded\n'`)
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels:\n  m:\n    runtime: example\n    path: /nonexistent.gguf\n")
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "doctor", "--format", "json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error")
	}
	var result map[string]any
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal = %v (output %q)", err, output.String())
	}
	if result["success"] != false {
		t.Fatalf("success = %#v", result["success"])
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("boom") }
func TestJSONWriteError(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'active\n'`)
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetOut(errWriter{})
	cmd.SetArgs([]string{"--config", path, "status", "--format", "json"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}
func TestModelsJSONWriteError(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels:\n  m:\n    runtime: example\n    path: /m.gguf\n")
	cmd := New("test")
	cmd.SetOut(errWriter{})
	cmd.SetArgs([]string{"--config", path, "models", "--format", "json"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}
func TestDoctorJSONWriteError(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'loaded\n'`)
	dir := t.TempDir()
	model := filepath.Join(dir, "m.gguf")
	if err := os.WriteFile(model, []byte("model"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels:\n  m:\n    runtime: example\n    path: %s\n", model))
	cmd := New("test")
	cmd.SetOut(errWriter{})
	cmd.SetArgs([]string{"--config", path, "doctor", "--format", "json"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}
