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
	// continued past under EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS (>1). Zero under
	// the default. Like RecoverableFailures it drives the rc=3 "completed but
	// with failures" exit contract.
	ContinuedFailures int `json:"continued_failures,omitempty"`
	// SpineFailOpens is the batch roll-up of spine-gate fail-opens
	// (spine-failopen-telemetry): the total, the per-phase breakdown, and the
	// cycles whose own count breached the WARN threshold. Absent on a clean
	// batch — the field appears only when there is an epidemic to see.
	SpineFailOpens *dossier.SpineFailOpenRollup `json:"spine_fail_opens,omitempty"`
	// CycleOutcomes is the R6 SLO classification per cycle (SHIPPED /
	// SALVAGED / FAILED_EXPLAINED / FAILED_UNEXPLAINED), computed from the
	// C1 records at emit time — the batch-level "every cycle delivers a
	// result" accounting the EVOLVE_PHASE_RECOVERY soak reads.
	CycleOutcomes []cycleOutcomeEntry `json:"cycle_outcomes,omitempty"`
	// classifyRoot, when set (the loop entry points set it once), makes
	// emit() populate CycleOutcomes from <root>/.evolve/runs/cycle-N.
	classifyRoot string
	// batchFirstCycle is the lowest cycle number this batch could have produced
	// (state.LastCycleNumber + 1 at boot). It bounds the committed-dossier window
	// the spine fail-open roll-up reads, which is how a batch whose cycles all ran
	// in lane subprocesses is counted at all. Zero = unknown ⇒ no corpus read (an
	// unbounded read would report the all-time total as this batch's).
	batchFirstCycle int
}

type cycleOutcomeEntry struct {
	Cycle   int    `json:"cycle"`
	Outcome string `json:"outcome"`
	Detail  string `json:"detail,omitempty"`
}

// emit writes lr to w as the canonical pretty-JSON dispatcher output.
// JSON format byte-identical to the previous inline marshaling — tests
// asserting on stop_reason / total_cost_usd / etc. continue to pass.
//
// Today loopResult only holds string/float64/bool/int/[]CycleResult,
// so MarshalIndent cannot fail. If a future field (channel, func,
// unencodable interface) breaks that, emit a structured error envelope
// instead of a silent empty line so the failure is observable —
// dispatchers and `evolve loop` consumers grep stop_reason.
func (lr *loopResult) emit(w io.Writer) {
	// R6: classify every cycle's ending from its C1 records at the single
	// output chokepoint (every exit path funnels here). A
	// FAILED_UNEXPLAINED additionally self-files an inbox defect — that
	// bucket means a terminal path escaped the C1 chokepoint, which is
	// itself a defect. Best-effort: classification must never break the
	// dispatcher contract.
	if lr.classifyRoot != "" && len(lr.Cycles) > 0 && lr.CycleOutcomes == nil {
		for _, c := range lr.Cycles {
			oc, detail := cyclehealth.ClassifyOutcome(cycleWorkspace(lr.classifyRoot, c.Cycle))
			lr.CycleOutcomes = append(lr.CycleOutcomes, cycleOutcomeEntry{Cycle: c.Cycle, Outcome: string(oc), Detail: detail})
			if oc == cyclehealth.OutcomeFailedUnexplained {
				fileUnexplainedOutcomeDefect(lr.classifyRoot, c.Cycle, detail)
			}
		}
		// R8.2 / I4 measured auto-enforce sweep rides the same batch-end
		// bookkeeping slot (evidence is evidence on every exit path).
		// Best-effort; never breaks the dispatcher contract.
		sweepRulePromotions(os.Stderr, lr.classifyRoot, lr.Cycles)
	}
	// Spine fail-open roll-up (spine-failopen-telemetry). Deliberately OUTSIDE the
	// classifyRoot block above, which is gated on len(lr.Cycles) > 0: a fleet batch
	// runs every cycle in a lane subprocess and appends NOTHING to lr.Cycles, yet
	// those are precisely the batches whose fail-opens need counting.
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
		// workspace-hygiene S5: an ABSENT gc.mode resolves to "shadow", not
		// "off". Shadow never mutates (plan + publish only), so the default is
		// safe, and a sweep that can only run after an operator edits
		// policy.json never drains the accumulated backlog. Enforce stays an
		// explicit policy.json decision.
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

// runRunDirGC is the L3.4 run-directory retention sweep: Discover→Plan→publish,
// and Apply only under enforce.
func runRunDirGC(cfg loopConfig, workspace, mode string, gcPol gc.Policy, stderr io.Writer) {
	runs, err := gc.Discover(cfg.EvolveDir, gc.DiscoverOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "[gc] WARN: discover failed: %v; writing empty manifest\n", err)
		runs = nil
	}
	manifest, err := gc.Plan(gc.Options{EvolveDir: cfg.EvolveDir, Runs: runs, Policy: gcPol})
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

// runWorktreeGC is workspace-hygiene S5: the batch-end wiring that finally gives
// S4's gc.PlanWorktrees/gc.ApplyWorktrees a production caller. It drains the
// accumulated worktree+branch backlog — plan and publish under shadow, plan,
// publish and apply under enforce. Fail-open: a ProjectRoot that is not a git
// repo warns and skips, leaving the run-dir sweep untouched.
func runWorktreeGC(cfg loopConfig, workspace, mode string, gcPol gc.Policy, stderr io.Writer) {
	if cfg.ProjectRoot == "" {
		// Never infer the repo from the process cwd: an unset ProjectRoot with
		// mode=enforce would aim `git branch -d` at whatever repo the binary
		// happens to run inside. Refuse loudly instead.
		fmt.Fprintf(stderr, "[gc] WARN: worktree sweep skipped: ProjectRoot is unset\n")
		return
	}
	opts := gc.WorktreeOptions{
		ProjectRoot:  cfg.ProjectRoot,
		WorktreeBase: filepath.Join(cfg.ProjectRoot, ".evolve", "worktrees"),
		EvolveDir:    cfg.EvolveDir,
		Policy:       gcPol.Worktrees,
		Exec:         sysexec.DefaultRunner,
	}
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

// publishGCManifest atomically writes v as pretty JSON to <workspace>/name
// (tmp+rename). Shared by both sweeps so the run-dir and worktree manifests are
// published identically. Returns false after warning on any failure.
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

// spineFailOpenWarnThreshold is the per-cycle fail-open count above which the
// batch summary escalates to a stderr WARN. A COMPILED constant, not policy: the
// threshold decides when a measurement is loud, and a policy knob here would let
// a noisy batch turn its own alarm off (the digestTopPatterns precedent). The
// value is the item's "e.g. 3" — a cycle's spine legitimately degrades once or
// twice under a driver bounce; four times is a pattern.
const spineFailOpenWarnThreshold = 3

// spineFailOpenRollup folds the batch's spine fail-opens into the summary block,
// WARNing on stderr for every cycle that breached the threshold. Returns nil for a
// clean batch so the summary omits the field entirely.
//
// TWO sources, because no single one sees a whole batch:
//
//   - the committed dossiers for cycles >= firstCycle. A fleet wave/pool lane is a
//     separate `evolve cycle run` subprocess whose CycleResult never returns to
//     this process (cmd_loop.go: the wave branch "does not append to lr.Cycles"),
//     and the standing fleet width is >1 — so folding only the in-memory results
//     would report zero for exactly the batches this telemetry was filed about.
//     Lanes DO commit their dossier into the parent's knowledge-base/cycles.
//   - the in-memory CycleResults, for any cycle with no dossier on disk (the
//     sequential/resume path, or a cycle whose best-effort dossier write failed).
//     Projected into the dossier shape the fold consumes — Cycle and
//     SpineFailOpens are the only fields it reads.
//
// The dossier wins when a cycle has both: it is the committed record, written FROM
// that same CycleResult, so preferring it costs nothing and dedupes the union.
// firstCycle <= 0 means "the batch window is unknown" and skips the corpus read
// entirely — reading the whole history would report an all-time total as if it were
// this batch's.
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

// sweepRulePromotions is the I4 measured auto-enforce sweep (R8.2): scan the
// batch's interaction ledgers for shadow-rule would-fire evidence and flip
// rules that cleared the bar — ≥minShadowFires fires, ZERO non-would_fire
// outcomes for that rule (conservative: any anomaly disqualifies), and a
// fresh healthy-corpus re-validation inside the flip itself
// (bridge.EnforceMeasuredRule). Until this sweep existed, "measured
// auto-enforce never fires" was true by construction (ADR-0045 I4 record).
// Best-effort throughout: a missing dir/ledger is just absent evidence.
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

// fileUnexplainedOutcomeDefect self-files an inbox item for the alarm
// bucket (R6.3): FAILED_UNEXPLAINED means the "every terminal path records
// its outcome" invariant (ADR-0044 C1) has a hole — exactly what the inbox
// exists to capture. Idempotent per cycle (fixed filename); best-effort.
func fileUnexplainedOutcomeDefect(projectRoot string, cycle int, detail string) {
	dir := filepath.Join(projectRoot, ".evolve", "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	path := filepath.Join(dir, fmt.Sprintf("auto-unexplained-outcome-cycle-%d.json", cycle))
	if _, err := os.Stat(path); err == nil {
		return // already filed
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

// emitFatal is emit for ABNORMAL exits: record the loop-fatal learning
// first (failure floor, inbox retro-always-invariant gap 3), then emit.
// Plain emit stays in use for success exits and for paths that already
// recorded their failure (quota-pause empty-output) or structurally
// cannot (state_unwritable — the record write is the thing that failed;
// unfinished-cycle guard — learning is captured downstream by the
// forced reset/resume).
func (lr *loopResult) emitFatal(w, stderr io.Writer, cfg loopConfig, cycle int) {
	recordLoopFatal(stderr, cfg, cycle, lr.StopReason)
	lr.emit(w)
}

// recordLoopFatal persists a batch-level failedApproaches entry
// (classification loop-fatal, stop_reason in the summary) plus a
// deterministic lesson artifact. Best-effort: a floor failure must
// never change the exit path — WARN is the only trace. cycle may be 0
// when unknown (Record's lastCycleNumber advance is monotonic, so a
// zero cycle cannot regress the counter).
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

// lastCycleIn returns the last attempted cycle number in the batch, 0
// when no cycle ran.
func lastCycleIn(lr loopResult) int {
	if n := len(lr.Cycles); n > 0 {
		return lr.Cycles[n-1].Cycle
	}
	return 0
}

// loopConfig is the resolved invocation. Extracted so --dry-run and
// tests can inspect what would be done without invoking the
// orchestrator.
