package evalgate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
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
