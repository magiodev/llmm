package app

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/magiodev/llmm/internal/config"
)

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiCyan   = "\x1b[36m"
)

// isTerminal reports whether the process's standard output is a character
// device (terminal). It is a package-level function variable so tests can
// inject both outcomes.
var isTerminal = func() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// useColor reports whether ANSI colors should be emitted: colors are enabled
// only when stdout is a terminal and NO_COLOR is not set.
func useColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return isTerminal()
}

func stateColor(state string) string {
	switch state {
	case "active", "running":
		return ansiGreen
	case "inactive", "exited", "failed":
		return ansiRed
	case "error":
		return ansiYellow
	default:
		return ""
	}
}

func portFromEndpoint(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil || u.Port() == "" {
		return "-"
	}
	return u.Port()
}

type statusEntry struct {
	Name     string
	Type     string
	State    string
	Port     string
	Endpoint string
	Models   string
}

func buildStatusEntries(cfg *config.Config, entries []runtimeStatus) []statusEntry {
	modelsByRuntime := map[string][]string{}
	contextByModel := map[string]int{}
	for name, model := range cfg.Models {
		modelsByRuntime[model.Runtime] = append(modelsByRuntime[model.Runtime], name)
		contextByModel[name] = model.Context
	}
	out := make([]statusEntry, 0, len(entries))
	for _, entry := range entries {
		se := statusEntry{Name: entry.Name, Type: entry.Type, State: entry.State}
		se.Endpoint = cfg.Runtimes[entry.Name].Endpoint
		se.Port = portFromEndpoint(se.Endpoint)
		models := modelsByRuntime[entry.Name]
		sort.Strings(models)
		labels := make([]string, 0, len(models))
		for _, name := range models {
			label := name
			if ctx := contextByModel[name]; ctx > 0 {
				label = fmt.Sprintf("%s (%dk)", name, ctx/1000)
			}
			labels = append(labels, label)
		}
		se.Models = strings.Join(labels, ", ")
		if se.Models == "" {
			se.Models = "-"
		}
		out = append(out, se)
	}
	return out
}

// renderStatus renders a fixed-width table with RUNTIME, TYPE, STATE, PORT,
// ENDPOINT, and MODELS (CTX) columns. When color is true, headers are bold,
// types are cyan, and states are green (active/running), red (inactive), or
// yellow (error).
func renderStatus(cfg *config.Config, entries []runtimeStatus, color bool) string {
	rows := buildStatusEntries(cfg, entries)
	headers := []string{"RUNTIME", "TYPE", "STATE", "PORT", "ENDPOINT", "MODELS (CTX)"}
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}
	values := make([][]string, len(rows))
	for r, row := range rows {
		values[r] = []string{row.Name, row.Type, row.State, row.Port, row.Endpoint, row.Models}
		for i, value := range values[r] {
			if len(value) > widths[i] {
				widths[i] = len(value)
			}
		}
	}

	var buf bytes.Buffer
	for i, header := range headers {
		if i > 0 {
			buf.WriteString("  ")
		}
		if color {
			buf.WriteString(ansiBold)
		}
		fmt.Fprintf(&buf, "%-*s", widths[i], header)
		if color {
			buf.WriteString(ansiReset)
		}
	}
	buf.WriteByte('\n')
	for r, row := range rows {
		for i := range headers {
			if i > 0 {
				buf.WriteString("  ")
			}
			if color {
				switch i {
				case 1:
					buf.WriteString(ansiCyan)
				case 2:
					if c := stateColor(row.State); c != "" {
						buf.WriteString(c)
					}
				}
			}
			fmt.Fprintf(&buf, "%-*s", widths[i], values[r][i])
			if color && (i == 1 || i == 2) {
				buf.WriteString(ansiReset)
			}
		}
		buf.WriteByte('\n')
	}
	return buf.String()
}
