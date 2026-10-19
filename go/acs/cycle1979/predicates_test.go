//go:build acs

package cycle1979

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cycleUnderAudit = 7
	runUnderAudit   = "run-1979"
)

type movedBase struct {
	t         *testing.T
	root      *gittest.Repo
	evolveDir string
	worktree  string
	workspace string
	oldBase   string
	newBase   string
}

func newMovedBase(t *testing.T, laneFile, peerFile string) *movedBase {
	t.Helper()
	root := gittest.Fixture(t)
	root.Write(".gitignore", ".evolve/\n")
	root.Write("shared.txt", "line a\nline b\nline c\n")
	root.Git("add", "-A")
	root.Git("commit", "-q", "-m", "base")
	fx := &movedBase{
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
		CycleID: cycleUnderAudit, Phase: "ship", RunID: runUnderAudit,
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

func (fx *movedBase) snapshot() [3]string {
	return [3]string{
		fx.root.Git("-C", fx.worktree, "rev-parse", "HEAD"),
		fx.root.Git("-C", fx.worktree, "status", "--porcelain"),
		fx.root.Git("-C", fx.worktree, "rev-parse", "--abbrev-ref", "HEAD"),
	}
}

type passingAuditor struct{ calls int }

func (a *passingAuditor) Name() string { return string(core.PhaseAudit) }

func (a *passingAuditor) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	a.calls++
	if err := os.WriteFile(filepath.Join(req.Workspace, "audit-report.md"), []byte("# Audit Report\n\nVerdict: PASS\n"), 0o644); err != nil {
		return core.PhaseResponse{}, err
	}
	return core.PhaseResponse{Phase: string(core.PhaseAudit), Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func (fx *movedBase) input(a *passingAuditor) core.ReauditInput {
	return core.ReauditInput{
		ProjectRoot: fx.root.Dir,
		EvolveDir:   fx.evolveDir,
		Cycle:       cycleUnderAudit,
		Ledger:      ledger.New(fx.evolveDir),
		Audit:       a,
	}
}

func TestC1979_001_MovedBaseIsRebasedAuditedAndBoundForShip(t *testing.T) {
	fx := newMovedBase(t, "lane.txt", "peer.txt")
	auditor := &passingAuditor{}

	res, err := core.ReauditCycle(context.Background(), fx.input(auditor))
	if err != nil {
		t.Fatalf("ReauditCycle: %v", err)
	}

	if auditor.calls != 1 {
		t.Errorf("audit ran %d times, want 1", auditor.calls)
	}
	if parent := fx.root.Git("-C", fx.worktree, "rev-parse", "HEAD^"); parent != fx.newBase {
		t.Errorf("worktree HEAD^ = %s, want the moved base %s", parent, fx.newBase)
	}
	if res.NewBase != fx.newBase || res.Verdict != core.VerdictPASS {
		t.Errorf("result = %+v, want base %s verdict PASS", res, fx.newBase)
	}
	entry, err := auditledger.LatestAuditorEntry(filepath.Join(fx.evolveDir, "ledger.jsonl"), runUnderAudit)
	if err != nil {
		t.Fatalf("ship cannot bind the recorded audit to run %s: %v", runUnderAudit, err)
	}
	if entry.GitHEAD != fx.newBase || entry.WorktreeBaseSHA != fx.newBase || entry.ArtifactPath != res.VerdictPath {
		t.Errorf("binding = head %s base %s artifact %s, want head and base %s artifact %s",
			entry.GitHEAD, entry.WorktreeBaseSHA, entry.ArtifactPath, fx.newBase, res.VerdictPath)
	}
	if entry.TreeStateSHA == "" || entry.WorktreeTreeSHA == "" || entry.ArtifactSHA256 == "" {
		t.Errorf("binding lacks tree or artifact identity: %+v", entry)
	}
}

func TestC1979_002_LiveLeaseAndRebaseConflictLeaveTheWorktreeUnchanged(t *testing.T) {
	t.Run("live lease", func(t *testing.T) {
		fx := newMovedBase(t, "lane.txt", "peer.txt")
		if err := runlease.Write(fx.workspace, runlease.Lease{RunID: runUnderAudit, OwnerPID: os.Getpid()}, time.Now()); err != nil {
			t.Fatal(err)
		}
		auditor := &passingAuditor{}
		before := fx.snapshot()

		_, err := core.ReauditCycle(context.Background(), fx.input(auditor))

		var re *core.ReauditError
		if !errors.As(err, &re) || re.Kind != core.ReauditLiveLease {
			t.Fatalf("error = %v, want a ReauditError of kind live-lease", err)
		}
		if fx.snapshot() != before || auditor.calls != 0 {
			t.Errorf("worktree changed or audit ran under a live lease")
		}
	})
	t.Run("rebase conflict", func(t *testing.T) {
		fx := newMovedBase(t, "shared.txt", "shared.txt")
		auditor := &passingAuditor{}
		before := fx.snapshot()

		_, err := core.ReauditCycle(context.Background(), fx.input(auditor))

		var re *core.ReauditError
		if !errors.As(err, &re) || re.Kind != core.ReauditConflict {
			t.Fatalf("error = %v, want a ReauditError of kind rebase-conflict", err)
		}
		if strings.Join(re.Conflicts, ",") != "shared.txt" {
			t.Errorf("Conflicts = %v, want [shared.txt]", re.Conflicts)
		}
		if fx.snapshot() != before || auditor.calls != 0 {
			t.Errorf("worktree changed or audit ran after a conflict")
		}
	})
}

const sharedRebaseStep = "rebaseRecordingConflicts"

func resolvedRebaseCallers(t *testing.T, sources map[string]string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	var files []*ast.File
	for name, src := range sources {
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, f)
	}
	aliases := map[string]bool{}
	for _, f := range files {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				if len(vs.Names) == 1 && len(vs.Values) == 1 {
					if id, ok := vs.Values[0].(*ast.Ident); ok && id.Name == sharedRebaseStep {
						aliases[vs.Names[0].Name] = true
					}
				}
			}
		}
	}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if as, ok := n.(*ast.AssignStmt); ok {
				for _, lhs := range as.Lhs {
					if id, ok := lhs.(*ast.Ident); ok {
						delete(aliases, id.Name)
					}
				}
			}
			return true
		})
	}
	resolved := map[string]bool{}
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if id, ok := call.Fun.(*ast.Ident); ok && (id.Name == sharedRebaseStep || aliases[id.Name]) {
						resolved[fn.Name.Name] = true
					}
				}
				return true
			})
		}
	}
	return resolved
}

func productionCoreSources(t *testing.T) map[string]string {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "core")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = string(raw)
	}
	return sources
}

func TestC1979_003_VerbAndRecoveryChainCallOneRebaseStep(t *testing.T) {
	resolved := resolvedRebaseCallers(t, productionCoreSources(t))

	for _, caller := range []string{"recoverFromShipError", "resumeFleetRebaseAfterDebugger", "ReauditCycle"} {
		if !resolved[caller] {
			t.Errorf("%s does not reach %s: the verb and the recovery chain must share one rebase step", caller, sharedRebaseStep)
		}
	}
}

func TestC1979_004_SharedStepCheckRejectsADivergentOrReassignedRebase(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{"direct call", "package p\nfunc ReauditCycle() { rebaseRecordingConflicts() }\n", true},
		{"alias of the step", "package p\nvar step = rebaseRecordingConflicts\nfunc ReauditCycle() { step() }\n", true},
		{"own rebase function", "package p\nfunc ReauditCycle() { localRebase() }\nfunc localRebase() {}\n", false},
		{"alias reassigned in production code", "package p\nvar step = rebaseRecordingConflicts\nfunc init() { step = other }\nfunc ReauditCycle() { step() }\n", false},
		{"no caller at all", "package p\nfunc ReauditCycle() {}\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvedRebaseCallers(t, map[string]string{"x.go": tt.src})["ReauditCycle"]

			if got != tt.want {
				t.Errorf("ReauditCycle resolved = %v, want %v", got, tt.want)
			}
		})
	}
}
