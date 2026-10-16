//go:build integration

package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const (
	auditLeakBinV1  = "EVOLVE-BINARY-v1\n"
	auditLeakNoteV1 = "operator note v1\n"
	auditLeakChurn  = "rebuilt-by-audit-v2\n"
)

type auditLeakRunner struct {
	name  string
	onRun func()
}

func (r *auditLeakRunner) Name() string { return r.name }
func (r *auditLeakRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	if r.onRun != nil {
		r.onRun()
	}
	return PhaseResponse{Phase: r.name, Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func buildAuditLeakRepo(r *gittest.Repo) error {
	r.Git("config", "user.name", "t")
	r.Git("config", "user.email", "t@t")
	r.Write("go/evolve", auditLeakBinV1)
	r.Write("docs/note.md", auditLeakNoteV1)
	r.Git("add", ".")
	r.Git("commit", "-q", "-m", "init")
	return nil
}

func initAuditLeakRepo(t *testing.T) string {
	t.Helper()
	return gittest.FromTemplate(t, "core-audit-leak-repo", buildAuditLeakRepo).Dir
}

func auditLeakReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestOrchestrator_AuditLeakRecover(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	cases := []struct {
		name      string
		leakPaths []string
		wantErr   bool
	}{
		{name: "clean_audit_no_leak"},
		{name: "binary_churn_recovered", leakPaths: []string{"go/evolve"}},
		{name: "non_binary_leak_aborts", leakPaths: []string{"docs/note.md"}, wantErr: true},
		{name: "mixed_leak_still_aborts", leakPaths: []string{"go/evolve", "docs/note.md"}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := initAuditLeakRepo(t)
			runners := buildRunners(nil)
			runners[PhaseAudit] = &auditLeakRunner{name: string(PhaseAudit), onRun: func() {
				for _, p := range tc.leakPaths {
					if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(p)), []byte(auditLeakChurn), 0o644); err != nil {
						t.Errorf("churn write %s: %v", p, err)
					}
				}
			}}
			st := &fakeStorage{}
			led := &fakeLedger{}
			o := NewOrchestrator(st, led, runners, WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}))

			res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})

			if tc.wantErr {
				if err == nil {
					t.Fatalf("non-artifact main-tree leak must abort the cycle; got nil error (phases=%v)", res.PhasesRun)
				}
				if !strings.Contains(err.Error(), "tree-diff") {
					t.Errorf("abort must come from the tree-diff guard; got: %v", err)
				}
				if got := auditLeakReadFile(t, filepath.Join(root, "docs", "note.md")); got != auditLeakChurn {
					t.Errorf("docs/note.md = %q — recovery must NOT revert non-artifact files", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("cycle must continue (binary rebuild churn is discardable, not a leak): %v", err)
			}
			if got := auditLeakReadFile(t, filepath.Join(root, "go", "evolve")); got != auditLeakBinV1 {
				t.Errorf("go/evolve = %q, want committed content %q (churn discarded)", got, auditLeakBinV1)
			}
			shipRan := false
			for _, p := range res.PhasesRun {
				if p == PhaseShip {
					shipRan = true
				}
			}
			if !shipRan {
				t.Errorf("ship never ran — cycle did not continue past the audit guard (phases=%v)", res.PhasesRun)
			}
		})
	}
}
