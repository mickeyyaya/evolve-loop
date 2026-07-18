package bridge

import (
	"reflect"
	"testing"
)

// model_failover_test.go — the within-tier model-failover loop logic. Proves the
// axis engages on exit=85 and NOT on other codes (the RED repro for the audit
// hard-block: model #1 walls, model #2 succeeds → the phase succeeds via #2).

func TestDispatchModelFailover_AdvancesOn85ToSuccess(t *testing.T) {
	var calls []string
	used, code := dispatchModelFailover([]string{"fable", "opus", "sonnet"}, func(m string) int {
		calls = append(calls, m)
		if m == "fable" {
			return ExitUnknownPrompt // 85 — Fable-5 per-model wall
		}
		return ExitOK
	}, nil)
	if used != "opus" || code != ExitOK {
		t.Errorf("used=%q code=%d, want opus/ExitOK (advanced past the walled model)", used, code)
	}
	if !reflect.DeepEqual(calls, []string{"fable", "opus"}) {
		t.Errorf("calls=%v, want [fable opus] — stops at the first success, never reaches sonnet", calls)
	}
}

func TestDispatchModelFailover_WholeChainWalls_ReturnsLast85(t *testing.T) {
	var calls []string
	used, code := dispatchModelFailover([]string{"fable", "opus"}, func(m string) int {
		calls = append(calls, m)
		return ExitUnknownPrompt
	}, nil)
	if used != "opus" || code != ExitUnknownPrompt {
		t.Errorf("used=%q code=%d, want opus/85 (chain exhausted → caller escalates to CLI/tier fallback)", used, code)
	}
	if len(calls) != 2 {
		t.Errorf("calls=%v, want both models attempted before giving up", calls)
	}
}

func TestDispatchModelFailover_NonQuotaFailureDoesNotAdvance(t *testing.T) {
	var calls []string
	used, code := dispatchModelFailover([]string{"fable", "opus"}, func(m string) int {
		calls = append(calls, m)
		return ExitREPLBootTimeout // 80 — a boot problem, not a model-quota wall
	}, nil)
	if used != "fable" || code != ExitREPLBootTimeout {
		t.Errorf("used=%q code=%d, want fable/80 — a non-85 failure must NOT switch models", used, code)
	}
	if len(calls) != 1 {
		t.Errorf("calls=%v, want only [fable] (non-85 short-circuits; switching models would waste attempts)", calls)
	}
}

func TestDispatchModelFailover_FirstModelSucceeds_NoAdvance(t *testing.T) {
	var calls []string
	used, code := dispatchModelFailover([]string{"fable", "opus"}, func(m string) int {
		calls = append(calls, m)
		return ExitOK
	}, nil)
	if used != "fable" || code != ExitOK || len(calls) != 1 {
		t.Errorf("used=%q code=%d calls=%v, want fable/ExitOK/[fable] (no fallback when the primary works)", used, code, calls)
	}
}

// TestDispatchModelsFor_NamedSessionIsSingleShot (go-review HIGH): a pinned
// SessionName (swarm dispatch) must yield a SINGLE-element chain regardless of any
// catalog chain — a named session reattaches the same REPL on attempt 2+, so a
// mid-chain model switch would silently not take effect and mis-attribute telemetry.
func TestDispatchModelsFor_NamedSessionIsSingleShot(t *testing.T) {
	got := dispatchModelsFor("claude-tmux", "deep", "swarm-session-1")
	if len(got) != 1 || got[0] != "deep" {
		t.Errorf("named-session dispatch must be single-shot [deep], got %v", got)
	}
}

// TestDispatchModelFailover_EmptyChainFailsLoud (go-review LOW): an empty chain
// must NOT fabricate an ExitOK success — it never launches and returns loud.
func TestDispatchModelFailover_EmptyChainFailsLoud(t *testing.T) {
	used, code := dispatchModelFailover(nil, func(string) int {
		t.Fatal("must not launch on an empty chain")
		return 0
	}, nil)
	if code == ExitOK {
		t.Errorf("empty chain must fail loud, not fabricate ExitOK; got used=%q code=%d", used, code)
	}
}

func TestDispatchModelFailover_OnStepLogsEachAdvance(t *testing.T) {
	var steps [][2]string
	dispatchModelFailover([]string{"fable", "opus", "sonnet"}, func(m string) int {
		if m == "sonnet" {
			return ExitOK
		}
		return ExitUnknownPrompt // fable + opus both wall
	}, func(from, to string) {
		steps = append(steps, [2]string{from, to})
	})
	want := [][2]string{{"fable", "opus"}, {"opus", "sonnet"}}
	if !reflect.DeepEqual(steps, want) {
		t.Errorf("steps=%v, want %v (every advance logged, never silent)", steps, want)
	}
}
