package backend

import (
	"testing"
)

func TestGetBackend(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		checkFunc func(Backend) bool
	}{
		{
			name:  "turboquant",
			input: "turboquant",
			checkFunc: func(b Backend) bool {
				return b.Name() == "turboquant" && b.Port() == 8003
			},
		},
		{
			name:  "ds4",
			input: "ds4",
			checkFunc: func(b Backend) bool {
				return b.Name() == "ds4" && b.Port() == 8001
			},
		},
		{
			name:  "vllm",
			input: "vllm",
			checkFunc: func(b Backend) bool {
				return b.Name() == "vllm" && b.Port() == 9000
			},
		},
		{
			name:  "comfyui",
			input: "comfyui",
			checkFunc: func(b Backend) bool {
				return b.Name() == "comfyui" && b.Port() == 8188
			},
		},
		{
			name:    "unknown",
			input:   "unknown",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := GetBackend(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBackend(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkFunc != nil && !tt.checkFunc(b) {
				t.Errorf("GetBackend(%q) failed check", tt.input)
			}
		})
	}
}

func TestTurboQuantModelKeys(t *testing.T) {
	b := NewTurboQuant()
	keys := b.ModelKeys()

	expected := []string{"35b", "122b", "qwopus", "nex", "27b", "gemma"}
	if len(keys) != len(expected) {
		t.Errorf("Expected %d model keys, got %d", len(expected), len(keys))
	}

	for _, key := range expected {
		found := false
		for _, k := range keys {
			if k == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Model key %q not found", key)
		}
	}
}

func TestDS4ModelKeys(t *testing.T) {
	b := NewDS4()
	keys := b.ModelKeys()

	if len(keys) != 1 || keys[0] != "default" {
		t.Errorf("Expected 1 model key (default), got %v", keys)
	}
}

func TestVLLMModelKeys(t *testing.T) {
	b := NewVLLM()
	keys := b.ModelKeys()

	expected := []string{"diffusiongemma", "ornith"}
	if len(keys) != len(expected) {
		t.Errorf("Expected %d model keys, got %d", len(expected), len(keys))
	}
}

func TestComfyUIModelKeys(t *testing.T) {
	b := NewComfyUI()
	keys := b.ModelKeys()

	if len(keys) != 1 || keys[0] != "default" {
		t.Errorf("Expected 1 model key (default), got %v", keys)
	}
}
