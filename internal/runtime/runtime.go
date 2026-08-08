package runtime

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/magiodev/llmm/internal/config"
)

func Names(cfg *config.Config) []string {
	names := make([]string, 0, len(cfg.Runtimes))
	for name := range cfg.Runtimes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func Action(runtime config.Runtime, action string) error {
	var command string
	var args []string
	switch runtime.Type {
	case "systemd":
		command = "systemctl"
		args = []string{"--user", action, runtime.Service}
	case "docker":
		command = "docker"
		args = []string{action, runtime.Container}
	default:
		return fmt.Errorf("unsupported runtime type %q", runtime.Type)
	}
	output, err := exec.Command(command, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", command, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func Status(runtime config.Runtime) string {
	var command string
	var args []string
	switch runtime.Type {
	case "systemd":
		command = "systemctl"
		args = []string{"--user", "is-active", runtime.Service}
	case "docker":
		command = "docker"
		args = []string{"inspect", "--format", "{{.State.Status}}", runtime.Container}
	default:
		return "invalid"
	}
	output, err := exec.Command(command, args...).Output()
	if err != nil {
		return "inactive"
	}
	return strings.TrimSpace(string(output))
}
