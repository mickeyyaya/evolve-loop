//go:build acs

package cycle1494

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC1494_001_ResearchRecallKDefaultsToFive(t *testing.T) {
	got := policy.Policy{}.ResearchConfig().RecallK
	if got != 5 {
		t.Errorf("RED: zero-value Policy{}.ResearchConfig().RecallK = %d, want 5 (the default MUST hold at today's research.maxResults — lowering it narrows advisor recall)", got)
	}
}

func TestC1494_002_ResearchRecallKClampsMalformedConfig(t *testing.T) {
	for _, tc := range []struct {
		name string
		pol  policy.Policy
		want int
	}{
		{"absent-block", policy.Policy{}, 5},
		{"zero", policy.Policy{Research: &policy.ResearchPolicy{RecallK: 0}}, 5},
		{"negative", policy.Policy{Research: &policy.ResearchPolicy{RecallK: -1}}, 5},
		{"absurdly-large", policy.Policy{Research: &policy.ResearchPolicy{RecallK: 100000}}, 5},
		{"in-range-3", policy.Policy{Research: &policy.ResearchPolicy{RecallK: 3}}, 3},
		{"in-range-8", policy.Policy{Research: &policy.ResearchPolicy{RecallK: 8}}, 8},
	} {
		if got := tc.pol.ResearchConfig().RecallK; got != tc.want {
			t.Errorf("RED: %s: RecallK = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestC1494_003_FileKBHonoursConfiguredRecall(t *testing.T) {
	root := writeLessonCorpus(t, 7)

	unbounded, err := research.NewFileKB([]string{root}).Lookup(context.Background(), recallQuery())
	if err != nil {
		t.Fatalf("baseline Lookup: %v", err)
	}
	if len(unbounded) != 5 {
		t.Fatalf("fixture broken: default Lookup returned %d lessons over a 7-match corpus, want 5", len(unbounded))
	}

	bounded, err := research.NewFileKBWithRecall([]string{root}, 3).Lookup(context.Background(), recallQuery())
	if err != nil {
		t.Fatalf("RED: bounded Lookup: %v", err)
	}
	if len(bounded) != 3 {
		t.Fatalf("RED: recall=3 returned %d lessons, want exactly 3 (the bound is not enforced)", len(bounded))
	}
	for i := range bounded {
		if bounded[i].ID != unbounded[i].ID {
			t.Errorf("RED: bounded[%d].ID = %q, want %q — the bound must take the top-k PREFIX of the existing deterministic ranking, not reorder or resample it", i, bounded[i].ID, unbounded[i].ID)
		}
	}
}

func TestC1494_004_FileKBDefaultConstructorRecallUnchanged(t *testing.T) {
	root := writeLessonCorpus(t, 7)

	legacy, err := research.NewFileKB([]string{root}).Lookup(context.Background(), recallQuery())
	if err != nil {
		t.Fatalf("legacy Lookup: %v", err)
	}
	if len(legacy) != 5 {
		t.Errorf("RED: NewFileKB Lookup returned %d lessons, want 5 (existing callers must see NO behaviour change)", len(legacy))
	}

	for _, k := range []int{0, -1} {
		got, err := research.NewFileKBWithRecall([]string{root}, k).Lookup(context.Background(), recallQuery())
		if err != nil {
			t.Fatalf("RED: NewFileKBWithRecall(%d) Lookup: %v", k, err)
		}
		if len(got) != 5 {
			t.Errorf("RED: NewFileKBWithRecall(%d) returned %d lessons, want the default 5 — a non-positive recall must never disable recall memory", k, len(got))
		}
	}
}

func TestC1494_005_KBCompositionRootDerivesRecallFromPolicy(t *testing.T) {
	root := acsassert.RepoRoot(t)
	src := filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle.go")

	file, err := parser.ParseFile(token.NewFileSet(), src, nil, 0)
	if err != nil {
		t.Fatalf("parse composition root %s: %v", src, err)
	}

	var withKBArg ast.Expr
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "WithKB" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "core" {
			return true
		}
		if len(call.Args) == 1 {
			withKBArg = call.Args[0]
		}
		return false
	})
	if withKBArg == nil {
		t.Fatalf("RED: no core.WithKB(<kb>) call found in %s — the KB composition root moved; re-point this predicate at the new one", src)
	}

	ctor, ok := withKBArg.(*ast.CallExpr)
	if !ok {
		t.Fatalf("RED: core.WithKB argument is not a constructor call in %s", src)
	}
	if len(ctor.Args) < 2 {
		t.Fatalf("RED: the KB is constructed with %d argument(s) in %s — the composition root still builds a KB with no recall bound, so policy.ResearchConfig().RecallK reaches nothing (dead config)", len(ctor.Args), src)
	}
	if _, isCall := ctor.Args[1].(*ast.CallExpr); !isCall {
		t.Errorf("RED: the KB recall argument in %s is %T, not a call expression — it must be RESOLVED from .evolve/policy.json (e.g. kbRecallK(projectRoot)), never a compiled literal", src, ctor.Args[1])
	}
}

func TestC1494_006_NoveltyThresholdDefaultsAndClamps(t *testing.T) {
	for _, tc := range []struct {
		name string
		pol  policy.Policy
		want float64
	}{
		{"absent-block", policy.Policy{}, 0.9},
		{"zero", policy.Policy{Research: &policy.ResearchPolicy{NoveltyThreshold: 0}}, 0.9},
		{"negative", policy.Policy{Research: &policy.ResearchPolicy{NoveltyThreshold: -0.5}}, 0.9},
		{"above-one", policy.Policy{Research: &policy.ResearchPolicy{NoveltyThreshold: 1.5}}, 0.9},
		{"in-range", policy.Policy{Research: &policy.ResearchPolicy{NoveltyThreshold: 0.75}}, 0.75},
		{"exactly-one", policy.Policy{Research: &policy.ResearchPolicy{NoveltyThreshold: 1}}, 1},
	} {
		if got := tc.pol.ResearchConfig().NoveltyThreshold; got != tc.want {
			t.Errorf("RED: %s: NoveltyThreshold = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestC1494_007_NoveltyGateSuppressesNearDuplicateLesson(t *testing.T) {
	lessonsDir := t.TempDir()
	runDir := t.TempDir()

	first := duplicateEvent(1494)
	if err := faillearn.WriteArtifacts(first, runDir, lessonsDir); err != nil {
		t.Fatalf("first WriteArtifacts: %v", err)
	}
	if n := countLessonFiles(t, lessonsDir); n != 1 {
		t.Fatalf("fixture broken: after the first write the corpus holds %d lesson(s), want 1", n)
	}

	second := duplicateEvent(1495)
	if err := faillearn.WriteArtifacts(second, t.TempDir(), lessonsDir); err != nil {
		t.Fatalf("RED: second WriteArtifacts must SKIP the near-duplicate, not error: %v", err)
	}
	if n := countLessonFiles(t, lessonsDir); n != 1 {
		t.Errorf("RED: the corpus holds %d lesson files after writing the same observation twice, want 1 — the novelty gate is not intercepting faillearn.WriteArtifacts (writer.go:59)", n)
	}
}

func TestC1494_008_NoveltyGateRetainsDistinctLesson(t *testing.T) {
	lessonsDir := t.TempDir()

	if err := faillearn.WriteArtifacts(duplicateEvent(1494), t.TempDir(), lessonsDir); err != nil {
		t.Fatalf("first WriteArtifacts: %v", err)
	}
	if err := faillearn.WriteArtifacts(distinctEvent(1495), t.TempDir(), lessonsDir); err != nil {
		t.Fatalf("RED: distinct WriteArtifacts: %v", err)
	}
	if n := countLessonFiles(t, lessonsDir); n != 2 {
		t.Errorf("RED: the corpus holds %d lesson files, want 2 — a materially different failure must NEVER be suppressed as a near-duplicate (unique failure evidence is the corpus's whole value)", n)
	}
}

func TestC1494_009_NoveltyGateMalformedCorpusEntryIsNonDestructive(t *testing.T) {
	lessonsDir := t.TempDir()
	rotten := filepath.Join(lessonsDir, "rotten.yaml")
	rottenBytes := []byte("id: [this is: not, valid yaml\n  - broken\n")
	if err := os.WriteFile(rotten, rottenBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := faillearn.WriteArtifacts(distinctEvent(1494), t.TempDir(), lessonsDir); err != nil {
		t.Fatalf("RED: a malformed corpus neighbour must not fail the lesson write: %v", err)
	}

	if n := countLessonFiles(t, lessonsDir); n != 2 {
		t.Errorf("RED: corpus holds %d files, want 2 (the rotten file plus the new lesson) — corpus rot must never suppress a new lesson", n)
	}
	after, err := os.ReadFile(rotten)
	if err != nil {
		t.Fatalf("RED: the malformed corpus file was DELETED by the write path: %v", err)
	}
	if string(after) != string(rottenBytes) {
		t.Errorf("RED: the malformed corpus file was rewritten by the write path (got %q, want %q) — consolidation must never mutate an operator's file it could not parse", after, rottenBytes)
	}
}

func recallQuery() research.Query {
	return research.Query{
		Source:      "build",
		FailureMode: "contract gate block",
		Consequence: "cycle-mid-execution-fail",
		Keywords:    []string{"worktree", "predicate"},
	}
}

func writeLessonCorpus(t *testing.T, n int) string {
	t.Helper()
	root := t.TempDir()
	for i := 0; i < n; i++ {
		body := fmt.Sprintf(`- id: lesson-%02d
  pattern: cycle-mid-execution-fail
  description: contract gate block in the build phase left the worktree predicate unsatisfied
  confidence: %.2f
  source: fixture
  type: failure-lesson
  category: episodic
  preventiveAction: re-dispatch the build with the contract escalation overlay
  failureContext:
    failedStep: build
    errorCategory: cycle-mid-execution-fail
    auditVerdict: FAIL
`, i, 0.9-float64(i)*0.05)
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("lesson-%02d.yaml", i)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func duplicateEvent(cycle int) faillearn.FailureEvent {
	return faillearn.FailureEvent{
		Cycle:          cycle,
		FailedPhase:    "build",
		Scope:          faillearn.ScopePhase,
		Classification: "cycle-mid-execution-fail",
		Verdict:        "FAIL",
		Summary:        "the build phase halted because the contract gate blocked the deliverable for the second consecutive re-dispatch",
		Defects:        []string{"contract-gate-block"},
		Now:            time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
	}
}

func distinctEvent(cycle int) faillearn.FailureEvent {
	return faillearn.FailureEvent{
		Cycle:          cycle,
		FailedPhase:    "ship",
		Scope:          faillearn.ScopePhase,
		Classification: "quota-exhausted",
		Verdict:        "FAIL",
		Summary:        "the ship phase aborted when the provider returned a quota exhaustion response and no fallback CLI family was reachable",
		Defects:        []string{"quota-exhausted"},
		Now:            time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
	}
}

func countLessonFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read lessons dir: %v", err)
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".yaml" {
			n++
		}
	}
	return n
}
