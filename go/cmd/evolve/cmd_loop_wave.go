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
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleetbudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
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
//
// Scheduling=="pool" is EXCLUDED here (routed to shouldRunPool /
// dispatchPoolIteration instead): the two dispatch gates are mutually exclusive
// so a single iteration never fires both a wave AND a pool (no double-dispatch).
// The default/absent Scheduling ("") keeps the shipped wave path unchanged.
func shouldRunWave(fc policy.FleetConfig) bool {
	return fc.Count > 1 && fc.PlanSource == "triage" && fc.Scheduling != "pool"
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
func dispatchIteration(ctx context.Context, fc policy.FleetConfig, preflight func() error, planFn wavePlanFn, launcher waveLauncher, routed fleet.RoutedFn, waveIndex int) (ran bool, specs []fleet.CycleSpec, results []fleet.Result, err error) {
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
	// ADR-0074 plan-time gate: refusals (console-routed ids) are logged by the
	// composition-root resolver wrapper the moment they fire, so the slice is
	// safely discarded here.
	specs, _, err = fleet.PlanFromTriage(decisionJSON, cardPackages, fc.Count, routed)
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
func forceOneLaneDispatch(ctx context.Context, preflight func() error, planFn wavePlanFn, launcher waveLauncher, routed fleet.RoutedFn, waveIndex int) (ran bool, specs []fleet.CycleSpec, results []fleet.Result, err error) {
	if err := preflight(); err != nil {
		return false, nil, nil, fmt.Errorf("wave %d: control-plane preflight: %w", waveIndex, err)
	}
	decisionJSON, cardPackages, err := planFn(ctx, waveIndex)
	if err != nil {
		return false, nil, nil, fmt.Errorf("wave %d: triage plan: %w", waveIndex, err)
	}
	// Cap at a single lane: this is the shrink-repair path, not a fan-out.
	specs, _, err = fleet.PlanFromTriage(decisionJSON, cardPackages, 1, routed)
	if err != nil {
		return false, nil, nil, fmt.Errorf("wave %d: adapt triage plan: %w", waveIndex, err)
	}
	if len(specs) == 0 {
		return false, nil, nil, nil // genuinely empty backlog → true sequential fallback
	}
	results = launcher.Run(ctx, specs)
	return true, specs, results, nil
}

// consoleRoutedResolver is the composition-root wiring of the ADR-0074
// plan-time gate: a fresh inbox load per wave (mid-batch inbox changes must be
// seen), the real protected-surface predicate, and a WARN the moment a
// console-routed id is refused — the refusal must never be silent.
func consoleRoutedResolver(projectRoot string, stderr io.Writer) fleet.RoutedFn {
	base := inboxbatch.RoutedResolver(filepath.Join(projectRoot, ".evolve", "inbox"), guards.IsProtectedSurface)
	return func(id string) (bool, string) {
		routed, reason := base(id)
		if routed {
			fmt.Fprintf(stderr, "[fleet] WARN: plan-time gate refused console-routed item %q (%s) — operator-owned, worked at a batch boundary via manual ship (ADR-0074)\n", id, reason)
		}
		return routed, reason
	}
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
//   - guard not met (fleetCfg.Count<=1 — the operator wanted a single lane):
//     WARN "empty triage plan", handled=false, forceOneLaneDispatch (and thus
//     preflight/planFn/launcher) never invoked. The empty-plan-at-full-capacity
//     shape (waveCfg.Count>1, zero planned lanes) is now IN-guard: it repairs to
//     one isolated lane instead of leaking to sequential.
//   - guard met, a candidate dispatched: log "min-width repair dispatched",
//     handled=true (caller must continue).
//   - guard met, genuinely empty backlog: WARN "empty backlog", handled=false
//     (the only case true sequential fallback stays reserved for).
//   - guard met, forceOneLaneDispatch errored: WARN "min-width repair failed"
//     with the wrapped error surfaced, handled=false — never silently swallowed.
func minWidthRepair(ctx context.Context, fleetCfg, waveCfg policy.FleetConfig, preflight func() error, planFn wavePlanFn, launcher waveLauncher, routed fleet.RoutedFn, waveIndex int, stderr io.Writer) (handled bool) {
	// Eligibility is the operator-asserted width alone: fleetCfg.Count>1 means
	// the operator wanted a fleet, so BOTH the quota-shrunk shape (waveCfg.Count
	// <=1) AND the empty-plan-at-full-capacity shape (waveCfg.Count>1 yet
	// dispatchIteration planned zero lanes) route through the isolated one-lane
	// repair rather than the leak-prone sequential fallthrough. True sequential
	// stays reserved for fleetCfg.Count<=1 only (the operator wanted one lane).
	if fleetCfg.Count <= 1 {
		fmt.Fprintf(stderr, "[loop] WARN: fleet: wave %d planned zero lanes (empty triage plan), falling back to sequential\n", waveIndex)
		return false
	}
	ran1, _, results1, oerr := forceOneLaneDispatch(ctx, preflight, planFn, launcher, routed, waveIndex)
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

// reloadFleetConfigAtWaveBoundary re-resolves the committed fleet block from
// .evolve/policy.json at a wave boundary (cycle 739,
// fleet-config-hot-reload-wave-boundary). The batch loop calls it every
// iteration BEFORE the quota/budget sizing, so an operator width directive
// (count/min_lanes) committed mid-batch takes effect at the next wave without
// a lane-killing supervisor bounce — the control-plane preflight already
// re-validates policy.json cleanliness per wave; this closes the other half
// (dispatch consuming stale VALUES). Same resolution semantics as batch
// start's loadFleetConfig, with ONE deliberate divergence: an unreadable or
// malformed policy.json HOLDS prev (the operator's standing width commitment,
// [fleet_width_always_respected]) and WARNs, instead of silently collapsing
// to the Count=1 defaults the batch-start loader degrades to. Logs one
// "fleet config reloaded" line only when the resolved dispatch-relevant
// values changed vs prev — the steady-state path stays silent.
func reloadFleetConfigAtWaveBoundary(evolveDir string, prev policy.FleetConfig, warn io.Writer) policy.FleetConfig {
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		fmt.Fprintf(warn, "[loop] WARN: fleet: policy.json unreadable at wave boundary (%v) — holding fleet config count=%d min_lanes=%d\n", err, prev.Count, prev.MinLanes)
		return prev
	}
	got := pol.FleetConfig()
	if got.Count != prev.Count || got.MinLanes != prev.MinLanes ||
		got.PlanSource != prev.PlanSource || got.Scheduling != prev.Scheduling {
		for _, w := range got.Warnings {
			fmt.Fprintf(warn, "[loop] WARN: fleet: %s\n", w)
		}
		fmt.Fprintf(warn, "[loop] fleet config reloaded: count=%d min_lanes=%d\n", got.Count, got.MinLanes)
	}
	return got
}

// productionWaveLauncher builds the real waveLauncher: a fleet.Supervisor
// wired to the same execCycleLaunch LaunchFn `evolve fleet` uses, so wave
// lanes inherit EVOLVE_FLEET=1 + EVOLVE_FLEET_SCOPE exactly like
// `evolve fleet --plan` lanes (scoped lane triage already exists at
// internal/core/cyclerun.go:443). The supervisor is wrapped in the dispatch
// freshness gate (cycle 767, dispatch-freshness-gate): immediately before
// launch, every spec's scope ids are re-resolved against the CURRENT inbox
// lifecycle + deps, stale ids are skipped with a logged reason, and freed
// slots are refilled from the pending backlog — a lane-slot is never burned
// on known-dead work. Decorating here (instead of threading a param through
// dispatchIteration) gates BOTH the wave path and the min-width repair path
// at one seam.
func productionWaveLauncher(fc policy.FleetConfig, binPath, projectRoot, goalHash, goalText string, stdout, stderr io.Writer) waveLauncher {
	return freshnessGatedLauncher{
		inner: &fleet.Supervisor{
			Concurrency: fc.Concurrency,
			Launch:      execCycleLaunch(binPath, false, projectRoot, goalHash, goalText, stdout, stderr),
		},
		probe:  productionFreshnessProbe(projectRoot),
		refill: productionRefillFn(filepath.Join(projectRoot, ".evolve")),
		warn:   stderr,
	}
}

// freshnessGatedLauncher decorates a waveLauncher with fleet.FreshenSpecs so
// the gate runs at the last moment before lanes launch (planning happened
// earlier and may be stale — the postmortem's whole failure class).
type freshnessGatedLauncher struct {
	inner  waveLauncher
	probe  fleet.FreshnessProbeFn
	refill fleet.RefillFn
	warn   io.Writer
}

func (l freshnessGatedLauncher) Run(ctx context.Context, specs []fleet.CycleSpec) []fleet.Result {
	kept, skipped := fleet.FreshenSpecs(specs, l.probe, l.refill, l.warn)
	if len(kept) == 0 {
		// Whole wave stale and backlog exhausted: launching nothing IS the fix
		// (a shorter wave, never a doomed lane). Skips were already WARN-logged.
		fmt.Fprintf(l.warn, "[fleet] freshness gate: all %d planned lane(s) stale (%d skip(s)), nothing to launch\n", len(specs), len(skipped))
		return nil
	}
	return l.inner.Run(ctx, kept)
}

// productionFreshnessProbe re-resolves one task id against the inbox
// lifecycle at dispatch time. Pending → fresh unless a declared dep is still
// undone (pending/processing/retry — postmortem shape (3)); any consumed
// lifecycle state → stale with the state as reason (shapes (1)/(2)); no
// lifecycle evidence at all → fresh (fail-open: not every planned id is
// inbox-backed, and a missing file must never false-skip a lane).
func productionFreshnessProbe(projectRoot string) fleet.FreshnessProbeFn {
	opts := inboxmover.Options{ProjectRoot: projectRoot, Stderr: io.Discard}
	return func(taskID string) fleet.TaskFreshness {
		ds := inboxmover.ResolveDispatchState(opts, taskID)
		switch ds.State {
		case inboxmover.StatePending:
			for _, dep := range ds.Deps {
				switch inboxmover.ResolveDispatchState(opts, dep).State {
				case inboxmover.StatePending, inboxmover.StateProcessing, inboxmover.StateRetry:
					return fleet.TaskFreshness{Fresh: false, Reason: "deps unmet: needs " + dep}
				}
			}
			return fleet.TaskFreshness{Fresh: true}
		case inboxmover.StateUnknown:
			return fleet.TaskFreshness{Fresh: true}
		default:
			reason := "consumed: " + ds.State
			if ds.Detail != "" {
				reason += " " + ds.Detail
			}
			return fleet.TaskFreshness{Fresh: false, Reason: reason}
		}
	}
}

// productionRefillFn pulls the highest-weight pending inbox todo not already
// owned by this wave into a freed slot, shaped exactly like a planned lane
// spec (Scope + EVOLVE_FLEET_SCOPE; EVOLVE_FLEET is forced by the supervisor).
// minimal: picks by weight only — it does not re-check file-disjointness
// against the kept lanes; upgrade path is routing the refill through
// triagecap.SelectFleetWidthTopN with the kept lanes' files as seeds.
func productionRefillFn(evolveDir string) fleet.RefillFn {
	return func(exclude map[string]bool) (fleet.CycleSpec, bool) {
		backlog := triagecap.ReadInboxBacklog(evolveDir)
		sort.SliceStable(backlog, func(i, j int) bool { return backlog[i].Weight > backlog[j].Weight })
		for _, c := range backlog {
			if exclude[c.ID] {
				continue
			}
			return fleet.CycleSpec{
				Scope: []string{c.ID},
				Env:   map[string]string{ipcenv.FleetScopeKey: c.ID},
			}, true
		}
		return fleet.CycleSpec{}, false
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
	// Only an unparseable decision returns unchanged. An EMPTY top_n is NOT a
	// no-op: it is the observed empty/narrow-decision starvation shape (cycle-554)
	// — fall through with an empty `committed` and widen fully from the inbox
	// backlog, exactly as if no lanes had been committed, so the wave plans
	// fleet-width lanes instead of returning the empty decision unchanged.
	if json.Unmarshal(data, &decision) != nil {
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
