package ciparitygate

import "strings"

type envExclusiveEntry struct {
	pkg      string
	why      string
	backstop string
}

const envExclusiveNoCIMarker = "NOT covered by CI"

var tierEnvExclusive = []envExclusiveEntry{{
	pkg:      "internal/bridge",
	why:      "requireTmux tests boot real tmux sessions; under a live wave those boots time out (13 offenders on cycle-1543, all exit=80; the same tests 7/7 PASS in 17.2s on a quiet host)",
	backstop: "internal/bridge's requireTmux tier is " + envExclusiveNoCIMarker + " (no tmux on runners — the #483 finding); its backstop is a quiet-host run (loop-boot preflight, or `go test -tags integration` with no wave active)",
}}

func envExclusiveEntryFor(p string) (envExclusiveEntry, bool) {
	p = strings.TrimSuffix(strings.TrimPrefix(p, "./"), "/...")
	p = strings.TrimSuffix(p, "/")
	for _, e := range tierEnvExclusive {
		if p == e.pkg || strings.HasSuffix(p, "/"+e.pkg) {
			return e, true
		}
	}
	return envExclusiveEntry{}, false
}

func envExclusivePkg(p string) bool {
	_, ok := envExclusiveEntryFor(p)
	return ok
}

func envExclusiveBackstopNote(pkgs []string) string {
	seen := map[string]bool{}
	var parts []string
	for _, p := range pkgs {
		if e, ok := envExclusiveEntryFor(p); ok && !seen[e.pkg] {
			seen[e.pkg] = true
			parts = append(parts, e.why+" — "+e.backstop)
		}
	}
	return strings.Join(parts, "; ")
}
