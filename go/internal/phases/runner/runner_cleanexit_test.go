package runner

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// The bounded settle-retry now sits on the common clean-exit artifact-read path, so
// any pre-existing test whose deliverable doesn't verify well-formed on the first
// probe would pay real time.Sleep. Flip the package clock to a no-op for the whole
// test binary — production keeps time.Sleep. Tests that assert retry COUNTS still
// inject their own SleepFn (that's an explicit override, unaffected by this).
func init() { settleSleep = func(time.Duration) {} }

// TestRun_NonTimeout_CleanExitIdle_DeliverableSettlesOnRetry_PrefersFile pins the
// cycle-603/899 class (≥10 identical-goal_hash false-FAILs, 877→899). An agent
// EXITS CLEANLY (exit 0 — no bridge error, no timeout, no ctx-cancel) after
// writing a valid PASS deliverable, then IDLES ("Contemplating…") without a clean
// completion signal. The presence probe (verifyFn) races that transient idle
// state and misreports the deliverable not-OK on the first check(s). The runner
// must settle-retry the on-disk deliverable — exactly as the reconcile path
// already does for the timeout race — and record PASS from the FILE, never
// synthesize FAIL from multi-phase-contaminated scrollback (which echoes other
// phases' Deliverable-Contract example sentinels).
//
// Before the fix the clean-exit path did a SINGLE-SHOT verify (runner.go:715), so
// a racy first check flipped a genuine PASS to a scrollback-synthesized FAIL.
func TestRun_NonTimeout_CleanExitIdle_DeliverableSettlesOnRetry_PrefersFile(t *testing.T) {
	genuine := "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"
	// Raw tmux scrollback contaminated with OTHER phases' prompt-echoed example
	// sentinels — exactly what cycle-899 Classified into a false FAIL.
	noisyStdout := "Deliverable Contract example (PASS):\n" +
		"<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n" +
		"Deliverable Contract example (FAIL):\n" +
		"<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\"} -->\n" +
		"(prompt-echoed examples, not the agent's real report)\n"
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	// noisyStdoutBridge exits CLEANLY (no error) — it writes fileContent to the
	// deliverable and returns stdout as scrollback. This is the clean-exit path.
	nb := &noisyStdoutBridge{fileContent: genuine, stdout: noisyStdout}

	// The presence probe races the idle state: the first two checks miss, the
	// third — within the bounded settle window — catches the well-formed PASS.
	calls := 0
	settling := func(string, phasecontract.Roots) (deliverable.Result, error) {
		calls++
		if calls < 3 {
			return deliverable.Result{OK: false}, nil
		}
		return deliverable.Result{OK: true}, nil
	}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   nb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: settling,
		SleepFn:  func(time.Duration) {}, // deterministic: no real settle delay
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hooks.gotArtifact != genuine {
		t.Errorf("clean-exit-idle: Classify received contaminated scrollback instead of the settled on-disk deliverable;\n got  %q\n want %q", hooks.gotArtifact, genuine)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%q, want PASS (on-disk deliverable settled on retry, must not synthesize FAIL from scrollback)", resp.Verdict)
	}
	if calls < 3 {
		t.Errorf("clean-exit path must settle-retry the presence probe (mirroring the reconcile path); got only %d verify call(s)", calls)
	}
}

// TestRun_NonTimeout_CleanExitIdle_GenuineFAILOnDisk_RecordsFromFileNotScrollback
// — the anti-gaming complement of the fix: preferring the on-disk deliverable must
// never LAUNDER a verdict in EITHER direction. A clean-exit agent that writes a
// genuine FAIL deliverable, with PASS-noise in the scrollback, must Classify the
// FILE (→ FAIL), not the scrollback (→ a laundered PASS). The file is authoritative
// for FAIL exactly as it is for PASS; the settle-retry can only converge on the
// on-disk verdict, never invent a better one.
func TestRun_NonTimeout_CleanExitIdle_GenuineFAILOnDisk_RecordsFromFileNotScrollback(t *testing.T) {
	genuineFail := "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\"} -->\n"
	noisyPassStdout := "prompt example (PASS): <!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictFAIL}
	nb := &noisyStdoutBridge{fileContent: genuineFail, stdout: noisyPassStdout}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   nb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(deliverable.Result{OK: true}, nil), // a well-formed FAIL verifies OK
		SleepFn:  func(time.Duration) {},
	})
	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hooks.gotArtifact != genuineFail {
		t.Errorf("on-disk FAIL must be Classified from the FILE, not laundered from PASS-noisy scrollback;\n got  %q\n want %q", hooks.gotArtifact, genuineFail)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("verdict=%q, want FAIL — the file is authoritative in BOTH directions", resp.Verdict)
	}
}

// TestRun_NonTimeout_CleanExitIdle_DeliverableNeverSettles_CoherentFailNotPane
// — the settle-WAIT is BOUNDED: a clean-exit CONTRACTED phase whose deliverable never
// verifies (a genuinely hung/absent report, not a settle race) must yield a coherent
// deliverable-production FAIL — Classify receives an EMPTY artifact, never the lossy
// pane. The loop must not spin, and it must never manufacture a PASS from an absent
// deliverable NOR a FAIL scraped from prompt-contaminated scrollback.
func TestRun_NonTimeout_CleanExitIdle_DeliverableNeverSettles_CoherentFailNotPane(t *testing.T) {
	stdout := "raw scrollback with no clean deliverable\n"
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictFAIL}
	nb := &noisyStdoutBridge{fileContent: "", stdout: stdout}
	calls := 0
	neverSettles := func(string, phasecontract.Roots) (deliverable.Result, error) {
		calls++
		return deliverable.Result{OK: false}, nil
	}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   nb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: neverSettles,
		SleepFn:  func(time.Duration) {},
	})
	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hooks.gotArtifact != "" {
		t.Errorf("a never-settling CONTRACTED deliverable must yield a coherent FAIL (empty artifact to Classify), never the lossy pane; got %q", hooks.gotArtifact)
	}
	if calls <= 1 {
		t.Errorf("clean-exit path must RE-VERIFY (settle-wait), not single-shot; got %d verify call(s)", calls)
	}
	if calls > reconcileSettleRetries+1 {
		t.Errorf("settle-retry must be BOUNDED at %d calls (1 + %d retries) so the loop cannot spin; got %d", reconcileSettleRetries+1, reconcileSettleRetries, calls)
	}
	_ = resp
}

// ── Verdict-authority regression matrix (the "deliverable is the source of truth"
// invariant, unified across every completion path) ─────────────────────────────
//
// The recorded verdict must come from the AGENT'S ON-DISK DELIVERABLE whenever it
// enforce-verifies, and NEVER from multi-phase-contaminated bridge scrollback. Each
// completion path re-verifies with the SAME bounded settle-retry
// (verifyReconcileDeliverable). This suite pins the full grid:
//
//   completion path        | valid PASS file | settles on retry | genuine FAIL file | never-settles / absent
//   -----------------------|-----------------|------------------|-------------------|-----------------------
//   clean-exit (exit 0)    | PrefersFile...  | CleanExitIdle... | ...GenuineFAIL... | ...NeverSettles...     ← THIS FILE (was the gap: cycle-603/899)
//   artifact-timeout (81)  | Timeout_Well... | Timeout_Settles..| Timeout_Sentinel- | Timeout_NeverSettles..
//   transient (80/85/86)   | Transient_Well..| Transient_Settle.| Transient_Sentinel| Transient_NotWell...
//   ctx-cancel (-1)        | (→ transient via IsInfraTeardownError; engine classifies -1+ctx.Err → transient)
//
// Invariants pinned across ALL cells:
//   • PASS-preservation: a genuine on-disk PASS is recorded PASS (never dropped to a scrollback FAIL).
//   • FAIL-preservation: a genuine on-disk FAIL is recorded FAIL (never laundered to a scrollback PASS).
//   • Bounded + fail-open: an absent/never-settling deliverable falls back to stdout after ≤ reconcileSettleRetries+1
//     verifies — the retry can only UPGRADE toward the real deliverable, never invent one, and the loop cannot spin.
