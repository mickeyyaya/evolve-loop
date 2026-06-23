package looppreflight

import (
	"sort"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/profiles"
)

// profileCLIs returns the non-empty CLI driver names a profile can run against:
// its primary CLI plus every cli_fallback entry, in declaration order.
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

// distinctDrivers returns the sorted, de-duplicated set of driver names across
// all loadable profiles (primary CLI + fallbacks). Profiles that fail to load
// are skipped here — checkPipelineStructure is the check that reports load
// failures; the CLI/boot checks only act on what resolves.
func distinctDrivers(list func() ([]string, error), get func(string) (profiles.Profile, error)) []string {
	seen := map[string]struct{}{}
	names, _ := list()
	for _, n := range names {
		prof, err := get(n)
		if err != nil {
			continue
		}
		for _, cli := range profileCLIs(prof) {
			seen[cli] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// sandboxWanted reports whether any loadable profile enables sandboxing. A
// profile that sets sandbox.enabled IS a write-phase that wants the bridge to
// sandbox it — so this doubles as "is there a write phase that needs sandbox".
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

// driverBinary maps a driver name to the underlying CLI executable that must be
// on PATH (claude-tmux→claude, codex-tmux→codex, agy-tmux→agy,
// ollama-tmux→ollama, claude-p→claude). The binary is the segment before the
// first dash, matching every driver the bridge registers today.
func driverBinary(driver string) string {
	if i := strings.IndexByte(driver, '-'); i > 0 {
		return driver[:i]
	}
	return driver
}
