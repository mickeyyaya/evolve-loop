package runner

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Test Amplification (cycle 435, black-box adversarial pass on top of the
// TDD-authored runner_dispatch_dedup_test.go). Reuses the scriptedBridge /
// writeFallbackProfile / fakeHooks / fakePromptsFS fixtures already present
// in this package (runner_fallback_test.go, runner_test.go) without reading
// runner.go's Run implementation. Covers the same gaps closed at the
// llmroute/core layers -- the full default trigger-code set and chains
// longer than 2 -- but end-to-end through r.Run(), proving the post-dedup
// delegation to llmroute.Dispatch preserves chain behavior for every
// trigger code, not just the one (exit=80) the pre-existing suite scripts.

// TestRunnerDispatch_AllDefaultTriggerExitCodesFallBackThroughRun (basic,
// table-driven, end-to-end): runner_fallback_test.go's canonical case only
// exercises exit=80. The cycle-435 goal names the full set [80 81 85 124
// 127] as what the shared llmroute.Dispatch must honor; this proves the
// runner's delegation carries all five through r.Run(), not just the one
// pre-existing example.
func TestRunnerDispatch_AllDefaultTriggerExitCodesFallBackThroughRun(t *testing.T) {
	for _, code := range []int{80, 81, 85, 124, 127} {
		code := code
		t.Run(fmt.Sprintf("exit=%d", code), func(t *testing.T) {
			hooks := &fakeHooks{
				phase: "auditor", agent: "evolve-auditor", model: "sonnet",
				prompt: "x", verdict: core.VerdictPASS, nextPhase: "ship",
			}
			sb := &scriptedBridge{
				responses: map[string]scriptedResp{
					"codex-tmux": {
						resp: core.BridgeResponse{ExitCode: code, Stderr: fmt.Sprintf("exit=%d", code)},
						err:  fmt.Errorf("bridge: launch exit=%d", code),
					},
					"claude-tmux": {}, // empty = success
				},
			}
			root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
			r := New(Options{
				Hooks:   hooks,
				Bridge:  sb,
				Prompts: fakePromptsFS("evolve-auditor", "x"),
			})

			resp, err := r.Run(context.Background(), core.PhaseRequest{
				ProjectRoot: root,
				Workspace:   t.TempDir(),
			})
			if err != nil {
				t.Fatalf("Run: exit=%d expected fallback to succeed, got err=%v", code, err)
			}
			if resp.Verdict != core.VerdictPASS {
				t.Errorf("Run: exit=%d verdict=%s, want PASS", code, resp.Verdict)
			}
			want := []string{"codex-tmux", "claude-tmux"}
			if len(sb.calls) != len(want) || sb.calls[0] != want[0] || sb.calls[1] != want[1] {
				t.Errorf("Run: exit=%d dispatched %v, want %v", code, sb.calls, want)
			}
		})
	}
}

// TestRunnerDispatch_ThreeCLIChainPreservesOrderThroughRun (edge: chain
// length != 2): the pre-existing dedup test only scripts a 1-fallback
// chain. A profile with TWO fallback entries must still walk past both
// failing intermediates in order before landing on the one that succeeds --
// proving the post-refactor Dispatch delegation isn't hardcoded to a
// 2-candidate assumption.
func TestRunnerDispatch_ThreeCLIChainPreservesOrderThroughRun(t *testing.T) {
	hooks := &fakeHooks{
		phase: "auditor", agent: "evolve-auditor", model: "sonnet",
		prompt: "x", verdict: core.VerdictPASS, nextPhase: "ship",
	}
	sb := &scriptedBridge{
		responses: map[string]scriptedResp{
			"agy-tmux": {
				resp: core.BridgeResponse{ExitCode: 81},
				err:  errors.New("bridge: launch exit=81"),
			},
			"codex-tmux": {
				resp: core.BridgeResponse{ExitCode: 80},
				err:  errors.New("bridge: launch exit=80"),
			},
			"claude-tmux": {}, // empty = success
		},
	}
	root := writeFallbackProfile(t, "evolve-auditor", "agy-tmux", []string{"codex-tmux", "claude-tmux"})
	r := New(Options{
		Hooks:   hooks,
		Bridge:  sb,
		Prompts: fakePromptsFS("evolve-auditor", "x"),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root,
		Workspace:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run: expected the third candidate to succeed, got err=%v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Run: verdict=%s, want PASS", resp.Verdict)
	}
	want := []string{"agy-tmux", "codex-tmux", "claude-tmux"}
	if len(sb.calls) != len(want) {
		t.Fatalf("Run: dispatched %v, want %v (both intermediate hops tried in order)", sb.calls, want)
	}
	for i, cli := range want {
		if sb.calls[i] != cli {
			t.Errorf("Run: dispatched[%d]=%q, want %q", i, sb.calls[i], cli)
		}
	}
}
