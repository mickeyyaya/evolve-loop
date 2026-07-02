// cmd_loop_wave.go — FLEET-AS-POLICY S2/S3 wave dispatch. Factored out of
// cmd_loop.go's batch for-loop (the per-iteration seam cmd_loop.go's batch
// loop calls) to respect file-size limits and to keep the decision/dispatch
// logic independently testable via injected preflight, triage-plan and
// launcher functions — mirrors the pure-decision-function precedent in
// cmd_loop_failbreaker.go.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
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
func productionWaveLauncher(fc policy.FleetConfig, binPath, projectRoot string, stdout, stderr io.Writer) *fleet.Supervisor {
	return &fleet.Supervisor{
		Concurrency: fc.Concurrency,
		Launch:      execCycleLaunch(binPath, false, projectRoot, stdout, stderr),
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

// quotaAwareWaveConfig returns fc with Count shrunk per the active quota
// benches (fleet.QuotaAwareCount over waveBenchedFamilies; min 1, one WARN
// per benched family naming family + reason). Operates on a copy — the
// batch-level fleet config is resolved once and must not compound shrink
// across iterations; benches expire, so each wave re-reads them.
func quotaAwareWaveConfig(fc policy.FleetConfig, projectRoot string, warn io.Writer) policy.FleetConfig {
	fc.Count = fleet.QuotaAwareCount(fc.Count, waveBenchedFamilies(projectRoot), warn)
	return fc
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
func productionWavePlanFn(cfg loopConfig, storage core.Storage) wavePlanFn {
	return func(ctx context.Context, waveIndex int) ([]byte, []string, error) {
		lastCycle, err := readLastCycleNumber(ctx, storage)
		if err != nil {
			return nil, nil, fmt.Errorf("read last cycle number: %w", err)
		}
		if lastCycle <= 0 {
			return nil, nil, fmt.Errorf("no prior cycle to source a triage plan from (wave %d)", waveIndex)
		}
		companion := filepath.Join(cycleWorkspace(cfg.ProjectRoot, lastCycle), triagecap.TriageDecisionName())
		data, err := os.ReadFile(companion)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", companion, err)
		}
		return data, nil, nil
	}
}
