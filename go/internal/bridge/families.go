package bridge

import (
	"os/exec"
	"sort"
	"strings"
)

func InteractiveFamilies() []string {
	return interactiveFamiliesFrom(DriverNames(), loadManifestRaw, func(bin string) bool {
		_, err := exec.LookPath(bin)
		return err == nil
	})
}

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
