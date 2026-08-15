package app

import (
	"os"

	"github.com/magiodev/llmm/internal/config"
)

// osStat is a package-level variable so tests can inject failing stat calls.
var osStat = os.Stat

// isRegular reports whether path exists and is a regular file.
func isRegular(path string) bool {
	info, err := osStat(path)
	return err == nil && info.Mode().IsRegular()
}

// preflightArtifacts returns the names of models bound to runtimeName whose
// declared primary path or any declared artifact is not present as a regular
// file on disk. Starting a runtime is gated on this so a model with a missing
// artifact (including a multi-file layout) is never served.
func preflightArtifacts(cfg *config.Config, runtimeName string) []string {
	var missing []string
	for name, model := range cfg.Models {
		if model.Runtime != runtimeName {
			continue
		}
		if !isRegular(model.Path) {
			missing = append(missing, name)
			continue
		}
		for _, artifact := range model.Artifacts {
			if !isRegular(artifact.Path) {
				missing = append(missing, name)
				break
			}
		}
	}
	return missing
}
