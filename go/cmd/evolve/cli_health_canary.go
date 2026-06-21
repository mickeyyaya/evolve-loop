package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
)

// defaultLiveProbe is the production canary probe: a bounded LiveSmokeTest of the
// driver. Shared by the loop and the campaign runner so the probe semantics (and
// its 4-minute bound) cannot drift between them.
func defaultLiveProbe(projectRoot string, stderr io.Writer) liveProbe {
	return func(driver string) (int, string, string) {
		probeCtx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		return bridge.LiveSmokeTest(probeCtx, driver,
			&bridge.Config{ProjectRoot: projectRoot}, bridge.Deps{Stderr: stderr})
	}
}

// liveProbe is the canary's probe seam: production passes a closure over
// bridge.LiveSmokeTest; tests inject a fake. Returns the bridge exit code,
// the escalation pattern name (empty unless the launch died on a classified
// wall), and the captured scrollback (carries the wall's reset hint).
type liveProbe func(driver string) (rc int, pattern, scrollback string)

// runCLIHealthCanary gives each EXPIRED bench one cheap live probe before a
// cycle starts (the per-cycle health seam cmd_loop never had): probe OK →
// the family is re-promoted (Clear) and normal dispatch resumes; walled
// again → re-benched with strikes+1 (doubled cooldown, or the wall's own
// reset hint); any other failure → cleared anyway — non-wall failure classes
// have their own machinery (capability probe, fallback chain), and looping
// the canary on them would re-probe every cycle forever. ACTIVE benches are
// untouched. Disabled by EVOLVE_CLI_HEALTH=0.
func runCLIHealthCanary(projectRoot string, env map[string]string, probe liveProbe, stderr io.Writer) {
	if !envchain.BoolValue(envchain.Resolve("EVOLVE_CLI_HEALTH", env, "", "1"), true) {
		return
	}
	store := clihealth.NewStore(projectRoot, nil)
	for family := range store.Expired() {
		driver := family + "-tmux"
		rc, pattern, scrollback := probe(driver)
		switch {
		case rc == 0:
			_ = store.Clear(family)
			fmt.Fprintf(stderr, "[loop] cli-health canary: %s recovered (probe OK) — bench cleared\n", family)
		case clihealth.Benchable(pattern):
			entry, _ := store.BenchWall(family, pattern, scrollback)
			fmt.Fprintf(stderr, "[loop] cli-health canary: %s still walled (pattern=%s) — re-benched until %s (strikes=%d)\n",
				family, pattern, entry.BenchedUntil.Format(time.RFC3339), entry.Strikes)
		default:
			_ = store.Clear(family)
			fmt.Fprintf(stderr, "[loop] cli-health canary: %s probe failed rc=%d (not a wall) — bench cleared; normal dispatch machinery owns this failure class\n",
				family, rc)
		}
	}
}
