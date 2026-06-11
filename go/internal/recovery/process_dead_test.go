// process_dead_test.go — R3.3/R3.4 (concurrency-factory plan; inbox
// codex-update-menu-swallows-injection, cycles 274/277): "alive" must mean
// the agent PROCESS, not the pane. A wedged shell echoing prompt-lookalikes
// read as busy/live for 25+ min while the CLI was long gone.
//
// Pinned here:
//  1. Chain: Kind "process_dead" → ActionKillRetry even when the pane LOOKS
//     busy (dead process outranks busy-extend — the false-liveness trap);
//     integrity still outranks everything (ADR-0044 locked decision).
//  2. chainStallPolicy maps process_dead → StallKillRetry (the observer's
//     vocabulary).
//  3. Detector seeds (R3.3): the remaining zsh continuation variants from
//     the cycle-274/277 transcripts (dquote>, heredoc>) classify as
//     CauseDeadShell; a healthy busy pane stays unclassified.
package recovery

import (
	"strings"
	"testing"
)

func TestChain_ProcessDeadOutranksBusy(t *testing.T) {
	t.Parallel()
	d := Recover(RecoverInput{Kind: "process_dead", Busy: true})
	if d.Action != ActionKillRetry {
		t.Fatalf("RED (cycle-274): process_dead with a busy-LOOKING pane → %s (%s); want kill_retry — a dead process can render busy chrome forever, pane echo is not liveness", d.Action, d.Reason)
	}
}

func TestChain_IntegrityStillOutranksProcessDead(t *testing.T) {
	t.Parallel()
	d := Recover(RecoverInput{Kind: "process_dead", Integrity: true})
	if d.Action != ActionEscalate {
		t.Fatalf("integrity-adjacent state must never auto-recover, even with a dead process: got %s", d.Action)
	}
}

func TestChainStallPolicy_ProcessDeadKills(t *testing.T) {
	t.Parallel()
	action, reason := NewChainStallPolicy(6).Decide(StallEvent{Kind: "process_dead", Phase: "build"})
	if action != StallKillRetry {
		t.Fatalf("stall policy must map process_dead → kill_retry (got %s: %s)", action, reason)
	}
	if !strings.Contains(reason, "process") {
		t.Errorf("justification must name the dead process: %q", reason)
	}
}

func TestSeedDetector_RemainingShellContinuationVariants(t *testing.T) {
	t.Parallel()
	det := SeedDetector()
	for _, pane := range []string{
		"pasted prompt fragment\ndquote> ",
		"pasted prompt fragment\nheredoc> ",
	} {
		cause, _, ok := det.Detect(pane)
		if !ok || cause != CauseDeadShell {
			t.Errorf("RED (R3.3): %q not classified dead_shell (ok=%v cause=%v) — the cycle-274/277 transcript variants must all be seeded", pane, ok, cause)
		}
	}
	// Healthy negative: mid-sentence prose must not classify even when the
	// token sits at a LINE BOUNDARY (the seeds' anchor) — the genuinely
	// hard false-positive shape. KNOWN ACCEPTED LIMIT: substring seeds
	// cannot tell a line-leading "dquote> …" in quoted prose from a real
	// continuation prompt; what keeps this safe in production is the
	// pairing with the authoritative process check (R3.4) and the fact
	// that real continuation prompts render with nothing after them on the
	// cursor line. Pin the form that must NOT match: token mid-line.
	if cause, sub, ok := det.Detect("the zsh dquote> prompt indicates an unclosed string in your script"); ok {
		t.Errorf("mid-line prose classified as %v via %q — seed must anchor on a line boundary", cause, sub)
	}
}
