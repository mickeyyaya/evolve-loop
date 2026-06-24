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

// newGatesForTest exposes the production gate list so wiring pins can assert
// against what NewReviewer actually composes.
func newGatesForTest() []gate {
	return NewReviewer(config.StageShadow).(*reviewer).gates
}

// floorbinding_test.go — R9.3 (triage capacity): EGPS floor predicates must
// bind ONLY floors triage committed this cycle. The pin the plan names:
// deferred tasks ⇒ zero binding predicates (cycle-280: TDD authored floor
// predicates for deferred tasks; the builder starved the committed task
// clearing gates that were never this cycle's work).

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

// buildFloorBindingFixture lays out workspace (triage artifact) + worktree
// (predicates file) for cycle 300.
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

// triageWithDualListedCore commits a core floor AND defers more core work —
// the committed listing must win (a floor predicate on this cycle's own
// committed package is a legitimate ratchet, never a cycle-280 starvation).
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

// Bilateral declaration: the companion declares core BOTH committed and
// deferred. Committed-wins applies at equal (declared) rank — the predicate
// must pass. Pins the committedDeclared branch of the provenance rule.
func TestFloorBinding_BilateralDeclarationCommittedWins(t *testing.T) {
	in := buildFloorBindingFixture(t, deferredCorePredicates) // binds ./internal/core/
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

// TestFloorBindingGate_EllipsisPathStillBinds: the go-test wildcard form
// ("./internal/core/...") targets the same package — it must not make the
// binding invisible to the gate.
func TestFloorBindingGate_EllipsisPathStillBinds(t *testing.T) {
	g := floorBindingGate{}
	reason, block := g.check(buildFloorBindingFixture(t, ellipsisDeferredPredicates))
	if reason == "" || !block {
		t.Fatalf("ellipsis floor predicate binding the DEFERRED core floor must block; got reason=%q block=%v", reason, block)
	}
}

// TestFloorBindingGate_NonTestHelpersIgnored: only Test* functions are
// predicates; a helper named like a floor must not be extracted (it could
// false-block a healthy cycle).
func TestFloorBindingGate_NonTestHelpersIgnored(t *testing.T) {
	g := floorBindingGate{}
	if reason, block := g.check(buildFloorBindingFixture(t, helperOnlyPredicates)); reason != "" || block {
		t.Errorf("non-Test helper literals must be ignored; got %q/%v", reason, block)
	}
}

// TestFloorBindingGate_CaseInsensitiveDeferredHeading: a model writing
// "## Deferred" must not evade the gate.
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
	// The gate must actually be in the composite reviewer's gate list —
	// otherwise the pin above tests an orphan.
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

// ----------------------------------------------------------------------------
// ADR-0046 Layer 1 (cycle 305): the floor-binding gate consumes the triage
// companion's deferred_floors[] declaration instead of scraping ## deferred /
// ## dropped prose. Builder rewires floorBindingGate.check() to read
// <workspace>/triage-decision.json via the declaration-primary
// triagecap.DeferredFloorPackagesDecl wrapper.
//
// RED guarantee: TestFloorBinding_DeclaredDivergenceMessage references the
// not-yet-existing symbol triagecap.DeferredFloorDivergence, so this whole test
// package fails to compile until Builder lands the Layer-1 functions — every
// pin below is RED in the unbuilt tree. The behavioral pins
// (DeferredFromCompanion, ProseIgnoredWithCompanion) additionally fail on
// behavior once the package compiles but the gate still reads prose.
// ----------------------------------------------------------------------------

// committedCoreNoDeferred: a triage artifact that COMMITS core in ## top_n and
// has no ## deferred section. The prose path therefore finds nothing deferred;
// only a companion declaration can mark core as deferred.
const committedCoreNoDeferred = `## top_n (commit to THIS cycle)
- coverage-core: push internal/core coverage to ≥98% — priority=H
`

// deferredCoreProse: a triage artifact that DEFERS core in prose (the legacy
// authority the companion now overrides).
const deferredCoreProse = `## top_n (commit to THIS cycle)
- coverage-bridge: adapters/bridge coverage to ≥98% — priority=H

## deferred (carry to NEXT cycle's carryoverTodos)
- coverage-core: push internal/core coverage to ≥98% — defer_reason=too large
`

// writeFixtureCompanion writes a triage-decision.json beside the triage report
// in the fixture workspace. A nil deferredFloors writes a companion WITHOUT the
// deferred_floors field (the present-file/absent-field fallback case).
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

// C1: a companion that declares deferred_floors:["core"] blocks a core floor
// predicate even though the prose COMMITS core (no ## deferred section). Proves
// the declaration is the gate's source of truth.
func TestFloorBinding_DeferredFromCompanion(t *testing.T) {
	in := buildFloorBindingFixture(t, deferredCorePredicates) // binds ./internal/core/
	writeFixtureArtifact(t, in, committedCoreNoDeferred)      // prose: core committed, nothing deferred
	writeFixtureCompanion(t, in, []string{"core"})            // declaration: core deferred
	reason, block := floorBindingGate{}.check(in)
	if reason == "" || !block {
		t.Fatalf("companion deferred_floors:[core] must block the core floor predicate; got reason=%q block=%v", reason, block)
	}
}

// C2: with NO companion present and a triage artifact that commits core (no
// prose deferral), the gate must fail open — introducing companion-reading must
// not spuriously block a committed floor.
func TestFloorBinding_MissingCompanion_FailOpen(t *testing.T) {
	in := buildFloorBindingFixture(t, deferredCorePredicates)
	writeFixtureArtifact(t, in, committedCoreNoDeferred)
	// no writeFixtureCompanion → companion absent
	if reason, block := (floorBindingGate{}).check(in); reason != "" || block {
		t.Errorf("missing companion + committed core must fail open; got reason=%q block=%v", reason, block)
	}
}

// N1: a companion declaring a DIFFERENT deferred package makes prose-deferred
// core non-authoritative — the gate must NOT block core. This is the cycle-280
// class retirement: prose can no longer over-bind a floor the agent committed.
func TestFloorBinding_ProseIgnoredWithCompanion(t *testing.T) {
	in := buildFloorBindingFixture(t, deferredCorePredicates) // binds core
	writeFixtureArtifact(t, in, deferredCoreProse)            // prose defers core
	writeFixtureCompanion(t, in, []string{"recovery"})        // declaration defers recovery, NOT core
	if reason, block := (floorBindingGate{}).check(in); reason != "" || block {
		t.Errorf("declaration (recovery) must override prose (core); core predicate must pass; got reason=%q block=%v", reason, block)
	}
}

// E1: a companion that exists but omits deferred_floors falls back to the prose
// scan — the field is optional and its absence is not "zero deferred floors".
func TestFloorBinding_CompanionNoField_FallbackProse(t *testing.T) {
	in := buildFloorBindingFixture(t, deferredCorePredicates)
	writeFixtureArtifact(t, in, deferredCoreProse) // prose defers core
	writeFixtureCompanion(t, in, nil)              // companion present, no deferred_floors
	reason, block := floorBindingGate{}.check(in)
	if reason == "" || !block {
		t.Fatalf("companion without deferred_floors must fall back to prose (core deferred → block); got reason=%q block=%v", reason, block)
	}
}

// C5: the divergence reporter (consumed by `evolve guard triage-floors`) returns
// an actionable message when prose-deferred packages and the companion's
// deferred_floors disagree, and "" when they agree. This pin references the
// new triagecap.DeferredFloorDivergence symbol, which keeps the whole evalgate
// test package RED until Builder lands Layer 1.
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
