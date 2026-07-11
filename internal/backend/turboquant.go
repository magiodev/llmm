package backend

import (
	"fmt"
	"os/exec"
	"strings"
)

// TurboQuant implements the Backend interface for llama-cpp-turboquant
type TurboQuant struct {
	serviceName string
	port        int
	models      map[string]string // key -> model path
}

func NewTurboQuant() *TurboQuant {
	return &TurboQuant{
		serviceName: "llama-turboquant-server",
		port:        8003,
		models: map[string]string{
			"35b":  "/home/magiodev/LLM/models/gguf/unsloth/Qwen3.6-35B-A3B-GGUF/Qwen3.6-35B-A3B-UD-Q5_K_XL.gguf",
			"122b": "/home/magiodev/LLM/models/gguf/mradermacher/Qwen3.5-122B-A10B-GGUF/Qwen3.5-122B-A10B.Q4_K_M.gguf",
			"qwopus": "/home/magiodev/LLM/models/gguf/Jackrong/Qwopus3.6-27B-v2-MTP-GGUF/Qwopus3.6-27B-v2-MTP-Q6_K.gguf",
			"nex": "/home/magiodev/LLM/models/gguf/bartowski/nex-agi_Nex-N2-mini-GGUF/nex-agi_Nex-N2-mini-Q4_K_M.gguf",
			"27b": "/home/magiodev/LLM/models/gguf/unsloth/Qwen3.6-27B-GGUF/Qwen3.6-27B-UD-Q5_K_XL.gguf",
			"gemma": "/home/magiodev/LLM/models/gguf/yuxinlu1/gemma-4-12B-coder-fable5-composer2.5-v1/gemma4-coding-Q8_0.gguf",
		},
	}
}

func (t *TurboQuant) Name() string {
	return "turboquant"
}

func (t *TurboQuant) ModelKeys() []string {
	keys := make([]string, 0, len(t.models))
	for k := range t.models {
		keys = append(keys, k)
	}
	return keys
}

func (t *TurboQuant) ServiceName() string {
	return t.serviceName
}

func (t *TurboQuant) Port() int {
	return t.port
}

func (t *TurboQuant) Start() error {
	return runCommand("systemctl", "--user", "start", t.serviceName)
}

func (t *TurboQuant) Stop() error {
	return runCommand("systemctl", "--user", "stop", t.serviceName)
}

func (t *TurboQuant) Restart() error {
	return runCommand("systemctl", "--user", "restart", t.serviceName)
}

func (t *TurboQuant) Status() string {
	out, err := exec.Command("systemctl", "--user", "is-active", t.serviceName).Output()
	if err != nil {
		return "inactive"
	}
	return strings.TrimSpace(string(out))
}

func (t *TurboQuant) SwitchModel(key string) error {
	path, ok := t.models[key]
	if !ok {
		return fmt.Errorf("unknown model: %s", key)
	}

	// Map key to alias
	aliasMap := map[string]string{
		"35b":    "qwen3.6-35b-a3b-moe",
		"122b":   "qwen3.5-122b-a10b",
		"qwopus": "qwopus-27b",
		"nex":    "nex-n2-mini",
		"27b":    "qwen3.6-27b-dense",
		"gemma":  "gemma4-12b-coder",
	}

	alias, ok := aliasMap[key]
	if !ok {
		return fmt.Errorf("unknown alias for model: %s", key)
	}

	// Set environment variables and restart service
	cmd := exec.Command("systemctl", "--user", "set-environment",
		fmt.Sprintf("LLAMA_MODEL=%s", path),
		fmt.Sprintf("LLAMA_ALIAS=%s", alias))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set environment: %w", err)
	}

	return runCommand("systemctl", "--user", "restart", t.serviceName)
}
