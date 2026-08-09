package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/magiodev/llmm/internal/config"
)

func installCommand(t *testing.T, name, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return dir
}

func TestNames(t *testing.T) {
	cfg := &config.Config{Runtimes: map[string]config.Runtime{"z": {}, "a": {}}}
	got := strings.Join(Names(cfg), ",")
	if got != "a,z" {
		t.Fatalf("Names = %q", got)
	}
}

func TestSystemdActionUsesOptionSeparator(t *testing.T) {
	log := filepath.Join(t.TempDir(), "args")
	installCommand(t, "systemctl", `printf '%s\n' "$*" > "`+log+`"`)
	if err := Action(context.Background(), config.Runtime{Type: "systemd", Service: "example.service"}, "restart"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(data)); got != "--user restart -- example.service" {
		t.Fatalf("args = %q", got)
	}
}

func TestDockerStatus(t *testing.T) {
	installCommand(t, "docker", `printf 'running\n'`)
	state, err := Status(context.Background(), config.Runtime{Type: "docker", Container: "open-webui"})
	if err != nil {
		t.Fatal(err)
	}
	if state != "running" {
		t.Fatalf("state = %q", state)
	}
}

func TestInactiveSystemdStatusIsNotAnExecutionError(t *testing.T) {
	installCommand(t, "systemctl", `printf 'inactive\n'; exit 3`)
	state, err := Status(context.Background(), config.Runtime{Type: "systemd", Service: "example.service"})
	if err != nil {
		t.Fatal(err)
	}
	if state != "inactive" {
		t.Fatalf("state = %q", state)
	}
}

func TestStatusPropagatesExecutionError(t *testing.T) {
	installCommand(t, "docker", `printf 'permission denied\n' >&2; exit 7`)
	_, err := Status(context.Background(), config.Runtime{Type: "docker", Container: "missing"})
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("error = %v", err)
	}
}

func TestActionHonorsContextDeadline(t *testing.T) {
	installCommand(t, "docker", `exec /bin/sleep 1`)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := Action(ctx, config.Runtime{Type: "docker", Container: "slow"}, "start")
	if err == nil || !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("error = %v", err)
	}
}

func TestUnsupportedRuntime(t *testing.T) {
	if err := Action(context.Background(), config.Runtime{Type: "process"}, "start"); err == nil {
		t.Fatal("expected unsupported runtime error")
	}
	if _, err := Status(context.Background(), config.Runtime{Type: "process"}); err == nil {
		t.Fatal("expected unsupported runtime error")
	}
}

func TestStatusEmptyOutput(t *testing.T) {
	installCommand(t, "docker", `true`)
	_, err := Status(context.Background(), config.Runtime{Type: "docker", Container: "web"})
	if err == nil || !strings.Contains(err.Error(), "empty status") {
		t.Fatalf("error = %v", err)
	}
}

func TestCommandErrorNoOutput(t *testing.T) {
	installCommand(t, "docker", `exit 1`)
	_, err := Status(context.Background(), config.Runtime{Type: "docker", Container: "web"})
	if err == nil || !strings.Contains(err.Error(), "docker:") {
		t.Fatalf("error = %v", err)
	}
}

func TestStatusSystemdUnknownState(t *testing.T) {
	installCommand(t, "systemctl", `printf 'weird\n'; exit 3`)
	_, err := Status(context.Background(), config.Runtime{Type: "systemd", Service: "example.service"})
	if err == nil || !strings.Contains(err.Error(), "systemctl:") {
		t.Fatalf("error = %v", err)
	}
}
