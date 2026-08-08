package runtime

import (
	"context"
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

func Action(ctx context.Context, runtime config.Runtime, action string) error {
	command, args, err := actionCommand(runtime, action)
	if err != nil {
		return err
	}
	output, err := exec.CommandContext(ctx, command, args...).CombinedOutput()
	if err != nil {
		return commandError(ctx, command, output, err)
	}
	return nil
}

func Status(ctx context.Context, runtime config.Runtime) (string, error) {
	command, args, err := statusCommand(runtime)
	if err != nil {
		return "", err
	}
	output, err := exec.CommandContext(ctx, command, args...).CombinedOutput()
	state := strings.TrimSpace(string(output))
	if err != nil {
		if runtime.Type == "systemd" && knownSystemdState(state) {
			return state, nil
		}
		return "", commandError(ctx, command, output, err)
	}
	if state == "" {
		return "", fmt.Errorf("%s returned an empty status", command)
	}
	return state, nil
}

func actionCommand(runtime config.Runtime, action string) (string, []string, error) {
	switch runtime.Type {
	case "systemd":
		return "systemctl", []string{"--user", action, "--", runtime.Service}, nil
	case "docker":
		return "docker", []string{action, "--", runtime.Container}, nil
	default:
		return "", nil, fmt.Errorf("unsupported runtime type %q", runtime.Type)
	}
}

func statusCommand(runtime config.Runtime) (string, []string, error) {
	switch runtime.Type {
	case "systemd":
		return "systemctl", []string{"--user", "is-active", "--", runtime.Service}, nil
	case "docker":
		return "docker", []string{"inspect", "--format", "{{.State.Status}}", "--", runtime.Container}, nil
	default:
		return "", nil, fmt.Errorf("unsupported runtime type %q", runtime.Type)
	}
}

func commandError(ctx context.Context, command string, output []byte, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%s: %w", command, ctxErr)
	}
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		return fmt.Errorf("%s: %w", command, err)
	}
	return fmt.Errorf("%s: %w: %s", command, err, detail)
}

func knownSystemdState(state string) bool {
	switch state {
	case "active", "reloading", "inactive", "failed", "activating", "deactivating", "maintenance":
		return true
	default:
		return false
	}
}
