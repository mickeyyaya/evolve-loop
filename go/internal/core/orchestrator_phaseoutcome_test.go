//go:build integration

package core

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/outcome"
)

// outcomeRunner PASSes its phase with a scripted cost/duration after running
// an optional side effect, modeling a build that did real work but left the
// main tree dirty in a way the tree-diff guard catches.
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

// initOutcomeRepo creates a real git repo so the tree-diff guard runs its
// production code path against actual git output rather than a stub.
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

func requireUsageSidecar(t *testing.T, workspace, phase string) outcome.UsageSidecar {
	t.Helper()
	path := filepath.Join(workspace, phase+"-usage.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s must be written for every terminal disposition (cycle-262: missing build-usage.json was the divergence): %v", path, err)
	}
	var sc outcome.UsageSidecar
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

// Fixture note: the original fixture (a tracked .evolve/commit-prefix-scope.json
// edit) is now RECOVERABLE by design (the deliverable allowlist relocates it;
// pinned in buildleak_recover_test.go), so this test uses a STAGED RENAME of a
// tracked file, which no recovery branch handles — the canonical
// still-unrecoverable main-tree mutation.
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
			// porcelain 'R ' (staged rename) matches no recovery branch, so the guard re-check stays dirty.
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

	if err == nil {
		t.Fatalf("tree-guard must still abort the cycle on a real leak; got nil error (phases=%v)", res.PhasesRun)
	}
	if !strings.Contains(err.Error(), "tree-diff") {
		t.Errorf("abort must come from the tree-diff guard; got: %v", err)
	}

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

func TestPhaseOutcome_AbortPaths_AlwaysRecordTimingAndUsage(t *testing.T) {
	t.Parallel()
	maxAtt := 2
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
			wantVerdict:  VerdictFAIL,
			wantAttempts: maxAtt,
		},
		{
			name: "non_canonical_verdict_exhausted",
			arrange: func(runners map[Phase]PhaseRunner, _ *fakeLedger, _ *[]Option, _ map[string]string) Phase {
				runners[PhaseScout] = &fakeRunner{name: "scout", verdict: "MAYBE"}
				return PhaseScout
			},
			wantVerdict:  VerdictFAIL,
			wantAttempts: maxAtt,
		},
		{
			name: "review_gate_reject",
			arrange: func(_ map[Phase]PhaseRunner, _ *fakeLedger, opts *[]Option, env map[string]string) Phase {
				*opts = append(*opts, WithReviewer(stubReviewer{result: ReviewResult{Approve: false, Reason: "deliverable contract violated"}}))
				env["EVOLVE_CONTRACT_CORRECTION_RETRIES"] = "0" // immediate abort, no correction re-dispatch
				return PhaseScout
			},
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
	case VerdictPASS, "PASSING":
		t.Errorf("recorded verdict=%q — reconciliation invented/laundered a PASS; must synthesize FAIL", v)
	default:
		t.Errorf("recorded verdict=%q, want synthesized FAIL", v)
	}
}

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
