// orchestrator_phaseoutcome_test.go — ADR-0044 C1 (Slice 1) RED tests:
// single-source phase-outcome recording.
//
// cycle-262 (2026-06-09) located fork: the build phase's CLI fallback
// SUCCEEDED (codex exit 81 → claude exit 0, build-report.md PASS, the runner
// returned PASS/nil), then the post-phase tree-diff guard CORRECTLY aborted
// the cycle (the fallback builder wrote the tracked config
// .evolve/commit-prefix-scope.json to the MAIN tree, and recoverBuildLeak
// deliberately skips .evolve/ paths). The abort path — like EVERY abort path
// between runner.Run returning and the happy-path recording site
// (orchestrator.go ~2084) — returned without recording the phase outcome: no
// phase-timing.json entry, no <phase>-usage.json, no PhasesRun membership.
// Reality (build ran, burned tokens, PASSed) diverged from the record (build
// never happened) — the D1 divergence ADR-0044 C1 makes structurally
// impossible.
//
// Contract encoded here (the C1 chokepoint): EVERY terminal disposition of a
// phase dispatch — happy advance AND each abort return (exhausted retries,
// non-canonical verdict, review-gate reject, tree-guard abort, ledger append
// failure) — records the outcome exactly once: PhasesRun membership +
// phase-timing.json entry + <phase>-usage.json sidecar, carrying the phase's
// own canonical verdict (synthesizing FAIL when none exists — NEVER PASS)
// and, on aborts, a non-empty abort_reason. Cycle-level semantics are
// unchanged: a tree-guard abort still fails the cycle; recording reflects
// reality, it does not resurrect the cycle.
//
// RED note: these tests compile against existing API only (same approach as
// orchestrator_timing_test.go) and fail at RUNTIME today — the abort paths
// return before any recording happens.
package core

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// outcomeRunner PASSes its phase with a scripted cost/duration after running
// a side effect — models cycle-262's build: real work done (tokens burned,
// PASS report) with an optional main-tree leak that trips the tree-diff guard.
type outcomeRunner struct {
	name       string
	verdict    string
	costUSD    float64
	durationMS int64
	onRun      func()
}

func (r *outcomeRunner) Name() string { return r.name }
func (r *outcomeRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	if r.onRun != nil {
		r.onRun()
	}
	return PhaseResponse{
		Phase:        r.name,
		Verdict:      r.verdict,
		ArtifactsDir: req.Workspace,
		CostUSD:      r.costUSD,
		DurationMS:   r.durationMS,
	}, nil
}

// initOutcomeRepo creates a real git repo whose committed tree contains the
// tracked config file cycle-262's builder leaked (.evolve/commit-prefix-scope.json).
// The default gitDirtyPaths runs real git against it, so the tree-diff guard
// exercises its production code path — and recoverBuildLeak's deliberate
// ".evolve/ paths are never relocated" skip applies exactly as it did live.
func initOutcomeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	p := filepath.Join(root, ".evolve", "commit-prefix-scope.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"prefixes":[]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("init", "-q")
	git("add", ".evolve/commit-prefix-scope.json")
	git("commit", "-q", "-m", "init")
	return root
}

// readTimingEntries unmarshals <workspace>/phase-timing.json. Fatal when the
// file is missing — every test here runs at least one phase, and the deferred
// writer flushes on both success AND abort returns.
func readTimingEntries(t *testing.T, workspace string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(workspace, "phase-timing.json"))
	if err != nil {
		t.Fatalf("phase-timing.json must exist (deferred writer flushes on abort too): %v", err)
	}
	var entries []map[string]any
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("phase-timing.json must be a JSON array: %v\n%s", err, data)
	}
	return entries
}

// timingEntryFor returns the single entry for phase, failing on absence or
// duplicates (the single-chokepoint invariant: one record per dispatch).
func timingEntryFor(t *testing.T, entries []map[string]any, phase string) map[string]any {
	t.Helper()
	var found []map[string]any
	for _, e := range entries {
		if e["phase"] == phase {
			found = append(found, e)
		}
	}
	if len(found) == 0 {
		t.Fatalf("phase-timing.json has NO entry for %q — the phase ran but the record says it never happened (entries=%v)", phase, entries)
	}
	if len(found) > 1 {
		t.Fatalf("phase-timing.json has %d entries for %q, want exactly 1 (single chokepoint): %v", len(found), phase, found)
	}
	return found[0]
}

func requireUsageSidecar(t *testing.T, workspace, phase string) phaseUsageSidecar {
	t.Helper()
	path := filepath.Join(workspace, phase+"-usage.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s must be written for every terminal disposition (cycle-262: missing build-usage.json was the divergence): %v", path, err)
	}
	var sc phaseUsageSidecar
	if err := json.Unmarshal(data, &sc); err != nil {
		t.Fatalf("%s must be valid JSON: %v\n%s", path, err, data)
	}
	return sc
}

func phasesRunContains(phases []Phase, want Phase) bool {
	for _, p := range phases {
		if p == want {
			return true
		}
	}
	return false
}

// TestPhaseOutcome_TreeGuardAbort_RecordsBuildOutcome pins the cycle-262
// CLASS: build PASSes with real cost, leaves the main tree dirty in a way
// recovery cannot repair, the tree-diff guard aborts the cycle (CORRECT),
// and the build outcome must STILL be recorded: PhasesRun membership, a
// phase-timing entry carrying the agent's own PASS + cost + duration + a
// non-empty abort_reason, and build-usage.json.
//
// Fixture note: the original fixture was 262's literal leak (a tracked
// .evolve/commit-prefix-scope.json edit) — that is now RECOVERABLE by design
// (the deliverable allowlist relocates it; pinned in
// buildleak_recover_test.go), so this test uses a STAGED RENAME of a tracked
// file, which no recovery branch handles — the canonical still-unrecoverable
// main-tree mutation.
func TestPhaseOutcome_TreeGuardAbort_RecordsBuildOutcome(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	t.Parallel()
	root := initOutcomeRepo(t)
	runners := buildRunners(nil)
	runners[PhaseBuild] = &outcomeRunner{
		name: string(PhaseBuild), verdict: VerdictPASS,
		costUSD: 0.42, durationMS: 1234,
		onRun: func() {
			// A staged rename of a tracked file: porcelain 'R ' matches no
			// recovery branch → the guard re-check stays dirty → abort.
			cmd := exec.Command("git", "-C", root, "mv", ".evolve/commit-prefix-scope.json", ".evolve/commit-prefix-scope.renamed.json")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("leak rename: %v\n%s", err, out)
			}
		},
	}
	st := &fakeStorage{}
	led := &fakeLedger{}
	// Non-empty worktree path arms the tree-diff guard for the guarded phases.
	o := NewOrchestrator(st, led, runners, WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})

	// The guard must still abort — recording reflects reality, it never
	// resurrects a cycle whose builder escaped its worktree.
	if err == nil {
		t.Fatalf("tree-guard must still abort the cycle on a real leak; got nil error (phases=%v)", res.PhasesRun)
	}
	if !strings.Contains(err.Error(), "tree-diff") {
		t.Errorf("abort must come from the tree-diff guard; got: %v", err)
	}

	// The record must reflect that build RAN.
	if !phasesRunContains(res.PhasesRun, PhaseBuild) {
		t.Errorf("PhasesRun=%v must contain build — the phase dispatched and completed (cycle-262 hid it entirely)", res.PhasesRun)
	}
	workspace := cycleWorkspaceDir(root, res.Cycle)
	entry := timingEntryFor(t, readTimingEntries(t, workspace), string(PhaseBuild))
	if v, _ := entry["verdict"].(string); v != VerdictPASS {
		t.Errorf("build timing verdict=%q, want PASS (the agent's own verdict — reconciliation never rewrites it)", v)
	}
	if c, _ := entry["cost_usd"].(float64); c != 0.42 {
		t.Errorf("build timing cost_usd=%v, want 0.42 (the burned tokens must be accounted)", entry["cost_usd"])
	}
	if d, _ := entry["duration_ms"].(float64); int64(d) != 1234 {
		t.Errorf("build timing duration_ms=%v, want 1234", entry["duration_ms"])
	}
	if reason, _ := entry["abort_reason"].(string); reason == "" {
		t.Errorf("build timing entry must carry a non-empty abort_reason (the guard abort), got %v", entry)
	}
	sc := requireUsageSidecar(t, workspace, string(PhaseBuild))
	if sc.Verdict != VerdictPASS {
		t.Errorf("build-usage.json verdict=%q, want PASS", sc.Verdict)
	}
	if sc.CostUSD != 0.42 {
		t.Errorf("build-usage.json cost_usd=%v, want 0.42", sc.CostUSD)
	}
}

// TestPhaseOutcome_AbortPaths_AlwaysRecordTimingAndUsage walks the reachable
// abort paths between runner.Run returning and the happy recording site.
// Every one of them must leave a timing entry + usage sidecar for the phase
// that ran (or exhausted its attempts), with abort_reason set.
func TestPhaseOutcome_AbortPaths_AlwaysRecordTimingAndUsage(t *testing.T) {
	t.Parallel()
	maxAtt := resolvePhaseMaxAttempts(nil)
	cases := []struct {
		name string
		// arrange mutates the harness; returns the phase whose record we check.
		arrange      func(runners map[Phase]PhaseRunner, led *fakeLedger, opts *[]Option, env map[string]string) Phase
		wantVerdict  string
		wantAttempts int // 0 = don't check
	}{
		{
			name: "bridge_error_exhausted",
			arrange: func(runners map[Phase]PhaseRunner, _ *fakeLedger, _ *[]Option, _ map[string]string) Phase {
				runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTimeout(), failUntil: 99}
				return PhaseScout
			},
			wantVerdict:  VerdictFAIL, // no canonical agent verdict exists → synthesized FAIL
			wantAttempts: maxAtt,
		},
		{
			name: "non_canonical_verdict_exhausted",
			arrange: func(runners map[Phase]PhaseRunner, _ *fakeLedger, _ *[]Option, _ map[string]string) Phase {
				runners[PhaseScout] = &fakeRunner{name: "scout", verdict: "MAYBE"}
				return PhaseScout
			},
			wantVerdict:  VerdictFAIL, // non-canonical is never recorded raw, never upgraded
			wantAttempts: maxAtt,
		},
		{
			name: "review_gate_reject",
			arrange: func(_ map[Phase]PhaseRunner, _ *fakeLedger, opts *[]Option, env map[string]string) Phase {
				*opts = append(*opts, WithReviewer(stubReviewer{result: ReviewResult{Approve: false, Reason: "deliverable contract violated"}}))
				env["EVOLVE_CONTRACT_CORRECTION_RETRIES"] = "0" // immediate abort, no correction re-dispatch
				return PhaseScout
			},
			// The agent's own verdict was PASS; the reject is a cycle-level
			// disposition recorded in abort_reason, not a verdict rewrite.
			wantVerdict:  VerdictPASS,
			wantAttempts: 1,
		},
		{
			name: "ledger_append_fail",
			arrange: func(_ map[Phase]PhaseRunner, led *fakeLedger, _ *[]Option, _ map[string]string) Phase {
				led.failOnAppend = true
				return PhaseScout
			},
			wantVerdict:  VerdictPASS,
			wantAttempts: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			runners := buildRunners(nil)
			led := &fakeLedger{}
			var opts []Option
			env := map[string]string{}
			phase := tc.arrange(runners, led, &opts, env)
			o := NewOrchestrator(&fakeStorage{}, led, runners, opts...)

			res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g", Env: env})
			if err == nil {
				t.Fatalf("cycle must abort in case %s; got nil error (phases=%v)", tc.name, res.PhasesRun)
			}

			if !phasesRunContains(res.PhasesRun, phase) {
				t.Errorf("PhasesRun=%v must contain %s — it dispatched", res.PhasesRun, phase)
			}
			workspace := cycleWorkspaceDir(root, res.Cycle)
			entry := timingEntryFor(t, readTimingEntries(t, workspace), string(phase))
			if v, _ := entry["verdict"].(string); v != tc.wantVerdict {
				t.Errorf("timing verdict=%q, want %q", v, tc.wantVerdict)
			}
			if reason, _ := entry["abort_reason"].(string); reason == "" {
				t.Errorf("timing entry must carry a non-empty abort_reason; got %v", entry)
			}
			if tc.wantAttempts > 0 {
				if ac, _ := entry["attempt_count"].(float64); int(ac) != tc.wantAttempts {
					t.Errorf("attempt_count=%v, want %d", entry["attempt_count"], tc.wantAttempts)
				}
			}
			if sc := requireUsageSidecar(t, workspace, string(phase)); sc.Verdict != tc.wantVerdict {
				t.Errorf("usage sidecar verdict=%q, want %q", sc.Verdict, tc.wantVerdict)
			}
		})
	}
}

// TestPhaseOutcome_NeverInventsPass pins the C1 invariant by name: when no
// canonical agent verdict exists, the synthesized record is FAIL — a
// PASS-looking non-canonical string must not be upgraded into a recorded PASS.
func TestPhaseOutcome_NeverInventsPass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", verdict: "PASSING"} // non-canonical on purpose
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err == nil {
		t.Fatal("non-canonical verdict must abort the cycle after exhausting attempts")
	}
	entry := timingEntryFor(t, readTimingEntries(t, cycleWorkspaceDir(root, res.Cycle)), "scout")
	switch v, _ := entry["verdict"].(string); v {
	case VerdictFAIL:
		// correct: synthesized FAIL
	case VerdictPASS, "PASSING":
		t.Errorf("recorded verdict=%q — reconciliation invented/laundered a PASS; must synthesize FAIL", v)
	default:
		t.Errorf("recorded verdict=%q, want synthesized FAIL", v)
	}
}

// TestPhaseOutcome_SingleChokepoint_OneRecordPerDispatch pins the
// exactly-once property on the happy path: one timing entry per phase run,
// no duplicates, no abort_reason, and the 1:1 PhasesRun↔timing invariant the
// pre-existing timing tests rely on. Baseline-GREEN today; must SURVIVE the
// chokepoint refactor byte-identically.
func TestPhaseOutcome_SingleChokepoint_OneRecordPerDispatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	entries := readTimingEntries(t, cycleWorkspaceDir(root, res.Cycle))
	if len(entries) != len(res.PhasesRun) {
		t.Errorf("timing entries=%d, want one per phase run (%d): %v", len(entries), len(res.PhasesRun), res.PhasesRun)
	}
	for _, p := range res.PhasesRun {
		e := timingEntryFor(t, entries, string(p)) // fails on duplicates
		if reason, ok := e["abort_reason"]; ok {
			t.Errorf("happy-path %s entry must NOT carry abort_reason (omitempty); got %v", p, reason)
		}
	}
}
