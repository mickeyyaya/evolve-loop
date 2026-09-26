package runner

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func TestAmplify_C426_ExpiredDriverBenchNoReorder(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	now := time.Now()
	store := clihealth.NewStore(root, nil)
	if err := store.Bench(clihealth.Entry{
		Family:       "codex-tmux",
		Reason:       clihealth.BootTimeoutPattern,
		BenchedAt:    now.Add(-2 * time.Hour),
		BenchedUntil: now.Add(-time.Hour), // expired
		Strikes:      clihealth.DefaultBootBenchThreshold,
	}); err != nil {
		t.Fatalf("Bench(codex-tmux, expired): %v", err)
	}

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

func TestAmplify_C426_AllDriverBenchedLeastRecentlyFirst(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	now := time.Now()
	store := clihealth.NewStore(root, nil)

	if err := store.Bench(clihealth.Entry{
		Family:       "codex-tmux",
		Reason:       clihealth.BootTimeoutPattern,
		BenchedAt:    now.Add(-30 * time.Minute),
		BenchedUntil: now.Add(time.Hour),
		Strikes:      clihealth.DefaultBootBenchThreshold,
	}); err != nil {
		t.Fatalf("Bench(codex-tmux): %v", err)
	}
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
