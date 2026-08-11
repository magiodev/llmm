package app

import (
	"os"

	"github.com/magiodev/llmm/internal/config"
)

// osStat is a package-level variable so tests can inject failing stat calls.
var osStat = os.Stat

// preflightArtifacts returns the names of models bound to runtimeName whose
// declared artifact is not present as a regular file on disk. Starting a
// runtime is gated on this so a model with a missing artifact is never served.
func preflightArtifacts(cfg *config.Config, runtimeName string) []string {
	var missing []string
	for name, model := range cfg.Models {
		if model.Runtime != runtimeName {
			continue
		}
		info, err := osStat(model.Path)
		if err != nil || !info.Mode().IsRegular() {
			missing = append(missing, name)
		}
	}
	return missing
}
