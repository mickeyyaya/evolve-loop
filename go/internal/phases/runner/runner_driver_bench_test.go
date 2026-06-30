package runner

// runner_driver_bench_test.go — RED contract for the driver-bench consumer
// (cycle-426 T1): applyBenchToPlan must route BootTimeoutPattern entries from
// clihealth.Active() to llmroute.ApplyDriverBench (driver-keyed) so a 2-strike
// codex-tmux is demoted behind claude-tmux in the dispatch chain.
//
// Currently RED: applyBenchToPlan calls only ApplyBench (family-keyed).
// Active() returns the boot entry keyed "codex-tmux", ApplyBench looks for
// Family("codex-tmux")="codex" → key mismatch → no demotion.
// GREEN after fix: entries with Reason==BootTimeoutPattern are routed to
// ApplyDriverBench (driver-keyed), which matches "codex-tmux" and demotes it.

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

// TestDriverBench_TwoStrikesDemotesDriver: after DefaultBootBenchThreshold
// RecordBootStrike calls for codex-tmux, the dispatch chain must start at
// claude-tmux (the healthy fallback), not codex-tmux (the boot-benched driver).
func TestDriverBench_TwoStrikesDemotesDriver(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	store := clihealth.NewStore(root, nil)
	for i := 0; i < clihealth.DefaultBootBenchThreshold; i++ {
		if _, err := store.RecordBootStrike("codex-tmux"); err != nil {
			t.Fatalf("RecordBootStrike call %d: %v", i+1, err)
		}
	}
	if _, ok := store.Active()["codex-tmux"]; !ok {
		t.Fatal("setup: codex-tmux not active after threshold strikes")
	}

	sb := &scriptedBridge{responses: map[string]scriptedResp{
		"codex-tmux":  {},
		"claude-tmux": {},
	}}
	runPhase(t, root, sb)

	if len(sb.calls) == 0 || sb.calls[0] != "claude-tmux" {
		t.Errorf("RED: dispatch order %v — expected claude-tmux first; "+
			"codex-tmux has %d boot strikes but applyBenchToPlan's ApplyBench call "+
			"misses the driver-keyed entry (key 'codex-tmux' != family 'codex'); "+
			"fix: route BootTimeoutPattern entries to ApplyDriverBench",
			sb.calls, clihealth.DefaultBootBenchThreshold)
	}
}

// TestDriverBench_NoStrikeNoReorder: without active boot strikes, the dispatch
// chain must start at the profile's primary CLI (codex-tmux).
// Guards against over-demotion: no bench = no reorder.
func TestDriverBench_NoStrikeNoReorder(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	// No strikes recorded.
	sb := &scriptedBridge{responses: map[string]scriptedResp{
		"codex-tmux":  {},
		"claude-tmux": {},
	}}
	runPhase(t, root, sb)

	if len(sb.calls) == 0 || sb.calls[0] != "codex-tmux" {
		t.Errorf("dispatch order %v: expected codex-tmux first (no active bench)", sb.calls)
	}
}

// TestDriverBench_SingleStrikeNotYetBenched: one RecordBootStrike (below threshold)
// must NOT demote the driver — transient retry must remain at the front.
func TestDriverBench_SingleStrikeNotYetBenched(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	store := clihealth.NewStore(root, nil)
	if _, err := store.RecordBootStrike("codex-tmux"); err != nil {
		t.Fatalf("RecordBootStrike: %v", err)
	}
	// 1 strike < DefaultBootBenchThreshold → not in Active().
	if _, ok := store.Active()["codex-tmux"]; ok {
		t.Skip("RecordBootStrike(1) is already active — threshold changed; skip")
	}

	sb := &scriptedBridge{responses: map[string]scriptedResp{
		"codex-tmux":  {},
		"claude-tmux": {},
	}}
	runPhase(t, root, sb)

	if len(sb.calls) == 0 || sb.calls[0] != "codex-tmux" {
		t.Errorf("dispatch order %v: single-strike (below threshold) must not demote; "+
			"expected codex-tmux first", sb.calls)
	}
}

// TestDriverBench_FamilyBenchCoexistsWithDriverBench: a rate_limit family bench
// for "codex" + a boot-timeout driver bench for "codex-tmux" must both work
// independently. The family bench should still demote codex-tmux via ApplyBench
// even without the driver bench.
func TestDriverBench_FamilyBenchCoexistsWithDriverBench(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	store := clihealth.NewStore(root, nil)
	now := time.Now()
	// Only family bench (rate_limit). Driver bench absent.
	if err := store.Bench(clihealth.Entry{
		Family:       "codex",
		Reason:       "rate_limit",
		BenchedAt:    now,
		BenchedUntil: now.Add(time.Hour),
		Strikes:      1,
	}); err != nil {
		t.Fatalf("Bench(codex, rate_limit): %v", err)
	}

	sb := &scriptedBridge{responses: map[string]scriptedResp{
		"codex-tmux":  {},
		"claude-tmux": {},
	}}
	runPhase(t, root, sb)

	if len(sb.calls) == 0 || sb.calls[0] == "codex-tmux" {
		t.Errorf("dispatch order %v: family bench (rate_limit/codex) must still demote codex-tmux — "+
			"ApplyBench (family-keyed) must run for non-boot entries even after ApplyDriverBench is added", sb.calls)
	}
}
