package app

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/magiodev/llmm/internal/config"
	runtimeops "github.com/magiodev/llmm/internal/runtime"
	"github.com/spf13/cobra"
)

type options struct {
	configPath string
	quiet      bool
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
	root.PersistentFlags().BoolVarP(&opts.quiet, "quiet", "q", false, "only print errors")
	root.AddCommand(configCommand(opts), doctorCommand(opts), statusCommand(opts), actionCommand(opts, "start"), actionCommand(opts, "stop"), actionCommand(opts, "restart"), modelsCommand(opts))
	return root
}

func load(opts *options) (*config.Config, error) {
	return config.Load(opts.configPath)
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
			if _, err := load(opts); err != nil {
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
			cfg, err := load(opts)
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
			cfg, err := load(opts)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			return runtimeops.Names(cfg), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load(opts)
			if err != nil {
				return err
			}
			rt, ok := cfg.Runtimes[args[0]]
			if !ok {
				return fmt.Errorf("unknown runtime %q", args[0])
			}
			if err := runtimeops.Action(rt, action); err != nil {
				return err
			}
			if !opts.quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", args[0], runtimeops.Status(rt))
			}
			return nil
		},
	}
}

func statusCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "status [runtime]", Short: "Show runtime status", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load(opts)
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
			for _, name := range names {
				fmt.Fprintf(cmd.OutOrStdout(), "%-16s %s\n", name, runtimeops.Status(cfg.Runtimes[name]))
			}
			return nil
		},
	}
}

func modelsCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "models", Short: "List configured models", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load(opts)
			if err != nil {
				return err
			}
			names := make([]string, 0, len(cfg.Models))
			for name := range cfg.Models {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				model := cfg.Models[name]
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", name, model.Runtime, model.Path)
			}
			return nil
		},
	}
}

func doctorCommand(opts *options) *cobra.Command {
	var deep bool
	command := &cobra.Command{
		Use: "doctor", Short: "Validate config, runtimes, executables, and model files", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load(opts)
			if err != nil {
				return err
			}
			var failures []string
			check := func(ok bool, label, detail string) {
				state := "ok"
				if !ok {
					state = "fail"
					failures = append(failures, label+": "+detail)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-5s %-24s %s\n", state, label, detail)
			}
			check(true, "config", opts.configPath)
			for _, name := range runtimeops.Names(cfg) {
				rt := cfg.Runtimes[name]
				if rt.Executable != "" {
					info, statErr := os.Stat(rt.Executable)
					check(statErr == nil && !info.IsDir(), "runtime "+name, rt.Executable)
				}
				if rt.Type == "systemd" {
					out, unitErr := exec.Command("systemctl", "--user", "show", rt.Service, "--property=LoadState", "--value").Output()
					check(unitErr == nil && strings.TrimSpace(string(out)) == "loaded", "service "+name, rt.Service)
				}
				if rt.Type == "docker" {
					_, dockerErr := exec.LookPath("docker")
					check(dockerErr == nil, "docker "+name, rt.Container)
				}
			}
			modelNames := make([]string, 0, len(cfg.Models))
			for name := range cfg.Models {
				modelNames = append(modelNames, name)
			}
			sort.Strings(modelNames)
			for _, name := range modelNames {
				model := cfg.Models[name]
				info, statErr := os.Stat(model.Path)
				ok := statErr == nil && !info.IsDir()
				detail := model.Path
				if ok && model.Size > 0 {
					ok = info.Size() == model.Size
					detail = fmt.Sprintf("%s (%d bytes)", model.Path, info.Size())
				}
				check(ok, "model "+name, detail)
				if deep && ok && model.SHA256 != "" {
					sum, hashErr := fileSHA256(model.Path)
					check(hashErr == nil && sum == strings.ToLower(model.SHA256), "sha256 "+name, sum)
				}
			}
			if len(failures) > 0 {
				return fmt.Errorf("doctor found %d problem(s): %s", len(failures), strings.Join(failures, "; "))
			}
			return nil
		},
	}
	command.Flags().BoolVar(&deep, "deep", false, "also verify model SHA-256 checksums")
	return command
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
