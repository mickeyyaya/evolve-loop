package core_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type materializationWorktree struct{ path string }

func (w materializationWorktree) Create(string, int) (string, error) { return w.path, nil }
func (materializationWorktree) Cleanup(string, string) error         { return nil }

func initMaterializationRepo(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.invalid"},
		{"config", "user.name", "Test"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", ".gitignore"}, {"commit", "-q", "-m", "test base"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

// materializationScoutRunner plays the scout phase across a correction
// round-trip: call 1 selects a slug but writes no eval (the live defect
// class); call 2+ writes the eval at the workspace path found in the
// CorrectionDirective it was re-dispatched with — the same way a complying
// agent would after reading the gate's remediation.
type materializationScoutRunner struct {
	slug       string
	requests   []core.PhaseRequest
	createEval bool // when false, the scout never writes the eval
}

func (r *materializationScoutRunner) Name() string { return string(core.PhaseScout) }

func (r *materializationScoutRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.requests = append(r.requests, req)
	report := "## Selected Tasks\n\n### Task 1\n- **Slug:** " + r.slug + "\n\n"
	if err := os.WriteFile(filepath.Join(req.Workspace, "scout-report.md"), []byte(report), 0o644); err != nil {
		return core.PhaseResponse{}, err
	}
	// A complying re-dispatch creates the eval at exactly the path named in
	// the correction directive — never at any other root (the sandbox
	// projectRoot is deny-write; only the workspace path is both writable and
	// gate-sufficient, per materialization.go's remediation() contract).
	if r.createEval && len(r.requests) > 1 {
		evalPath := filepath.Join(req.Workspace, ".evolve", "evals", r.slug+".md")
		if err := os.MkdirAll(filepath.Dir(evalPath), 0o755); err != nil {
			return core.PhaseResponse{}, err
		}
		// A complying scout writes the [code] grader the remediation requires,
		// not only the legacy bash fence.
		body := "# Eval " + r.slug + "\n\n- [code] `go test ./internal/widget/...`\n\n```bash\ngo test ./internal/widget/...\n```\n"
		if err := os.WriteFile(evalPath, []byte(body), 0o644); err != nil {
			return core.PhaseResponse{}, err
		}
	}
	return core.PhaseResponse{Phase: string(core.PhaseScout), Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

// runMaterializationCycle wires the REAL evalgate reviewer (production
// composition, not a fake) at enforce, with the given scout runner standing
// in for every other phase runner unchanged.
func runMaterializationCycle(t *testing.T, scout *materializationScoutRunner) error {
	t.Helper()
	root := t.TempDir()
	initMaterializationRepo(t, root)
	st := &fixtures.FakeStorage{}
	runners := fixtures.BuildRunners(nil)
	runners[core.PhaseScout] = scout
	reviewer := evalgate.NewReviewer(config.StageEnforce)
	o := core.NewOrchestrator(st, &fixtures.FakeLedger{}, runners,
		core.WithReviewer(reviewer), core.WithWorktreeProvisioner(materializationWorktree{path: root}))
	_, err := o.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true,
	})
	return err
}

func TestMaterializationCorrectionDelivery_RemediationReachesReDispatchAndClearsTheRealGate(t *testing.T) {
	scout := &materializationScoutRunner{slug: "widget-thing", createEval: true}
	if err := runMaterializationCycle(t, scout); err != nil {
		t.Fatalf("complying correction must clear the real materialization gate: %v", err)
	}

	if len(scout.requests) < 2 {
		t.Fatalf("want at least 2 scout dispatches (initial rejection + complying correction); got %d", len(scout.requests))
	}
	directive := scout.requests[1].CorrectionDirective
	if directive == "" {
		t.Fatalf("second dispatch must carry a non-empty correction directive")
	}
	wantPath := filepath.Join(scout.requests[1].Workspace, ".evolve", "evals", "widget-thing.md")
	if !strings.Contains(directive, wantPath) {
		t.Fatalf("correction directive must name the EXACT workspace eval path %q; got %q", wantPath, directive)
	}
	// The gate's own "no unrelated edits" clause must not survive into a
	// remediation-carrying directive — a remediation exists precisely because
	// the fix is to CREATE a file, and the default clause forbids that.
	if strings.Contains(directive, "Do not change unrelated files.") {
		t.Errorf("a remediation-carrying directive must not also forbid the required file creation: %q", directive)
	}
	// The eval the agent wrote (in response to the directive) must be present
	// at the exact path the gate names — proving the round trip actually
	// closed, not just that a directive was composed.
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("complying correction must have created the eval at %q: %v", wantPath, err)
	}
}

func TestMaterializationCorrectionDelivery_NonComplyingReDispatchStaysRejected(t *testing.T) {
	scout := &materializationScoutRunner{slug: "widget-thing", createEval: false}
	if err := runMaterializationCycle(t, scout); err == nil {
		t.Fatal("non-complying correction must remain rejected by the real materialization gate")
	}

	if len(scout.requests) < 2 {
		t.Fatalf("want at least 2 scout dispatches even on the non-complying path (correction must still be attempted); got %d", len(scout.requests))
	}
	evalPath := filepath.Join(scout.requests[0].Workspace, ".evolve", "evals", "widget-thing.md")
	if _, err := os.Stat(evalPath); err == nil {
		t.Fatalf("non-complying scout must never have produced the eval file — the test fixture is broken if it exists")
	}
	// Every dispatch's directive must keep naming the same missing path — the
	// signal is not a one-shot warning that silently drops on repeat.
	for i, req := range scout.requests[1:] {
		if !strings.Contains(req.CorrectionDirective, "widget-thing.md") {
			t.Errorf("dispatch %d correction directive lost the missing slug's path: %q", i+1, req.CorrectionDirective)
		}
	}
}
