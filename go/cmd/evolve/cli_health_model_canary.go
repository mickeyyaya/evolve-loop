package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

type modelLiveProbe func(driver, model string) (rc int, pattern, scrollback string)

var preWaveModelLiveProbeFn = func(ctx context.Context, projectRoot string, deps bridge.Deps) modelLiveProbe {
	return func(driver, model string) (int, string, string) {
		return liveProbeWith(ctx, projectRoot, model, deps)(driver)
	}
}

func runCLIHealthModelCanary(ctx context.Context, projectRoot string, probe modelLiveProbe, stderr io.Writer) {
	if !bridgechain.CLIHealthOn(projectRoot) {
		return
	}
	if ctx.Err() != nil {
		fmt.Fprintf(stderr, "[loop] cli-health model canary: cancelled (%v) — model benches untouched\n", ctx.Err())
		return
	}
	store := clihealth.NewStore(projectRoot, nil)
	expired := store.ExpiredModels()
	keys := make([]string, 0, len(expired))
	for key := range expired {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !canaryModelBench(ctx, store, expired[key], probe, stderr) {
			return
		}
	}
}

func canaryModelBench(ctx context.Context, store *clihealth.Store, e clihealth.Entry, probe modelLiveProbe, stderr io.Writer) bool {
	target := e.Family + "/" + e.Model
	resolved, err := bridge.Resolve(driverSet, e.Family)
	if err != nil {
		fmt.Fprintf(stderr, "[loop] cli-health model canary: %v — bench untouched\n", err)
		return true
	}
	model, mapped := bridge.UsageModelFor(resolved.Driver, e.Model)
	if !mapped {
		_ = store.ClearModel(e.Family, e.Model)
		fmt.Fprintf(stderr, "[loop] cli-health model canary: %s maps to no model of %s — it demoted nothing, bench cleared\n", target, resolved.Driver)
		return true
	}
	rc, pattern, scrollback := probe(resolved.Driver, model)
	if ctx.Err() != nil {
		fmt.Fprintf(stderr, "[loop] cli-health model canary: cancelled (%v) during the %s probe — bench untouched\n", ctx.Err(), target)
		return false
	}
	switch {
	case rc == 0:
		_ = store.ClearModel(e.Family, e.Model)
		fmt.Fprintf(stderr, "[loop] cli-health model canary: %s recovered (probe OK with model %s) — bench cleared\n", target, model)
	case clihealth.Benchable(pattern):
		entry, _ := store.BenchModelUntil(e.Family, e.Model, clihealth.Wall{Pattern: pattern, Evidence: scrollback})
		fmt.Fprintf(stderr, "[loop] cli-health model canary: %s still walled (pattern=%s) — re-benched until %s (strikes=%d)\n",
			target, pattern, entry.BenchedUntil.Format(time.RFC3339), entry.Strikes)
	case rc == bridge.ExitModelMismatch:
		fmt.Fprintf(stderr, "[loop] cli-health model canary: %s booted a model outside its family (exit %d) — bench kept until a probe reaches the family's own model\n", target, rc)
	case rc == bridge.ExitSeatTokenRefused:
		fmt.Fprintf(stderr, "[loop] cli-health model canary: %s probe refused by the seat token holder (exit %d) — bench kept until a probe reaches the family; run evolve doctor seat\n", target, rc)
	default:
		_ = store.ClearModel(e.Family, e.Model)
		fmt.Fprintf(stderr, "[loop] cli-health model canary: %s probe failed rc=%d (not a wall) — bench cleared; normal dispatch machinery owns this failure class\n", target, rc)
	}
	return true
}
