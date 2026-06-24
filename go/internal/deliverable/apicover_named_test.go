package deliverable

// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names AND
// exercises the exported symbols apicover flagged UNCOVERED:
//
//	const CodeMissingArtifact / CodeStrayInWorktree / CodeInvalidJSON /
//	      CodeMissingKey — each is the code Verify returns on the corresponding
//	      well-formedness failure. We drive Verify against a fixture that triggers
//	      each one and assert the returned violation carries that exact code.
//	func  NewVerifier — the builtin-only core.ContractVerifier constructor;
//	      exercised via VerifyDeliverable on a builtin phase (resolves) and a
//	      user phase (fails to resolve → the documented fail-open error).
//	func  NewVerifierWithCatalog — the catalog-aware constructor; exercised via
//	      VerifyDeliverable on a user phase the builtin verifier cannot resolve.
//
// (NewVerifier / NewVerifierWithCatalog return core.ContractVerifier, whose only
// method is VerifyDeliverable — so naming the constructor and invoking that
// method exercises the whole symbol, not a no-op reference.)

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// TestVerifyCodes_MissingArtifactAndStray — build's deliverable is absent from
// the workspace: Verify returns CodeMissingArtifact. When the agent instead
// wrote it into the worktree root, Verify additionally returns
// CodeStrayInWorktree (the recoverBuildLeak failure class).
func TestVerifyCodes_MissingArtifactAndStray(t *testing.T) {
	t.Parallel()
	ws, wt := t.TempDir(), t.TempDir()
	// Stray: report lives in the worktree, not the contracted workspace path.
	writeFile(t, wt, "build-report.md", "## Changes\n- x\nVerdict: PASS\n")

	res, err := Verify("build", phasecontract.Roots{Workspace: ws, Worktree: wt})
	if err != nil {
		t.Fatalf("missing+stray is a confirmed violation, not ambiguity; err=%v", err)
	}
	if res.OK {
		t.Fatal("want !OK: artifact missing from workspace")
	}
	if !hasCode(res, CodeMissingArtifact) {
		t.Errorf("want %s, got %+v", CodeMissingArtifact, res.Violations)
	}
	if !hasCode(res, CodeStrayInWorktree) {
		t.Errorf("want %s when the report is stray in the worktree, got %+v", CodeStrayInWorktree, res.Violations)
	}
}

// TestVerifyCodes_InvalidJSON — the orchestrator phase's deliverable
// (cycle-state.json, KindJSON) must be valid JSON. Garbage content → Verify
// returns CodeInvalidJSON.
func TestVerifyCodes_InvalidJSON(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// orchestrator → WriteTarget evolve_dir → artifact resolves under .evolve/.
	writeFile(t, evolveDir, "cycle-state.json", "this is not json {")

	res, err := Verify("orchestrator", phasecontract.Roots{EvolveDir: evolveDir})
	if err != nil {
		t.Fatalf("malformed JSON is a confirmed violation, not ambiguity; err=%v", err)
	}
	if res.OK {
		t.Fatal("want !OK for invalid JSON deliverable")
	}
	if !hasCode(res, CodeInvalidJSON) {
		t.Errorf("want %s, got %+v", CodeInvalidJSON, res.Violations)
	}
}

// TestVerifyCodes_MissingKey — orchestrator's contract requires top-level keys
// "cycle_id" and "phase". A valid JSON object missing "cycle_id" → Verify
// returns CodeMissingKey (tolerant reader: only required keys are checked).
func TestVerifyCodes_MissingKey(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Valid JSON object, but the required "cycle_id" key is absent.
	writeFile(t, evolveDir, "cycle-state.json", `{"phase":"build","extra":true}`)

	res, err := Verify("orchestrator", phasecontract.Roots{EvolveDir: evolveDir})
	if err != nil {
		t.Fatalf("missing required key is a confirmed violation, not ambiguity; err=%v", err)
	}
	if res.OK {
		t.Fatal("want !OK when a required JSON key is absent")
	}
	if !hasCode(res, CodeMissingKey) {
		t.Errorf("want %s for absent \"cycle_id\", got %+v", CodeMissingKey, res.Violations)
	}
}

// TestNewVerifier_BuiltinResolution — NewVerifier() returns a builtin-only
// ContractVerifier. Its VerifyDeliverable resolves a builtin phase (missing
// artifact ⇒ confirmed !OK, no error) and surfaces the fail-open ERROR for a
// user phase it cannot resolve — the contrast that proves it is builtin-only.
func TestNewVerifier_BuiltinResolution(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, "ws")

	v := NewVerifier()

	res, err := v.VerifyDeliverable(context.Background(), core.ReviewInput{
		Phase: "build", Workspace: ws, ProjectRoot: root,
	})
	if err != nil {
		t.Fatalf("builtin phase must resolve (missing artifact is !OK, not an error); err=%v", err)
	}
	if res.OK {
		t.Fatal("missing build-report.md must verify !OK")
	}

	// A user phase is unknown to the builtin resolver ⇒ fail-open error.
	if _, err := v.VerifyDeliverable(context.Background(), core.ReviewInput{
		Phase: "widget-scan", Workspace: ws, ProjectRoot: root,
	}); err == nil {
		t.Error("NewVerifier (builtin-only) must NOT resolve a user phase — want fail-open error")
	}
}

// TestNewVerifierWithCatalog_ResolvesUserPhase — NewVerifierWithCatalog falls
// back to spec-derived contracts, so it resolves a user phase the builtin-only
// NewVerifier cannot. We build the catalog from a seeded project (the same
// registry + .evolve/phases layout the host gate sees) and confirm the user
// phase now resolves (absent artifact ⇒ violation, not a resolution error).
func TestNewVerifierWithCatalog_ResolvesUserPhase(t *testing.T) {
	t.Parallel()
	root, _, ws := seedCatalogProject(t)

	cat, _, _, err := phasespec.MergedCatalog(root)
	if err != nil {
		t.Fatalf("MergedCatalog: %v", err)
	}

	in := core.ReviewInput{Phase: "widget-scan", Workspace: ws, ProjectRoot: root}

	// Precondition: builtin-only verifier cannot resolve the user phase.
	if _, err := NewVerifier().VerifyDeliverable(context.Background(), in); err == nil {
		t.Fatal("precondition: builtin-only NewVerifier must NOT resolve the user phase")
	}

	res, err := NewVerifierWithCatalog(cat).VerifyDeliverable(context.Background(), in)
	if err != nil {
		t.Fatalf("NewVerifierWithCatalog must resolve the user phase via the catalog; err=%v", err)
	}
	if res.OK {
		t.Error("user-phase artifact is absent — expected violations, got OK")
	}
}
