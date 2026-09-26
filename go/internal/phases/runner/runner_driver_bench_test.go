package runner

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

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

func TestDriverBench_NoStrikeNoReorder(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	sb := &scriptedBridge{responses: map[string]scriptedResp{
		"codex-tmux":  {},
		"claude-tmux": {},
	}}
	runPhase(t, root, sb)

	if len(sb.calls) == 0 || sb.calls[0] != "codex-tmux" {
		t.Errorf("dispatch order %v: expected codex-tmux first (no active bench)", sb.calls)
	}
}

func TestDriverBench_SingleStrikeNotYetBenched(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	store := clihealth.NewStore(root, nil)
	if _, err := store.RecordBootStrike("codex-tmux"); err != nil {
		t.Fatalf("RecordBootStrike: %v", err)
	}
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

func TestDriverBench_FamilyBenchCoexistsWithDriverBench(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	store := clihealth.NewStore(root, nil)
	now := time.Now()
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
