package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNewRootCmd(t *testing.T) {
	cmd := NewRootCmd()
	if cmd == nil {
		t.Fatal("Expected root command, got nil")
	}

	if cmd.Use != "llmm" {
		t.Errorf("Expected use 'llmm', got '%s'", cmd.Use)
	}

	// Check that expected commands exist
	expectedCommands := []string{
		"start", "stop", "restart", "switch", "status", "sync", "models", "init",
	}

	for _, expected := range expectedCommands {
		found := false
		for _, cmd := range cmd.Commands() {
			if cmd.Use == expected || cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Command %q not found", expected)
		}
	}
}

func TestStartCommand(t *testing.T) {
	cmd := NewRootCmd()
	startCmd := cmd.CommandAtPath("start")
	if startCmd == nil {
		t.Fatal("Expected 'start' command, got nil")
	}

	if startCmd.Use != "start [target...]" {
		t.Errorf("Expected use 'start [target...]', got '%s'", startCmd.Use)
	}
}

func TestStopCommand(t *testing.T) {
	cmd := NewRootCmd()
	stopCmd := cmd.CommandAtPath("stop")
	if stopCmd == nil {
		t.Fatal("Expected 'stop' command, got nil")
	}

	if stopCmd.Use != "stop [target...]" {
		t.Errorf("Expected use 'stop [target...]', got '%s'", stopCmd.Use)
	}
}

func TestSwitchCommand(t *testing.T) {
	cmd := NewRootCmd()
	switchCmd := cmd.CommandAtPath("switch")
	if switchCmd == nil {
		t.Fatal("Expected 'switch' command, got nil")
	}

	if switchCmd.Use != "switch <backend> <model>" {
		t.Errorf("Expected use 'switch <backend> <model>', got '%s'", switchCmd.Use)
	}
}

func TestStatusCommand(t *testing.T) {
	cmd := NewRootCmd()
	statusCmd := cmd.CommandAtPath("status")
	if statusCmd == nil {
		t.Fatal("Expected 'status' command, got nil")
	}

	// Check that --tailnet flag exists
	flag := statusCmd.Flags().Lookup("tailnet")
	if flag == nil {
		t.Error("Expected --tailnet flag on status command")
	}
}

// CommandAtPath returns the command at the given path
func (c *cobra.Command) CommandAtPath(path string) *cobra.Command {
	if c.Name() == path {
		return c
	}
	for _, cmd := range c.Commands() {
		if found := cmd.CommandAtPath(path); found != nil {
			return found
		}
	}
	return nil
}
