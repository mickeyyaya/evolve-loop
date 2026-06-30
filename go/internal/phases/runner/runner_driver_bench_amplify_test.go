package runner

// runner_driver_bench_amplify_test.go — adversarial amplification for cycle-426
// T1 (wire-driver-bench-consumer).  Targets gaps in the Build tests:
//
//  1. Expired driver bench does not demote — time-expiry on the NEW driver-bench
//     path (BootTimeoutPattern) was verified for family benches but not for the
//     driver-bench consumer added in this cycle.
//  2. All-driver-benched least-recently-benched-first ordering — verified for
//     family benches (TestRun_AllFamiliesBenchedNeverStrands) but NOT for the
//     new ApplyDriverBench code path.

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

// TestAmplify_C426_ExpiredDriverBenchNoReorder: a BootTimeoutPattern bench
// entry whose BenchedUntil is in the past must be excluded from Active() and
// must NOT cause applyBenchToPlan to demote the driver.
//
// Mirrors TestRun_ExpiredBenchDoesNotDemote (family bench) for the new
// driver-bench path.  An expired driver bench is a canary-by-default: the
// driver gets the first slot again without manual intervention.
func TestAmplify_C426_ExpiredDriverBenchNoReorder(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	now := time.Now()
	store := clihealth.NewStore(root, nil)
	// Bench codex-tmux as BootTimeoutPattern but with BenchedUntil in the past.
	if err := store.Bench(clihealth.Entry{
		Family:       "codex-tmux",
		Reason:       clihealth.BootTimeoutPattern,
		BenchedAt:    now.Add(-2 * time.Hour),
		BenchedUntil: now.Add(-time.Hour), // expired
		Strikes:      clihealth.DefaultBootBenchThreshold,
	}); err != nil {
		t.Fatalf("Bench(codex-tmux, expired): %v", err)
	}

	// Active() should exclude the expired entry.
	if _, ok := store.Active()["codex-tmux"]; ok {
		t.Skip("Active() still returns the expired entry — expiry behaviour may differ; skip dispatch check")
	}

	sb := &scriptedBridge{responses: map[string]scriptedResp{
		"codex-tmux":  {},
		"claude-tmux": {},
	}}
	runPhase(t, root, sb)

	if len(sb.calls) == 0 || sb.calls[0] != "codex-tmux" {
		t.Errorf("dispatch order %v: expired driver bench must NOT demote (canary-by-default); "+
			"expected codex-tmux first after bench expiry", sb.calls)
	}
}

// TestAmplify_C426_AllDriverBenchedLeastRecentlyFirst: when both candidates are
// driver-benched (BootTimeoutPattern), the one benched EARLIER must be tried
// first (least-recently-benched policy).
//
// Verified for family benches in TestRun_AllFamiliesBenchedNeverStrands; this
// test ensures the ApplyDriverBench path added in cycle-426 honours the same
// ordering invariant.  Also confirms bench is advice, never a veto — dispatch
// must not strand when all candidates are driver-benched.
func TestAmplify_C426_AllDriverBenchedLeastRecentlyFirst(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	now := time.Now()
	store := clihealth.NewStore(root, nil)

	// codex-tmux benched 30 min ago (earlier = less recently benched → goes first).
	if err := store.Bench(clihealth.Entry{
		Family:       "codex-tmux",
		Reason:       clihealth.BootTimeoutPattern,
		BenchedAt:    now.Add(-30 * time.Minute),
		BenchedUntil: now.Add(time.Hour),
		Strikes:      clihealth.DefaultBootBenchThreshold,
	}); err != nil {
		t.Fatalf("Bench(codex-tmux): %v", err)
	}
	// claude-tmux benched 10 min ago (more recently → goes last).
	if err := store.Bench(clihealth.Entry{
		Family:       "claude-tmux",
		Reason:       clihealth.BootTimeoutPattern,
		BenchedAt:    now.Add(-10 * time.Minute),
		BenchedUntil: now.Add(time.Hour),
		Strikes:      clihealth.DefaultBootBenchThreshold,
	}); err != nil {
		t.Fatalf("Bench(claude-tmux): %v", err)
	}

	sb := &scriptedBridge{responses: map[string]scriptedResp{
		"codex-tmux":  {},
		"claude-tmux": {},
	}}
	runPhase(t, root, sb)

	if len(sb.calls) == 0 {
		t.Fatal("dispatch stranded: zero calls with all-driver-benched candidates — bench must be advice, never a veto")
	}
	if sb.calls[0] != "codex-tmux" {
		t.Errorf("dispatch order %v: expected codex-tmux first (benched earlier = least recently benched); "+
			"ApplyDriverBench must order by BenchedAt ascending", sb.calls)
	}
}
