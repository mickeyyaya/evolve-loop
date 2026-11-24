package bridge

import (
	"os/exec"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

func InteractiveFamilies() []string {
	return interactiveFamiliesFrom(DriverNames(), loadManifestRaw, func(bin string) bool {
		_, err := exec.LookPath(bin)
		return err == nil
	})
}

func interactiveFamiliesFrom(names []string, manifest func(string) (Manifest, error), installed func(bin string) bool) []string {
	return familyBinariesFrom(names, manifest, func(name, bin string) bool {
		return strings.HasSuffix(name, "-tmux") && installed(bin)
	})
}

func familyBinariesFrom(names []string, manifest func(string) (Manifest, error), keep func(name, bin string) bool) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		m, err := manifest(name)
		if err != nil || m.Binary == "" || !keep(name, m.Binary) {
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

func FamilyBinaries() []string {
	return familyBinariesFrom(DriverNames(), loadManifestRaw, func(string, string) bool { return true })
}

func TagKillable() proctree.Proof {
	return proctree.ProgramIn(append(proctree.Launchers(), FamilyBinaries()...))
}
