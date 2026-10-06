package looppreflight

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func profileCLIs(p profiles.Profile) []string {
	out := make([]string, 0, 1+len(p.CLIFallback))
	if p.CLI != "" {
		out = append(out, p.CLI)
	}
	for _, f := range p.CLIFallback {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// sandboxWanted reports whether any loadable profile enables sandboxing, i.e. a write phase needs it.
func sandboxWanted(list func() ([]string, error), get func(string) (profiles.Profile, error)) bool {
	names, _ := list()
	for _, n := range names {
		prof, err := get(n)
		if err != nil {
			continue
		}
		if prof.Sandbox != nil && prof.Sandbox.Enabled {
			return true
		}
	}
	return false
}

// driverBinary maps a driver to its executable, the segment before the first dash (claude-tmux → claude).
func driverBinary(driver string) string {
	if i := strings.IndexByte(driver, '-'); i > 0 {
		return driver[:i]
	}
	return driver
}
