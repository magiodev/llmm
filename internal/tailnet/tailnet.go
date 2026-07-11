package tailnet

import (
	"fmt"
	"os/exec"
	"strings"
)

// TailnetManager handles Tailscale service exposure
type TailnetManager struct {
	ports []int
}

// NewTailnetManager creates a new TailnetManager
func NewTailnetManager(ports []int) *TailnetManager {
	return &TailnetManager{
		ports: ports,
	}
}

// Sync exposes all ports via Tailscale
func (t *TailnetManager) Sync() error {
	for _, port := range t.ports {
		cmd := exec.Command("tailscale", "serve", "--bg", "--yes",
			fmt.Sprintf("--tcp=%d", port),
			fmt.Sprintf("tcp://127.0.0.1:%d", port))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to expose port %d: %w", port, err)
		}
	}
	return nil
}

// Status returns the Tailscale serve status
func (t *TailnetManager) Status() (string, error) {
	cmd := exec.Command("tailscale", "serve", "status")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get tailscale status: %w", err)
	}
	return string(output), nil
}

// IsAvailable checks if tailscale is available
func (t *TailnetManager) IsAvailable() bool {
	cmd := exec.Command("tailscale", "status", "--json")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// GetStatusString returns a formatted status string
func (t *TailnetManager) GetStatusString() string {
	status, err := t.Status()
	if err != nil {
		return "Tailscale not available"
	}
	
	lines := strings.Split(strings.TrimSpace(status), "\n")
	var result strings.Builder
	result.WriteString("Tailscale Status:\n")
	for _, line := range lines {
		if line != "" && !strings.HasPrefix(line, "TCP:") {
			result.WriteString("  " + line + "\n")
		}
	}
	return result.String()
}
