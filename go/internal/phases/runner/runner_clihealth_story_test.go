package runner

// Slice-5 full-story replay (deterministic clock): the complete
// detect → remember → react → recover loop the cycle-283 incident lacked.
//
//	t0      dispatch: codex walls (85 + rate_limit report) → benched, claude
//	        carries the phase; benched_until comes from the pane's own
//	        "try again at 6:11 AM" hint
//	t0+5m   dispatch: chain starts at claude — zero codex boots
//	t0+7h   dispatch (bench expired): codex gets its canary shot first; it
//	        walls AGAIN → re-benched with strikes=2
//	then    dispatch: codex demoted again while the strike bench is active

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func storyRun(t *testing.T, root string, bridge core.Bridge, now time.Time) {
	t.Helper()
	hooks := &fakeHooks{
		phase: "auditor", agent: "evolve-auditor", model: "sonnet",
		prompt: "x", verdict: core.VerdictPASS, nextPhase: "ship",
	}
	r := New(Options{
		Hooks: hooks, Bridge: bridge, Prompts: fakePromptsFS("evolve-auditor", "x"),
		NowFn: func() time.Time { return now },
	})
	if _, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root,
		Workspace:   t.TempDir(),
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestCLIHealthFullStory(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	// Deterministic zone (NOT time.Local): ParseResetHint interprets the wall
	// clock in now's location, and the assertion below must mean the same
	// instant on every host (review: injected-clock discipline).
	loc := time.FixedZone("STORY", 8*3600)
	t0 := time.Date(2026, 6, 11, 0, 30, 0, 0, loc)

	// t0 — codex walls; the report's capturedAt must be >= run start (the
	// injected clock returns t0 for the runner, so stamp exactly t0).
	wallBridge := func() *escalatingBridge {
		return &escalatingBridge{
			scriptedBridge: scriptedBridge{responses: map[string]scriptedResp{
				"codex-tmux":  exit85(),
				"claude-tmux": {},
			}},
			escalateCLI: "codex-tmux", reportCLI: "codex-tmux",
			capturedAt: t0, pattern: "rate_limit",
		}
	}
	eb := wallBridge()
	storyRun(t, root, eb, t0)
	if eb.calls[0] != "codex-tmux" || eb.calls[1] != "claude-tmux" {
		t.Fatalf("t0 dispatch order=%v, want codex then claude fallback", eb.calls)
	}
	store := clihealth.NewStore(root, func() time.Time { return t0 })
	bench := store.Active()["codex"]
	if bench.Family == "" {
		t.Fatal("t0: wall not benched")
	}
	// The pane's own hint ("try again at 6:11 AM" +2min margin) wins over the
	// default cooldown.
	wantUntil := time.Date(2026, 6, 11, 6, 13, 0, 0, loc)
	if !bench.BenchedUntil.Equal(wantUntil) {
		t.Errorf("benched_until=%v, want %v (the wall's own reset hint)", bench.BenchedUntil, wantUntil)
	}

	// t0+5m — active bench: the chain must start at claude, zero codex boots.
	sb := &scriptedBridge{responses: map[string]scriptedResp{"claude-tmux": {}}}
	storyRun(t, root, sb, t0.Add(5*time.Minute))
	if len(sb.calls) != 1 || sb.calls[0] != "claude-tmux" {
		t.Fatalf("t0+5m dispatch=%v, want exactly [claude-tmux] (benched codex never boots)", sb.calls)
	}

	// t0+7h — bench expired: codex gets the canary-by-dispatch shot first and
	// walls AGAIN → re-benched with strikes incremented.
	t1 := t0.Add(7 * time.Hour)
	eb2 := wallBridge()
	eb2.capturedAt = t1
	storyRun(t, root, eb2, t1)
	if eb2.calls[0] != "codex-tmux" {
		t.Fatalf("t0+7h dispatch=%v, want codex first (expired bench = canary shot)", eb2.calls)
	}
	store1 := clihealth.NewStore(root, func() time.Time { return t1 })
	rebench := store1.Active()["codex"]
	if rebench.Family == "" {
		t.Fatal("t0+7h: second wall not re-benched")
	}
	if rebench.Strikes != 2 {
		t.Errorf("strikes=%d, want 2 (consecutive re-bench)", rebench.Strikes)
	}

	// While the strike bench is active, codex is demoted again.
	sb2 := &scriptedBridge{responses: map[string]scriptedResp{"claude-tmux": {}}}
	storyRun(t, root, sb2, t1.Add(time.Minute))
	if len(sb2.calls) != 1 || sb2.calls[0] != "claude-tmux" {
		t.Fatalf("post-re-bench dispatch=%v, want exactly [claude-tmux]", sb2.calls)
	}
}
