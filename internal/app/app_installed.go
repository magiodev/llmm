package app

import (
	"fmt"
	"sort"

	"github.com/magiodev/llmm/internal/config"
	"github.com/magiodev/llmm/internal/install"
	"github.com/spf13/cobra"
)

// installedCommand reports machine-managed install state: which declared
// models are recorded as installed versus missing, plus a summary count.
func installedCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "installed",
		Short: "Report installed-artifact state from machine-managed install data",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			st, err := install.Load(install.Path(opts.configPath))
			if err != nil {
				return err
			}
			names := make([]string, 0, len(cfg.Models))
			for name := range cfg.Models {
				names = append(names, name)
			}
			sort.Strings(names)
			installed := 0
			for _, name := range names {
				if ms, ok := st.Models[name]; ok {
					installed++
					fmt.Fprintf(cmd.OutOrStdout(), "%s\tinstalled\t%s\n", name, ms.Path)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\tmissing\n", name)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "total %d (%d installed, %d missing)\n", len(names), installed, len(names)-installed)
			return nil
		},
	}
}
