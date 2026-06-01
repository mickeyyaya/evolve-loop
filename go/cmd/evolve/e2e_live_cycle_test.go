//go:build e2e

// Tier 1 — LIVE per-CLI happy cycle. Each available CLI runs a full cycle
// against its cheapest real model, through both the headless and tmux drivers.
// Assertions are STRUCTURAL: the real CLIs must drive the pipeline far enough to
// produce the core phase artifacts (scout → build → audit in the ledger). A
// PASS-and-ship vs. a legitimate audit-block both count as integration success;
// only a crash/contract break before audit (non-transient) fails the test.
// Gate: EVOLVE_E2E_LIVE=1.
package main

import (
	"testing"
	"time"
)

// corePhaseRoles are the phases that prove the real CLI drove the pipeline.
// (intent is gated off; ship/retro depend on the synthetic task's verdict.)
var corePhaseRoles = []string{"scout", "build", "audit"}

func TestE2ELiveCycleHeadless(t *testing.T) {
	liveGate(t, "EVOLVE_E2E_LIVE")
	repoRoot := mustRepoRoot(t)
	evolveBin := buildBinary(t, t.TempDir(), "evolve", "./cmd/evolve", repoRoot)
	for _, cli := range liveHeadlessCLIs {
		cli := cli
		t.Run(cli.Driver, func(t *testing.T) {
			runLiveCycleTier1(t, repoRoot, evolveBin, cli,
				envDurationSeconds("EVOLVE_E2E_LIVE_TIMEOUT_S", 10*time.Minute))
		})
	}
}

func TestE2ELiveCycleTmux(t *testing.T) {
	liveGate(t, "EVOLVE_E2E_LIVE")
	requireTmuxForLive(t)
	repoRoot := mustRepoRoot(t)
	evolveBin := buildBinary(t, t.TempDir(), "evolve", "./cmd/evolve", repoRoot)
	for _, cli := range liveTmuxCLIs {
		cli := cli
		t.Run(cli.Driver, func(t *testing.T) {
			runLiveCycleTier1(t, repoRoot, evolveBin, cli,
				envDurationSeconds("EVOLVE_E2E_LIVE_TMUX_TIMEOUT_S", 15*time.Minute))
		})
	}
}

func runLiveCycleTier1(t *testing.T, repoRoot, evolveBin string, cli liveCLI, timeout time.Duration) {
	t.Helper()
	if ok, why := liveCLIAvailable(cli); !ok {
		t.Skip(why)
	}
	res := runLiveCycle(t, liveCycleCfg{
		EvolveBin: evolveBin,
		RepoRoot:  repoRoot,
		Driver:    cli.Driver,
		Tier:      cli.CheapTier,
		GoalHash:  "live-" + cli.Driver,
		Timeout:   timeout,
		BudgetUSD: 1.00,
	})
	t.Logf("[live-cycle] %s shipped=%v cost=$%.4f roles=%v", cli.Driver, res.Shipped, res.Cost, ledgerRoles(res.Entries))

	if res.TransientExhausted {
		t.Skipf("%s live cycle quarantined after transient retries:\n%s", cli.Driver, lastN(res.Out, 800))
	}

	// Structural success = the real CLI drove the pipeline to the core phases.
	reachedCore := true
	for _, role := range corePhaseRoles {
		if !ledgerHasRole(res.Entries, role) {
			reachedCore = false
			break
		}
	}
	if reachedCore {
		// Integration proven. Final verdict (ship vs block) depends on the
		// synthetic task and is not a CLI-integration concern — just report it.
		return
	}
	// Did not reach audit. If the error looks transient, quarantine; else it is
	// a real contract break — capture for triage and fail.
	if isTransient(res.Out, res.Err) {
		t.Skipf("%s live cycle: provider failure before reaching audit (quarantined):\nerr=%v\n%s", cli.Driver, res.Err, lastN(res.Out, 800))
	}
	captureLiveFailure(t, repoRoot, res.ProjRoot, "live-"+cli.Driver)
	t.Errorf("%s live cycle did NOT reach the core phases %v (contract break); roles=%v err=%v\n%s",
		cli.Driver, corePhaseRoles, ledgerRoles(res.Entries), res.Err, lastN(res.Out, 1500))
}
