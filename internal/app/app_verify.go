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
				emitCheck(cmd, format, &checks, &failures, ok, label, detail)
			}
			modelNames := make([]string, 0, len(cfg.Models))
			for name := range cfg.Models {
				modelNames = append(modelNames, name)
			}
			sort.Strings(modelNames)
			for _, name := range modelNames {
				model := cfg.Models[name]
				label := "model " + name
				checkArtifact(cmd, check, label, "sha256 "+label, model.Path, model.Size, model.SHA256, true)
				for i, artifact := range model.Artifacts {
					label := fmt.Sprintf("model %s artifact %d", name, i)
					checkArtifact(cmd, check, label, "sha256 "+label, artifact.Path, artifact.Size, artifact.SHA256, true)
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

// emitCheck records one check into checks/failures and prints it unless the
// output format is json. Shared by doctor and verify.
func emitCheck(cmd *cobra.Command, format string, checks *[]doctorCheck, failures *[]string, ok bool, label, detail string) {
	*checks = append(*checks, doctorCheck{OK: ok, Label: label, Detail: detail})
	if !ok {
		*failures = append(*failures, label+": "+detail)
	}
	if format != "json" {
		state := "ok"
		if !ok {
			state = "fail"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-5s %-24s %s\n", state, label, detail)
	}
}

// checkArtifact verifies one declared artifact: the path must exist as a
// regular file, match the declared size when non-zero, and match the declared
// SHA-256 when non-empty and hash is requested. label and shaLabel are the
// emit labels for the path/size check and the digest check respectively.
func checkArtifact(cmd *cobra.Command, emit func(bool, string, string), label, shaLabel, path string, size int64, sha string, hash bool) {
	info, statErr := os.Stat(path)
	ok := statErr == nil && info.Mode().IsRegular()
	detail := path
	if ok && size > 0 {
		ok = info.Size() == size
		detail = fmt.Sprintf("%s (%d bytes)", path, info.Size())
	}
	emit(ok, label, detail)
	if hash && ok && sha != "" {
		hashCtx, cancel := context.WithTimeout(cmd.Context(), deepHashTimeout)
		sum, hashErr := fileSHA256(hashCtx, path)
		cancel()
		emit(hashErr == nil && sum == strings.ToLower(sha), shaLabel, sum)
	}
}
