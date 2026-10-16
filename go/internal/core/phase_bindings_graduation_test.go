package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gradWrite(t *testing.T, wt, rel, content string) {
	t.Helper()
	fp := filepath.Join(wt, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gradGit(t *testing.T, wt string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = wt
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gradCommitAll(t *testing.T, wt string) {
	t.Helper()
	gradGit(t, wt, "add", "-A")
	gradGit(t, wt, "commit", "-q", "-m", "seed")
}

func TestBuildGraduationCheck(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	if got := buildGraduationCheck(ctx, ""); got != "" {
		t.Fatalf("buildGraduationCheck(empty worktree) = %q, want \"\" (no-op)", got)
	}

	cases := []struct {
		name         string
		setup        func(t *testing.T, wt string)
		wantContains []string
	}{
		{
			name: "new-ungraduated-package-fails",
			setup: func(t *testing.T, wt string) {
				gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n")
				gradCommitAll(t, wt)
				gradWrite(t, wt, "go/internal/brandnew/x.go", "package brandnew\n")
			},
			wantContains: []string{"./internal/brandnew", ".apicover-enforce"},
		},
		{
			name: "test-only-package-does-not-abort",
			setup: func(t *testing.T, wt string) {
				gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n")
				gradCommitAll(t, wt)
				gradWrite(t, wt, "go/internal/testonly/testonly_test.go",
					"package testonly\n\nimport \"testing\"\n\nfunc TestRed(t *testing.T) { t.Fatal(\"red first\") }\n")
			},
			wantContains: nil,
		},
		{
			name: "test-only-plus-production-file-still-aborts",
			setup: func(t *testing.T, wt string) {
				gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n")
				gradCommitAll(t, wt)
				gradWrite(t, wt, "go/internal/mixed/mixed_test.go", "package mixed\n")
				gradWrite(t, wt, "go/internal/mixed/mixed.go", "package mixed\n\n// Exported is surface the gate must inspect.\nfunc Exported() int { return 1 }\n")
			},
			wantContains: []string{"./internal/mixed", ".apicover-enforce"},
		},
		{
			name: "enrolled-same-diff-passes",
			setup: func(t *testing.T, wt string) {
				gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n")
				gradCommitAll(t, wt)
				gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n./internal/brandnew\n")
				gradWrite(t, wt, "go/internal/brandnew/x.go", "package brandnew\n")
			},
		},
		{
			name: "cmd-package-out-of-scope",
			setup: func(t *testing.T, wt string) {
				gradWrite(t, wt, "go/.apicover-enforce", "")
				gradCommitAll(t, wt)
				gradWrite(t, wt, "go/cmd/evolve/newcmd.go", "package main\n")
			},
		},
		{
			name: "missing-enforce-file-fail-open",
			setup: func(t *testing.T, wt string) {
				gradWrite(t, wt, "go/internal/brandnew/x.go", "package brandnew\n")
			},
		},
		{
			name: "deleted-package-not-flagged",
			setup: func(t *testing.T, wt string) {
				gradWrite(t, wt, "go/internal/oldpkg/x.go", "package oldpkg\n")
				gradWrite(t, wt, "go/.apicover-enforce", "./internal/oldpkg\n")
				gradCommitAll(t, wt)
				gradGit(t, wt, "rm", "-q", "go/internal/oldpkg/x.go")
				gradWrite(t, wt, "go/.apicover-enforce", "")
			},
		},
		{
			name: "renamed-package-reenrolled-passes",
			setup: func(t *testing.T, wt string) {
				gradWrite(t, wt, "go/internal/oldpkg/x.go", "package oldpkg\n")
				gradWrite(t, wt, "go/.apicover-enforce", "./internal/oldpkg\n")
				gradCommitAll(t, wt)
				gradGit(t, wt, "rm", "-q", "go/internal/oldpkg/x.go")
				gradWrite(t, wt, "go/internal/newpkg/x.go", "package newpkg\n")
				gradWrite(t, wt, "go/.apicover-enforce", "./internal/newpkg\n")
			},
		},
		{
			name: "non-go-change-noop",
			setup: func(t *testing.T, wt string) {
				gradWrite(t, wt, "go/.apicover-enforce", "")
				gradCommitAll(t, wt)
				gradWrite(t, wt, "docs/note.md", "# note\n")
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wt := initGitWorktree(t)
			c.setup(t, wt)
			got := buildGraduationCheck(ctx, wt)
			if len(c.wantContains) == 0 {
				if got != "" {
					t.Fatalf("buildGraduationCheck = %q, want \"\" (no false positive)", got)
				}
				return
			}
			if got == "" {
				t.Fatal("buildGraduationCheck = \"\", want a graduation abort reason")
			}
			for _, sub := range c.wantContains {
				if !strings.Contains(got, sub) {
					t.Errorf("abort reason %q must contain %q", got, sub)
				}
			}
		})
	}
}

func gradStubSelfCheckRunner(t *testing.T) {
	t.Helper()
	old := buildSelfCheckRunner
	t.Cleanup(func() { buildSelfCheckRunner = old })
	buildSelfCheckRunner = func(context.Context, string, string) (string, bool) { return "", true }
}

func gradCycleRun(t *testing.T, wt string) *cycleRun {
	t.Helper()
	return &cycleRun{
		o:       NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil)),
		ctx:     context.Background(),
		req:     CycleRequest{ProjectRoot: t.TempDir()},
		cycle:   675,
		cs:      CycleState{WorkspacePath: t.TempDir(), ActiveWorktree: wt},
		envSnap: map[string]string{},
	}
}

func TestRecordAndBranch_BuildGraduationGuardAborts(t *testing.T) {
	wt := initGitWorktree(t)
	gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n")
	gradCommitAll(t, wt)
	gradWrite(t, wt, "go/internal/brandnew/x.go", "package brandnew\n")
	gradStubSelfCheckRunner(t)

	cr := gradCycleRun(t, wt)
	dr := dispatchResult{resp: PhaseResponse{Verdict: VerdictPASS}, attemptCount: 1}
	act, err := cr.recordAndBranch(PhaseBuild, dr)
	if err == nil {
		t.Fatal("recordAndBranch(build) over an ungraduated new package must fail the phase; got nil error")
	}
	for _, sub := range []string{"./internal/brandnew", ".apicover-enforce"} {
		if !strings.Contains(err.Error(), sub) {
			t.Errorf("build-phase failure %q must name %q", err.Error(), sub)
		}
	}
	if act == loopNext {
		t.Error("recordAndBranch(build) must not advance (loopNext) when the graduation guard fires")
	}
	found := false
	for _, e := range cr.phaseTimings {
		if strings.Contains(e.AbortReason, ".apicover-enforce") {
			found = true
		}
	}
	if !found {
		t.Errorf("graduation failure must record an explicit abort_reason on the phase outcome; timings = %+v", cr.phaseTimings)
	}
}

func TestRecordAndBranch_BuildGraduationGuardEnrolledProceeds(t *testing.T) {
	wt := initGitWorktree(t)
	gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n")
	gradCommitAll(t, wt)
	gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n./internal/brandnew\n")
	gradWrite(t, wt, "go/internal/brandnew/x.go", "package brandnew\n")
	gradStubSelfCheckRunner(t)

	cr := gradCycleRun(t, wt)
	dr := dispatchResult{resp: PhaseResponse{Verdict: VerdictPASS}, attemptCount: 1}
	act, err := cr.recordAndBranch(PhaseBuild, dr)
	if err != nil {
		t.Fatalf("recordAndBranch(build) over an enrolled new package must proceed; got error: %v", err)
	}
	if act != loopNext {
		t.Fatalf("recordAndBranch(build) = %v, want loopNext (enrolled package must not abort)", act)
	}
	for _, e := range cr.phaseTimings {
		if strings.Contains(e.AbortReason, ".apicover-enforce") {
			t.Errorf("enrolled package must not record a graduation abort_reason; got %+v", e)
		}
	}
}
