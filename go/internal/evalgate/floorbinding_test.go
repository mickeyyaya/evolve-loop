package evalgate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

func newGatesForTest() []gate {
	return NewReviewer(config.StageShadow).(*reviewer).gates
}

const deferredCorePredicates = `//go:build acs

package cycle300

import "testing"

func TestC300_020_CoreCoverageFloor(t *testing.T) {
	pct, _ := coverageTotal(t, "./internal/core/")
	if pct < 98.0 {
		t.Errorf("RED: internal/core coverage = %.1f%%", pct)
	}
}
`

const committedBridgePredicates = `//go:build acs

package cycle300

import "testing"

func TestC300_020_BridgeCoverageFloor(t *testing.T) {
	pct, _ := coverageTotal(t, "./internal/adapters/bridge/")
	if pct < 98.0 {
		t.Errorf("RED: bridge coverage = %.1f%%", pct)
	}
}

func TestC300_001_SomeBehaviour(t *testing.T) {
	// non-floor predicate naming a deferred package is FINE — only floor
	// predicates bind floors.
	_ = "./internal/core/"
}
`

const triageWithDeferredCore = `## top_n (commit to THIS cycle)
- coverage-bridge: adapters/bridge coverage to ≥98% — priority=H

## deferred (carry to NEXT cycle's carryoverTodos)
- coverage-core: push core coverage to ≥98% — defer_reason=too large
`

func buildFloorBindingFixture(t *testing.T, predicates string) core.ReviewInput {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-300")
	wt := filepath.Join(root, "wt")
	acs := filepath.Join(wt, "go", "acs", "cycle300")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(acs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(triageWithDeferredCore), 0o644); err != nil {
		t.Fatal(err)
	}
	if predicates != "" {
		if err := os.WriteFile(filepath.Join(acs, "predicates_test.go"), []byte(predicates), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return core.ReviewInput{Phase: "tdd", Workspace: ws, Worktree: wt, ProjectRoot: root}
}

func TestFloorBindingGate_DeferredFloorPredicateBlocks(t *testing.T) {
	g := floorBindingGate{}
	if !g.appliesTo("tdd") || g.appliesTo("triage") {
		t.Fatal("floorBindingGate must apply to the tdd phase only")
	}
	reason, block := g.check(buildFloorBindingFixture(t, deferredCorePredicates))
	if reason == "" || !block {
		t.Fatalf("floor predicate binding the DEFERRED core floor must block; got reason=%q block=%v", reason, block)
	}
}

const triageWithDualListedCore = `## top_n (commit to THIS cycle)
- coverage-core-now: push internal/core coverage to ≥90% — priority=H

## deferred (carry to NEXT cycle's carryoverTodos)
- coverage-core-later: push core coverage the rest of the way to ≥98% — defer_reason=too large for one cycle
`

func TestFloorBindingGate_CommittedWinsOverDeferredMention(t *testing.T) {
	g := floorBindingGate{}
	in := buildFloorBindingFixture(t, deferredCorePredicates)
	if err := os.WriteFile(filepath.Join(in.Workspace, "triage-report.md"), []byte(triageWithDualListedCore), 0o644); err != nil {
		t.Fatal(err)
	}
	reason, block := g.check(in)
	if reason != "" || block {
		t.Errorf("core is COMMITTED this cycle (and also deferred-for-later) — its floor predicate must pass; got reason=%q block=%v", reason, block)
	}
}

func TestFloorBinding_BilateralDeclarationCommittedWins(t *testing.T) {
	in := buildFloorBindingFixture(t, deferredCorePredicates)
	if err := os.WriteFile(filepath.Join(in.Workspace, triagecap.TriageDecisionName()),
		[]byte(`{"committed_floors":["core"],"deferred_floors":["core"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if reason, block := (floorBindingGate{}).check(in); reason != "" || block {
		t.Errorf("bilateral declaration: committed wins at equal rank; got reason=%q block=%v", reason, block)
	}
}

func TestFloorBindingGate_CommittedFloorPredicatePasses(t *testing.T) {
	g := floorBindingGate{}
	reason, block := g.check(buildFloorBindingFixture(t, committedBridgePredicates))
	if reason != "" || block {
		t.Errorf("committed-floor predicate must pass; got reason=%q block=%v", reason, block)
	}
}

const ellipsisDeferredPredicates = `//go:build acs

package cycle300

import "testing"

func TestC300_022_CoreCoverageFloor(t *testing.T) {
	pct, _ := coverageTotal(t, "./internal/core/...")
	if pct < 98.0 {
		t.Errorf("RED: core coverage = %.1f%%", pct)
	}
}
`

const helperOnlyPredicates = `//go:build acs

package cycle300

import "testing"

// floorPercentHelper is a non-Test helper — its literals must NOT be
// treated as floor-predicate targets.
func floorPercentHelper() string { return "./internal/core/" }

func TestC300_001_SomeBehaviour(t *testing.T) {
	_ = floorPercentHelper()
}
`

func TestFloorBindingGate_EllipsisPathStillBinds(t *testing.T) {
	g := floorBindingGate{}
	reason, block := g.check(buildFloorBindingFixture(t, ellipsisDeferredPredicates))
	if reason == "" || !block {
		t.Fatalf("ellipsis floor predicate binding the DEFERRED core floor must block; got reason=%q block=%v", reason, block)
	}
}

func TestFloorBindingGate_NonTestHelpersIgnored(t *testing.T) {
	g := floorBindingGate{}
	if reason, block := g.check(buildFloorBindingFixture(t, helperOnlyPredicates)); reason != "" || block {
		t.Errorf("non-Test helper literals must be ignored; got %q/%v", reason, block)
	}
}

func TestFloorBindingGate_CaseInsensitiveDeferredHeading(t *testing.T) {
	in := buildFloorBindingFixture(t, deferredCorePredicates)
	artifact := "## top_n (commit to THIS cycle)\n- coverage-bridge: adapters/bridge coverage to ≥98%\n\n" +
		"## Deferred (carry to NEXT cycle)\n- coverage-core: push core coverage to ≥98%\n"
	if err := os.WriteFile(filepath.Join(in.Workspace, "triage-report.md"), []byte(artifact), 0o644); err != nil {
		t.Fatal(err)
	}
	g := floorBindingGate{}
	if reason, block := g.check(in); reason == "" || !block {
		t.Errorf("capitalised ## Deferred heading must still be recognised; got %q/%v", reason, block)
	}
}

func TestFloorBindingGate_NoPredicatesFileFailsOpen(t *testing.T) {
	g := floorBindingGate{}
	if reason, block := g.check(buildFloorBindingFixture(t, "")); reason != "" || block {
		t.Errorf("missing predicates file is ambiguity — fail open; got %q/%v", reason, block)
	}
}

func TestFloorBindingGate_WiredIntoReviewer(t *testing.T) {
	found := false
	for _, g := range newGatesForTest() {
		if g.name() == "floor-binding" {
			found = true
		}
	}
	if !found {
		t.Fatal("floorBindingGate is not wired into NewReviewer's gate list")
	}
}

func TestCycleNumFromWorkspace_NonMatchingPath(t *testing.T) {
	if got := cycleNumFromWorkspace(filepath.Join(t.TempDir(), "not-a-cycle")); got != 0 {
		t.Fatalf("non-cycle workspace basename parsed as %d, want 0", got)
	}
}

func TestCycleNumFromWorkspace_PlainDir(t *testing.T) {
	if got := cycleNumFromWorkspace("workspace"); got != 0 {
		t.Fatalf("plain workspace basename parsed as %d, want 0", got)
	}
}

func TestCycleNumFromWorkspace_NumericOverflow(t *testing.T) {
	overflowing := "cycle-9999999999999999999999999"
	if got := cycleNumFromWorkspace(filepath.Join(t.TempDir(), overflowing)); got != 0 {
		t.Fatalf("overflowing cycle number parsed as %d, want 0", got)
	}
}

const committedCoreNoDeferred = `## top_n (commit to THIS cycle)
- coverage-core: push internal/core coverage to ≥98% — priority=H
`

const deferredCoreProse = `## top_n (commit to THIS cycle)
- coverage-bridge: adapters/bridge coverage to ≥98% — priority=H

## deferred (carry to NEXT cycle's carryoverTodos)
- coverage-core: push internal/core coverage to ≥98% — defer_reason=too large
`

// writeFixtureCompanion omits the deferred_floors field when deferredFloors is nil.
func writeFixtureCompanion(t *testing.T, in core.ReviewInput, deferredFloors []string) {
	t.Helper()
	var body string
	if deferredFloors == nil {
		body = `{"cycle":300,"top_n":[]}`
	} else {
		q := make([]string, len(deferredFloors))
		for i, f := range deferredFloors {
			q[i] = `"` + f + `"`
		}
		body = `{"cycle":300,"deferred_floors":[` + strings.Join(q, ",") + `]}`
	}
	path := filepath.Join(in.Workspace, triagecap.TriageDecisionName())
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeFixtureArtifact(t *testing.T, in core.ReviewInput, artifact string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(in.Workspace, "triage-report.md"), []byte(artifact), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFloorBinding_DeferredFromCompanion(t *testing.T) {
	in := buildFloorBindingFixture(t, deferredCorePredicates)
	writeFixtureArtifact(t, in, committedCoreNoDeferred)
	writeFixtureCompanion(t, in, []string{"core"})
	reason, block := floorBindingGate{}.check(in)
	if reason == "" || !block {
		t.Fatalf("companion deferred_floors:[core] must block the core floor predicate; got reason=%q block=%v", reason, block)
	}
}

func TestFloorBinding_MissingCompanion_FailOpen(t *testing.T) {
	in := buildFloorBindingFixture(t, deferredCorePredicates)
	writeFixtureArtifact(t, in, committedCoreNoDeferred)
	if reason, block := (floorBindingGate{}).check(in); reason != "" || block {
		t.Errorf("missing companion + committed core must fail open; got reason=%q block=%v", reason, block)
	}
}

func TestFloorBinding_ProseIgnoredWithCompanion(t *testing.T) {
	in := buildFloorBindingFixture(t, deferredCorePredicates)
	writeFixtureArtifact(t, in, deferredCoreProse)
	writeFixtureCompanion(t, in, []string{"recovery"})
	if reason, block := (floorBindingGate{}).check(in); reason != "" || block {
		t.Errorf("declaration (recovery) must override prose (core); core predicate must pass; got reason=%q block=%v", reason, block)
	}
}

func TestFloorBinding_CompanionNoField_FallbackProse(t *testing.T) {
	in := buildFloorBindingFixture(t, deferredCorePredicates)
	writeFixtureArtifact(t, in, deferredCoreProse)
	writeFixtureCompanion(t, in, nil)
	reason, block := floorBindingGate{}.check(in)
	if reason == "" || !block {
		t.Fatalf("companion without deferred_floors must fall back to prose (core deferred → block); got reason=%q block=%v", reason, block)
	}
}

func TestFloorBinding_DeclaredDivergenceMessage(t *testing.T) {
	dir := t.TempDir()
	companion := filepath.Join(dir, triagecap.TriageDecisionName())
	if err := os.WriteFile(companion, []byte(`{"cycle":305,"deferred_floors":["core"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	known := []string{"core", "bridge"}

	// Prose defers bridge; declaration defers core → divergence.
	diverge := "## top_n\n- x: y\n\n## deferred\n- coverage-bridge: bridge coverage ≥98%\n"
	if msg := triagecap.DeferredFloorDivergence(diverge, companion, known); msg == "" {
		t.Error("prose/declaration divergence must yield a non-empty corrective message")
	}

	// Prose defers core; declaration defers core → agreement, silent.
	agree := "## top_n\n- x: y\n\n## deferred\n- coverage-core: core coverage ≥98%\n"
	if msg := triagecap.DeferredFloorDivergence(agree, companion, known); msg != "" {
		t.Errorf("matching prose/declaration must be silent, got %q", msg)
	}
}
