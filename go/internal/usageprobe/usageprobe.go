// Package usageprobe is the proactive, per-cycle CLI-cap detector. Before a
// cycle's first phase boots an LLM CLI, it concurrently asks each interactive
// family for its usage/quota standing (through the clicontrol abstraction) and,
// for any family it classifies as currently capped, writes a bench into the
// shared clihealth store. The dispatcher's existing pre-skip
// (runner.applyBenchToPlan → llmroute.ApplyBench) then demotes that family on
// EVERY phase this cycle — so no phase wastes a boot attempt on a CLI that is
// already out of quota. This complements the reactive bench (which only learns a
// family is capped AFTER a phase has already burned a boot on it).
//
// The package is a stdlib-only consumer of two leaves (clihealth, clicontrol):
// the tmux execution and the manifest-backed classifier are injected as seams,
// so the core is unit-testable with no tmux and no real manifests. Every path is
// fail-open — the probe is advisory and must never break a cycle.
package usageprobe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

// benchReason labels a bench written by the proactive probe (vs the reactive
// wall classifier's "rate_limit"). Readers (applyBenchToPlan, the canary) are
// reason-agnostic; this is for forensics only.
const benchReason = "usage_probe"

// Prober runs the proactive usage probe across Families. All fields are
// injected so the core carries no I/O of its own:
//   - Probe drives the family's usage command over the bridge and returns the
//     captured pane; an error wrapping clicontrol.ErrUnsupported means the
//     family has no usage command (a silent skip, e.g. ollama).
//   - Classify reports whether a captured pane shows the family is currently
//     capped (manifest-backed exhausted_regex in production; conservative —
//     only a strong exhaustion signal benches).
//   - Store is the shared, flock-protected bench store.
type Prober struct {
	Families []string
	Probe    func(ctx context.Context, family string) (pane string, err error)
	Classify func(family, pane string) bool
	Store    *clihealth.Store
	Log      io.Writer
}

// Run probes every not-already-benched family concurrently and benches the
// capped ones. It blocks until all probes settle. Already-active benches are
// skipped (the family is already pre-skipped; re-probing would re-boot a capped
// REPL). Safe for concurrent fleet cycles: the bench write is flock-protected.
// Run's lifetime contract: it returns when every probe has answered OR when
// ctx is done — whichever comes first. On cancel the probe goroutines are
// deliberately abandoned (they end when their bridge call returns; the
// bridge's settle loop now honors ctx) and neither benches nor logs after
// the interrupt; the per-family `usage-probe` tmux session they drove is
// reaped by the tmux session GC, not here.
func (p *Prober) Run(ctx context.Context) {
	if ctx.Err() != nil {
		fmt.Fprintf(p.Log, "[usage-probe] cancelled before any probe ran (%v) — the loop's interrupt wins\n", ctx.Err())
		return
	}
	active := p.Store.Active() // snapshot: skip families already benched
	var wg sync.WaitGroup
	var launched, completed atomic.Int32
	for _, family := range p.Families {
		if _, benched := active[family]; benched {
			continue
		}
		family := family
		launched.Add(1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer completed.Add(1)
			p.probeOne(ctx, family)
		}()
	}
	// A probe over the bridge may not return on cancel (a pane that never
	// answers); the loop's interrupt must not wait for it — 2026-09-15, the
	// wave-4 boundary needed SIGKILL after SIGINT ×2 and SIGTERM sat here.
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		fmt.Fprintf(p.Log, "[usage-probe] cancelled (%v) — abandoning %d in-flight probe(s); the loop's interrupt wins\n", ctx.Err(), launched.Load()-completed.Load())
	}
}

// probeOne runs the full probe→classify→bench for one family. Fail-open at every
// step: unsupported and errored probes return without benching.
func (p *Prober) probeOne(ctx context.Context, family string) {
	pane, err := p.Probe(ctx, family)
	if ctx.Err() != nil {
		return // cancelled while probing: never bench on a pane read after the interrupt
	}
	if errors.Is(err, clicontrol.ErrUnsupported) {
		return // family has no usage command (e.g. ollama) — silent skip
	}
	if err != nil {
		fmt.Fprintf(p.Log, "[usage-probe] %s probe error: %v (skip, advisory)\n", family, err)
		return
	}
	if ctx.Err() != nil {
		return // never author a bench after the interrupt, even from a pane read just before it
	}
	if !p.Classify(family, pane) {
		return // healthy (or unclassifiable) — never a false bench
	}
	// BenchWall does the rest: parse the pane's reset hint (else strike-scaled
	// cooldown), accumulate strikes, keep the evidence line, all under the flock.
	entry, berr := p.Store.BenchWall(family, benchReason, pane)
	if berr != nil {
		fmt.Fprintf(p.Log, "[usage-probe] WARN bench %s failed: %v\n", family, berr)
		return
	}
	fmt.Fprintf(p.Log, "[usage-probe] %s capped — benched until %s (strikes=%d)\n",
		family, entry.BenchedUntil.Format(time.RFC3339), entry.Strikes)
}
