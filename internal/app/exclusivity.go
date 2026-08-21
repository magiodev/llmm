package app

import (
	"context"
	"fmt"
	"sort"

	"github.com/magiodev/llmm/internal/config"
	runtimeops "github.com/magiodev/llmm/internal/runtime"
)

// modelBackedRuntimes returns the set of runtime names that back at least one
// model. Starting or restarting one of these implies the others should be
// stopped, because model runtimes typically cannot co-reside on a GPU.
func modelBackedRuntimes(cfg *config.Config) map[string]bool {
	backed := map[string]bool{}
	for _, model := range cfg.Models {
		backed[model.Runtime] = true
	}
	return backed
}

// stopOtherActiveModelRuntimes stops every other model-backed runtime that is
// currently active, so starting target is exclusive. It returns the names of
// the runtimes it stopped. Runtimes that cannot be queried are skipped;
// stopping an active runtime returns an error with the names stopped so far.
func stopOtherActiveModelRuntimes(cmd interface{ Context() context.Context }, cfg *config.Config, target string) ([]string, error) {
	backed := modelBackedRuntimes(cfg)
	if !backed[target] {
		return nil, nil
	}
	names := make([]string, 0, len(backed))
	for name := range backed {
		if name != target {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var stopped []string
	for _, name := range names {
		rt := cfg.Runtimes[name]

		ctx, cancel := context.WithTimeout(cmd.Context(), supervisorTimeout)
		state, err := runtimeops.Status(ctx, rt)
		cancel()
		if err != nil {
			continue
		}
		if state != "active" && state != "running" {
			continue
		}

		ctx, cancel = context.WithTimeout(cmd.Context(), supervisorTimeout)
		err = runtimeops.Action(ctx, rt, "stop")
		cancel()
		if err != nil {
			return stopped, fmt.Errorf("stop %s: %w", name, err)
		}
		stopped = append(stopped, name)
	}
	return stopped, nil
}
