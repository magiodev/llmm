package backend

import (
	"fmt"
	"os/exec"
	"strings"
)

// ComfyUI implements the Backend interface for ComfyUI
type ComfyUI struct {
	serviceName string
	port        int
}

func NewComfyUI() *ComfyUI {
	return &ComfyUI{
		serviceName: "comfyui",
		port:        8188,
	}
}

func (c *ComfyUI) Name() string {
	return "comfyui"
}

func (c *ComfyUI) ModelKeys() []string {
	return []string{"default"}
}

func (c *ComfyUI) ServiceName() string {
	return c.serviceName
}

func (c *ComfyUI) Port() int {
	return c.port
}

func (c *ComfyUI) Start() error {
	return runCommand("systemctl", "--user", "start", c.serviceName)
}

func (c *ComfyUI) Stop() error {
	return runCommand("systemctl", "--user", "stop", c.serviceName)
}

func (c *ComfyUI) Restart() error {
	return runCommand("systemctl", "--user", "restart", c.serviceName)
}

func (c *ComfyUI) Status() string {
	out, err := exec.Command("systemctl", "--user", "is-active", c.serviceName).Output()
	if err != nil {
		return "inactive"
	}
	return strings.TrimSpace(string(out))
}

func (c *ComfyUI) SwitchModel(key string) error {
	return fmt.Errorf("ComfyUI only supports one model set")
}
