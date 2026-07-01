package llmroute

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// TDD RED (cycle 435, task advisor-cli-fallback-chain / A1 + runner-dispatch-
// dedup / A2): Dispatch is the single home for the "walk the CLI chain,
// advance on a trigger exit, stop on success or a real failure" loop —
// extracted from runner.go's inline WS-G1 for-loop (runner.go:485-537) so the
// advisor (which today does a single un-fallback-able Launch,
// phase_advisor.go:248-288) and the runner consume ONE implementation
// ([[never_duplicate_centralize_via_design_patterns]]). These tests exercise
// Dispatch directly against a scripted launch closure — no bridge, no I/O —
// pinning the walk semantics independent of either caller.
//
// AC1 (primary exit=81 → falls back → succeeds) is exercised at this layer by
// TestDispatch_FallsBackOnTriggerExit; the advisor-level equivalent lives in
// core/phase_advisor_fallback_test.go (the advisor must actually WIRE this
// walk in — a green Dispatch alone doesn't prove the advisor calls it).

// apicover naming pin: every test above exercises DispatchResult's fields via
// a type-inferred `got := Dispatch(...)`, which never spells the type name in
// the test AST (apicover's naming check is a bare-identifier scan, not a type
// resolver — cycle-413/426/430 CI-break class). This explicit reference keeps
// the exported type named without changing any test's behavior.
var _ DispatchResult

// scriptedLaunch returns canned (exitCode, err) pairs keyed by call order and
// records every cli it was asked to launch, in order.
type scriptedLaunch struct {
	seq   []scriptedAttempt
	calls []string
}

type scriptedAttempt struct {
	exitCode int
	err      error
}

func (s *scriptedLaunch) launch(cli string) (int, error) {
	s.calls = append(s.calls, cli)
	i := len(s.calls) - 1
	if i >= len(s.seq) {
		i = len(s.seq) - 1
	}
	a := s.seq[i]
	return a.exitCode, a.err
}

// TestDispatch_SuccessOnFirstCandidate: a single-candidate plan (empty
// cli_fallback) that succeeds must dispatch exactly once — the byte-identical
// baseline every fallback test differs from (AC4: single-CLI chain must be
// byte-identical to today's un-chained behavior).
func TestDispatch_SuccessOnFirstCandidate(t *testing.T) {
	plan := Plan{Candidates: []string{"claude-tmux"}, Triggers: defaultFallbackOnExit}
	sl := &scriptedLaunch{seq: []scriptedAttempt{{exitCode: 0, err: nil}}}

	got := Dispatch(plan, sl.launch)

	if got.Err != nil {
		t.Fatalf("Dispatch: unexpected error %v", got.Err)
	}
	if got.CLI != "claude-tmux" {
		t.Errorf("Dispatch: CLI=%q, want claude-tmux", got.CLI)
	}
	if !reflect.DeepEqual(sl.calls, []string{"claude-tmux"}) {
		t.Errorf("Dispatch: launched %v, want exactly one call to claude-tmux (byte-identical single-CLI chain)", sl.calls)
	}
	if !reflect.DeepEqual(got.Attempts, []string{"claude-tmux"}) {
		t.Errorf("Dispatch: Attempts=%v, want [claude-tmux]", got.Attempts)
	}
}

// TestDispatch_FallsBackOnTriggerExit (AC1): primary exits 81 (a trigger),
// fallback succeeds — Dispatch must advance to and return the fallback's
// result, having launched BOTH candidates in order. This is the exact
// cycle-435 live failure (router-launch-error.txt: agy-tmux exit 81) replayed
// at the Dispatch layer.
func TestDispatch_FallsBackOnTriggerExit(t *testing.T) {
	plan := Plan{Candidates: []string{"agy-tmux", "claude-tmux"}, Triggers: []int{81}}
	sl := &scriptedLaunch{seq: []scriptedAttempt{
		{exitCode: 81, err: errors.New("bridge: launch exit=81")},
		{exitCode: 0, err: nil},
	}}

	got := Dispatch(plan, sl.launch)

	if got.Err != nil {
		t.Fatalf("Dispatch: expected fallback success, got err=%v", got.Err)
	}
	if got.CLI != "claude-tmux" {
		t.Errorf("Dispatch: CLI=%q, want claude-tmux (the fallback that succeeded)", got.CLI)
	}
	want := []string{"agy-tmux", "claude-tmux"}
	if !reflect.DeepEqual(sl.calls, want) {
		t.Errorf("Dispatch: launched %v, want %v (both candidates tried in order)", sl.calls, want)
	}
	if !reflect.DeepEqual(got.Attempts, want) {
		t.Errorf("Dispatch: Attempts=%v, want %v", got.Attempts, want)
	}
}

// TestDispatch_StopsOnNonTriggerExit (AC3, negative — the strongest
// anti-no-op signal): a non-trigger exit is a REAL failure, not a
// dispatch-layer stall. Dispatch must stop immediately and must NEVER call
// the second candidate — silently rerouting a genuine FAIL would hide it as
// a CLI hiccup.
func TestDispatch_StopsOnNonTriggerExit(t *testing.T) {
	plan := Plan{Candidates: []string{"agy-tmux", "claude-tmux"}, Triggers: []int{81}}
	sl := &scriptedLaunch{seq: []scriptedAttempt{
		{exitCode: 1, err: errors.New("real failure, not a trigger")},
	}}

	got := Dispatch(plan, sl.launch)

	if got.Err == nil {
		t.Fatalf("Dispatch: expected the non-trigger error to surface, got nil")
	}
	if len(sl.calls) != 1 {
		t.Fatalf("Dispatch: launched %v, want exactly ONE call — a non-trigger exit must never reroute to the fallback", sl.calls)
	}
	if got.CLI != "agy-tmux" {
		t.Errorf("Dispatch: CLI=%q, want agy-tmux (the sole attempt)", got.CLI)
	}
}

// TestDispatch_AllCandidatesFailReturnsLastError (AC2): every candidate
// exhausts its trigger exit — Dispatch must return the LAST attempt's error
// (not silently swallow it), so the caller can degrade to its backstop (the
// advisor's static-spine degrade; the runner's FAIL classification).
func TestDispatch_AllCandidatesFailReturnsLastError(t *testing.T) {
	plan := Plan{Candidates: []string{"agy-tmux", "claude-tmux"}, Triggers: []int{81}}
	lastErr := errors.New("claude-tmux: launch exit=81 too")
	sl := &scriptedLaunch{seq: []scriptedAttempt{
		{exitCode: 81, err: errors.New("agy-tmux: launch exit=81")},
		{exitCode: 81, err: lastErr},
	}}

	got := Dispatch(plan, sl.launch)

	if got.Err == nil {
		t.Fatalf("Dispatch: expected an error when every candidate is exhausted, got nil")
	}
	if !errors.Is(got.Err, lastErr) && got.Err.Error() != lastErr.Error() {
		t.Errorf("Dispatch: Err=%v, want the LAST candidate's error (%v)", got.Err, lastErr)
	}
	want := []string{"agy-tmux", "claude-tmux"}
	if !reflect.DeepEqual(sl.calls, want) {
		t.Errorf("Dispatch: launched %v, want %v (both candidates exhausted)", sl.calls, want)
	}
}

// TestChainFor_PrimaryPlusDedupedFallback (H2/H3 seam): ChainFor builds a Plan
// from an EXPLICIT already-resolved primary (never re-derived via
// resolvePrimary, which would re-read profile.cli and ignore a bench-aware
// composition-root swap) plus the profile's cli_fallback chain, deduped.
func TestChainFor_PrimaryPlusDedupedFallback(t *testing.T) {
	prof := &profiles.Profile{
		CLI:               "agy-tmux", // must be IGNORED — primary is passed explicitly
		CLIFallback:       []string{"claude-tmux", "agy-tmux"},
		CLIFallbackOnExit: []int{81},
	}
	got := ChainFor("codex-tmux", prof)

	want := []string{"codex-tmux", "claude-tmux"} // primary first; dup of primary in fallback dropped
	if !reflect.DeepEqual(got.Candidates, want) {
		t.Errorf("ChainFor: Candidates=%v, want %v (explicit primary honored over profile.cli, dedup preserved)", got.Candidates, want)
	}
	if !reflect.DeepEqual(got.Triggers, []int{81}) {
		t.Errorf("ChainFor: Triggers=%v, want [81] (profile.cli_fallback_on_exit)", got.Triggers)
	}
}

// TestChainFor_NilProfileSingleCandidateDefaultTriggers (AC4 edge case): no
// profile on disk must degrade to a single-candidate chain on the default
// trigger set — never panic on a nil profile.
func TestChainFor_NilProfileSingleCandidateDefaultTriggers(t *testing.T) {
	got := ChainFor("claude-tmux", nil)

	if !reflect.DeepEqual(got.Candidates, []string{"claude-tmux"}) {
		t.Errorf("ChainFor: Candidates=%v, want [claude-tmux]", got.Candidates)
	}
	if !reflect.DeepEqual(got.Triggers, defaultFallbackOnExit) {
		t.Errorf("ChainFor: Triggers=%v, want the package default %v", got.Triggers, defaultFallbackOnExit)
	}
}
