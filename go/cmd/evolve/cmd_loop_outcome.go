package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type loopResult struct {
	StopReason          string             `json:"stop_reason"`
	Cycles              []core.CycleResult `json:"cycles"`
	TotalCost           float64            `json:"total_cost_usd"`
	Resumed             bool               `json:"resumed,omitempty"`
	RecoverableFailures int                `json:"recoverable_failures,omitempty"`
	// ContinuedFailures counts verdict-FAIL cycles the batch absorbed and
	// continued past (>1 only under a raised consecutive-fails ceiling); like
	// RecoverableFailures it drives the rc=3 "completed but with failures" exit.
	ContinuedFailures int `json:"continued_failures,omitempty"`
	// SpineFailOpens is the batch roll-up of spine-gate fail-opens: the total,
	// the per-phase breakdown, and the cycles that breached the WARN threshold.
	// Absent on a clean batch.
	SpineFailOpens *dossier.SpineFailOpenRollup `json:"spine_fail_opens,omitempty"`
	// CycleOutcomes is the per-cycle SLO classification (SHIPPED / SALVAGED /
	// FAILED_EXPLAINED / FAILED_UNEXPLAINED) computed from the C1 records at
	// emit time.
	CycleOutcomes []cycleOutcomeEntry `json:"cycle_outcomes,omitempty"`
	// classifyRoot, when set, makes emit() populate CycleOutcomes from
	// <root>/.evolve/runs/cycle-N.
	classifyRoot    string
	batchFirstCycle int
	// BoundaryRefresh mirrors chainResult.BoundaryRefresh for the non-chain
	// wave/fleet boundary path: nil-when-clean, populated only when this
	// batch's stop is "loop_boundary_refresh_reexec".
	BoundaryRefresh *chainBoundaryRefreshLogEntry `json:"boundary_refresh,omitempty"`
}

type cycleOutcomeEntry struct {
	Cycle   int    `json:"cycle"`
	Outcome string `json:"outcome"`
	Detail  string `json:"detail,omitempty"`
}

// emit writes lr to w as the canonical pretty-JSON dispatcher output. If a
// future field breaks MarshalIndent, it emits a structured error envelope
// instead of a silent empty line, since dispatchers and `evolve loop`
// consumers grep stop_reason.
func (lr *loopResult) emit(w io.Writer) {
	// Classify every cycle's ending from its C1 records at the single output
	// chokepoint every exit path funnels through. A FAILED_UNEXPLAINED
	// additionally self-files an inbox defect: that bucket means a terminal
	// path escaped the chokepoint, which is itself a defect. Best-effort:
	// classification must never break the dispatcher contract.
	if lr.classifyRoot != "" && len(lr.Cycles) > 0 && lr.CycleOutcomes == nil {
		for _, c := range lr.Cycles {
			oc, detail := cyclehealth.ClassifyOutcome(cycleWorkspace(lr.classifyRoot, c.Cycle))
			lr.CycleOutcomes = append(lr.CycleOutcomes, cycleOutcomeEntry{Cycle: c.Cycle, Outcome: string(oc), Detail: detail})
			if oc == cyclehealth.OutcomeFailedUnexplained {
				fileUnexplainedOutcomeDefect(lr.classifyRoot, c.Cycle, detail)
			}
		}
		sweepRulePromotions(os.Stderr, lr.classifyRoot, lr.Cycles)
	}
	// Deliberately outside the classifyRoot block above, which is gated on
	// len(lr.Cycles) > 0: a fleet batch runs every cycle in a lane subprocess
	// and appends nothing to lr.Cycles, yet those are precisely the batches
	// whose fail-opens need counting.
	if lr.classifyRoot != "" {
		publishPendingDossiers(lr.classifyRoot, os.Stderr)
	}
	if lr.SpineFailOpens == nil {
		lr.SpineFailOpens = spineFailOpenRollup(lr.Cycles, lr.classifyRoot, lr.batchFirstCycle, os.Stderr)
	}
	buf, err := json.MarshalIndent(lr, "", "  ")
	if err != nil {
		fmt.Fprintf(w, `{"stop_reason":"marshal_error","error":%q}`+"\n", err.Error())
		return
	}
	fmt.Fprintln(w, string(buf))
}

var gcHookFn = runGCHook

func runGCHook(cfg loopConfig, workspace string, stderr io.Writer) {
	pol, err := policy.Load(filepath.Join(cfg.EvolveDir, "policy.json"))
	if err != nil {
		fmt.Fprintf(stderr, "[gc] WARN: policy load failed: %v; using zero-value gc policy\n", err)
	}
	gcPol := gc.Policy{}
	if pol.GC != nil {
		gcPol = *pol.GC
	}
	mode := gcPol.Mode
	if mode == "" {
		mode = "shadow"
	}
	switch mode {
	case "off":
		return
	case "shadow", "enforce":
	default:
		fmt.Fprintf(stderr, "[gc] WARN: invalid gc.mode=%q (want off|shadow|enforce); skipping\n", mode)
		return
	}

	runRunDirGC(cfg, workspace, mode, gcPol, stderr)
	// Deliberately independent of the run-dir sweep: either half failing (bad
	// plan, unwritable workspace, non-git ProjectRoot) must never silently
	// disable the other. Both are best-effort batch-end hygiene.
	runWorktreeGC(cfg, workspace, mode, gcPol, stderr)
}

func runRunDirGC(cfg loopConfig, workspace, mode string, gcPol gc.Policy, stderr io.Writer) {
	manifest, err := planRunDirGC(cfg.EvolveDir, gcPol, func(err error) {
		fmt.Fprintf(stderr, "[gc] WARN: discover failed: %v; writing empty manifest\n", err)
	})
	if err != nil {
		fmt.Fprintf(stderr, "[gc] WARN: plan failed: %v\n", err)
		return
	}
	if !publishGCManifest(workspace, "gc-shadow-manifest.json", manifest, stderr) {
		return
	}
	archive, del := gcActionCounts(manifest)
	fmt.Fprintf(stderr, "[gc] shadow: %d items (%d archive, %d delete)\n", len(manifest.Items), archive, del)

	if mode != "enforce" {
		return
	}
	if err := gc.Apply(cfg.EvolveDir, manifest); err != nil {
		fmt.Fprintf(stderr, "[gc] WARN: enforce apply failed: %v\n", err)
		return
	}
	fmt.Fprintf(stderr, "[gc] enforce: applied %d items\n", len(manifest.Items))
}

func runWorktreeGC(cfg loopConfig, workspace, mode string, gcPol gc.Policy, stderr io.Writer) {
	if cfg.ProjectRoot == "" {
		// Never infer the repo from the process cwd: an unset ProjectRoot with
		// mode=enforce would aim `git branch -d` at whatever repo the binary
		// happens to run inside. Refuse loudly instead.
		fmt.Fprintf(stderr, "[gc] WARN: worktree sweep skipped: ProjectRoot is unset\n")
		return
	}
	opts := worktreeGCOptions(cfg.ProjectRoot, cfg.EvolveDir, gcPol.Worktrees)
	manifest, err := gc.PlanWorktrees(opts)
	if err != nil {
		fmt.Fprintf(stderr, "[gc] WARN: worktree plan failed: %v; skipping worktree sweep\n", err)
		return
	}
	if !publishGCManifest(workspace, "workspace-gc-manifest.json", manifest, stderr) {
		return
	}
	fmt.Fprintf(stderr, "[gc] worktree shadow: %d items\n", len(manifest.Items))

	if mode != "enforce" {
		return
	}
	if err := gc.ApplyWorktrees(opts, manifest); err != nil {
		// Partial application is normal (errors.Join over per-item refusals):
		// report and continue — the manifest is the evidence of record.
		fmt.Fprintf(stderr, "[gc] WARN: worktree enforce apply: %v\n", err)
	}
	fmt.Fprintf(stderr, "[gc] worktree enforce: applied %d items\n", len(manifest.Items))
}

func gcManifestDir(evolveDir string) string {
	return filepath.Join(evolveDir, "gc")
}

func planRunDirGC(evolveDir string, pol gc.Policy, discoverFailed func(error)) (gc.Manifest, error) {
	runs, err := gc.Discover(evolveDir, gc.DiscoverOptions{})
	if err != nil {
		discoverFailed(err)
		runs = nil
	}
	return gc.Plan(gc.Options{EvolveDir: evolveDir, Runs: runs, Policy: pol})
}

// worktreeGCOptions is the single construction site shared by the in-loop
// hook and the operator command (`evolve gc`, cmd_gc.go), so both sweeps aim
// at the same worktree base and carry the same policy.
func worktreeGCOptions(projectRoot, evolveDir string, pol gc.WorktreesPolicy) gc.WorktreeOptions {
	return gc.WorktreeOptions{
		ProjectRoot:  projectRoot,
		WorktreeBase: filepath.Join(projectRoot, ".evolve", "worktrees"),
		EvolveDir:    evolveDir,
		Policy:       pol,
		Exec:         sysexec.DefaultRunner,
	}
}

func publishGCManifest(workspace, name string, v any, stderr io.Writer) bool {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "[gc] WARN: %s encode failed: %v\n", name, err)
		return false
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		fmt.Fprintf(stderr, "[gc] WARN: create workspace for %s: %v\n", name, err)
		return false
	}
	target := filepath.Join(workspace, name)
	tmp := fmt.Sprintf("%s.tmp.%d", target, os.Getpid())
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o644); err != nil {
		fmt.Fprintf(stderr, "[gc] WARN: write %s temp: %v\n", name, err)
		return false
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		fmt.Fprintf(stderr, "[gc] WARN: publish %s: %v\n", name, err)
		return false
	}
	return true
}

func gcActionCounts(manifest gc.Manifest) (archive, del int) {
	for _, item := range manifest.Items {
		switch item.Action {
		case gc.ActionArchive:
			archive++
		case gc.ActionDelete:
			del++
		}
	}
	return archive, del
}

// spineFailOpenWarnThreshold is a compiled constant, not a policy knob: a
// policy knob here would let a noisy batch turn its own alarm off. A cycle's
// spine legitimately degrades once or twice under a driver bounce; four times
// is a pattern.
const spineFailOpenWarnThreshold = 3

// spineFailOpenRollup unions committed dossiers for cycles >= firstCycle (a
// fleet lane's CycleResult never returns to this process, so its dossier is
// the only record) with the in-memory CycleResults for any cycle with no
// dossier yet on disk; the dossier wins on overlap since it was written from
// that same CycleResult. firstCycle <= 0 skips the corpus read entirely,
// rather than report all-time history as this batch's. Returns nil for a
// clean batch so the summary omits the field entirely.
func spineFailOpenRollup(cycles []core.CycleResult, projectRoot string, firstCycle int, warn io.Writer) *dossier.SpineFailOpenRollup {
	var ds []*dossier.Dossier
	if projectRoot != "" && firstCycle > 0 {
		ds = dossier.ReadCommitted(projectRoot, firstCycle)
	}
	committed := make(map[int]bool, len(ds))
	for _, d := range ds {
		committed[d.Cycle] = true
	}
	for _, c := range cycles {
		if committed[c.Cycle] {
			continue
		}
		ds = append(ds, &dossier.Dossier{Cycle: c.Cycle, SpineFailOpens: c.SpineFailOpens})
	}
	rollup := dossier.RollupSpineFailOpens(ds, spineFailOpenWarnThreshold)
	if rollup.Total == 0 {
		return nil
	}
	for _, cycle := range rollup.OverThresholdCycles {
		fmt.Fprintf(warn, "[loop] WARN cycle %d took more than %d spine fail-opens "+
			"(a mandatory predecessor's handoff artifact was missing and the phase ran anyway) — "+
			"see spine_fail_opens in the summary and knowledge-base/cycles/cycle-%d.json\n",
			cycle, spineFailOpenWarnThreshold, cycle)
	}
	return &rollup
}

// sweepRulePromotions flips a shadow rule to enforce only after
// minShadowFires clean fires with zero anomalies; bridge.EnforceMeasuredRule
// re-validates against a fresh corpus before flipping. Best-effort: a missing
// dir/ledger is just absent evidence.
func sweepRulePromotions(stderr io.Writer, projectRoot string, cycles []core.CycleResult) {
	const minShadowFires = 5
	fires := map[string]int{}
	disqualified := map[string]bool{}
	for _, c := range cycles {
		ws := cycleWorkspace(projectRoot, c.Cycle)
		entries, err := os.ReadDir(ws)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), "-interactions.ndjson") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(ws, e.Name()))
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(data), "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				var out struct {
					Kind   string `json:"kind"`
					RuleID string `json:"rule_id"`
					Result string `json:"result"`
				}
				if json.Unmarshal([]byte(line), &out) != nil ||
					out.Kind != "rule_shadow_fire" || out.RuleID == "" {
					continue
				}
				if out.Result == "would_fire" {
					fires[out.RuleID]++
				} else {
					disqualified[out.RuleID] = true
				}
			}
		}
	}
	for id, n := range fires {
		if n < minShadowFires || disqualified[id] {
			continue
		}
		if err := bridge.EnforceMeasuredRule(projectRoot, id); err != nil {
			fmt.Fprintf(stderr, "[loop] WARN I4 enforce flip %s: %v\n", id, err)
			continue
		}
		fmt.Fprintf(stderr, "[loop] I4: rule %s measured-clean (%d shadow fires, 0 anomalies) — flipped to enforce\n", id, n)
	}
}

// fileUnexplainedOutcomeDefect signals that a terminal path escaped without
// recording its outcome, which is itself a defect. Idempotent per cycle
// (fixed filename); best-effort.
func fileUnexplainedOutcomeDefect(projectRoot string, cycle int, detail string) {
	dir := filepath.Join(projectRoot, ".evolve", "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	path := filepath.Join(dir, fmt.Sprintf("auto-unexplained-outcome-cycle-%d.json", cycle))
	if _, err := os.Stat(path); err == nil {
		return
	}
	body, err := json.MarshalIndent(map[string]any{
		"id":               fmt.Sprintf("unexplained-outcome-cycle-%d", cycle),
		"action":           fmt.Sprintf("Cycle %d ended FAILED_UNEXPLAINED (%s). Every terminal path must record a ship PASS, a salvage, or an abort_reason (ADR-0044 C1) — locate the escaping path and route it through recordPhaseOutcome.", cycle, detail),
		"priority":         "HIGH",
		"weight":           0.8,
		"evidence_pointer": fmt.Sprintf(".evolve/runs/cycle-%d/phase-timing.json", cycle),
		"injected_at":      time.Now().UTC().Format(time.RFC3339),
		"injected_by":      "loop-outcome-classifier",
	}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, body, 0o644)
}

// emitFatal is emit for abnormal exits: it records loop-fatal failure
// learning first, then emits. Plain emit stays in use for success exits and
// for paths that already recorded their failure (quota-pause empty-output) or
// structurally cannot (state_unwritable: the record write is what failed;
// unfinished-cycle guard: learning is captured downstream by the forced
// reset/resume).
func (lr *loopResult) emitFatal(w, stderr io.Writer, cfg loopConfig, cycle int) {
	recordLoopFatal(stderr, cfg, cycle, lr.StopReason)
	lr.emit(w)
}

// cycle may be 0 when unknown: Record's lastCycleNumber advance is monotonic,
// so a zero cycle cannot regress the counter.
func recordLoopFatal(stderr io.Writer, cfg loopConfig, cycle int, stopReason string) {
	now := time.Now().UTC()
	stop := "stop_reason=" + stopReason
	if _, err := failurelog.Record(filepath.Join(cfg.EvolveDir, "state.json"), "", failurelog.RecordRequest{
		Cycle:          cycle,
		Classification: string(failurelog.LoopFatal),
		Summary:        stop,
		Now:            now,
	}); err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: could not record loop-fatal (%s): %v\n", stopReason, err)
	}
	ev := faillearn.FailureEvent{
		Cycle:          cycle,
		FailedPhase:    stop,
		Scope:          faillearn.ScopeLoop,
		Classification: string(failurelog.LoopFatal),
		Verdict:        "FATAL",
		Summary:        fmt.Sprintf("batch stopped abnormally (%s) at cycle %d", stop, cycle),
		Now:            now,
	}
	if err := faillearn.WriteArtifacts(ev, "", filepath.Join(cfg.EvolveDir, "instincts", "lessons")); err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: could not write loop-fatal lesson: %v\n", err)
	}
}

func lastCycleIn(lr loopResult) int {
	if n := len(lr.Cycles); n > 0 {
		return lr.Cycles[n-1].Cycle
	}
	return 0
}
