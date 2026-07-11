package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/magiodev/llmm/internal/backend"
	"github.com/magiodev/llmm/internal/model"
	"github.com/magiodev/llmm/internal/service"
	"github.com/magiodev/llmm/internal/tailnet"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	backendName string
	modelKey    string
	userHome    string
	repoRoot    string
)

// NewRootCmd creates the root command
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "llmm",
		Short: "LLM Manager - manage local LLM runtimes",
		Long: `llmm is a CLI tool to manage local LLM runtimes (turboquant, ds4, vllm, comfyui)
with systemd user services, model switching, and Tailscale integration.

Inspired by nvm/fnm for Node.js, llmm provides a unified interface for managing
multiple LLM backends on your local machine.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Set defaults
			if userHome == "" {
				userHome = os.Getenv("HOME")
			}
			if repoRoot == "" {
				// Default repo root (override with --repo flag)
				repoRoot = "/home/magiodev/LLM"
			}
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&userHome, "home", os.Getenv("HOME"), "User home directory")
	rootCmd.PersistentFlags().StringVar(&repoRoot, "repo", "/home/magiodev/LLM", "LLM runtime repo root")

	// Add commands
	rootCmd.AddCommand(newStartCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newRestartCmd())
	rootCmd.AddCommand(newSwitchCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newSyncCmd())
	rootCmd.AddCommand(newModelsCmd())
	rootCmd.AddCommand(newInitCmd())

	return rootCmd
}

func main() {
	cmd := NewRootCmd()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// newStartCmd creates the start command
func newStartCmd() *cobra.Command {
	var targets []string

	cmd := &cobra.Command{
		Use:   "start [target...]",
		Short: "Start one or more backends",
		Long:  `Start one or more LLM backends. Use 'all' to start all backends.`,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, target := range args {
				if target == "all" {
					targets = []string{"webui", "comfyui", "ds4", "turboquant", "vllm"}
				} else {
					targets = append(targets, target)
				}
			}

			svc := service.NewServiceManager(
				fmt.Sprintf("%s/.config/systemd/user", userHome),
				fmt.Sprintf("%s/systemd/user", repoRoot),
			)

			for _, target := range targets {
				serviceName := svc.GetServiceName(target)
				fmt.Printf("Starting %s (%s)... ", target, serviceName)
				if err := svc.Start(serviceName); err != nil {
					fmt.Println("FAILED")
					return err
				}
				fmt.Println("OK")
			}

			return nil
		},
	}

	return cmd
}

// newStopCmd creates the stop command
func newStopCmd() *cobra.Command {
	var targets []string

	cmd := &cobra.Command{
		Use:   "stop [target...]",
		Short: "Stop one or more backends",
		Long:  `Stop one or more LLM backends. Use 'all' to stop all backends.`,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, target := range args {
				if target == "all" {
					targets = []string{"webui", "comfyui", "ds4", "turboquant", "vllm"}
				} else {
					targets = append(targets, target)
				}
			}

			svc := service.NewServiceManager(
				fmt.Sprintf("%s/.config/systemd/user", userHome),
				fmt.Sprintf("%s/systemd/user", repoRoot),
			)

			for _, target := range targets {
				serviceName := svc.GetServiceName(target)
				fmt.Printf("Stopping %s (%s)... ", target, serviceName)
				if err := svc.Stop(serviceName); err != nil {
					fmt.Println("FAILED")
					return err
				}
				fmt.Println("OK")
			}

			return nil
		},
	}

	return cmd
}

// newRestartCmd creates the restart command
func newRestartCmd() *cobra.Command {
	var target string

	cmd := &cobra.Command{
		Use:   "restart <target>",
		Short: "Restart a backend",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target = args[0]
			svc := service.NewServiceManager(
				fmt.Sprintf("%s/.config/systemd/user", userHome),
				fmt.Sprintf("%s/systemd/user", repoRoot),
			)

			serviceName := svc.GetServiceName(target)
			fmt.Printf("Restarting %s (%s)... ", target, serviceName)
			if err := svc.Restart(serviceName); err != nil {
				fmt.Println("FAILED")
				return err
			}
			fmt.Println("OK")

			return nil
		},
	}

	return cmd
}

// newSwitchCmd creates the switch command
func newSwitchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "switch <backend> <model>",
		Short: "Switch model for a backend",
		Long:  `Switch the model for a backend. Use 'llmm models <backend>' to see available models.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			backendName = args[0]
			modelKey = args[1]

			be, err := backend.GetBackend(backendName)
			if err != nil {
				return err
			}

			if err := be.SwitchModel(modelKey); err != nil {
				return err
			}

			fmt.Printf("Switched %s to model %s\n", backendName, modelKey)
			return nil
		},
	}

	return cmd
}

// newStatusCmd creates the status command
func newStatusCmd() *cobra.Command {
	var target string
	var tailnetStatus bool

	cmd := &cobra.Command{
		Use:   "status [target]",
		Short: "Show status of backends",
		Long:  `Show status of one or all backends. Use --tailnet to show Tailscale status.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewServiceManager(
				fmt.Sprintf("%s/.config/systemd/user", userHome),
				fmt.Sprintf("%s/systemd/user", repoRoot),
			)

			if len(args) == 0 {
				// Show status for all backends
				targets := []string{"webui", "comfyui", "ds4", "turboquant", "vllm"}
				fmt.Println("Backend Status:")
				fmt.Println(strings.Repeat("-", 50))
				for _, target := range targets {
					serviceName := svc.GetServiceName(target)
					status := svc.Status(serviceName)
					fmt.Printf("  %-12s %-10s %s\n", target, serviceName, status)
				}
			} else {
				target = args[0]
				serviceName := svc.GetServiceName(target)
				status := svc.Status(serviceName)
				fmt.Printf("%s (%s): %s\n", target, serviceName, status)
			}

			if tailnetStatus {
				tm := tailnet.NewTailnetManager([]int{8001, 8003, 9000, 8188})
				if tm.IsAvailable() {
					fmt.Println("\nTailscale Status:")
					fmt.Println(tm.GetStatusString())
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&tailnetStatus, "tailnet", false, "Show Tailscale status")

	return cmd
}

// newSyncCmd creates the sync command
func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync service files and Tailscale exposure",
		Long:  `Sync service files from repo to user unit dir and expose via Tailscale.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewServiceManager(
				fmt.Sprintf("%s/.config/systemd/user", userHome),
				fmt.Sprintf("%s/systemd/user", repoRoot),
			)

			fmt.Println("Syncing service files...")
			if err := svc.Sync(); err != nil {
				return err
			}
			fmt.Println("Service files synced.")

			fmt.Println("Syncing Tailscale exposure...")
			tm := tailnet.NewTailnetManager([]int{8001, 8003, 9000, 8188})
			if !tm.IsAvailable() {
				fmt.Println("Warning: Tailscale not available, skipping sync.")
				return nil
			}

			if err := tm.Sync(); err != nil {
				return err
			}
			fmt.Println("Tailscale exposure synced.")

			return nil
		},
	}

	return cmd
}

// newModelsCmd creates the models command
func newModelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models [backend]",
		Short: "List available models for a backend",
		Long:  `List available models for a backend. If no backend specified, show all backends.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backends := []string{"turboquant", "ds4", "vllm", "comfyui"}
			if len(args) == 1 {
				backends = []string{args[0]}
			}

			for _, backend := range backends {
				be, err := backend.GetBackend(backend)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					continue
				}

				fmt.Printf("\n%s (port %d):\n", backend.Name(), backend.Port())
				for _, key := range be.ModelKeys() {
					info, err := model.GetModelInfo(backend.Name(), key)
					if err == nil {
						fmt.Printf("  %-12s %-30s %s\n", info.Key, info.Name, info.Path)
					} else {
						fmt.Printf("  %-12s\n", key)
					}
				}
			}

			return nil
		},
	}

	return cmd
}

// newInitCmd creates the init command
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize llmm for this machine",
		Long:  `Initialize llmm by syncing service files and setting up Tailscale exposure.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Initializing llmm...")

			svc := service.NewServiceManager(
				fmt.Sprintf("%s/.config/systemd/user", userHome),
				fmt.Sprintf("%s/systemd/user", repoRoot),
			)

			if err := svc.Sync(); err != nil {
				return fmt.Errorf("failed to sync services: %w", err)
			}

			tm := tailnet.NewTailnetManager([]int{8001, 8003, 9000, 8188})
			if tm.IsAvailable() {
				if err := tm.Sync(); err != nil {
					return fmt.Errorf("failed to sync tailscale: %w", err)
				}
			}

			fmt.Println("llmm initialized successfully!")
			fmt.Println("Usage: llmm start <backend> | llmm switch <backend> <model> | llmm status")
			return nil
		},
	}

	return cmd
}
