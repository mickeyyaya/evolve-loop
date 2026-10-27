package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type loopResult struct {
	StopReason          string                       `json:"stop_reason"`
	Cycles              []core.CycleResult           `json:"cycles"`
	TotalCost           float64                      `json:"total_cost_usd"`
	Resumed             bool                         `json:"resumed,omitempty"`
	RecoverableFailures int                          `json:"recoverable_failures,omitempty"`
	ContinuedFailures   int                          `json:"continued_failures,omitempty"`
	SpineFailOpens      *dossier.SpineFailOpenRollup `json:"spine_fail_opens,omitempty"`
	CycleOutcomes       []cycleOutcomeEntry          `json:"cycle_outcomes,omitempty"`
	classifyRoot        string
	batchFirstCycle     int
	BoundaryRefresh     *chainBoundaryRefreshLogEntry `json:"boundary_refresh,omitempty"`
	CLIUpdateHalt       []cliupdate.Result            `json:"cli_update_halt,omitempty"`
}

type cycleOutcomeEntry struct {
	Cycle   int    `json:"cycle"`
	Outcome string `json:"outcome"`
	Detail  string `json:"detail,omitempty"`
}

func (lr *loopResult) emit(w io.Writer) {
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
	sealLedgerGC(cfg, ledgerSealMode(mode, gcPol), gcPol.LedgerSealKeep(), stderr)
	runWorktreeGC(cfg, workspace, mode, gcPol, stderr)
	runDevWorktreeGC(cfg, workspace, gcPol, stderr)
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
	for _, w := range manifest.Warnings {
		fmt.Fprintf(stderr, "[gc] WARN: %s\n", w)
	}

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
		fmt.Fprintf(stderr, "[gc] WARN: worktree sweep skipped: ProjectRoot is unset\n")
		return
	}
	opts := worktreeGCOptions(cfg.ProjectRoot, cfg.EvolveDir, gcPol.Worktrees, stderr)
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
		fmt.Fprintf(stderr, "[gc] WARN: worktree enforce apply: %v\n", err)
	}
	fmt.Fprintf(stderr, "[gc] worktree enforce: applied %d items\n", len(manifest.Items))
}

func runDevWorktreeGC(cfg loopConfig, workspace string, gcPol gc.Policy, stderr io.Writer) {
	if cfg.ProjectRoot == "" {
		return
	}
	opts, err := devWorktreeGCOptions(cfg.ProjectRoot, cfg.EvolveDir, gcPol)
	if err != nil {
		fmt.Fprintf(stderr, "[gc] WARN: dev worktree sweep skipped: %v\n", err)
		return
	}
	manifest, err := gc.PlanDevWorktrees(context.Background(), opts)
	if err != nil {
		fmt.Fprintf(stderr, "[gc] WARN: dev worktree plan failed: %v\n", err)
		return
	}
	if manifest.Skipped != "" || !publishGCManifest(workspace, "dev-worktree-gc-manifest.json", manifest, stderr) {
		return
	}
	archive, _, _ := gcDevCounts(manifest)
	fmt.Fprintf(stderr, "[gc] dev worktree shadow: %d of %d idle past the archive limit (only evolve gc archives them)\n", archive, len(manifest.Items))
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

func worktreeGCOptions(projectRoot, evolveDir string, pol gc.WorktreesPolicy, log io.Writer) gc.WorktreeOptions {
	return gc.WorktreeOptions{
		ProjectRoot:  projectRoot,
		WorktreeBase: filepath.Join(projectRoot, ".evolve", "worktrees"),
		EvolveDir:    evolveDir,
		Policy:       pol,
		Exec:         sysexec.DefaultRunner,
		Log:          log,
	}
}

func devWorktreeGCOptions(projectRoot, evolveDir string, pol gc.Policy) (gc.DevOptions, error) {
	cwd, err := processGetwd()
	if err != nil {
		return gc.DevOptions{}, fmt.Errorf("the caller's directory is unreadable, so no dev worktree can be proven free of the caller: %w", err)
	}
	return gc.DevOptions{
		ProjectRoot: projectRoot,
		EvolveDir:   evolveDir,
		Cwd:         cwd,
		Policy:      pol.DevWorktrees,
		Exec:        sysexec.DefaultRunner,
		OpenPRs:     ghOpenPRUpdates,
	}, nil
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

const spineFailOpenWarnThreshold = 3

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

var unexplainedOutcomeMarshal = json.MarshalIndent

func fileUnexplainedOutcomeDefect(projectRoot string, cycle int, detail string) {
	dir := filepath.Join(projectRoot, ".evolve", "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	path := filepath.Join(dir, fmt.Sprintf("auto-unexplained-outcome-cycle-%d.json", cycle))
	if _, err := os.Stat(path); err == nil {
		return
	}
	body, err := unexplainedOutcomeMarshal(map[string]any{
		"id":               fmt.Sprintf("unexplained-outcome-cycle-%d", cycle),
		"action":           fmt.Sprintf("Cycle %d ended FAILED_UNEXPLAINED (%s). Every terminal path must record a ship PASS, a salvage, or an abort_reason (ADR-0044 C1) — locate the escaping path and route it through recordPhaseOutcome.", cycle, detail),
		"priority":         "HIGH",
		"priority_class":   inboxbatch.ClassDebuggability,
		"origin":           inboxbatch.OriginCycle,
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

func (lr *loopResult) emitFatal(w, stderr io.Writer, cfg loopConfig, cycle int) {
	recordLoopFatal(stderr, cfg, cycle, lr.StopReason)
	lr.emit(w)
}

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
