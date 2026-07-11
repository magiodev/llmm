package backend

import (
	"fmt"
	"os/exec"
	"strings"
)

// VLLM implements the Backend interface for vLLM
type VLLM struct {
	serviceName string
	port        int
	models      map[string]string // key -> model path
}

func NewVLLM() *VLLM {
	return &VLLM{
		serviceName: "vllm-server",
		port:        9000,
		models: map[string]string{
			"diffusiongemma": "/home/magiodev/LLM/models/safetensors/google/diffusiongemma-26B-A4B-IT",
			"ornith":         "/home/magiodev/LLM/models/safetensors/ornith-1.0-35b",
		},
	}
}

func (v *VLLM) Name() string {
	return "vllm"
}

func (v *VLLM) ModelKeys() []string {
	keys := make([]string, 0, len(v.models))
	for k := range v.models {
		keys = append(keys, k)
	}
	return keys
}

func (v *VLLM) ServiceName() string {
	return v.serviceName
}

func (v *VLLM) Port() int {
	return v.port
}

func (v *VLLM) Start() error {
	return runCommand("systemctl", "--user", "start", v.serviceName)
}

func (v *VLLM) Stop() error {
	return runCommand("systemctl", "--user", "stop", v.serviceName)
}

func (v *VLLM) Restart() error {
	return runCommand("systemctl", "--user", "restart", v.serviceName)
}

func (v *VLLM) Status() string {
	out, err := exec.Command("systemctl", "--user", "is-active", v.serviceName).Output()
	if err != nil {
		return "inactive"
	}
	return strings.TrimSpace(string(out))
}

func (v *VLLM) SwitchModel(key string) error {
	path, ok := v.models[key]
	if !ok {
		return fmt.Errorf("unknown model: %s", key)
	}

	cmd := exec.Command("systemctl", "--user", "set-environment",
		fmt.Sprintf("VLLM_MODEL=%s", path))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set environment: %w", err)
	}

	return runCommand("systemctl", "--user", "restart", v.serviceName)
}
