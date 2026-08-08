package config

import (
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
	cfg := validConfig()
	cfg.Models["flash"] = Model{Runtime: "missing", Path: "/model"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "unknown runtime") {
		t.Fatalf("unexpected error: %v", err)
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
