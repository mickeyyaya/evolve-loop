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

type materializationScoutRunner struct {
	slug       string
	requests   []core.PhaseRequest
	createEval bool
}

func (r *materializationScoutRunner) Name() string { return string(core.PhaseScout) }

func (r *materializationScoutRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.requests = append(r.requests, req)
	report := "## Selected Tasks\n\n### Task 1\n- **Slug:** " + r.slug + "\n\n"
	if err := os.WriteFile(filepath.Join(req.Workspace, "scout-report.md"), []byte(report), 0o644); err != nil {
		return core.PhaseResponse{}, err
	}
	if r.createEval && len(r.requests) > 1 {
		evalPath := filepath.Join(req.Workspace, ".evolve", "evals", r.slug+".md")
		if err := os.MkdirAll(filepath.Dir(evalPath), 0o755); err != nil {
			return core.PhaseResponse{}, err
		}
		body := "# Eval " + r.slug + "\n\n- [code] `go test ./internal/widget/...`\n\n```bash\ngo test ./internal/widget/...\n```\n"
		if err := os.WriteFile(evalPath, []byte(body), 0o644); err != nil {
			return core.PhaseResponse{}, err
		}
	}
	return core.PhaseResponse{Phase: string(core.PhaseScout), Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

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
	t.Parallel()

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
	if strings.Contains(directive, "Do not change unrelated files.") {
		t.Errorf("a remediation-carrying directive must not also forbid the required file creation: %q", directive)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("complying correction must have created the eval at %q: %v", wantPath, err)
	}
}

func TestMaterializationCorrectionDelivery_NonComplyingReDispatchStaysRejected(t *testing.T) {
	t.Parallel()

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
	for i, req := range scout.requests[1:] {
		if !strings.Contains(req.CorrectionDirective, "widget-thing.md") {
			t.Errorf("dispatch %d correction directive lost the missing slug's path: %q", i+1, req.CorrectionDirective)
		}
	}
}
