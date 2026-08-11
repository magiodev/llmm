package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/magiodev/llmm/internal/config"
	"github.com/magiodev/llmm/internal/install"
	"github.com/spf13/cobra"
)

const installTimeout = 30 * time.Minute

func installCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "install <model>",
		Short: "Fetch and install a declared model artifact",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			var names []string
			for name, model := range cfg.Models {
				if model.Source != "" {
					names = append(names, name)
				}
			}
			sort.Strings(names)
			return names, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			name := args[0]
			model, ok := cfg.Models[name]
			if !ok {
				return fmt.Errorf("unknown model %q", name)
			}
			if model.Source == "" {
				return fmt.Errorf("model %q has no source to fetch", name)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), installTimeout)
			defer cancel()
			if err := install.Fetch(ctx, model.Source, model.Path, model.Size, model.SHA256); err != nil {
				return err
			}
			st := install.ModelState{
				Source:      model.Source,
				Path:        model.Path,
				Size:        model.Size,
				SHA256:      model.SHA256,
				InstalledAt: time.Now().UTC(),
			}
			if err := install.Record(install.Path(opts.configPath), name, st); err != nil {
				return err
			}
			if !opts.quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "installed %s\n", name)
			}
			return nil
		},
	}
}
