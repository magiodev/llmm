package app

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/magiodev/llmm/internal/config"
	"github.com/spf13/cobra"
)

// verifyCommand reports declared-artifact integrity (path presence, declared
// size, and SHA-256 when declared) as a standalone check, independent of
// doctor's config/runtime/executable checks. Output shape reuses doctor's
// contract-aligned check/result objects.
func verifyCommand(opts *options) *cobra.Command {
	var format string
	command := &cobra.Command{
		Use:   "verify",
		Short: "Verify declared artifact integrity (size + SHA-256)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			var failures []string
			var checks []doctorCheck
			check := func(ok bool, label, detail string) {
				checks = append(checks, doctorCheck{OK: ok, Label: label, Detail: detail})
				if !ok {
					failures = append(failures, label+": "+detail)
				}
				if format != "json" {
					state := "ok"
					if !ok {
						state = "fail"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%-5s %-24s %s\n", state, label, detail)
				}
			}
			modelNames := make([]string, 0, len(cfg.Models))
			for name := range cfg.Models {
				modelNames = append(modelNames, name)
			}
			sort.Strings(modelNames)
			for _, name := range modelNames {
				model := cfg.Models[name]
				checkArtifact(cmd, check, "model "+name, model.Path, model.Size, model.SHA256)
				for i, artifact := range model.Artifacts {
					checkArtifact(cmd, check, fmt.Sprintf("model %s artifact %d", name, i), artifact.Path, artifact.Size, artifact.SHA256)
				}
			}
			if format == "json" {
				if err := writeJSON(cmd, doctorResult{Success: len(failures) == 0, Checks: checks}); err != nil {
					return err
				}
			}
			if len(failures) > 0 {
				return fmt.Errorf("verify found %d problem(s): %s", len(failures), strings.Join(failures, "; "))
			}
			return nil
		},
	}
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

// checkArtifact verifies one declared artifact: the path must exist as a
// regular file, match the declared size when non-zero, and match the declared
// SHA-256 when non-empty. ok is the aggregate success of every sub-check.
func checkArtifact(cmd *cobra.Command, check func(bool, string, string), label, path string, size int64, sha string) {
	info, statErr := os.Stat(path)
	ok := statErr == nil && info.Mode().IsRegular()
	detail := path
	if ok && size > 0 {
		ok = info.Size() == size
		detail = fmt.Sprintf("%s (%d bytes)", path, info.Size())
	}
	check(ok, label, detail)
	if ok && sha != "" {
		hashCtx, cancel := context.WithTimeout(cmd.Context(), deepHashTimeout)
		sum, hashErr := fileSHA256(hashCtx, path)
		cancel()
		check(hashErr == nil && sum == strings.ToLower(sha), "sha256 "+label, sum)
	}
}
