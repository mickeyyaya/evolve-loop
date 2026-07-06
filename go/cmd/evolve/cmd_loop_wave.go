// cmd_loop_wave.go — FLEET-AS-POLICY S2/S3 wave dispatch. Factored out of
// cmd_loop.go's batch for-loop (the per-iteration seam cmd_loop.go's batch
// loop calls) to respect file-size limits and to keep the decision/dispatch
// logic independently testable via injected preflight, triage-plan and
// launcher functions — mirrors the pure-decision-function precedent in
// cmd_loop_failbreaker.go.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleetbudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// waveLauncher launches one wave's disjoint lane specs and reports their
// results. *fleet.Supervisor satisfies this via its existing Run method —
// production wiring needs no adapter.
type waveLauncher interface {
	Run(ctx context.Context, specs []fleet.CycleSpec) []fleet.Result
}

// wavePlanFn produces one wave's single-writer triage output: the raw
// triage-decision.json bytes (committed_floors) plus the committed cards'
// target packages to use as a fallback when floors are absent. ctx is the
// loop's cancellable context (S3, PR #298 reviewer note): plan-path reads
// must observe loop shutdown — a re-minted root context would orphan them.
// waveIndex lets production wiring pick per-wave state (e.g. the workspace of
// the cycle that ran the wave's triage step).
type wavePlanFn func(ctx context.Context, waveIndex int) (decisionJSON []byte, cardPackages []string, err error)

// shouldRunWave reports whether a batch iteration should dispatch a wave
// (fan out disjoint lanes) instead of running the existing sequential
// orch.RunCycle path. Both conditions gate it: Count>1 (a fleet block is
// configured) AND the resolved PlanSource is "triage" (native wave dispatch,
// as opposed to "manual" — operator-only via `evolve fleet`, or the closed-
// vocab fallback for an unknown plan_source). This is the ONLY seam that
// decides sequential-vs-wave; keeping it a small pure function makes the
// golden (absent-block ⇒ sequential) regression trivial to pin.
func shouldRunWave(fc policy.FleetConfig) bool {
	return fc.Count > 1 && fc.PlanSource == "triage"
}

// dispatchIteration runs one batch iteration's wave path when shouldRunWave
// gates it on, and reports ran=false (no side effects) otherwise so the
// caller falls through to the existing sequential orch.RunCycle body
// unchanged. On the wave path, in order: runs preflight (S3 dirty-control-
// plane guard — a refusal surfaces wrapped, errors.Is-matchable, with
// ran=false and NEITHER planFn NOR launcher invoked, so the caller WARNs and
// falls back to sequential); obtains the wave's triage plan via planFn (the
// caller's ctx threads through — PR #298 reviewer note); adapts it through
// fleet.PlanFromTriage into <=fc.Count disjoint lane specs; and launches
// them through launcher — UNLESS the adapted plan is empty (D1: a triage
// decision committing nothing has zero lanes to launch), in which case
// dispatchIteration reports ran=false and the launcher is never invoked, so
// the caller falls back to sequential instead of silently consuming a
// --max-cycles iteration doing no work. A planFn or adapter error is
// surfaced (wrapped, so errors.Is still matches) with ran=false and the
// launcher never invoked. The preflight only ever runs on the wave path —
// the sequential (Count==1) path returns before it, so the production guard
// (which shells git against the main checkout) costs sequential loops
// nothing.
func dispatchIteration(ctx context.Context, fc policy.FleetConfig, preflight func() error, planFn wavePlanFn, launcher waveLauncher, waveIndex int) (ran bool, specs []fleet.CycleSpec, results []fleet.Result, err error) {
	if !shouldRunWave(fc) {
		return false, nil, nil, nil
	}
	if err := preflight(); err != nil {
		return false, nil, nil, fmt.Errorf("wave %d: control-plane preflight: %w", waveIndex, err)
	}
	decisionJSON, cardPackages, err := planFn(ctx, waveIndex)
	if err != nil {
		return false, nil, nil, fmt.Errorf("wave %d: triage plan: %w", waveIndex, err)
	}
	specs, err = fleet.PlanFromTriage(decisionJSON, cardPackages, fc.Count)
	if err != nil {
		return false, nil, nil, fmt.Errorf("wave %d: adapt triage plan: %w", waveIndex, err)
	}
	if len(specs) == 0 {
		return false, nil, nil, nil
	}
	results = launcher.Run(ctx, specs)
	return true, specs, results, nil
}

// forceOneLaneDispatch is the min-width repair seam (cycle-547,
// fleet-min-width-lane-fallback): when a wave-sized Count shrank to <=1 (a quota
// bench or a budget resize) but the operator's fleet.count wanted >1 lanes, the
// batch loop reaches dispatchIteration's ran=false path and would otherwise drop
// to the legacy sequential orch.RunCycle body — the ONLY execution mode that
// runs unisolated in the main-tree process cwd (cmd_loop_wave.go's doc: "the
// ONLY path that can leak into the main tree"), giving the operator width ZERO
// instead of width 1. forceOneLaneDispatch drives up to ONE disjoint candidate
// through the SAME isolated-worktree launcher path dispatchIteration uses,
// capped at a single lane, WITHOUT the shouldRunWave(Count>1) gate (the caller
// already knows the original config wanted a fleet and only reached here because
// the wave-sized Count shrank — this is the shrink-repair path, not the general
// multi-lane entry point). Every safety contract dispatchIteration has is
// preserved: a preflight refusal surfaces wrapped (errors.Is-matchable) with
// ran=false and NEITHER planFn NOR launcher invoked; a genuinely empty adapted
// plan reports ran=false, err=nil so the caller falls back to TRUE sequential —
// now the only case sequential is reserved for.
func forceOneLaneDispatch(ctx context.Context, preflight func() error, planFn wavePlanFn, launcher waveLauncher, waveIndex int) (ran bool, specs []fleet.CycleSpec, results []fleet.Result, err error) {
	if err := preflight(); err != nil {
		return false, nil, nil, fmt.Errorf("wave %d: control-plane preflight: %w", waveIndex, err)
	}
	decisionJSON, cardPackages, err := planFn(ctx, waveIndex)
	if err != nil {
		return false, nil, nil, fmt.Errorf("wave %d: triage plan: %w", waveIndex, err)
	}
	// Cap at a single lane: this is the shrink-repair path, not a fan-out.
	specs, err = fleet.PlanFromTriage(decisionJSON, cardPackages, 1)
	if err != nil {
		return false, nil, nil, fmt.Errorf("wave %d: adapt triage plan: %w", waveIndex, err)
	}
	if len(specs) == 0 {
		return false, nil, nil, nil // genuinely empty backlog → true sequential fallback
	}
	results = launcher.Run(ctx, specs)
	return true, specs, results, nil
}

// minWidthRepair is the extracted, independently-testable call-site wiring for
// the cycle-547 min-width fleet-dispatch repair. RunLoop's batch loop reaches
// it on dispatchIteration's default (ran=false, err=nil) case: the wave-sized
// Count shrank below shouldRunWave's Count>1 gate (a quota bench / budget
// resize) but the operator's fleet.count wanted >1 lanes. Rather than drop to
// width ZERO (the leak-prone sequential path), when eligible it drives up to
// ONE disjoint candidate through the SAME isolated-worktree launcher via
// forceOneLaneDispatch (capped at a single lane); the caller `continue`s the
// batch iteration when handled=true. Previously this switch lived inline in
// RunLoop and nothing exercised the guard / WARN-vs-dispatch branching — the
// wiring could silently regress (inverted guard, deleted call site) uncaught.
// The four stderr messages and the control flow are byte-identical to the
// inline switch they replace:
//   - guard not met (fleetCfg.Count<=1 || waveCfg.Count>1): WARN "empty triage
//     plan", handled=false, forceOneLaneDispatch (and thus preflight/planFn/
//     launcher) never invoked.
//   - guard met, a candidate dispatched: log "min-width repair dispatched",
//     handled=true (caller must continue).
//   - guard met, genuinely empty backlog: WARN "empty backlog", handled=false
//     (the only case true sequential fallback stays reserved for).
//   - guard met, forceOneLaneDispatch errored: WARN "min-width repair failed"
//     with the wrapped error surfaced, handled=false — never silently swallowed.
func minWidthRepair(ctx context.Context, fleetCfg, waveCfg policy.FleetConfig, preflight func() error, planFn wavePlanFn, launcher waveLauncher, waveIndex int, stderr io.Writer) (handled bool) {
	if !(fleetCfg.Count > 1 && waveCfg.Count <= 1) {
		fmt.Fprintf(stderr, "[loop] WARN: fleet: wave %d planned zero lanes (empty triage plan), falling back to sequential\n", waveIndex)
		return false
	}
	ran1, _, results1, oerr := forceOneLaneDispatch(ctx, preflight, planFn, launcher, waveIndex)
	switch {
	case oerr != nil:
		fmt.Fprintf(stderr, "[loop] WARN: fleet: wave %d min-width repair failed, falling back to sequential: %v\n", waveIndex, oerr)
		return false
	case ran1:
		failedLanes := 0
		for _, r := range results1 {
			if r.Err != nil || r.ExitCode != 0 {
				failedLanes++
			}
		}
		fmt.Fprintf(stderr, "[loop] wave %d: min-width repair dispatched %d/%d isolated lane (fleet.count=%d shrank to %d)\n", waveIndex, len(results1)-failedLanes, len(results1), fleetCfg.Count, waveCfg.Count)
		return true
	default:
		fmt.Fprintf(stderr, "[loop] WARN: fleet: wave %d planned zero lanes (empty backlog), falling back to sequential\n", waveIndex)
		return false
	}
}

// loadFleetConfig loads .evolve/policy.json and returns the resolved fleet
// configuration. Absent or malformed policy falls back to built-in defaults
// (Count=1 — the existing sequential path), mirroring loadWorkflowConfig.
func loadFleetConfig(evolveDir string) policy.FleetConfig {
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		return policy.Policy{}.FleetConfig()
	}
	return pol.FleetConfig()
}

// productionWaveLauncher builds the real waveLauncher: a fleet.Supervisor
// wired to the same execCycleLaunch LaunchFn `evolve fleet` uses, so wave
// lanes inherit EVOLVE_FLEET=1 + EVOLVE_FLEET_SCOPE exactly like
// `evolve fleet --plan` lanes (scoped lane triage already exists at
// internal/core/cyclerun.go:443).
func productionWaveLauncher(fc policy.FleetConfig, binPath, projectRoot, goalHash, goalText string, stdout, stderr io.Writer) *fleet.Supervisor {
	return &fleet.Supervisor{
		Concurrency: fc.Concurrency,
		Launch:      execCycleLaunch(binPath, false, projectRoot, goalHash, goalText, stdout, stderr),
	}
}

// productionWavePreflight is the production S3 dirty-control-plane guard: a
// closure over fleet.PreflightControlPlane against the MAIN checkout
// (cfg.ProjectRoot — waves ship from the main tree, so that is the tree whose
// uncommitted control-plane edits kill audit-PASSED lanes at ship time).
func productionWavePreflight(projectRoot string) func() error {
	return func() error { return fleet.PreflightControlPlane(projectRoot) }
}

// waveBenchedFamilies reads the quota-bench SSOT (clihealth.Store.Active())
// and returns the active benches as family → reason for the wave's capacity
// shrink. Every actively benched family is wave-relevant: lanes are full
// `evolve cycle run` subprocesses that may route to any installed family, so
// a benched family always shrinks the shared capacity pool. Store.Active()
// degrades a missing/corrupt bench file to an empty map by design — bench
// state must never break a dispatch.
func waveBenchedFamilies(projectRoot string) map[string]string {
	active := clihealth.NewStore(projectRoot, nil).Active()
	if len(active) == 0 {
		return nil
	}
	benched := make(map[string]string, len(active))
	for fam, e := range active {
		benched[fam] = e.Reason
	}
	return benched
}

// quotaAwareWaveConfig returns fc with Count sized for this wave, plus the
// inter-wave PaceDelay the loop should idle before the next wave (0 unless the
// budget is enforcing a floor-forced pace).
//
// Two composed layers: (1) the availability envelope — fleet.QuotaAwareCount
// shrinks Count per the active quota benches (min 1, one WARN per benched
// family); (2) the budget sizing — when the operator opted into a fleet.budget
// block, fleetbudget.Plan sizes the (already bench-shrunk) count against the
// measured quota headroom + pace. Absent the block (fc.Budget==nil) layer 2 is
// skipped entirely: byte-identical to the pre-Q4 bench-only behavior, and the
// caller never even probes quota. With the block, Stage governs application:
// "shadow" (default) computes + LOGS the decision but HOLDS the bench-shrunk
// count (a genuine soak); "enforce" applies plan.Lanes + returns plan.PaceDelay.
//
// Operates on a copy — the batch-level fleet config is resolved once and must
// not compound shrink across iterations; benches expire and quota moves, so
// each wave re-reads them.
func quotaAwareWaveConfig(fc policy.FleetConfig, projectRoot string, warn io.Writer, states []quotastate.QuotaState, tp budgethistory.Throughput, now time.Time) (policy.FleetConfig, time.Duration) {
	fc.Count = fleet.QuotaAwareCount(fc.Count, waveBenchedFamilies(projectRoot), fc.MinLanes, warn)
	if fc.Budget == nil {
		return fc, 0
	}
	plan := fleetbudget.Plan(states, tp, fleetbudget.Config{
		Count:          fc.Count,
		Floor:          fc.MinLanes,
		CapacityCycles: fc.Budget.CapacityCycles,
		Safety:         fc.Budget.Safety,
	}, now)
	switch fc.Budget.Stage {
	case "enforce":
		fmt.Fprintf(warn, "[budget] enforce: sizing wave to %d lane(s) [%s] — %s\n", plan.Lanes, plan.DerivedFrom, plan.Reason)
		fc.Count = plan.Lanes
		return fc, plan.PaceDelay
	default:
		// shadow (the resolved default; any non-enforce stage): compute + log the
		// would-be decision, hold the count — a genuine soak that never resizes.
		fmt.Fprintf(warn, "[budget] shadow: would size wave to %d lane(s) (holding at %d) [%s] — %s\n", plan.Lanes, fc.Count, plan.DerivedFrom, plan.Reason)
		return fc, 0
	}
}

// productionWavePlanFn reads the previous cycle's triage-decision.json as
// the wave's single-writer plan. A dedicated triage-only phase runner (the
// seam scout-report.md cycle 465 calls "Missing piece #2") does not exist
// yet, so this minimal seam reuses the most recently committed triage
// decision instead of running a fresh planning step per wave — the first
// wave of a fresh batch (no prior cycle) errors and the caller falls back to
// sequential, which produces one for the next wave to pick up. cardPackages
// is always nil here (no dedicated card-package reader exists yet); real
// decisions' top_n[].id cards flow through fleet.PlanFromTriage's own
// fallback instead (D1 severity amplifier fix), so this stays intentionally
// thin. The threaded ctx (PR #298 reviewer note) reaches the storage read,
// so loop shutdown cancels the plan path too. Deferred: a dedicated
// single-writer triage-only runner.
func productionWavePlanFn(cfg loopConfig, storage core.Storage, count int) wavePlanFn {
	return func(ctx context.Context, waveIndex int) ([]byte, []string, error) {
		// Preferred source: the immediately-prior cycle's single-writer triage
		// decision — partition the work it already selected.
		if lastCycle, err := readLastCycleNumber(ctx, storage); err == nil && lastCycle > 0 {
			companion := filepath.Join(cycleWorkspace(cfg.ProjectRoot, lastCycle), triagecap.TriageDecisionName())
			if data, rerr := os.ReadFile(companion); rerr == nil {
				// A present-but-NARROW prior decision (fewer than `count`
				// file-disjoint top_n items) would collapse the fleet to a single
				// lane. Widen it from the durable inbox backlog before partitioning
				// so the primary path un-starves too — not just the absent-decision
				// fallback below (cycle-503 starvation on the primary path).
				return widenNarrowDecision(data, cfg.EvolveDir, count), nil, nil
			}
			// Prior decision absent (fresh start, `evolve cycle reset` sealed the
			// run dir, or the prior cycle failed before triage) — fall through to
			// the inbox seed instead of erroring the wave into a sequential
			// fallback. This is what keeps fleet 2-wide from failing to start on
			// the first cycle / after any reset (the sequential fallback is also
			// the ONLY path that can leak into the main tree, so avoiding it here
			// is doubly load-bearing).
		}
		// Seed from the durable inbox backlog so 2-wide starts on the FIRST cycle
		// without a prior cycle's on-disk decision.
		data, err := seedWavePlanFromInbox(cfg.EvolveDir, count)
		if err != nil {
			return nil, nil, fmt.Errorf("wave %d: no prior triage decision and %w", waveIndex, err)
		}
		return data, nil, nil
	}
}

// seedWavePlanFromInbox synthesizes a triage-decision.json (top_n[].id) from the
// top-`count` inbox todos (by weight) so the wave planner (fleet.PlanFromTriage)
// can partition them into disjoint lanes without a prior cycle's decision. count
// is the caller's already-resolved wave width (clamped to >= 2 here); requires
// >= 2 todos to fill >= 2 lanes — fewer returns an error so the caller falls back
// to sequential (one item cannot run 2-wide anyway).
func seedWavePlanFromInbox(evolveDir string, count int) ([]byte, error) {
	if count < 2 {
		count = 2
	}
	// SelectWaveSeedTopN packs the inbox backlog into up to `count` mutually
	// file-disjoint lane reps (highest weight first) — replacing the raw
	// weight-sorted top-N, which could seed two lanes that collide on a shared
	// file. The seam is single-sourced in triagecap (also the SelectFleetWidthTopN
	// home) so this caller stays a thin adapter to the top_n decision shape.
	reps := triagecap.SelectWaveSeedTopN(evolveDir, count)
	if len(reps) < 2 {
		return nil, fmt.Errorf("inbox seed: %d disjoint lane(s) — need >= 2 file-disjoint inbox todos to fill a wave", len(reps))
	}
	topN := make([]map[string]string, 0, len(reps))
	for _, r := range reps {
		topN = append(topN, map[string]string{"id": r.ID})
	}
	return json.Marshal(map[string]any{"top_n": topN})
}

// widenNarrowDecision is the thin main-package adapter that turns a present-but-
// narrow prior triage-decision.json into a fleet-width one. It parses the
// decision's top_n[] into triagecap.FleetCandidate (id + declared files), calls
// the single-sourced triagecap.WidenTopNToFleetWidth seam to backfill it from
// the inbox backlog up to `count` mutually file-disjoint lanes, and re-marshals
// the widened top_n (files preserved so fleet.PlanFromTriage stays disjoint-
// aware). Best-effort: any parse/marshal failure returns the original bytes
// unchanged — widening is an optimization, never a correctness dependency.
func widenNarrowDecision(data []byte, evolveDir string, count int) []byte {
	if count < 2 {
		return data
	}
	var decision struct {
		TopN []struct {
			ID    string   `json:"id"`
			Files []string `json:"files"`
		} `json:"top_n"`
	}
	if json.Unmarshal(data, &decision) != nil || len(decision.TopN) == 0 {
		return data
	}
	committed := make([]triagecap.FleetCandidate, 0, len(decision.TopN))
	for _, c := range decision.TopN {
		if c.ID != "" {
			committed = append(committed, triagecap.FleetCandidate{ID: c.ID, Files: c.Files})
		}
	}
	if len(committed) >= count {
		return data // already fleet-width — no inbox read, no re-marshal.
	}
	widened := triagecap.WidenTopNToFleetWidth(committed, triagecap.ReadInboxBacklog(evolveDir), count)
	if len(widened) <= len(committed) {
		return data // nothing disjoint to add — leave the decision as-is.
	}
	topN := make([]map[string]any, 0, len(widened))
	for _, w := range widened {
		card := map[string]any{"id": w.ID}
		if len(w.Files) > 0 {
			card["files"] = w.Files
		}
		topN = append(topN, card)
	}
	out, err := json.Marshal(map[string]any{"top_n": topN})
	if err != nil {
		return data
	}
	return out
}
