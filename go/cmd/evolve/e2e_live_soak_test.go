//go:build e2e

// Tier 3 — LIVE cross-family adversarial soak. Runs a full cycle with the
// builder and auditor on DIFFERENT model families (e.g. claude builder × codex
// auditor) so the adversarial-audit integrity the offline tests can only assert
// structurally is exercised against real models. Output is non-deterministic,
// so this tier is OBSERVATIONAL: it asserts only that the cycle reaches a valid
// terminal state, logs whether the auditor PASS/FAIL'd, and surfaces the verdict
// for human review / catch-rate tracking across runs. Gate: EVOLVE_E2E_LIVE_SOAK=1.
package main

import (
	"strings"
	"testing"
	"time"
)

// crossFamilyPairs are builder/auditor combos on different families. Each entry
// is {builderDriver, auditorDriver}; both must be available or the pair skips.
var crossFamilyPairs = [][2]liveCLI{
	{{Driver: "claude-tmux", Binary: "claude", Family: "anthropic"}, {Driver: "codex-tmux", Binary: "codex", Family: "openai"}},
	{{Driver: "agy-tmux", Binary: "agy", Family: "google"}, {Driver: "codex-tmux", Binary: "codex", Family: "openai"}},
}

func TestE2ELiveCrossFamilySoak(t *testing.T) {
	liveGate(t, "EVOLVE_E2E_LIVE_SOAK")
	requireTmuxForLive(t)
	repoRoot := mustRepoRoot(t)
	evolveBin := buildBinary(t, t.TempDir(), "evolve", "./cmd/evolve", repoRoot)

	for _, pair := range crossFamilyPairs {
		builder, auditor := pair[0], pair[1]
		if builder.Family == auditor.Family {
			t.Fatalf("misconfigured soak pair: %s and %s share family %s", builder.Driver, auditor.Driver, builder.Family)
		}
		name := builder.Driver + "_x_" + auditor.Driver
		t.Run(name, func(t *testing.T) {
			if ok, why := liveCLIAvailable(builder); !ok {
				t.Skip("builder " + why)
			}
			if ok, why := liveCLIAvailable(auditor); !ok {
				t.Skip("auditor " + why)
			}
			// Default every phase to the builder's CLI, then pin the auditor to a
			// different-family CLI via the per-agent override (cli_chain precedence:
			// EVOLVE_<AGENT>_CLI > EVOLVE_CLI > profile).
			res := runLiveCycle(t, liveCycleCfg{
				EvolveBin: evolveBin,
				RepoRoot:  repoRoot,
				Driver:    builder.Driver,
				Tier:      "fast",
				GoalHash:  "soak-" + name,
				ExtraEnv:  []string{"EVOLVE_AUDITOR_CLI=" + auditor.Driver},
				Timeout:   envDurationSeconds("EVOLVE_E2E_LIVE_TMUX_TIMEOUT_S", 15*time.Minute),
				BudgetUSD: 2.00,
			})

			if res.TransientExhausted {
				t.Skipf("%s soak quarantined after transient retries:\n%s", name, lastN(res.Out, 800))
			}

			// Observational: derive the auditor verdict from the cycle output and
			// log the catch/miss. We do NOT hard-assert a specific verdict (real
			// models vary); we DO require the cycle reached audit (integration).
			if !ledgerHasRole(res.Entries, "audit") {
				if isTransient(res.Out, res.Err) {
					t.Skipf("%s: provider failure before audit (quarantined)", name)
				}
				captureLiveFailure(t, repoRoot, res.ProjRoot, "soak-"+name)
				t.Errorf("%s cross-family cycle never reached audit; roles=%v err=%v\n%s",
					name, ledgerRoles(res.Entries), res.Err, lastN(res.Out, 1200))
				return
			}
			verdict := "unknown"
			switch {
			case strings.Contains(res.Out, "FAIL"):
				verdict = "FAIL (auditor flagged the builder)"
			case res.Shipped:
				verdict = "PASS+shipped"
			}
			t.Logf("[live-soak] %s (builder=%s auditor=%s) → %s  cost=$%.4f  [catch-rate signal; not a gate]",
				name, builder.Family, auditor.Family, verdict, res.Cost)
		})
	}
}
