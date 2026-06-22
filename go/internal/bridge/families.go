package bridge

// families.go enumerates the interactive LLM CLI families the usage probe can
// target: registered *-tmux drivers whose binary is installed. The probe needs
// the family name (the manifest binary, e.g. "claude") and must skip
// uninstalled CLIs — probing one would burn a boot timeout every cycle.

import (
	"os/exec"
	"sort"
	"strings"
)

// InteractiveFamilies returns the installed interactive families (sorted,
// deduped) — the production enumeration over the driver registry, each driver's
// manifest binary, and exec.LookPath for installation.
func InteractiveFamilies() []string {
	return interactiveFamiliesFrom(DriverNames(), loadManifestRaw, func(bin string) bool {
		_, err := exec.LookPath(bin)
		return err == nil
	})
}

// interactiveFamiliesFrom is the testable core: from driver names, keep the
// *-tmux drivers whose manifest loads and whose binary `installed` reports
// present, mapping each to its family (binary) name, deduped + sorted.
func interactiveFamiliesFrom(names []string, manifest func(string) (Manifest, error), installed func(bin string) bool) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		if !strings.HasSuffix(name, "-tmux") {
			continue
		}
		m, err := manifest(name)
		if err != nil || m.Binary == "" {
			continue
		}
		if !installed(m.Binary) {
			continue
		}
		if !seen[m.Binary] {
			seen[m.Binary] = true
			out = append(out, m.Binary)
		}
	}
	sort.Strings(out)
	return out
}
