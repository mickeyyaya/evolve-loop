package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const (
	reauditCycle = 7
	reauditRunID = "run-reaudit"
)

type reauditFixture struct {
	t         *testing.T
	root      *gittest.Repo
	evolveDir string
	worktree  string
	workspace string
	oldBase   string
	newBase   string
}

func newReauditFixture(t *testing.T, laneFile, peerFile string) *reauditFixture {
	t.Helper()
	root := gittest.Fixture(t)
	root.Write(".gitignore", ".evolve/\n")
	root.Write("shared.txt", "line a\nline b\nline c\n")
	root.Git("add", "-A")
	root.Git("commit", "-q", "-m", "base")
	fx := &reauditFixture{
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
	fx.writeIn(fx.worktree, laneFile, "lane edit\n")
	root.Git("-C", fx.worktree, "add", "-A")
	root.Git("-C", fx.worktree, "commit", "-q", "-m", "lane change")
	fx.writeIn(root.Dir, peerFile, "peer edit\n")
	root.Git("add", "-A")
	root.Git("commit", "-q", "-m", "peer change")
	fx.newBase = root.Git("rev-parse", "HEAD")
	fx.writeCycleState(reauditCycle)
	return fx
}

func (fx *reauditFixture) writeIn(dir, rel, body string) {
	fx.t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fx.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		fx.t.Fatal(err)
	}
}

func (fx *reauditFixture) writeCycleState(cycle int) {
	fx.t.Helper()
	raw, err := json.Marshal(CycleState{
		CycleID:         cycle,
		Phase:           string(PhaseShip),
		RunID:           reauditRunID,
		WorkspacePath:   fx.workspace,
		ActiveWorktree:  fx.worktree,
		WorktreeBaseSHA: fx.oldBase,
	})
	if err != nil {
		fx.t.Fatal(err)
	}
	if err := os.MkdirAll(fx.evolveDir, 0o755); err != nil {
		fx.t.Fatal(err)
	}
	if err := os.WriteFile(ResolveCycleStatePath(fx.evolveDir), raw, 0o644); err != nil {
		fx.t.Fatal(err)
	}
}

func (fx *reauditFixture) writeLease(heartbeat time.Time) {
	fx.t.Helper()
	if err := runlease.Write(fx.workspace, runlease.Lease{RunID: reauditRunID, OwnerPID: os.Getpid()}, heartbeat); err != nil {
		fx.t.Fatal(err)
	}
}

func (fx *reauditFixture) worktreeHead() string {
	return fx.root.Git("-C", fx.worktree, "rev-parse", "HEAD")
}

func (fx *reauditFixture) worktreeStatus() string {
	return fx.root.Git("-C", fx.worktree, "status", "--porcelain")
}

func (fx *reauditFixture) worktreeBranch() string {
	return fx.root.Git("-C", fx.worktree, "rev-parse", "--abbrev-ref", "HEAD")
}

func (fx *reauditFixture) input(runner PhaseRunner, ledger *fakeLedger) ReauditInput {
	return ReauditInput{
		ProjectRoot: fx.root.Dir,
		EvolveDir:   fx.evolveDir,
		Cycle:       reauditCycle,
		Ledger:      ledger,
		Audit:       runner,
	}
}

type reauditRunner struct {
	verdict string
	err     error
	calls   int
	req     PhaseRequest
	headAt  string
	fx      *reauditFixture
}

func (r *reauditRunner) Name() string { return string(PhaseAudit) }

func (r *reauditRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.calls++
	r.req = req
	r.headAt = r.fx.worktreeHead()
	if r.err != nil {
		return PhaseResponse{}, r.err
	}
	report := "# Audit Report\n\nVerdict: " + r.verdict + "\n"
	if err := os.WriteFile(filepath.Join(req.Workspace, "audit-report.md"), []byte(report), 0o644); err != nil {
		return PhaseResponse{}, err
	}
	return PhaseResponse{Phase: string(PhaseAudit), Verdict: r.verdict, ArtifactsDir: req.Workspace}, nil
}

func (fx *reauditFixture) runner(verdict string) *reauditRunner {
	return &reauditRunner{verdict: verdict, fx: fx}
}

type worktreeSnapshot struct{ head, status, branch string }

func (fx *reauditFixture) snapshot() worktreeSnapshot {
	return worktreeSnapshot{fx.worktreeHead(), fx.worktreeStatus(), fx.worktreeBranch()}
}

func requireReauditKind(t *testing.T, err error, want ReauditErrorKind) *ReauditError {
	t.Helper()
	var re *ReauditError
	if !errors.As(err, &re) {
		t.Fatalf("error = %v (%T), want *ReauditError", err, err)
	}
	if re.Kind != want {
		t.Fatalf("Kind = %q, want %q (err: %v)", re.Kind, want, err)
	}
	return re
}

func TestReauditCycle_MovedBaseRebasesAuditsAndRecordsTheBinding(t *testing.T) {
	fx := newReauditFixture(t, "lane.txt", "peer.txt")
	runner := fx.runner(VerdictPASS)
	ledger := &fakeLedger{}
	mainBefore := fx.root.Git("rev-parse", "main")

	res, err := ReauditCycle(context.Background(), fx.input(runner, ledger))
	if err != nil {
		t.Fatalf("ReauditCycle: %v", err)
	}

	if runner.calls != 1 {
		t.Fatalf("audit ran %d times, want 1", runner.calls)
	}
	if parent := fx.root.Git("-C", fx.worktree, "rev-parse", "HEAD^"); parent != fx.newBase {
		t.Fatalf("worktree HEAD^ = %s, want the moved base %s", parent, fx.newBase)
	}
	if runner.headAt != fx.worktreeHead() {
		t.Fatalf("audit saw HEAD %s, but the rebased worktree is at %s: the audit ran before the rebase", runner.headAt, fx.worktreeHead())
	}
	wantReport := filepath.Join(fx.workspace, "audit-report.md")
	if res.Cycle != reauditCycle || res.NewBase != fx.newBase || res.Verdict != VerdictPASS || res.VerdictPath != wantReport {
		t.Fatalf("result = %+v, want cycle=%d base=%s verdict=PASS path=%s", res, reauditCycle, fx.newBase, wantReport)
	}
	if mainAfter := fx.root.Git("rev-parse", "main"); mainAfter != mainBefore {
		t.Fatalf("main moved %s -> %s: reaudit must never ship", mainBefore, mainAfter)
	}

	if len(ledger.entries) != 1 {
		t.Fatalf("ledger rows = %d, want exactly the one audit binding", len(ledger.entries))
	}
	row := ledger.entries[0]
	body, err := os.ReadFile(wantReport)
	if err != nil {
		t.Fatal(err)
	}
	if row.Role != "auditor" || row.Kind != "agent_subprocess" || row.Cycle != reauditCycle || row.RunID != reauditRunID {
		t.Fatalf("binding identity = role %q kind %q cycle %d run %q", row.Role, row.Kind, row.Cycle, row.RunID)
	}
	if row.GitHEAD != fx.newBase || row.WorktreeBaseSHA != fx.newBase {
		t.Fatalf("binding git_head=%s worktree_base=%s, want both %s", row.GitHEAD, row.WorktreeBaseSHA, fx.newBase)
	}
	if row.ArtifactPath != wantReport || row.ArtifactSHA256 != reauditSHA(body) {
		t.Fatalf("binding artifact = %s sha %s, want %s sha %s", row.ArtifactPath, row.ArtifactSHA256, wantReport, reauditSHA(body))
	}
	if row.TreeStateSHA == "" || row.WorktreeTreeSHA == "" {
		t.Fatalf("binding lacks tree identity: tree_state=%q worktree_tree=%q", row.TreeStateSHA, row.WorktreeTreeSHA)
	}
}

func TestReauditCycle_RequestIsDerivedFromTheCycleState(t *testing.T) {
	fx := newReauditFixture(t, "lane.txt", "peer.txt")
	runner := fx.runner(VerdictPASS)

	if _, err := ReauditCycle(context.Background(), fx.input(runner, &fakeLedger{})); err != nil {
		t.Fatal(err)
	}

	req := runner.req
	if req.Cycle != reauditCycle || req.RunID != reauditRunID || req.ProjectRoot != fx.root.Dir ||
		req.Workspace != fx.workspace || req.Worktree != fx.worktree || req.WorktreeBaseSHA != fx.newBase {
		t.Fatalf("audit request = cycle %d run %q root %q workspace %q worktree %q base %q", req.Cycle, req.RunID, req.ProjectRoot, req.Workspace, req.Worktree, req.WorktreeBaseSHA)
	}
}

func TestReauditCycle_VerdictIsDataNotAnError(t *testing.T) {
	tests := []struct {
		verdict  string
		wantExit int
	}{
		{VerdictPASS, 0},
		{VerdictWARN, 1},
		{VerdictFAIL, 2},
	}
	for _, tt := range tests {
		t.Run(tt.verdict, func(t *testing.T) {
			fx := newReauditFixture(t, "lane.txt", "peer.txt")
			ledger := &fakeLedger{}

			res, err := ReauditCycle(context.Background(), fx.input(fx.runner(tt.verdict), ledger))

			if err != nil {
				t.Fatalf("a %s verdict returned error %v", tt.verdict, err)
			}
			if res.Verdict != tt.verdict {
				t.Fatalf("Verdict = %q, want %q", res.Verdict, tt.verdict)
			}
			if len(ledger.entries) != 1 || ledger.entries[0].ExitCode != tt.wantExit {
				t.Fatalf("binding rows = %+v, want one row with exit_code %d", ledger.entries, tt.wantExit)
			}
		})
	}
}

func TestReauditCycle_LiveLeaseRefusesBeforeAnyGitWrite(t *testing.T) {
	fx := newReauditFixture(t, "lane.txt", "peer.txt")
	fx.writeLease(time.Now())
	runner := fx.runner(VerdictPASS)
	ledger := &fakeLedger{}
	before := fx.snapshot()

	_, err := ReauditCycle(context.Background(), fx.input(runner, ledger))

	requireReauditKind(t, err, ReauditLiveLease)
	if after := fx.snapshot(); after != before {
		t.Fatalf("worktree changed under a live lease: before %+v after %+v", before, after)
	}
	if runner.calls != 0 || len(ledger.entries) != 0 {
		t.Fatalf("audit calls=%d ledger rows=%d, want none under a live lease", runner.calls, len(ledger.entries))
	}
}

func TestReauditCycle_StaleLeaseDoesNotBlock(t *testing.T) {
	fx := newReauditFixture(t, "lane.txt", "peer.txt")
	fx.writeLease(time.Now().Add(-2 * runlease.DefaultTTL))

	res, err := ReauditCycle(context.Background(), fx.input(fx.runner(VerdictPASS), &fakeLedger{}))

	if err != nil {
		t.Fatalf("a stale lease blocked the reaudit: %v", err)
	}
	if res.NewBase != fx.newBase {
		t.Fatalf("NewBase = %s, want %s", res.NewBase, fx.newBase)
	}
}

func TestReauditCycle_RebaseConflictLeavesTheWorktreeUnchanged(t *testing.T) {
	fx := newReauditFixture(t, "shared.txt", "shared.txt")
	runner := fx.runner(VerdictPASS)
	ledger := &fakeLedger{}
	before := fx.snapshot()

	_, err := ReauditCycle(context.Background(), fx.input(runner, ledger))

	re := requireReauditKind(t, err, ReauditConflict)
	if len(re.Conflicts) != 1 || re.Conflicts[0] != "shared.txt" {
		t.Fatalf("Conflicts = %v, want [shared.txt]", re.Conflicts)
	}
	if after := fx.snapshot(); after != before {
		t.Fatalf("worktree changed after a conflict: before %+v after %+v", before, after)
	}
	if runner.calls != 0 || len(ledger.entries) != 0 {
		t.Fatalf("audit calls=%d ledger rows=%d, want none after a conflict", runner.calls, len(ledger.entries))
	}
}

func TestReauditCycle_UnknownCycleIsNoSuchCycle(t *testing.T) {
	tests := []struct {
		name  string
		cycle int
	}{
		{"other cycle id", 99},
		{"zero", 0},
		{"negative", -3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fx := newReauditFixture(t, "lane.txt", "peer.txt")
			runner := fx.runner(VerdictPASS)
			in := fx.input(runner, &fakeLedger{})
			in.Cycle = tt.cycle
			before := fx.snapshot()

			_, err := ReauditCycle(context.Background(), in)

			requireReauditKind(t, err, ReauditNoCycle)
			if fx.snapshot() != before || runner.calls != 0 {
				t.Fatalf("a missing cycle touched the worktree or ran the audit")
			}
		})
	}
}

func TestReauditCycle_AuditRunnerFailureIsInfraAndRecordsNoBinding(t *testing.T) {
	fx := newReauditFixture(t, "lane.txt", "peer.txt")
	boom := errors.New("auditor pane died")
	runner := &reauditRunner{err: boom, fx: fx}
	ledger := &fakeLedger{}

	_, err := ReauditCycle(context.Background(), fx.input(runner, ledger))

	re := requireReauditKind(t, err, ReauditInfra)
	if !errors.Is(err, boom) || !errors.Is(re, boom) {
		t.Fatalf("the runner error is not reachable through Unwrap: %v", err)
	}
	if len(ledger.entries) != 0 {
		t.Fatalf("ledger rows = %d, want none when the audit did not finish", len(ledger.entries))
	}
}

func TestReauditError_ErrorNamesTheKindAndConflicts(t *testing.T) {
	err := &ReauditError{Kind: ReauditConflict, Conflicts: []string{"a.go", "b.go"}}

	msg := err.Error()

	for _, want := range []string{string(ReauditConflict), "a.go", "b.go"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("Error() = %q, missing %q", msg, want)
		}
	}
	for _, kind := range []ReauditErrorKind{ReauditLiveLease, ReauditConflict, ReauditNoCycle, ReauditInfra} {
		if (&ReauditError{Kind: kind}).Error() == "" {
			t.Fatalf("Error() is empty for kind %q", kind)
		}
	}
}

func TestReauditCycle_RebasesThroughTheSharedRebaseStep(t *testing.T) {
	var seen []string
	real := reauditRebase
	reauditRebase = func(ctx context.Context, projectRoot string, cs *CycleState) (bool, bool) {
		seen = append(seen, cs.ActiveWorktree)
		return real(ctx, projectRoot, cs)
	}
	t.Cleanup(func() { reauditRebase = real })

	fx := newReauditFixture(t, "lane.txt", "peer.txt")
	if _, err := ReauditCycle(context.Background(), fx.input(fx.runner(VerdictPASS), &fakeLedger{})); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0] != fx.worktree {
		t.Fatalf("the shared rebase step saw %v, want [%s]", seen, fx.worktree)
	}
}

func reauditSHA(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestReauditCycle_TheAuditRunsBehindTheReadOnlyFence(t *testing.T) {
	fx := newReauditFixture(t, "lane.txt", "peer.txt")
	runner := fx.runner(VerdictPASS)

	if _, err := ReauditCycle(context.Background(), fx.input(runner, &fakeLedger{})); err != nil {
		t.Fatal(err)
	}

	if runner.req.Fence == nil || runner.req.WritesSource() {
		t.Fatalf("audit fence = %+v; the re-audit is read-only, so the runner must fence the worktree", runner.req.Fence)
	}
}
