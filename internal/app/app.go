package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/magiodev/llmm/internal/config"
	runtimeops "github.com/magiodev/llmm/internal/runtime"
	"github.com/spf13/cobra"
)

const supervisorTimeout = 30 * time.Second
const deepHashTimeout = 30 * time.Minute

// Package-level function variables let tests inject failing file opens and
// hash writers without changing behavior in production.
var (
	openFile = os.Open
	newHash  = sha256.New
	readFile = func(f *os.File, b []byte) (int, error) { return f.Read(b) }
)

type options struct {
	configPath string
	quiet      bool
}

type runtimeStatus struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	State string `json:"state"`
}

type modelInfo struct {
	Name      string            `json:"name"`
	Runtime   string            `json:"runtime"`
	Path      string            `json:"path"`
	Context   int               `json:"context,omitempty"`
	Output    int               `json:"output,omitempty"`
	Default   bool              `json:"default,omitempty"`
	Artifacts []config.Artifact `json:"artifacts,omitempty"`
}

type doctorCheck struct {
	OK     bool   `json:"ok"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

type doctorResult struct {
	Success bool          `json:"success"`
	Checks  []doctorCheck `json:"checks"`
}

// writeJSON marshals value as indented JSON and writes it with a trailing
// newline. value is always a fixed-shape slice or struct whose fields are
// plain scalars or slices, so json.MarshalIndent cannot return an error;
// only the write itself can fail.
func writeJSON(cmd *cobra.Command, value any) error {
	payload, _ := json.MarshalIndent(value, "", "  ")
	if _, err := cmd.OutOrStdout().Write(payload); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout())
	return nil
}

func New(version string) *cobra.Command {
	opts := &options{}
	root := &cobra.Command{
		Use:           "llmm",
		Short:         "Manage local LLM runtimes from one YAML manifest",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.PersistentFlags().StringVar(&opts.configPath, "config", config.DefaultPath(), "path to config file")
	root.PersistentFlags().BoolVarP(&opts.quiet, "quiet", "q", false, "suppress confirmation output")
	root.AddCommand(configCommand(opts), doctorCommand(opts), statusCommand(opts), actionCommand(opts, "start"), actionCommand(opts, "stop"), actionCommand(opts, "restart"), modelsCommand(opts), installCommand(opts), verifyCommand(opts), installedCommand(opts))
	return root
}

func configCommand(opts *options) *cobra.Command {
	command := &cobra.Command{Use: "config", Short: "Create and validate manifests"}
	var force bool
	var format string
	initCommand := &cobra.Command{
		Use: "init", Short: "Create a minimal starter manifest", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			starter := &config.Config{Version: config.Version, Runtimes: map[string]config.Runtime{"example": {Type: "systemd", Service: "example.service"}}, Models: map[string]config.Model{}}
			if err := config.Write(opts.configPath, starter, force); err != nil {
				return err
			}
			if !opts.quiet {
				fmt.Fprintln(cmd.OutOrStdout(), opts.configPath)
			}
			return nil
		},
	}
	initCommand.Flags().BoolVar(&force, "force", false, "replace an existing manifest")
	validateCommand := &cobra.Command{
		Use: "validate", Short: "Validate the manifest", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := config.Load(opts.configPath); err != nil {
				return err
			}
			if !opts.quiet {
				fmt.Fprintln(cmd.OutOrStdout(), "config: ok")
			}
			return nil
		},
	}
	showCommand := &cobra.Command{
		Use: "show", Short: "Print the normalized manifest", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			payload, err := config.Marshal(cfg, format)
			if err != nil {
				return err
			}
			if _, err := cmd.OutOrStdout().Write(payload); err != nil {
				return err
			}
			if len(payload) == 0 || payload[len(payload)-1] != '\n' {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	showCommand.Flags().StringVar(&format, "format", "yaml", "output format: yaml or json")
	command.AddCommand(initCommand, validateCommand, showCommand)
	return command
}

func actionCommand(opts *options, action string) *cobra.Command {
	return &cobra.Command{
		Use: action + " <runtime>", Short: strings.ToUpper(action[:1]) + action[1:] + " a runtime", Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			return runtimeops.Names(cfg), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			rt, ok := cfg.Runtimes[args[0]]
			if !ok {
				return fmt.Errorf("unknown runtime %q", args[0])
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), supervisorTimeout)
			defer cancel()
			if err := runtimeops.Action(ctx, rt, action); err != nil {
				return err
			}
			if !opts.quiet {
				state, err := runtimeops.Status(ctx, rt)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", args[0], state)
			}
			return nil
		},
	}
}

func statusCommand(opts *options) *cobra.Command {
	var format string
	command := &cobra.Command{
		Use: "status [runtime]", Short: "Show runtime status", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			names := runtimeops.Names(cfg)
			if len(args) == 1 {
				if _, ok := cfg.Runtimes[args[0]]; !ok {
					return fmt.Errorf("unknown runtime %q", args[0])
				}
				names = args
			}
			var failures []string
			entries := make([]runtimeStatus, 0, len(names))
			for _, name := range names {
				ctx, cancel := context.WithTimeout(cmd.Context(), supervisorTimeout)
				state, statusErr := runtimeops.Status(ctx, cfg.Runtimes[name])
				cancel()
				entries = append(entries, runtimeStatus{Name: name, Type: cfg.Runtimes[name].Type, State: state})
				if statusErr != nil {
					if format == "json" {
						entries[len(entries)-1].State = "error"
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "%-16s error\n", name)
					}
					failures = append(failures, name+": "+statusErr.Error())
					continue
				}
				if format != "json" {
					fmt.Fprintf(cmd.OutOrStdout(), "%-16s %s\n", name, state)
				}
			}
			if format == "json" {
				if err := writeJSON(cmd, entries); err != nil {
					return err
				}
			}
			if len(failures) > 0 {
				return errors.New(strings.Join(failures, "; "))
			}
			return nil
		},
	}
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

func modelsCommand(opts *options) *cobra.Command {
	var format string
	command := &cobra.Command{
		Use: "models", Short: "List configured models", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			names := make([]string, 0, len(cfg.Models))
			for name := range cfg.Models {
				names = append(names, name)
			}
			sort.Strings(names)
			if format == "json" {
				infos := make([]modelInfo, 0, len(names))
				for _, name := range names {
					model := cfg.Models[name]
					infos = append(infos, modelInfo{
						Name: name, Runtime: model.Runtime, Path: model.Path,
						Context: model.Context, Output: model.Output,
						Default: name == cfg.DefaultModel, Artifacts: model.Artifacts,
					})
				}
				if err := writeJSON(cmd, infos); err != nil {
					return err
				}
			} else {
				for _, name := range names {
					model := cfg.Models[name]
					fmt.Fprintf(cmd.OutOrStdout(), "%s	%s	%s\n", name, model.Runtime, model.Path)
				}
			}
			return nil
		},
	}
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

func doctorCommand(opts *options) *cobra.Command {
	var deep bool
	var format string
	command := &cobra.Command{
		Use: "doctor", Short: "Validate config, runtimes, executables, and model files", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			var failures []string
			var checks []doctorCheck
			check := func(ok bool, label, detail string) {
				emitCheck(cmd, format, &checks, &failures, ok, label, detail)
			}
			check(true, "config", opts.configPath)
			for _, name := range runtimeops.Names(cfg) {
				rt := cfg.Runtimes[name]
				if rt.Executable != "" {
					info, statErr := os.Stat(rt.Executable)
					ok := statErr == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
					check(ok, "runtime "+name, rt.Executable)
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), supervisorTimeout)
				switch rt.Type {
				case "systemd":
					out, unitErr := exec.CommandContext(ctx, "systemctl", "--user", "show", "--property=LoadState", "--value", "--", rt.Service).CombinedOutput()
					check(unitErr == nil && strings.TrimSpace(string(out)) == "loaded", "service "+name, rt.Service)
				case "docker":
					out, dockerErr := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Name}}", "--", rt.Container).CombinedOutput()
					detail := rt.Container
					if dockerErr != nil && strings.TrimSpace(string(out)) != "" {
						detail += ": " + strings.TrimSpace(string(out))
					}
					check(dockerErr == nil, "docker "+name, detail)
				}
				cancel()
			}
			modelNames := make([]string, 0, len(cfg.Models))
			for name := range cfg.Models {
				modelNames = append(modelNames, name)
			}
			sort.Strings(modelNames)
			for _, name := range modelNames {
				model := cfg.Models[name]
				checkArtifact(cmd, check, "model "+name, "sha256 "+name, model.Path, model.Size, model.SHA256, deep)
				for i, artifact := range model.Artifacts {
					checkArtifact(cmd, check, "artifact "+name, fmt.Sprintf("sha256 %s artifact %d", name, i), artifact.Path, artifact.Size, artifact.SHA256, deep)
				}
			}
			if format == "json" {
				if err := writeJSON(cmd, doctorResult{Success: len(failures) == 0, Checks: checks}); err != nil {
					return err
				}
			}
			if len(failures) > 0 {
				return fmt.Errorf("doctor found %d problem(s): %s", len(failures), strings.Join(failures, "; "))
			}
			return nil
		},
	}
	command.Flags().BoolVar(&deep, "deep", false, "also verify model SHA-256 checksums")
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

func fileSHA256(ctx context.Context, path string) (string, error) {
	file, err := openFile(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := newHash()
	buffer := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		count, readErr := readFile(file, buffer)
		if count > 0 {
			if _, err := hash.Write(buffer[:count]); err != nil {
				return "", err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
