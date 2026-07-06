package router

// router_memo_disabled_test.go — cycle-563 fix-memo-phase-dispatch, criterion 3
// (negative case): whatever fixes the silent post-ship memo-dispatch drop must
// not force memo to run unconditionally. With memo explicitly disabled via
// policy (PhaseEnable["memo"]==EnableOff), the exact same post-ship routing
// input that cycle-561's routing-decision-12.json shows resolving to "memo"
// must instead clamp to the next legal phase (here: "end", since retrospective
// is untriggered on a plain PASS cycle and memo is last in canonicalOrder) —
// and must NEVER return "memo". TestRoute_PostShip_MemoEnabled_RoutesToMemo is
// the contrasting positive case, proving the negative isn't vacuously true
// because memo was already unreachable from this input.

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
