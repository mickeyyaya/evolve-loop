//go:build integration

package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
)

type coverageKB struct{}

func (coverageKB) Lookup(context.Context, research.Query) ([]research.Lesson, error) { return nil, nil }

type errVerifier struct{}

func (errVerifier) VerifyDeliverable(context.Context, ReviewInput) (ContractVerification, error) {
	return ContractVerification{}, errors.New("unknown phase")
}

func TestFailureAdvisorOptionsPromptAndParseCoverage(t *testing.T) {
	a := NewFailureAdvisor(nil,
		WithFailureAdvisorCLI("codex-tmux"),
		WithFailureAdvisorModel("sonnet"),
		WithFailureAdvisorPersona("PERSONA"),
		WithFailureAdvisorCLI(""),
		WithFailureAdvisorModel(""),
		WithFailureAdvisorPersona(""),
	)
	if a.identity.CLI != "codex-tmux" || a.identity.Model != "sonnet" || a.identity.Persona != "PERSONA" {
		t.Fatalf("advisor options not applied: %+v", a)
	}
	prompt := a.composePrompt(FailureAdviseInput{
		Phase: "build", CLI: "codex", ExitCode: 80, PaneTail: "fatal pane text", Cycle: 281,
	}, "/tmp/advice.json")
	for _, want := range []string{"PERSONA", "phase: build", "codex", "/tmp/advice.json", "pane_substr"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
	ok, err := parseFailureAdvice(`{"cause":"dead_shell","pane_substr":"plain shell prompt","justification":"pane is no longer an agent"}`)
	if err != nil || ok.Cause != "dead_shell" {
		t.Fatalf("parse valid advice=%+v err=%v", ok, err)
	}
	for _, raw := range []string{
		`{bad json`,
		`{"cause":"","justification":"not fatal"}`,
		`{"cause":"bogus","pane_substr":"plain shell prompt","justification":"bad cause"}`,
		`{"cause":"dead_shell","pane_substr":"plain shell prompt"}`,
	} {
		if _, err := parseFailureAdvice(raw); err == nil {
			t.Fatalf("parseFailureAdvice(%q) unexpectedly succeeded", raw)
		}
	}
	if _, err := NewFailureAdvisor(nil).Advise(context.Background(), FailureAdviseInput{Workspace: t.TempDir()}); err == nil {
		t.Fatal("nil bridge should fail loudly")
	}
}

func TestCorrectionLadderEvidenceCoverage(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", root, "config", "user.email", "t@example.com").CombinedOutput(); err != nil {
		t.Fatalf("git config email: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", root, "config", "user.name", "Test").CombinedOutput(); err != nil {
		t.Fatalf("git config name: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base"), 0o644); err != nil {
		t.Fatalf("write tracked: %v", err)
	}
	if out, err := exec.Command("git", "-C", root, "add", ".").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", root, "commit", "-m", "base").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatalf("modify tracked: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatalf("write new: %v", err)
	}
	names := gitStatusNames(root, 1)
	if len(names) != 1 || !strings.Contains(names[0], "tracked.txt") {
		t.Fatalf("gitStatusNames cap/status mismatch: %v", names)
	}
	digest := kernelEvidenceDigest(root, filepath.Join(root, "bad-build-report.md"))
	if !strings.Contains(digest, "kernel-verified") || !strings.Contains(digest, "bad-build-report.md") {
		t.Fatalf("evidence digest missing expected facts:\n%s", digest)
	}
	if got := gitStatusNames("", 10); got != nil {
		t.Fatalf("empty worktree status names = %v", got)
	}
}

func TestCorrectionLadderSafetyHelpersCoverage(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if !withinRoot(child, root) || !withinRoot(root, root) || withinRoot(filepath.Dir(root), root) || withinRoot(child, "") {
		t.Fatal("withinRoot boundary mismatch")
	}
	now := time.Now()
	okFile := filepath.Join(root, "ok.md")
	if err := os.WriteFile(okFile, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write ok: %v", err)
	}
	if ok, why := fileSalvageable(okFile, now); !ok || why != "" {
		t.Fatalf("ok file salvageable=(%v,%q)", ok, why)
	}
	empty := filepath.Join(root, "empty.md")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	if ok, why := fileSalvageable(empty, now); ok || why != "empty" {
		t.Fatalf("empty salvageable=(%v,%q)", ok, why)
	}
	dir := filepath.Join(root, "dir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if ok, why := fileSalvageable(dir, now); ok || !strings.Contains(why, "not a regular file") {
		t.Fatalf("dir salvageable=(%v,%q)", ok, why)
	}
	stale := filepath.Join(root, "stale.md")
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	old := now.Add(-salvageMaxAge - time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if ok, why := fileSalvageable(stale, now); ok || !strings.Contains(why, "stale") {
		t.Fatalf("stale salvageable=(%v,%q)", ok, why)
	}
}

func TestShipErrorAndWrapCoverage(t *testing.T) {
	se := NewShipError(CodeIntegrityTreeDrift, ShipClassIntegrity, StageAtomicShip, "detail", "path", "worktree", "odd")
	if !strings.Contains(se.Error(), "TREE_DRIFT") || !strings.Contains(se.DebugString(), "path=worktree") || !strings.Contains(se.DebugString(), "odd=") {
		t.Fatalf("ship error strings missing detail: %s / %s", se.Error(), se.DebugString())
	}
	if wrapCycleLevelError(PhaseBuild, nil) != nil {
		t.Fatal("nil wrap should stay nil")
	}
	if !errors.Is(wrapCycleLevelError(PhaseBuild, ErrPhaseGateFailed), ErrPhaseGateFailed) {
		t.Fatal("sentinel errors should pass through")
	}
	if _, ok := wrapCycleLevelError(PhaseBuild, errors.New("boom")).(*ErrCycleLevelFailure); !ok {
		t.Fatal("ordinary errors should wrap as cycle-level failures")
	}
}

func TestResetJSONAndOptionCoverage(t *testing.T) {
	root := t.TempDir()
	if pathWithin(filepath.Join(root, "child"), root) != true || pathWithin(filepath.Dir(root), root) || pathWithin(root, "") {
		t.Fatal("pathWithin boundary mismatch")
	}
	missing := filepath.Join(root, "missing.json")
	if got, err := readJSONMapFile(missing); err != nil || len(got) != 0 {
		t.Fatalf("missing JSON map = %+v err=%v", got, err)
	}
	empty := filepath.Join(root, "empty.json")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	if got, err := readJSONMapFile(empty); err != nil || len(got) != 0 {
		t.Fatalf("empty JSON map = %+v err=%v", got, err)
	}
	path := filepath.Join(root, "nested", "state.json")
	want := map[string]any{"expected_ship_sha": "abc", "n": float64(2)}
	if err := writeJSONMapFileAtomic(path, want); err != nil {
		t.Fatalf("writeJSONMapFileAtomic: %v", err)
	}
	got, err := readJSONMapFile(path)
	if err != nil {
		t.Fatalf("readJSONMapFile: %v", err)
	}
	if got["expected_ship_sha"] != "abc" || got["n"] != float64(2) {
		t.Fatalf("JSON round trip = %+v", got)
	}
	bad := filepath.Join(root, "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatalf("write bad: %v", err)
	}
	if _, err := readJSONMapFile(bad); err == nil {
		t.Fatal("invalid JSON should fail")
	}
	if err := writeJSONMapFileAtomic(filepath.Join(root, "bad-marshal.json"), map[string]any{"bad": func() {}}); err == nil {
		t.Fatal("unmarshalable map should fail")
	}
	o := NewOrchestrator(nil, nil, nil, WithKB(coverageKB{}), WithKB(nil))
	if o.kb == nil {
		t.Fatal("WithKB should install non-nil KB and ignore nil override")
	}
	if _, err := json.Marshal(got); err != nil {
		t.Fatalf("read map should remain JSON marshalable: %v", err)
	}
}

func TestOrchestratorForensicsHelpersCoverage(t *testing.T) {
	ws := t.TempDir()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, nil)
	o.now = func() time.Time { return time.Date(2026, 6, 11, 1, 2, 3, 0, time.UTC) }

	var result CycleResult
	var timings []phaseTimingEntry
	o.recordPhaseOutcome(&result, &timings, ws, recovery.PhaseOutcome{
		Phase: "build", Verdict: "FAIL", DurationMS: 12, BootMS: 3, CostUSD: 0.01, AttemptCount: 2, AbortReason: "boom",
	})
	if len(result.PhasesRun) != 1 || result.PhasesRun[0] != PhaseBuild || len(timings) != 1 {
		t.Fatalf("phase outcome not recorded: result=%+v timings=%+v", result, timings)
	}
	if _, err := os.Stat(filepath.Join(ws, "build-usage.json")); err != nil {
		t.Fatalf("usage sidecar missing: %v", err)
	}
	writePhaseTimings(ws, []phaseTimingEntry{{Phase: "scout", Verdict: "PASS"}})
	writePhaseTimings(ws, []phaseTimingEntry{{Phase: "build", Verdict: "FAIL"}})
	raw, err := os.ReadFile(filepath.Join(ws, "phase-timing.json"))
	if err != nil {
		t.Fatalf("phase timing missing: %v", err)
	}
	var got []phaseTimingEntry
	if err := json.Unmarshal(raw, &got); err != nil || len(got) != 2 {
		t.Fatalf("phase timing merge got=%+v err=%v raw=%s", got, err, raw)
	}
	writePhaseFailureDiag(ws, "build", 281, ErrArtifactTimeout, 3, o.now)
	diagRaw, err := os.ReadFile(filepath.Join(ws, "build-failure-diag.json"))
	if err != nil {
		t.Fatalf("failure diag missing: %v", err)
	}
	if !strings.Contains(string(diagRaw), `"exit_code":81`) {
		t.Fatalf("failure diag should record artifact timeout exit 81: %s", diagRaw)
	}
	summary := failureLearningSummary(281, PhaseBuild, fmt.Errorf("%s", strings.Repeat("x", maxFailureLearningSummaryChars+20)))
	if !strings.Contains(summary, "truncated") || !strings.Contains(summary, "cycle 281 failed during build") {
		t.Fatalf("failure summary did not truncate with context: %s", summary)
	}
	if !carryoverTodoExists([]CarryoverTodo{{ID: "x"}}, "x") || carryoverTodoExists(nil, "x") {
		t.Fatal("carryoverTodoExists mismatch")
	}
}

func TestRecoveryDecisionForensicsCoverage(t *testing.T) {
	ws := t.TempDir()
	led := &fakeLedger{}
	o := NewOrchestrator(&fakeStorage{}, led, nil)
	o.now = func() time.Time { return time.Date(2026, 6, 11, 1, 2, 3, 0, time.UTC) }
	cs := CycleState{WorkspacePath: ws}
	se := NewShipError(CodeGitIO, ShipClassTransient, StageAtomicShip, "push failed", "remote", "origin")
	o.recordShipError(context.Background(), 281, cs, se)
	if len(led.entries) != 1 || led.entries[0].Kind != "ship_error" || led.entries[0].ArtifactSHA256 == "" {
		t.Fatalf("ship error ledger not recorded: %+v", led.entries)
	}
	if _, err := os.Stat(filepath.Join(ws, "ship-error.json")); err != nil {
		t.Fatalf("ship-error artifact missing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "debug-decision.json"), []byte(`{"action":"RESHIP"}`), 0o644); err != nil {
		t.Fatalf("write debug decision: %v", err)
	}
	o.recordDebuggerDecision(context.Background(), 281, cs, PhaseResponse{})
	if len(led.entries) != 2 || led.entries[1].Kind != "debugger_decision" || led.entries[1].ArtifactSHA256 == "" {
		t.Fatalf("debugger decision ledger not recorded: %+v", led.entries)
	}
	if got := o.decideAfterDebugger(PhaseResponse{Signals: map[string]interface{}{"debugger.action": "RERUN_PHASE", "debugger.rerun_phase": "build"}}); got != PhaseBuild {
		t.Fatalf("debugger rerun build -> %s", got)
	}
	if got := o.decideAfterDebugger(PhaseResponse{Signals: map[string]interface{}{"debugger.action": "RESHIP"}}); got != PhaseShip {
		t.Fatalf("debugger reship -> %s", got)
	}
	if got := o.decideAfterDebugger(PhaseResponse{Signals: map[string]interface{}{"debugger.action": "RERUN_PHASE", "debugger.rerun_phase": "ship"}}); got != PhaseAudit {
		t.Fatalf("debugger rerun ship must clamp to audit, got %s", got)
	}
	if next, env, reason := o.decideAfterRetro(VerdictPASS, nil); next != PhaseShip || env != nil || !strings.Contains(reason, "retro-recovered") {
		t.Fatalf("retro pass decision = %s %+v %q", next, env, reason)
	}
	if next, _, reason := o.decideAfterRetro(VerdictFAIL, nil); next != PhaseEnd || !strings.Contains(reason, "proceed") {
		t.Fatalf("retro fail with empty history decision = %s %q", next, reason)
	}
	st := &fakeStorage{}
	o.writeFailureLearningState(context.Background(), nil)
	o.storage = st
	o.writeFailureLearningState(context.Background(), &State{LastCycleNumber: 9})
	if st.state.LastCycleNumber != 9 {
		t.Fatalf("failure-learning state not written: %+v", st.state)
	}
}

func TestSalvageDeliverableBranchCoverage(t *testing.T) {
	ctx := context.Background()
	ws := t.TempDir()
	wt := t.TempDir()
	in := ReviewInput{Phase: "build", Workspace: ws, Worktree: wt, ProjectRoot: filepath.Dir(ws)}
	if got := NewOrchestrator(nil, nil, nil).salvageDeliverable(ctx, in); !strings.Contains(got.Reason, "no breaker-neutral verifier") {
		t.Fatalf("nil verifier salvage reason = %+v", got)
	}
	if got := NewOrchestrator(nil, nil, nil, WithContractVerifier(errVerifier{})).salvageDeliverable(ctx, in); !strings.Contains(got.Reason, "verifier ambiguity") {
		t.Fatalf("err verifier salvage reason = %+v", got)
	}
	destSub := filepath.Join("contracted", "build-report.md")
	dest := filepath.Join(ws, destSub)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	if err := os.WriteFile(dest, []byte("already valid"), 0o644); err != nil {
		t.Fatalf("write dest: %v", err)
	}
	fv := &fakeVerifier{destSub: destSub}
	if got := NewOrchestrator(nil, nil, nil, WithContractVerifier(fv)).salvageDeliverable(ctx, in); !strings.Contains(got.Reason, "destination already well-formed") {
		t.Fatalf("existing dest salvage reason = %+v", got)
	}
	if err := os.Remove(dest); err != nil {
		t.Fatalf("remove dest: %v", err)
	}
	stray := filepath.Join(wt, "build-report.md")
	if err := os.WriteFile(stray, []byte("relocate me"), 0o644); err != nil {
		t.Fatalf("write stray: %v", err)
	}
	if got := NewOrchestrator(nil, nil, nil, WithContractVerifier(fv)).salvageDeliverable(ctx, in); !got.Relocated || !got.Verified || got.From != stray {
		t.Fatalf("salvage relocate result = %+v", got)
	}
}

func TestRunCycleFromPhaseAdditionalBranchesCoverage(t *testing.T) {
	baseCS := CycleState{CycleID: 0, WorkspacePath: t.TempDir()}
	req := CycleRequest{ProjectRoot: t.TempDir(), Env: map[string]string{"K": "V"}, Context: map[string]string{"C": "D"}}
	t.Run("write cycle state failure", func(t *testing.T) {
		st := &fakeStorage{state: State{LastCycleNumber: 4}, cycleState: baseCS, failOnWriteCS: true}
		o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
		if _, err := o.RunCycleFromPhase(context.Background(), req, &ResumePoint{Phase: string(PhaseBuild), CycleID: 5}); err == nil {
			t.Fatal("expected pre-phase cycle-state write failure")
		}
	})
	t.Run("runner error records outcome", func(t *testing.T) {
		st := &fakeStorage{state: State{LastCycleNumber: 4}, cycleState: baseCS}
		runners := buildRunners(nil)
		runners[PhaseBuild] = &fakeRunner{name: "build", failErr: errors.New("phase failed"), failUntil: 1}
		o := NewOrchestrator(st, &fakeLedger{}, runners)
		res, err := o.RunCycleFromPhase(context.Background(), req, &ResumePoint{Phase: string(PhaseBuild), CycleID: 5})
		if err == nil || len(res.PhasesRun) == 0 {
			t.Fatalf("expected runner error with recorded phase, res=%+v err=%v", res, err)
		}
	})
	t.Run("non canonical verdict", func(t *testing.T) {
		st := &fakeStorage{state: State{LastCycleNumber: 4}, cycleState: baseCS}
		runners := buildRunners(nil)
		runners[PhaseBuild] = &fakeRunner{name: "build", verdict: "MAYBE"}
		o := NewOrchestrator(st, &fakeLedger{}, runners)
		if _, err := o.RunCycleFromPhase(context.Background(), req, &ResumePoint{Phase: string(PhaseBuild), CycleID: 5}); err == nil {
			t.Fatal("expected non-canonical verdict error")
		}
	})
	t.Run("read state failure", func(t *testing.T) {
		st := &fakeStorage{failOnReadState: true}
		o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
		if _, err := o.RunCycleFromPhase(context.Background(), req, &ResumePoint{Phase: string(PhaseBuild), CycleID: 5}); err == nil {
			t.Fatal("expected read state failure")
		}
	})
}

func TestGitAndFileHelperCoverage(t *testing.T) {
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "base"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir repo: %v", err)
	}
	head, err := defaultGitHEAD()
	if chErr := os.Chdir(oldwd); chErr != nil {
		t.Fatalf("restore cwd: %v", chErr)
	}
	if err != nil || len(head) != 40 {
		t.Fatalf("defaultGitHEAD=%q err=%v", head, err)
	}

	base := t.TempDir()
	t.Setenv("EVOLVE_WORKTREE_BASE", base)
	if got := (gitWorktree{}).base("/tmp/project"); got != base {
		t.Fatalf("worktree env base=%q want %q", got, base)
	}
	t.Setenv("EVOLVE_WORKTREE_BASE", "")
	if got := (gitWorktree{}).base("/tmp/project"); !strings.HasSuffix(got, filepath.Join(".evolve", "worktrees")) {
		t.Fatalf("worktree default base=%q", got)
	}

	root := t.TempDir()
	src := filepath.Join(root, "src.txt")
	dst := filepath.Join(root, "nested", "dst.txt")
	if err := os.WriteFile(src, []byte("body"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	if got, err := os.ReadFile(dst); err != nil || string(got) != "body" {
		t.Fatalf("copied content=%q err=%v", got, err)
	}
	moved := filepath.Join(root, "moved.txt")
	if err := moveFile(dst, moved); err != nil {
		t.Fatalf("moveFile: %v", err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("moveFile should remove source, stat err=%v", err)
	}
	link := filepath.Join(root, "link")
	symlinkForce(src, link)
	if target, err := os.Readlink(link); err != nil || target != src {
		t.Fatalf("symlinkForce target=%q err=%v", target, err)
	}
	if err := copyFile(filepath.Join(root, "missing"), filepath.Join(root, "x")); err == nil {
		t.Fatal("copyFile should fail on missing source")
	}
	if err := moveFile(filepath.Join(root, "missing"), filepath.Join(root, "y")); err == nil {
		t.Fatal("moveFile should fail when rename and copy both fail")
	}
	symlinkForce(src, filepath.Join(root, "missing-parent", "link"))
}

func TestVerdictReasonTruncationCoverage(t *testing.T) {
	short := strings.Repeat("界", maxReasonSummaryLen/2)
	if got := truncateReason(short); got != short {
		t.Fatalf("short rune-heavy reason changed: %q", got)
	}
	long := strings.Repeat("x", maxReasonSummaryLen+10)
	if got := truncateReason(long); len(got) != maxReasonSummaryLen {
		t.Fatalf("long reason len=%d want %d", len(got), maxReasonSummaryLen)
	}
}
