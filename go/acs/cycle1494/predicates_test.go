//go:build acs

// Package cycle1494 materialises the cycle-1494 acceptance criteria for the one
// fleet-scoped task pinned to this lane, `sleep-time-kb-consolidation`.
//
// SCOPE NOTE (why this is not the Scout plan verbatim). The premise-challenge
// gate returned FAIL/BLOCK on the plan as framed, and this phase re-probed and
// CONFIRMED its two fatal seam findings before authoring:
//
//   - `research.maxResults = 5` (go/internal/research/filekb.go:21) has exactly
//     one production consumer — `Orchestrator.recallForPlan`
//     (go/internal/core/routing_dispatch.go:281), the ADVISOR's recall memory.
//     Scout receives no KB injection at all, so a "Scout top-k" framed against
//     this seam moves no Scout tokens and would silently NARROW advisor
//     failure-recall. The criteria below therefore make the bound TYPED POLICY
//     with the default HELD AT 5 — reproducible tuning, zero behaviour change.
//   - memo does NOT write the lessons corpus (agents/evolve-memo.md:19,91,133;
//     .evolve/profiles/memo.json allows Write only to carryover-todos.json and
//     memo.md). The REAL Go lesson-write seam is
//     `faillearn.WriteArtifacts` (go/internal/faillearn/writer.go:41, line 59),
//     reached from three production call sites
//     (cmd/evolve/cmd_loop_outcome.go:452, internal/core/failure_learning.go:477,
//     internal/core/reset.go:248). The novelty gate is materialised THERE, so a
//     passing predicate cannot be vacuous against an ungated production path.
//
// The inbox item's warm-start-brief criterion is NOT materialised this cycle;
// test-report.md records that omission and its reason explicitly rather than
// minting a second, competing brief contract next to the existing (dead)
// operator→scout one.
//
// Predicate strategy — every predicate invokes the system under test in-process
// and asserts on returned values or real on-disk side effects (the cycle-85
// degenerate-predicate ban): no source greps as the load-bearing check, no
// `go test` subprocess, no whole-package sweep, no wall-clock bound, no literal
// PID, no bare `git` against process cwd. 005 is the one structural predicate —
// the composition root lives in `package main` (cmd/evolve), which cannot be
// imported and whose whole-package `go test` is a banned flaky shape, so it is
// asserted over the parsed AST (not a text grep) and Builder must additionally
// name the caller file:line in build-report.md.
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

// ---------------------------------------------------------------------------
// Task A — kb-recall-k-policy: the KB recall bound becomes typed policy with the
// default HELD at 5 (behaviour-preserving), clamped against operator typos, and
// derived from policy at the composition root instead of a compiled constant.
// ---------------------------------------------------------------------------

// TestC1494_001_ResearchRecallKDefaultsToFive pins the behaviour-preservation
// contract: an absent "research" block must resolve to the value the compiled
// constant carries today (5), so introducing the knob cannot narrow advisor
// failure-recall on any existing install.
func TestC1494_001_ResearchRecallKDefaultsToFive(t *testing.T) {
	got := policy.Policy{}.ResearchConfig().RecallK
	if got != 5 {
		t.Errorf("RED: zero-value Policy{}.ResearchConfig().RecallK = %d, want 5 (the default MUST hold at today's research.maxResults — lowering it narrows advisor recall)", got)
	}
}

// TestC1494_002_ResearchRecallKClampsMalformedConfig drives the resolver across
// the operator-typo shapes. Absent/zero/negative/out-of-range must fall back to
// the visible built-in; an in-range value must override. Malformed config that
// silently disarms or unbounds recall is the failure mode being excluded.
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

// TestC1494_003_FileKBHonoursConfiguredRecall proves the bound is actually
// enforced by the KB and that narrowing is a strict PREFIX of the existing
// deterministic ranking — i.e. the k highest-ranked lessons, not an arbitrary
// subset. Behavioural: builds a real corpus on disk and runs the real Lookup.
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

// TestC1494_004_FileKBDefaultConstructorRecallUnchanged is the negative /
// anti-regression half: the existing constructor — the one the advisor path
// uses when no policy is supplied — must keep returning 5, and a non-positive
// recall must fall back to that default rather than returning zero lessons
// (a zero-recall KB silently disables recall memory, the exact degradation the
// premise-challenge flagged).
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

// TestC1494_005_KBCompositionRootDerivesRecallFromPolicy is the WIRING proof: a
// policy knob whose value never reaches the production KB construction is dead
// config, and every predicate above would still pass. cmd/evolve is `package
// main` (unimportable) and its whole-package `go test` is a banned flaky shape,
// so this asserts over the PARSED AST of the composition root: the argument to
// core.WithKB must be a KB constructed with a recall argument that is itself a
// call expression (a policy-derived resolution), never a literal constant.
//
// Builder must ALSO name the caller file:line in build-report.md.
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

// ---------------------------------------------------------------------------
// Task B — kb-novelty-gate: near-duplicate suppression on the REAL Go
// lesson-write seam (faillearn.WriteArtifacts), threshold from typed policy.
// ---------------------------------------------------------------------------

// TestC1494_006_NoveltyThresholdDefaultsAndClamps pins the second typed knob.
// A similarity threshold is only meaningful inside (0,1]: 0 would suppress
// every write (evidence loss) and >1 would suppress none (gate disarmed), and
// both are typo shapes an operator would never intend.
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

// TestC1494_007_NoveltyGateSuppressesNearDuplicateLesson is the inbox item's
// literal regression: "identical observation twice -> one write". The two
// events differ ONLY in cycle number, so faillearn's own id derivation
// ("cycle-N-<scope>-<slug>") gives them DIFFERENT filenames — writeIfAbsent's
// exact-path dedupe cannot catch it, which is why the corpus grows unbounded
// today. Behavioural: runs the real production writer against a real directory
// and counts what actually landed on disk.
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

// TestC1494_008_NoveltyGateRetainsDistinctLesson is the negative test that keeps
// the gate honest: a gate that suppresses everything trivially passes 007 while
// destroying the corpus. A materially different failure must still be written.
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

// TestC1494_009_NoveltyGateMalformedCorpusEntryIsNonDestructive is the edge/OOD
// case. Corpus rot is real (parseLessonFile documents it). A gate that treats an
// unparseable neighbour as a reason to drop the incoming lesson would delete the
// very failure evidence it exists to preserve; a gate that rewrites or removes
// the rotten file would destroy an operator's data. Neither is permitted: the
// new lesson lands, and the malformed bytes are left exactly as found.
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

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

// recallQuery is the query every recall predicate scores against. Shared so the
// bounded and unbounded rankings are comparable.
func recallQuery() research.Query {
	return research.Query{
		Source:      "build",
		FailureMode: "contract gate block",
		Consequence: "cycle-mid-execution-fail",
		Keywords:    []string{"worktree", "predicate"},
	}
}

// writeLessonCorpus writes n distinct, all-matching lessons into a fresh temp
// dir and returns it. Confidence descends with the index so the deterministic
// ranking has a stable, non-tied order the prefix assertion can rely on.
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

// duplicateEvent renders the SAME observation at a different cycle number — the
// shape that defeats writeIfAbsent's exact-path dedupe today.
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

// distinctEvent is a materially different failure: different phase, different
// classification, different summary vocabulary.
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

// countLessonFiles counts the *.yaml lessons actually on disk — the corpus as
// research.listLessonFiles would see it.
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
