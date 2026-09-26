package deliverable

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestVerifyCodes_MissingArtifactAndStray(t *testing.T) {
	t.Parallel()
	ws, wt := t.TempDir(), t.TempDir()
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

func TestVerifyCodes_InvalidJSON(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// orchestrator's WriteTarget is evolve_dir, so the artifact resolves under .evolve/.
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

func TestVerifyCodes_MissingKey(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
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

	if _, err := v.VerifyDeliverable(context.Background(), core.ReviewInput{
		Phase: "widget-scan", Workspace: ws, ProjectRoot: root,
	}); err == nil {
		t.Error("NewVerifier (builtin-only) must NOT resolve a user phase — want fail-open error")
	}
}

func TestNewVerifierWithCatalog_ResolvesUserPhase(t *testing.T) {
	t.Parallel()
	root, _, ws := seedCatalogProject(t)

	cat, _, _, err := phasespec.MergedCatalog(root)
	if err != nil {
		t.Fatalf("MergedCatalog: %v", err)
	}

	in := core.ReviewInput{Phase: "widget-scan", Workspace: ws, ProjectRoot: root}

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

func TestSummarizeBadVerdictBaseline_NamesAndExercises(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, BadVerdictBaselineFile)
	body := `{"event_type":"bad_verdict_classified","recoverable":true,"pattern":"fenced-json"}
{"event_type":"bad_verdict_classified","recoverable":false,"pattern":""}
{"event_type":"phase_started"}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", BadVerdictBaselineFile, err)
	}
	defer func() { _ = f.Close() }()

	var got BaselineSummary
	got, err = SummarizeBadVerdictBaseline(f)
	if err != nil {
		t.Fatalf("SummarizeBadVerdictBaseline: %v", err)
	}
	if got.Total != 2 || got.Recoverable != 1 {
		t.Errorf("Total/Recoverable = %d/%d, want 2/1 (phase_started is a foreign event)", got.Total, got.Recoverable)
	}
	if got.Rate != 0.5 {
		t.Errorf("Rate = %v, want 0.5", got.Rate)
	}
	if got.ByPattern[SalvagePatternFencedJSON] != 1 {
		t.Errorf("ByPattern[%q] = %d, want 1", SalvagePatternFencedJSON, got.ByPattern[SalvagePatternFencedJSON])
	}
	if _, ok := got.ByPattern[SalvagePatternNone]; ok {
		t.Errorf("the empty pattern of a non-recoverable record manufactured a phantom bucket: %+v", got.ByPattern)
	}
}
