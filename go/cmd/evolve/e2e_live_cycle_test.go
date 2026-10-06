//go:build e2e

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
	})
	t.Logf("[live-cycle] %s shipped=%v cost=$%.4f roles=%v", cli.Driver, res.Shipped, res.Cost, ledgerRoles(res.Entries))

	if res.TransientExhausted {
		t.Skipf("%s live cycle quarantined after transient retries:\n%s", cli.Driver, lastN(res.Out, 800))
	}

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
	if isTransient(res.Out, res.Err) {
		t.Skipf("%s live cycle: provider failure before reaching audit (quarantined):\nerr=%v\n%s", cli.Driver, res.Err, lastN(res.Out, 800))
	}
	captureLiveFailure(t, res.ProjRoot, "live-"+cli.Driver)
	t.Errorf("%s live cycle did NOT reach the core phases %v (contract break); roles=%v err=%v\n%s",
		cli.Driver, corePhaseRoles, ledgerRoles(res.Entries), res.Err, lastN(res.Out, 1500))
}
