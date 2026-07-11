package service

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// ServiceManager handles systemd user service management
type ServiceManager struct {
	userUnitDir string
	repoUnitDir string
}

// NewServiceManager creates a new ServiceManager
func NewServiceManager(userUnitDir, repoUnitDir string) *ServiceManager {
	return &ServiceManager{
		userUnitDir: userUnitDir,
		repoUnitDir: repoUnitDir,
	}
}

// Sync copies service files from repo to user unit dir
func (s *ServiceManager) Sync() error {
	cmd := exec.Command("mkdir", "-p", s.userUnitDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create user unit dir: %w", err)
	}

	// Copy service files
	copyCmd := exec.Command("install", "-m", "644",
		filepath.Join(s.repoUnitDir, "*.service"),
		s.userUnitDir)
	if err := copyCmd.Run(); err != nil {
		return fmt.Errorf("failed to sync service files: %w", err)
	}

	// Reload systemd
	reloadCmd := exec.Command("systemctl", "--user", "daemon-reload")
	if err := reloadCmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	return nil
}

// Start starts a service
func (s *ServiceManager) Start(serviceName string) error {
	return s.runCommand("systemctl", "--user", "start", serviceName)
}

// Stop stops a service
func (s *ServiceManager) Stop(serviceName string) error {
	return s.runCommand("systemctl", "--user", "stop", serviceName)
}

// Restart restarts a service
func (s *ServiceManager) Restart(serviceName string) error {
	return s.runCommand("systemctl", "--user", "restart", serviceName)
}

// Status returns the status of a service
func (s *ServiceManager) Status(serviceName string) string {
	out, err := exec.Command("systemctl", "--user", "is-active", serviceName).Output()
	if err != nil {
		return "inactive"
	}
	return string(out)
}

// GetServiceName returns the systemd service name for a backend
func (s *ServiceManager) GetServiceName(backend string) string {
	serviceNames := map[string]string{
		"turboquant": "llama-turboquant-server",
		"ds4":        "ds4-server",
		"vllm":       "vllm-server",
		"comfyui":    "comfyui",
		"webui":      "open-webui",
	}
	if name, ok := serviceNames[backend]; ok {
		return name
	}
	return backend
}

func (s *ServiceManager) runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %w\nOutput: %s", name, args, err, string(output))
	}
	return nil
}
