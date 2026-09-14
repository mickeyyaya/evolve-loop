package ipcenv

import "strings"

// namespace is the prefix every IPC-protocol key lives under. Scrub relies on
// it: a child that must see CI's default configuration drops the whole
// namespace instead of enumerating keys, so a key added tomorrow is scrubbed
// the day it is added (TestScrub_CoversEveryIPCKey pins the rule).
const namespace = "EVOLVE_"

// Scrub returns environ without every EVOLVE_-namespaced entry, order kept.
//
// It is the environment for any `go test` / `go run` a cycle spawns to JUDGE
// the repository — the ship repo-contract gate, the build floor, the
// phase-bindings self-check. A lane exports its runtime state process-wide
// (EVOLVE_FLEET=1, EVOLVE_CYCLE_STATE_FILE=<its own run dir>, EVOLVE_TMUX_SOCKET,
// …), and a child inheriting it flips env-sensitive tests into false REDs that
// pass in CI and in a clean shell: guards' "outside a cycle" tests, cycle-reset
// lease fencing, ship's fleet-off goldens (lane 1677, 2026-09-14). Everything
// outside the namespace (PATH, HOME, GOFLAGS, GOTOOLCHAIN, …) is kept so the
// toolchain still works; a test that needs an EVOLVE_ key sets it with t.Setenv
// inside the child, after the scrub, unaffected. Only the KEY is inspected —
// FOO=EVOLVE_BAR survives.
func Scrub(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		if strings.HasPrefix(kv, namespace) {
			continue
		}
		out = append(out, kv)
	}
	return out
}
