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

// defaultLiveProbe is the one canary probe the loop and the campaign runner
// share, so its semantics and bound cannot drift between them.
func defaultLiveProbe(ctx context.Context, projectRoot string, stderr io.Writer) liveProbe {
	return func(driver string) (int, string, string) {
		probeCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
		defer cancel()
		return bridge.LiveSmokeTest(probeCtx, driver,
			&bridge.Config{ProjectRoot: projectRoot}, bridge.Deps{Stderr: stderr})
	}
}

// liveProbe returns the bridge exit code, the escalation pattern of a
// classified wall (else empty), and the scrollback that carries its reset hint.
type liveProbe func(driver string) (rc int, pattern, scrollback string)

// runCLIHealthCanary probes each expired bench once before a cycle: recovered
// clears it, walled again re-benches it, and any other failure clears it,
// because non-wall classes belong to the normal dispatch machinery.
func runCLIHealthCanary(ctx context.Context, projectRoot string, env map[string]string, probe liveProbe, stderr io.Writer) {
	if !envchain.BoolValue(envchain.Resolve("EVOLVE_CLI_HEALTH", env, "", "1"), true) {
		return
	}
	// A cancelled probe reports "not a wall", which would clear a bench that is
	// still walled, so cancellation touches no bench.
	if ctx.Err() != nil {
		fmt.Fprintf(stderr, "[loop] cli-health canary: cancelled (%v) — benches untouched\n", ctx.Err())
		return
	}
	store := clihealth.NewStore(projectRoot, nil)
	for family := range store.Expired() {
		driver := family + "-tmux"
		rc, pattern, scrollback := probe(driver)
		if ctx.Err() != nil {
			fmt.Fprintf(stderr, "[loop] cli-health canary: cancelled (%v) during the %s probe — bench untouched\n", ctx.Err(), family)
			return
		}
		switch {
		case rc == 0:
			_ = store.Clear(family)
			fmt.Fprintf(stderr, "[loop] cli-health canary: %s recovered (probe OK) — bench cleared\n", family)
		case clihealth.Benchable(pattern):
			entry, _ := store.BenchWall(family, pattern, scrollback)
			fmt.Fprintf(stderr, "[loop] cli-health canary: %s still walled (pattern=%s) — re-benched until %s (strikes=%d)\n",
				family, pattern, entry.BenchedUntil.Format(time.RFC3339), entry.Strikes)
			if entry.OperatorAction != "" {
				fmt.Fprintf(stderr, "[loop] cli-health canary: %s\n", entry.OperatorAction)
			}
		default:
			_ = store.Clear(family)
			fmt.Fprintf(stderr, "[loop] cli-health canary: %s probe failed rc=%d (not a wall) — bench cleared; normal dispatch machinery owns this failure class\n",
				family, rc)
		}
	}
}
