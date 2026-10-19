package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const (
	reauditVerbCycle = 7
	reauditVerbRun   = "run-verb"
)

type reauditVerbFixture struct {
	t         *testing.T
	root      *gittest.Repo
	evolveDir string
	worktree  string
	workspace string
	oldBase   string
	newBase   string
}

func newReauditVerbFixture(t *testing.T, laneFile, peerFile string) *reauditVerbFixture {
	t.Helper()
	root := gittest.Fixture(t)
	root.Write(".gitignore", ".evolve/\n")
	root.Write("shared.txt", "line a\nline b\nline c\n")
	root.Git("add", "-A")
	root.Git("commit", "-q", "-m", "base")
	fx := &reauditVerbFixture{
		t:         t,
		root:      root,
		evolveDir: filepath.Join(root.Dir, ".evolve"),
		worktree:  filepath.Join(root.Dir, ".evolve", "worktrees", "cycle-7"),
		workspace: filepath.Join(root.Dir, ".evolve", "runs", "cycle-7"),
		oldBase:   root.Git("rev-parse", "HEAD"),
	}
	root.Git("worktree", "add", "-q", "-b", "cycle-7", fx.worktree, "main")
	if err := os.MkdirAll(fx.workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	root.Write(".evolve/worktrees/cycle-7/"+laneFile, "lane edit\n")
	root.Git("-C", fx.worktree, "add", "-A")
	root.Git("-C", fx.worktree, "commit", "-q", "-m", "lane change")
	root.Write(peerFile, "peer edit\n")
	root.Git("add", "-A")
	root.Git("commit", "-q", "-m", "peer change")
	fx.newBase = root.Git("rev-parse", "HEAD")
	raw, err := json.Marshal(core.CycleState{
		CycleID: reauditVerbCycle, Phase: "ship", RunID: reauditVerbRun,
		WorkspacePath: fx.workspace, ActiveWorktree: fx.worktree, WorktreeBaseSHA: fx.oldBase,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(core.ResolveCycleStatePath(fx.evolveDir), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return fx
}

func (fx *reauditVerbFixture) args(extra ...string) []string {
	return append([]string{"reaudit", "--cycle", strconv.Itoa(reauditVerbCycle), "--project-root", fx.root.Dir, "--evolve-dir", fx.evolveDir}, extra...)
}

func (fx *reauditVerbFixture) snapshot() [3]string {
	return [3]string{
		fx.root.Git("-C", fx.worktree, "rev-parse", "HEAD"),
		fx.root.Git("-C", fx.worktree, "status", "--porcelain"),
		fx.root.Git("-C", fx.worktree, "rev-parse", "--abbrev-ref", "HEAD"),
	}
}

type reauditVerbRunner struct {
	verdict string
	calls   int
	req     core.PhaseRequest
}

func (r *reauditVerbRunner) Name() string { return string(core.PhaseAudit) }

func (r *reauditVerbRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.calls++
	r.req = req
	report := "# Audit Report\n\nVerdict: " + r.verdict + "\n"
	if err := os.WriteFile(filepath.Join(req.Workspace, "audit-report.md"), []byte(report), 0o644); err != nil {
		return core.PhaseResponse{}, err
	}
	return core.PhaseResponse{Phase: string(core.PhaseAudit), Verdict: r.verdict, ArtifactsDir: req.Workspace}, nil
}

func useReauditRunner(t *testing.T, r *reauditVerbRunner) {
	t.Helper()
	prev, prevWire := cycleReauditAuditRunner, wireOrchestratorDepsFn
	cycleReauditAuditRunner = func(orchDeps) (core.PhaseRunner, error) { return r, nil }
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		return orchDeps{CloseSignals: func() {}}
	}
	t.Cleanup(func() { cycleReauditAuditRunner, wireOrchestratorDepsFn = prev, prevWire })
}

func runReauditVerb(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = runCycle(args, nil, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestCycleReaudit_VerbIsRegistered(t *testing.T) {
	code, _, stderr := runReauditVerb("reaudit")

	if strings.Contains(stderr, "unknown subcommand") {
		t.Fatalf("evolve cycle reaudit is not a subcommand: %q", stderr)
	}
	if code != 10 || !strings.Contains(stderr, "--cycle") {
		t.Fatalf("exit=%d stderr=%q, want exit 10 naming --cycle", code, stderr)
	}
}

func TestCycleReaudit_UsageErrorsExitTen(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"missing cycle", []string{"reaudit"}},
		{"zero cycle", []string{"reaudit", "--cycle", "0"}},
		{"negative cycle", []string{"reaudit", "--cycle", "-2"}},
		{"non numeric cycle", []string{"reaudit", "--cycle", "abc"}},
		{"unknown flag", []string{"reaudit", "--cycle", "7", "--bogus"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &reauditVerbRunner{verdict: core.VerdictPASS}
			useReauditRunner(t, r)

			code, _, stderr := runReauditVerb(tt.args...)

			if code != 10 || strings.Contains(stderr, "unknown subcommand") {
				t.Fatalf("exit=%d stderr=%q, want a usage error from the reaudit verb", code, stderr)
			}
			if r.calls != 0 {
				t.Fatalf("the audit ran %d times on a usage error", r.calls)
			}
		})
	}
}

func TestCycleReaudit_MovedBaseRebasesAuditsAndShipBindingAccepts(t *testing.T) {
	fx := newReauditVerbFixture(t, "lane.txt", "peer.txt")
	r := &reauditVerbRunner{verdict: core.VerdictPASS}
	useReauditRunner(t, r)
	mainBefore := fx.root.Git("rev-parse", "main")

	code, stdout, stderr := runReauditVerb(fx.args()...)

	if code != 0 {
		t.Fatalf("exit=%d stderr=%q, want 0", code, stderr)
	}
	reportPath := filepath.Join(fx.workspace, "audit-report.md")
	wantLine := "evolve cycle reaudit: cycle 7 verdict=PASS base=" + fx.newBase + " verdict_path=" + reportPath
	if strings.TrimSpace(stdout) != wantLine {
		t.Fatalf("stdout = %q, want %q", strings.TrimSpace(stdout), wantLine)
	}
	if r.calls != 1 {
		t.Fatalf("audit ran %d times, want 1", r.calls)
	}
	if parent := fx.root.Git("-C", fx.worktree, "rev-parse", "HEAD^"); parent != fx.newBase {
		t.Fatalf("worktree HEAD^ = %s, want the moved base %s", parent, fx.newBase)
	}
	if mainAfter := fx.root.Git("rev-parse", "main"); mainAfter != mainBefore {
		t.Fatalf("main moved %s -> %s: reaudit must never ship", mainBefore, mainAfter)
	}

	ledgerPath := filepath.Join(fx.evolveDir, "ledger.jsonl")
	entry, err := auditledger.LatestAuditorEntry(ledgerPath, reauditVerbRun)
	if err != nil {
		t.Fatalf("ship cannot bind the recorded audit to run %s: %v", reauditVerbRun, err)
	}
	body, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	if entry.GitHEAD != fx.newBase || entry.WorktreeBaseSHA != fx.newBase {
		t.Fatalf("binding git_head=%s worktree_base=%s, want both %s", entry.GitHEAD, entry.WorktreeBaseSHA, fx.newBase)
	}
	if entry.ArtifactPath != reportPath || entry.ArtifactSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("binding artifact = %s sha %s, want %s sha %x", entry.ArtifactPath, entry.ArtifactSHA256, reportPath, sum)
	}
	if entry.TreeStateSHA == "" || entry.WorktreeTreeSHA == "" {
		t.Fatalf("binding lacks tree identity: %+v", entry)
	}
	if err := ledger.New(fx.evolveDir).Verify(context.Background()); err != nil {
		t.Fatalf("the ledger chain broke after the reaudit append: %v", err)
	}
}

func TestCycleReaudit_RequestComesFromTheCycleState(t *testing.T) {
	fx := newReauditVerbFixture(t, "lane.txt", "peer.txt")
	r := &reauditVerbRunner{verdict: core.VerdictPASS}
	useReauditRunner(t, r)

	if code, _, stderr := runReauditVerb(fx.args()...); code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}

	req := r.req
	if req.Cycle != reauditVerbCycle || req.RunID != reauditVerbRun || req.Workspace != fx.workspace ||
		req.Worktree != fx.worktree || req.WorktreeBaseSHA != fx.newBase {
		t.Fatalf("audit request = cycle %d run %q workspace %q worktree %q base %q", req.Cycle, req.RunID, req.Workspace, req.Worktree, req.WorktreeBaseSHA)
	}
}

func TestCycleReaudit_FailVerdictStillExitsZero(t *testing.T) {
	fx := newReauditVerbFixture(t, "lane.txt", "peer.txt")
	useReauditRunner(t, &reauditVerbRunner{verdict: core.VerdictFAIL})

	code, stdout, stderr := runReauditVerb(fx.args()...)

	if code != 0 || !strings.Contains(stdout, "verdict=FAIL") {
		t.Fatalf("exit=%d stdout=%q stderr=%q, want exit 0 with verdict=FAIL: the verdict is data", code, stdout, stderr)
	}
}

func TestCycleReaudit_LiveLeaseExitsOneAndLeavesWorktreeUnchanged(t *testing.T) {
	fx := newReauditVerbFixture(t, "lane.txt", "peer.txt")
	if err := runlease.Write(fx.workspace, runlease.Lease{RunID: reauditVerbRun, OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	r := &reauditVerbRunner{verdict: core.VerdictPASS}
	useReauditRunner(t, r)
	before := fx.snapshot()

	code, stdout, stderr := runReauditVerb(fx.args()...)

	if code != 1 || stdout != "" {
		t.Fatalf("exit=%d stdout=%q, want exit 1 and no stdout", code, stdout)
	}
	if lower := strings.ToLower(stderr); !strings.Contains(lower, "lease") || !strings.Contains(lower, "live") {
		t.Fatalf("stderr = %q, want the live-lease reason", stderr)
	}
	if fx.snapshot() != before || r.calls != 0 {
		t.Fatalf("the worktree changed or the audit ran under a live lease")
	}
	if _, err := os.Stat(filepath.Join(fx.evolveDir, "ledger.jsonl")); err == nil {
		t.Fatal("a ledger row was written under a live lease")
	}
}

func TestCycleReaudit_RebaseConflictExitsOneAndLeavesWorktreeUnchanged(t *testing.T) {
	fx := newReauditVerbFixture(t, "shared.txt", "shared.txt")
	r := &reauditVerbRunner{verdict: core.VerdictPASS}
	useReauditRunner(t, r)
	before := fx.snapshot()

	code, stdout, stderr := runReauditVerb(fx.args()...)

	if code != 1 || stdout != "" {
		t.Fatalf("exit=%d stdout=%q, want exit 1 and no stdout", code, stdout)
	}
	if !strings.Contains(stderr, "shared.txt") || !strings.Contains(strings.ToLower(stderr), "conflict") {
		t.Fatalf("stderr = %q, want the conflict reason naming shared.txt", stderr)
	}
	if fx.snapshot() != before || r.calls != 0 {
		t.Fatalf("the worktree changed or the audit ran after a conflict")
	}
}

func TestCycleReaudit_UnknownCycleExitsOne(t *testing.T) {
	fx := newReauditVerbFixture(t, "lane.txt", "peer.txt")
	r := &reauditVerbRunner{verdict: core.VerdictPASS}
	useReauditRunner(t, r)
	args := []string{"reaudit", "--cycle", "99", "--project-root", fx.root.Dir, "--evolve-dir", fx.evolveDir}

	code, _, stderr := runReauditVerb(args...)

	if code != 1 || !strings.Contains(stderr, "99") || r.calls != 0 {
		t.Fatalf("exit=%d stderr=%q audit calls=%d, want exit 1 naming cycle 99 with no audit", code, stderr, r.calls)
	}
}

func TestRunCycle_DispatchesEverySubcommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		args   []string
		code   int
		stderr string
	}{
		{"missing subcommand", nil, 10, "missing subcommand"},
		{"unknown subcommand", []string{"nope"}, 10, "unknown subcommand"},
		{"run", []string{"run", "--bogus"}, 10, "bogus"},
		{"reset", []string{"reset", "--bogus"}, 10, "bogus"},
		{"timing", []string{"timing", "--bogus"}, 10, "bogus"},
		{"outputs", []string{"outputs", "--bogus"}, 10, "bogus"},
		{"scope-delta", []string{"scope-delta", "--bogus"}, 10, "bogus"},
		{"reaudit", []string{"reaudit", "--bogus"}, 10, "bogus"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out, errOut bytes.Buffer

			code := runCycle(tt.args, nil, &out, &errOut)

			if code != tt.code || !strings.Contains(errOut.String(), tt.stderr) {
				t.Fatalf("exit=%d stderr=%q, want exit %d naming %q", code, errOut.String(), tt.code, tt.stderr)
			}
		})
	}
}

func TestCycleReauditAuditRunner_DefaultWiresTheAuditPhase(t *testing.T) {
	want := &reauditVerbRunner{}
	tests := []struct {
		name    string
		deps    orchDeps
		wantErr string
	}{
		{"routing error", orchDeps{RoutingErr: errors.New("no route")}, "no route"},
		{"no audit runner", orchDeps{Runners: map[core.Phase]core.PhaseRunner{}}, "no audit runner"},
		{"audit runner", orchDeps{Runners: map[core.Phase]core.PhaseRunner{core.PhaseAudit: want}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := defaultCycleReauditAuditRunner(tt.deps)

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != core.PhaseRunner(want) {
				t.Fatalf("got %v, err %v, want the wired audit runner", got, err)
			}
		})
	}
}

func TestCycleReaudit_EvolveDirDefaultsUnderTheProjectRoot(t *testing.T) {
	fx := newReauditVerbFixture(t, "lane.txt", "peer.txt")
	r := &reauditVerbRunner{verdict: core.VerdictPASS}
	useReauditRunner(t, r)

	code, stdout, stderr := runReauditVerb("reaudit", "--cycle", strconv.Itoa(reauditVerbCycle), "--project-root", fx.root.Dir)

	if code != 0 || r.calls != 1 || !strings.Contains(stdout, "verdict=PASS") {
		t.Fatalf("exit=%d calls=%d stdout=%q stderr=%q, want a PASS audit from <project-root>/.evolve", code, r.calls, stdout, stderr)
	}
}

func TestCycleReaudit_UnwiredAuditRunnerExitsOne(t *testing.T) {
	fx := newReauditVerbFixture(t, "lane.txt", "peer.txt")
	prev, prevWire := cycleReauditAuditRunner, wireOrchestratorDepsFn
	cycleReauditAuditRunner = func(orchDeps) (core.PhaseRunner, error) {
		return nil, errors.New("no audit runner is wired")
	}
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		return orchDeps{CloseSignals: func() {}}
	}
	t.Cleanup(func() { cycleReauditAuditRunner, wireOrchestratorDepsFn = prev, prevWire })
	before := fx.snapshot()

	code, _, stderr := runReauditVerb(fx.args()...)

	if code != 1 || !strings.Contains(stderr, "no audit runner is wired") {
		t.Fatalf("exit=%d stderr=%q, want exit 1 naming the wiring failure", code, stderr)
	}
	if fx.snapshot() != before {
		t.Fatal("the worktree changed although no audit runner was wired")
	}
}

type signalWatchingRunner struct {
	reauditVerbRunner
	closes        *int
	closesAtRun   int
	closedWhenRan bool
}

func (r *signalWatchingRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.closesAtRun = *r.closes
	r.closedWhenRan = *r.closes > 0
	return r.reauditVerbRunner.Run(ctx, req)
}

func TestCycleReaudit_SignalsStayOpenUntilTheAuditHasRun(t *testing.T) {
	fx := newReauditVerbFixture(t, "lane.txt", "peer.txt")
	prev, prevWire := cycleReauditAuditRunner, wireOrchestratorDepsFn
	t.Cleanup(func() { cycleReauditAuditRunner, wireOrchestratorDepsFn = prev, prevWire })
	cycleReauditAuditRunner = defaultCycleReauditAuditRunner
	closes := 0
	runner := &signalWatchingRunner{reauditVerbRunner: reauditVerbRunner{verdict: core.VerdictPASS}, closes: &closes}
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		return orchDeps{
			Runners:      map[core.Phase]core.PhaseRunner{core.PhaseAudit: runner},
			CloseSignals: func() { closes++ },
		}
	}

	code, _, stderr := runReauditVerb(fx.args()...)

	if code != 0 || runner.calls != 1 {
		t.Fatalf("exit=%d audit calls=%d stderr=%q, want a PASS audit", code, runner.calls, stderr)
	}
	if runner.closedWhenRan {
		t.Fatalf("the signal publisher was closed %d time(s) before the audit ran: its events are lost", runner.closesAtRun)
	}
	if closes != 1 {
		t.Fatalf("the signal publisher closed %d times after the verb, want exactly 1", closes)
	}
}

func TestCycleReaudit_SignalsCloseOnceWhenTheVerbRefuses(t *testing.T) {
	fx := newReauditVerbFixture(t, "lane.txt", "peer.txt")
	prev, prevWire := cycleReauditAuditRunner, wireOrchestratorDepsFn
	t.Cleanup(func() { cycleReauditAuditRunner, wireOrchestratorDepsFn = prev, prevWire })
	cycleReauditAuditRunner = defaultCycleReauditAuditRunner
	closes := 0
	runner := &signalWatchingRunner{reauditVerbRunner: reauditVerbRunner{verdict: core.VerdictPASS}, closes: &closes}
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		return orchDeps{
			Runners:      map[core.Phase]core.PhaseRunner{core.PhaseAudit: runner},
			CloseSignals: func() { closes++ },
		}
	}

	code, _, _ := runReauditVerb("reaudit", "--cycle", "99", "--project-root", fx.root.Dir, "--evolve-dir", fx.evolveDir)

	if code != 1 || runner.calls != 0 {
		t.Fatalf("exit=%d audit calls=%d, want a refusal with no audit", code, runner.calls)
	}
	if closes != 1 {
		t.Fatalf("the signal publisher closed %d times on a refusal, want exactly 1 (no leak)", closes)
	}
}
