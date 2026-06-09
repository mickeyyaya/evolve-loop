package bridge

// fatalpane.go — ADR-0044 C2: the fatal-pane fast-fail seam at the
// stop-review checkpoint (the bridge half of the Phase Recovery Pipeline's
// Analyze stage; the registry itself lives in internal/recovery, the single
// recovery owner).
//
// cycle-262 burned ~40 min waiting out the maxExtends backstop on two
// self-describing fatal pane states — the pane literally said "There's an
// issue with the selected model" / "Please restart Codex", but nothing read
// it. Worse, the bridge's own one-shot nudge echoed into the dead pane and
// counted as "progress" the next interval, buying extensions for a REPL that
// no longer existed. This seam consults the deterministic
// recovery.FatalPaneDetector BEFORE the StopReviewer each checkpoint, so a
// known-fatal state exits the wait in ONE interval and hands the phase to
// the runner's exit-81 fallback chain (which is exactly what rescued the
// cycle-262 build — 20 minutes too late).
//
// Stage discipline (EVOLVE_PHASE_RECOVERY, the one dial for the whole
// ADR-0044 program): classification must never act by default.
//
//	off     → detector not consulted; byte-identical legacy flow
//	shadow  → detect + log the would-be fast-fail; legacy verdict decides
//	          (the DEFAULT — behavior-neutral soak)
//	enforce → a fatal match on a non-Busy pane preempts the reviewer
//
// A Busy pane is never preempted regardless of stage: the stop-review
// layer's prime directive (never kill a working agent — see the cycle-254/255
// false-FAIL post-mortem in stopreview.go) outranks fast-fail.

import (
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// recoveryStageFromEnv resolves the bridge-side EVOLVE_PHASE_RECOVERY stage.
// The bridge is a subprocess, so it reads env directly (same pattern as
// EVOLVE_COMMIT_EVIDENCE; config.RolloutStages holds the orchestrator's
// view). Unset → "shadow" (the behavior-neutral first-ship default); an
// unrecognized value → "off" — a typo must never silently enable a
// kill-path (same posture as config.parseEvidenceStage).
func recoveryStageFromEnv(deps Deps) string {
	v, ok := lookupEnv(deps, "EVOLVE_PHASE_RECOVERY")
	if !ok || strings.TrimSpace(v) == "" {
		return "shadow"
	}
	switch s := strings.ToLower(strings.TrimSpace(v)); s {
	case "off", "shadow", "enforce":
		return s
	default:
		return "off"
	}
}

// fatalPaneVerdict consults the fatal-pane registry for one stop-review
// checkpoint. It returns (verdict, true) when enforcement preempts the
// reviewer — the caller skips StopReviewer.Review and applies the verdict —
// or (zero, false) when the legacy flow decides (off stage, shadow stage,
// busy pane, or no match). Shadow logs the would-be action to stderr so the
// soak leaves an auditable trail without changing behavior.
func fatalPaneVerdict(det *recovery.FatalPaneDetector, ev StopEvent, stage string, stderr io.Writer, pfx string) (ReviewVerdict, bool) {
	// "" treated as "off": recoveryStageFromEnv never returns "" (unset →
	// "shadow"), but a direct caller passing the zero value must not silently
	// enable a kill-path. Same posture for a nil detector — Detect is
	// nil-receiver-safe, but the safety belongs visibly at THIS boundary,
	// not buried in the callee.
	if det == nil || stage == "off" || stage == "" {
		return ReviewVerdict{}, false
	}
	cause, sig, ok := det.Detect(ev.StdoutTail)
	if !ok || ev.Busy {
		return ReviewVerdict{}, false
	}
	if stage == "enforce" {
		return ReviewVerdict{
			Action: ReviewStop,
			Reason: fmt.Sprintf("fatal pane state (%s): matched %q — fast-fail instead of burning the maxExtends backstop (ADR-0044 C2)", cause, sig),
		}, true
	}
	fmt.Fprintf(stderr, "%s stop-review shadow: would fast-fail — fatal pane state (%s) matched %q (EVOLVE_PHASE_RECOVERY=shadow; legacy verdict decides)\n", pfx, cause, sig)
	return ReviewVerdict{}, false
}
