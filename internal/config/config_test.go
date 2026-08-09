package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validConfig() *Config {
	return &Config{Version: Version, Runtimes: map[string]Runtime{"ds4": {Type: "systemd", Service: "ds4.service"}}, Models: map[string]Model{"flash": {Runtime: "ds4", Format: "gguf", Path: "/models/flash.gguf"}}}
}

func TestValidate(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"unknown runtime", func(c *Config) { c.Models["flash"] = Model{Runtime: "missing", Path: "/model"} }, "unknown runtime"},
		{"negative size", func(c *Config) { m := c.Models["flash"]; m.Size = -1; c.Models["flash"] = m }, "size must not be negative"},
		{"negative context", func(c *Config) { m := c.Models["flash"]; m.Context = -1; c.Models["flash"] = m }, "context must not be negative"},
		{"negative output", func(c *Config) { m := c.Models["flash"]; m.Output = -1; c.Models["flash"] = m }, "output must not be negative"},
		{"invalid checksum", func(c *Config) { m := c.Models["flash"]; m.SHA256 = "nope"; c.Models["flash"] = m }, "sha256 must be 64 hexadecimal"},
		{"empty runtime name", func(c *Config) { c.Runtimes[""] = Runtime{Type: "docker", Container: "one"} }, "runtime name must not be empty"},
		{"empty model name", func(c *Config) { c.Models[""] = Model{Runtime: "ds4", Path: "/model"} }, "model name must not be empty"},
		{"systemd container", func(c *Config) {
			c.Runtimes["ds4"] = Runtime{Type: "systemd", Service: "ds4.service", Container: "wrong"}
		}, "cannot set container"},
		{"docker service", func(c *Config) { c.Runtimes["ds4"] = Runtime{Type: "docker", Container: "ds4", Service: "wrong"} }, "cannot set service"},
		{"leading dash", func(c *Config) { c.Runtimes["ds4"] = Runtime{Type: "systemd", Service: "--system"} }, "must not start"},
		{"invalid endpoint", func(c *Config) {
			c.Runtimes["ds4"] = Runtime{Type: "systemd", Service: "ds4.service", Endpoint: "not-a-url"}
		}, "absolute URL"},
		{"credential endpoint", func(c *Config) {
			c.Runtimes["ds4"] = Runtime{Type: "systemd", Service: "ds4.service", Endpoint: "https://user:secret@example.test/v1"}
		}, "must not contain credentials"},
		{"empty reasoning level", func(c *Config) {
			m := c.Models["flash"]
			m.Reasoning = []string{""}
			c.Models["flash"] = m
		}, "empty reasoning level"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			test.mutate(cfg)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestWriteLoadAndKnownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Write(path, validConfig(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, validConfig(), false); err == nil {
		t.Fatal("expected overwrite refusal")
	}
	if err := os.WriteFile(path, []byte("version: 1\nruntimes: {}\nmodels: {}\nunknown: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown field rejection")
	}
}

func TestLoadRejectsTrailingDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\n---\nunknown: true\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "multiple YAML documents") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarshalNormalizesOmittedModels(t *testing.T) {
	cfg := &Config{Version: Version, Runtimes: map[string]Runtime{"ds4": {Type: "systemd", Service: "ds4.service"}}}
	data, err := Marshal(cfg, "json")
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["models"].(map[string]any); !ok {
		t.Fatalf("models = %#v, want object", decoded["models"])
	}
}

func TestMarshalIncludesReasoning(t *testing.T) {
	cfg := &Config{Version: Version, Runtimes: map[string]Runtime{"ds4": {Type: "systemd", Service: "ds4.service"}}, Models: map[string]Model{"flash": {Runtime: "ds4", Path: "/models/flash.gguf", Reasoning: []string{"none", "high"}}}}
	data, err := Marshal(cfg, "json")
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	models := decoded["models"].(map[string]any)
	flash := models["flash"].(map[string]any)
	levels, ok := flash["reasoning"].([]any)
	if !ok || len(levels) != 2 || levels[0] != "none" || levels[1] != "high" {
		t.Fatalf("reasoning = %#v, want [none high]", flash["reasoning"])
	}
}

func TestForceWriteRestoresPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, validConfig(), true); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %o, want 600", got)
	}
}

func TestForceWriteRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(victim, []byte("untouched"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, path); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, validConfig(), true); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(victim)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "untouched" {
		t.Fatalf("victim = %q", data)
	}
}

func TestForceWritePreservesDestinationOnRenameFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, validConfig(), true); err == nil {
		t.Fatal("expected rename failure")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("destination directory was replaced")
	}
}

func TestMarshal(t *testing.T) {
	cfg := validConfig()
	cfg.Node = "dgx"
	for _, format := range []string{"yaml", "json"} {
		data, err := Marshal(cfg, format)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "dgx") {
			t.Fatalf("%s output = %q", format, data)
		}
	}
	if _, err := Marshal(cfg, "toml"); err == nil {
		t.Fatal("expected unsupported format error")
	}
}
