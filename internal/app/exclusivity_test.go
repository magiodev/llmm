package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/magiodev/llmm/internal/config"
)

func newCobraCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	return cmd
}

func TestStopOtherActiveStopsOnlyBackedActive(t *testing.T) {
	installFakeCommand(t, "systemctl", `if [ "$2" = "is-active" ]; then printf 'active\n'; fi; exit 0`)
	path := writeManifest(t, "version: 1\n"+
		"runtimes:\n"+
		"  a:\n    type: systemd\n    service: a.service\n"+
		"  b:\n    type: systemd\n    service: b.service\n"+
		"  ui:\n    type: systemd\n    service: ui.service\n"+
		"models:\n"+
		"  ma:\n    runtime: a\n    path: /a\n"+
		"  mb:\n    runtime: b\n    path: /b\n")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	stopped, err := stopOtherActiveModelRuntimes(newCobraCmd(), cfg, "a")
	if err != nil {
		t.Fatal(err)
	}
	if len(stopped) != 1 || stopped[0] != "b" {
		t.Fatalf("stopped = %v, want [b]", stopped)
	}
}

func TestStopOtherActiveSkipsInactive(t *testing.T) {
	installFakeCommand(t, "systemctl", `if [ "$2" = "is-active" ]; then printf 'inactive\n'; fi; exit 0`)
	path := writeManifest(t, "version: 1\n"+
		"runtimes:\n"+
		"  a:\n    type: systemd\n    service: a.service\n"+
		"  b:\n    type: systemd\n    service: b.service\n"+
		"models:\n"+
		"  ma:\n    runtime: a\n    path: /a\n"+
		"  mb:\n    runtime: b\n    path: /b\n")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	stopped, err := stopOtherActiveModelRuntimes(newCobraCmd(), cfg, "a")
	if err != nil {
		t.Fatal(err)
	}
	if len(stopped) != 0 {
		t.Fatalf("stopped = %v, want none", stopped)
	}
}

func TestStopOtherActiveSkipsQueryError(t *testing.T) {
	installFakeCommand(t, "systemctl", `exit 7`)
	path := writeManifest(t, "version: 1\n"+
		"runtimes:\n"+
		"  a:\n    type: systemd\n    service: a.service\n"+
		"  b:\n    type: systemd\n    service: b.service\n"+
		"models:\n"+
		"  ma:\n    runtime: a\n    path: /a\n"+
		"  mb:\n    runtime: b\n    path: /b\n")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	stopped, err := stopOtherActiveModelRuntimes(newCobraCmd(), cfg, "a")
	if err != nil {
		t.Fatal(err)
	}
	if len(stopped) != 0 {
		t.Fatalf("stopped = %v, want none", stopped)
	}
}

func TestStopOtherActiveStopFailure(t *testing.T) {
	installFakeCommand(t, "systemctl", `if [ "$2" = "is-active" ]; then printf 'active\n'; exit 0; fi; if [ "$2" = "stop" ]; then printf 'boom\n' >&2; exit 7; fi; exit 0`)
	path := writeManifest(t, "version: 1\n"+
		"runtimes:\n"+
		"  a:\n    type: systemd\n    service: a.service\n"+
		"  b:\n    type: systemd\n    service: b.service\n"+
		"models:\n"+
		"  ma:\n    runtime: a\n    path: /a\n"+
		"  mb:\n    runtime: b\n    path: /b\n")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	stopped, err := stopOtherActiveModelRuntimes(newCobraCmd(), cfg, "a")
	if err == nil || !strings.Contains(err.Error(), "stop b") {
		t.Fatalf("err = %v", err)
	}
	if len(stopped) != 0 {
		t.Fatalf("stopped = %v, want none", stopped)
	}
}

func TestStopOtherActiveNonModelTarget(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'active\n'`)
	path := writeManifest(t, "version: 1\n"+
		"runtimes:\n"+
		"  a:\n    type: systemd\n    service: a.service\n"+
		"  ui:\n    type: systemd\n    service: ui.service\n"+
		"models:\n"+
		"  ma:\n    runtime: a\n    path: /a\n")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	stopped, err := stopOtherActiveModelRuntimes(newCobraCmd(), cfg, "ui")
	if err != nil {
		t.Fatal(err)
	}
	if len(stopped) != 0 {
		t.Fatalf("stopped = %v, want none", stopped)
	}
}

func TestStatusRenderTable(t *testing.T) {
	cfg, err := config.Load(writeManifest(t, "version: 1\n"+
		"runtimes:\n"+
		"  a:\n    type: systemd\n    service: a.service\n    endpoint: http://x:8001/v1\n"+
		"  b:\n    type: docker\n    container: b\n"+
		"models:\n"+
		"  ma:\n    runtime: a\n    path: /a\n    context: 196608\n"+
		"  mz:\n    runtime: a\n    path: /z\n    context: 0\n"))
	if err != nil {
		t.Fatal(err)
	}
	entries := []runtimeStatus{{Name: "a", Type: "systemd", State: "active"}, {Name: "b", Type: "docker", State: "inactive"}}
	out := renderStatus(cfg, entries, false)
	for _, want := range []string{"RUNTIME", "ENDPOINT", "MODELS (CTX)", "8001", "ma (196k)", "mz", "-"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output %q missing %q", out, want)
		}
	}
}

func TestStatusRenderColor(t *testing.T) {
	cfg, err := config.Load(writeManifest(t, "version: 1\n"+
		"runtimes:\n"+
		"  a:\n    type: systemd\n    service: a.service\n"+
		"models:\n"+
		"  ma:\n    runtime: a\n    path: /a\n"))
	if err != nil {
		t.Fatal(err)
	}
	entries := []runtimeStatus{{Name: "a", Type: "systemd", State: "active"}}
	out := renderStatus(cfg, entries, true)
	if !strings.Contains(out, ansiGreen) || !strings.Contains(out, ansiCyan) || !strings.Contains(out, ansiReset) {
		t.Fatalf("output missing expected ANSI codes: %q", out)
	}
}

func TestStateColor(t *testing.T) {
	if stateColor("active") != ansiGreen || stateColor("running") != ansiGreen {
		t.Fatal("active/running should be green")
	}
	if stateColor("inactive") != ansiRed || stateColor("exited") != ansiRed || stateColor("failed") != ansiRed {
		t.Fatal("inactive/exited/failed should be red")
	}
	if stateColor("error") != ansiYellow {
		t.Fatal("error should be yellow")
	}
	if stateColor("weird") != "" {
		t.Fatal("unknown state should have no color")
	}
}

func TestUseColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if useColor() {
		t.Fatal("NO_COLOR set should disable color")
	}
	t.Setenv("NO_COLOR", "")
	isTerminal = func() bool { return true }
	if !useColor() {
		t.Fatal("terminal without NO_COLOR should enable color")
	}
	isTerminal = func() bool { return false }
	if useColor() {
		t.Fatal("non-terminal should disable color")
	}
}

func TestPortFromEndpoint(t *testing.T) {
	if got := portFromEndpoint("http://dgx:8002/v1"); got != "8002" {
		t.Fatalf("got %q", got)
	}
	if got := portFromEndpoint("http://dgx/v1"); got != "-" {
		t.Fatalf("got %q", got)
	}
	if got := portFromEndpoint("not a url"); got != "-" {
		t.Fatalf("got %q", got)
	}
}

func TestActionStartExclusiveStopsOther(t *testing.T) {
	installFakeCommand(t, "systemctl", `case "$2" in is-active) printf 'active\n';; start|stop) :;; esac; exit 0`)
	dir := t.TempDir()
	ma := filepath.Join(dir, "ma")
	mb := filepath.Join(dir, "mb")
	for _, p := range []string{ma, mb} {
		if err := os.WriteFile(p, []byte("model"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\n"+
		"runtimes:\n"+
		"  a:\n    type: systemd\n    service: a.service\n"+
		"  b:\n    type: systemd\n    service: b.service\n"+
		"models:\n"+
		"  ma:\n    runtime: a\n    path: %s\n"+
		"  mb:\n    runtime: b\n    path: %s\n", ma, mb))
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "start", "a"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "stopped b (starting a, exclusive)") {
		t.Fatalf("output = %q", output.String())
	}
	if !strings.Contains(output.String(), "a: active") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestActionStartExclusiveQuiet(t *testing.T) {
	installFakeCommand(t, "systemctl", `case "$2" in is-active) printf 'active\n';; start|stop) :;; esac; exit 0`)
	dir := t.TempDir()
	ma := filepath.Join(dir, "ma")
	mb := filepath.Join(dir, "mb")
	for _, p := range []string{ma, mb} {
		if err := os.WriteFile(p, []byte("model"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\n"+
		"runtimes:\n"+
		"  a:\n    type: systemd\n    service: a.service\n"+
		"  b:\n    type: systemd\n    service: b.service\n"+
		"models:\n"+
		"  ma:\n    runtime: a\n    path: %s\n"+
		"  mb:\n    runtime: b\n    path: %s\n", ma, mb))
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "-q", "start", "a"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "stopped b") {
		t.Fatalf("quiet output should omit exclusivity note: %q", output.String())
	}
}

func TestActionStartExclusiveStopFailure(t *testing.T) {
	installFakeCommand(t, "systemctl", `if [ "$2" = "is-active" ]; then printf 'active\n'; exit 0; fi; if [ "$2" = "stop" ]; then printf 'nope\n' >&2; exit 7; fi; exit 0`)
	dir := t.TempDir()
	ma := filepath.Join(dir, "ma")
	mb := filepath.Join(dir, "mb")
	for _, p := range []string{ma, mb} {
		if err := os.WriteFile(p, []byte("model"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\n"+
		"runtimes:\n"+
		"  a:\n    type: systemd\n    service: a.service\n"+
		"  b:\n    type: systemd\n    service: b.service\n"+
		"models:\n"+
		"  ma:\n    runtime: a\n    path: %s\n"+
		"  mb:\n    runtime: b\n    path: %s\n", ma, mb))
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "start", "a"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "stop b") {
		t.Fatalf("err = %v", err)
	}
}
