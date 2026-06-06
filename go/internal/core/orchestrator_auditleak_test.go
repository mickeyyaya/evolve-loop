// orchestrator_auditleak_test.go — cycle-235 task `audit-phase-leak-recover` (RED).
//
// Inbox defect (2026-06-06T05-48-00Z-audit-leak-recover): `evolve acs suite`
// during AUDIT rebuilds go/evolve in the MAIN tree; the post-phase tree-diff
// guard (orchestrator.go ~1632) sees the binary as a newly-dirty main-tree
// path and aborts the whole cycle. recoverBuildLeak is gated on PhaseBuild,
// so the audit phase has no recovery path — a rebuilt binary kills the cycle.
//
// Contract encoded here (scout-report cycle-235, Task 2):
//   - when the ONLY newly-dirty main-tree paths after a guarded phase are
//     tracked build artifacts (buildArtifacts: go/evolve, go/bin/evolve),
//     the orchestrator discards the churn in the MAIN tree, re-checks, WARNs,
//     and CONTINUES the cycle (the binary is restored to committed content);
//   - any non-artifact leak still aborts the cycle via the tree-diff guard;
//   - the recovery path never reverts non-artifact files (operator work).
//
// RED note: this test compiles against existing API and fails at RUNTIME
// today — subtest binary_churn_recovered aborts with the tree-diff error.
// Builder makes it GREEN by adding the phase-agnostic binary discard before
// the `res.Error(...)` return in the tree-diff guard. The other subtests are
// pre-existing-GREEN regression pins (clean cycle + non-artifact abort) that
// must SURVIVE the fix.
package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	auditLeakBinV1  = "EVOLVE-BINARY-v1\n"    // committed go/evolve content
	auditLeakNoteV1 = "operator note v1\n"    // committed docs/note.md content
	auditLeakChurn  = "rebuilt-by-audit-v2\n" // what the audit phase writes
)

// auditLeakRunner PASSes its phase after running a side effect — models the
// audit phase rebuilding go/evolve (or writing a real leak) in the MAIN tree.
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

// initAuditLeakRepo creates a real git repo whose committed tree contains a
// tracked fake release binary (go/evolve) and a tracked operator file
// (docs/note.md). The orchestrator's default gitDirtyPaths runs real git
// against it, so the tree-diff guard exercises its production code path.
func initAuditLeakRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	write("go/evolve", auditLeakBinV1)
	write("docs/note.md", auditLeakNoteV1)
	git("add", ".")
	git("commit", "-q", "-m", "init")
	return root
}

func auditLeakReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// TestOrchestrator_AuditLeakRecover — scout AC (audit-phase-leak-recover):
// binary rebuild churn during a guarded non-build phase is discarded and the
// cycle continues; real leaks still abort; operator files are never reverted.
func TestOrchestrator_AuditLeakRecover(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	cases := []struct {
		name      string
		leakPaths []string // tracked main-tree files the audit runner overwrites
		wantErr   bool
	}{
		// Regression pin: an audit that touches nothing keeps working.
		{name: "clean_audit_no_leak"},
		// THE defect: rebuilt binary alone must NOT kill the cycle. (RED today)
		{name: "binary_churn_recovered", leakPaths: []string{"go/evolve"}},
		// Negative: a real source leak must still abort — the recovery path
		// must not become a hole in the trust kernel.
		{name: "non_binary_leak_aborts", leakPaths: []string{"docs/note.md"}, wantErr: true},
		// Negative: binary churn must not launder a real leak alongside it.
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
			// Non-empty worktree path activates the tree-diff guard for the
			// guarded phases (tdd/build/audit); gitDirtyPaths stays the
			// production default so real git answers the snapshots.
			o := NewOrchestrator(st, led, runners, WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}))

			res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})

			if tc.wantErr {
				if err == nil {
					t.Fatalf("non-artifact main-tree leak must abort the cycle; got nil error (phases=%v)", res.PhasesRun)
				}
				if !strings.Contains(err.Error(), "tree-diff") {
					t.Errorf("abort must come from the tree-diff guard; got: %v", err)
				}
				// The recovery path may only ever discard build artifacts —
				// the operator's (leaked) file content must be left in place
				// for forensics, never silently checked out.
				if got := auditLeakReadFile(t, filepath.Join(root, "docs", "note.md")); got != auditLeakChurn {
					t.Errorf("docs/note.md = %q — recovery must NOT revert non-artifact files", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("cycle must continue (binary rebuild churn is discardable, not a leak): %v", err)
			}
			// The churned binary must be restored to its committed content —
			// continuing WITHOUT discarding would commit binary drift (cycle-153).
			if got := auditLeakReadFile(t, filepath.Join(root, "go", "evolve")); got != auditLeakBinV1 {
				t.Errorf("go/evolve = %q, want committed content %q (churn discarded)", got, auditLeakBinV1)
			}
			// And the cycle must actually have proceeded past audit.
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
