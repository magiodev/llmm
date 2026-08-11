package config

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestMarshalNormalizesOmittedModels(t *testing.T) {
	cfg := &Config{Version: Version, Runtimes: map[string]Runtime{"example": {Type: "systemd", Service: "example.service"}}}
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
func TestMarshalIncludesDefaultModel(t *testing.T) {
	cfg := &Config{Version: Version, DefaultModel: "flash", Runtimes: map[string]Runtime{"example": {Type: "systemd", Service: "example.service"}}, Models: map[string]Model{"flash": {Runtime: "example", Path: "/models/flash.gguf"}}}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := Marshal(cfg, "json")
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["default_model"] != "flash" {
		t.Fatalf("default_model = %#v, want flash", decoded["default_model"])
	}
}
func TestMarshalIncludesReasoning(t *testing.T) {
	cfg := &Config{Version: Version, Runtimes: map[string]Runtime{"example": {Type: "systemd", Service: "example.service"}}, Models: map[string]Model{"flash": {Runtime: "example", Path: "/models/flash.gguf", Reasoning: []string{"none", "high"}}}}
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
func TestMarshalNormalizesOmittedRuntimes(t *testing.T) {
	cfg := &Config{Version: Version}
	data, err := Marshal(cfg, "json")
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["runtimes"].(map[string]any); !ok {
		t.Fatalf("runtimes = %#v, want object", decoded["runtimes"])
	}
}
func TestMarshalYAMLError(t *testing.T) {
	old := marshalYAML
	marshalYAML = func(any) ([]byte, error) { return nil, errors.New("boom") }
	defer func() { marshalYAML = old }()
	if _, err := Marshal(validConfig(), "yaml"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}
