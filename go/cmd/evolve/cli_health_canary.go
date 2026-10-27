package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func defaultLiveProbe(ctx context.Context, projectRoot string, deps bridge.Deps) liveProbe {
	return liveProbeWith(ctx, projectRoot, "", deps)
}

func liveProbeWith(ctx context.Context, projectRoot, model string, deps bridge.Deps) liveProbe {
	return func(driver string) (rc int, pattern, scrollback string) {
		bridge.BootProbe{Driver: driver, Log: deps.Stderr}.Retry(ctx, func() (int, string) {
			probeCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
			defer cancel()
			probe := bridge.LiveSmokeTest(probeCtx, driver, &bridge.Config{ProjectRoot: projectRoot, Model: model}, deps)
			rc, pattern, scrollback = probe.RC, probe.Pattern, probe.Scrollback
			return rc, pattern
		})
		return rc, pattern, scrollback
	}
}

type liveProbe func(driver string) (rc int, pattern, scrollback string)

func runCLIHealthCanary(ctx context.Context, projectRoot string, probe liveProbe, stderr io.Writer) {
	if !bridgechain.CLIHealthOn(projectRoot) {
		return
	}
	if ctx.Err() != nil {
		fmt.Fprintf(stderr, "[loop] cli-health canary: cancelled (%v) — benches untouched\n", ctx.Err())
		return
	}
	store := clihealth.NewStore(projectRoot, nil)
	for family := range store.Expired() {
		resolved, err := bridge.Resolve(driverSet, family)
		if err != nil {
			fmt.Fprintf(stderr, "[loop] cli-health canary: %v — bench untouched\n", err)
			continue
		}
		rc, pattern, scrollback := probe(resolved.Driver)
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
		case rc == bridge.ExitModelMismatch:
			fmt.Fprintf(stderr, "[loop] cli-health canary: %s booted a model outside its family (exit %d) — bench kept until a probe reaches the family's own model\n", family, rc)
		default:
			_ = store.Clear(family)
			fmt.Fprintf(stderr, "[loop] cli-health canary: %s probe failed rc=%d (not a wall) — bench cleared; normal dispatch machinery owns this failure class\n",
				family, rc)
		}
	}
}
