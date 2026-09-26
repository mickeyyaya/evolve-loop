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
	// A line-leading token in prose is an accepted limit that the process check
	// covers; pin the form that must not match: the token mid-line.
	if cause, sub, ok := det.Detect("the zsh dquote> prompt indicates an unclosed string in your script"); ok {
		t.Errorf("mid-line prose classified as %v via %q — seed must anchor on a line boundary", cause, sub)
	}
}
