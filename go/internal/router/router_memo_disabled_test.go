package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestRoute_PostShip_MemoEnabled_RoutesToMemo(t *testing.T) {
	in := base("ship")
	in.Completed = []string{"scout", "build", "audit", "ship"}
	in.Cfg.PhaseEnable["memo"] = config.EnableOn
	d := Route(in, nil)
	if d.NextPhase != "memo" {
		t.Fatalf("post-ship with memo enabled → %q, want memo (precondition for the disabled-clamp contrast test)", d.NextPhase)
	}
}

func TestRoute_PostShip_MemoDisabled_ClampsSafely(t *testing.T) {
	in := base("ship")
	in.Completed = []string{"scout", "build", "audit", "ship"}
	in.Cfg.PhaseEnable["memo"] = config.EnableOff
	d := Route(in, nil)
	if d.NextPhase == "memo" {
		t.Fatalf("memo explicitly disabled (PhaseEnable[memo]=off) but Route() still returned memo — a fix that force-runs memo regardless of policy must fail this test")
	}
	if d.NextPhase != PhaseEnd {
		t.Errorf("post-ship with memo disabled → %q, want %q (retrospective untriggered on a plain PASS cycle, memo is last in canonicalOrder)", d.NextPhase, PhaseEnd)
	}
	found := false
	for _, p := range d.SkipPhases {
		if p == "memo" {
			found = true
		}
	}
	if !found {
		t.Errorf("SkipPhases = %v, want it to record memo as explicitly skipped (forensic trail)", d.SkipPhases)
	}
}
