package deliverable

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phasespec"
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

// Phase 3.8 (ADR-0050): the catalog-aware ...Stage constructors actually thread
// the EVOLVE_PHASE_IO dial onto the gate/verifier. A build report that
// self-reports FAIL without a structured failure block is blocked at enforce and
// approved (dormant) at off — proving the phaseIO param is wired, not dropped.
func TestNewReviewerWithCatalogStage_ThreadsPhaseIO(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, ws, "build-report.md", failReport("build", "## Changes", false))
	in := core.ReviewInput{Phase: "build", Workspace: ws, ProjectRoot: root}
	for _, tc := range []struct {
		phaseIO   config.Stage
		wantBlock bool
	}{
		{config.StageOff, false},
		{config.StageEnforce, true},
	} {
		rev := NewReviewerWithCatalogStage(config.StageEnforce, userCatalogWithFoo(), tc.phaseIO).(*Reviewer)
		rev.breakerPath = filepath.Join(t.TempDir(), "breaker.json")
		rev.logf = func(string, ...any) {}
		got := rev.Review(context.Background(), in)
		if tc.wantBlock && got.Approve {
			t.Errorf("phaseIO=%s: want BLOCK, got approve", tc.phaseIO)
		}
		if !tc.wantBlock && !got.Approve {
			t.Errorf("phaseIO=%s: want approve (dormant), got block (%s)", tc.phaseIO, got.Reason)
		}
	}
}

func TestNewVerifierWithCatalogStage_ThreadsPhaseIO(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, ws, "build-report.md", failReport("build", "## Changes", false))
	in := core.ReviewInput{Phase: "build", Workspace: ws, ProjectRoot: root}

	offRes, err := NewVerifierWithCatalogStage(userCatalogWithFoo(), config.StageOff).VerifyDeliverable(context.Background(), in)
	if err != nil {
		t.Fatalf("off: unexpected error: %v", err)
	}
	if !offRes.OK {
		t.Errorf("off: ladder re-check must be dormant; got %+v", offRes.Violations)
	}

	enfRes, err := NewVerifierWithCatalogStage(userCatalogWithFoo(), config.StageEnforce).VerifyDeliverable(context.Background(), in)
	if err != nil {
		t.Fatalf("enforce: unexpected error: %v", err)
	}
	if enfRes.OK {
		t.Errorf("enforce: ladder re-check must surface failure_context_missing; got OK")
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
