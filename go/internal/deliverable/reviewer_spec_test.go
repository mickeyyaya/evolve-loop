package deliverable

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// userCatalogWithFoo builds a catalog carrying one user phase "foo" with a
// spec-derived contract requiring a ## Findings section.
func userCatalogWithFoo() phasespec.Catalog {
	foo := phasespec.PhaseSpec{
		Name:     "foo",
		Role:     "evaluate",
		Classify: &phasespec.ClassifyRules{RequireSections: []string{"Findings"}},
		Outputs:  phasespec.IO{Files: []string{".evolve/runs/cycle-{cycle}/foo-report.md"}},
	}
	cat, _ := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{foo})
	return cat
}

func TestReviewerWithCatalog_BlocksMalformedUserPhase(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	// A user-phase report MISSING the required section.
	if err := os.WriteFile(filepath.Join(ws, "foo-report.md"), []byte("# Foo\nno heading\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	breaker := filepath.Join(root, "breaker.json")
	rev := NewReviewerWithCatalog(config.StageEnforce, userCatalogWithFoo())
	// Reach into the concrete type to point the breaker at a temp file.
	rev.(*Reviewer).breakerPath = breaker

	got := rev.Review(context.Background(), core.ReviewInput{
		Phase: "foo", Workspace: ws, ProjectRoot: root,
	})
	if got.Approve {
		t.Errorf("expected the host gate to BLOCK a malformed user phase (was fail-open before WS-A); got Approve=true")
	}
}

func TestReviewerWithCatalog_ApprovesWellFormedUserPhase(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "foo-report.md"), []byte("## Findings\n- clean\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rev := NewReviewerWithCatalog(config.StageEnforce, userCatalogWithFoo())
	rev.(*Reviewer).breakerPath = filepath.Join(root, "breaker.json")
	got := rev.Review(context.Background(), core.ReviewInput{Phase: "foo", Workspace: ws, ProjectRoot: root})
	if !got.Approve {
		t.Errorf("expected approval for a well-formed user phase; got %+v", got)
	}
}

func TestNewReviewer_BackCompatBuiltins(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte("## Changes\n- x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rev := NewReviewer(config.StageEnforce)
	rev.(*Reviewer).breakerPath = filepath.Join(root, "breaker.json")
	got := rev.Review(context.Background(), core.ReviewInput{Phase: "build", Workspace: ws, ProjectRoot: root})
	if !got.Approve {
		t.Errorf("built-in build via NewReviewer should approve; got %+v", got)
	}
}

// TestReviewerWithCatalog_FailsOpenWhenNoContractResolves — test-plan P0 #5:
// a native/no-output phase that resolves to NO contract (CatalogResolver
// miss) is AMBIGUITY at the gate even at StageEnforce: approve (fail open),
// and the consecutive-block breaker must not move. This is the
// "[contract-gate] ship: ambiguity, failing open" line from the 2026-06-12
// soak — pinned so the fail-open never silently becomes a block (or a
// breaker leak) for contract-less phases.
func TestReviewerWithCatalog_FailsOpenWhenNoContractResolves(t *testing.T) {
	root := t.TempDir()
	breaker := filepath.Join(root, "breaker.json")
	// Catalog with NO matching spec — "ghost-phase" resolves nowhere.
	rev := NewReviewerWithCatalog(config.StageEnforce, phasespec.Catalog{})
	rev.(*Reviewer).breakerPath = breaker

	got := rev.Review(context.Background(), core.ReviewInput{
		Phase: "ghost-phase", Workspace: filepath.Join(root, "ws"), ProjectRoot: root,
	})
	if !got.Approve {
		t.Errorf("no resolvable contract = ambiguity; the gate must FAIL OPEN at enforce, got %+v", got)
	}
	if _, err := os.Stat(breaker); err == nil {
		raw, _ := os.ReadFile(breaker)
		t.Errorf("ambiguity must not touch the circuit breaker; breaker file written: %s", raw)
	}
}
