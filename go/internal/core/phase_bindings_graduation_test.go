package core

// phase_bindings_graduation_test.go — cycle-675 RED contract for the
// build-entry new-package graduation guard (inbox
// new-package-graduation-buildentry-gate, 3rd recurrence: cycles 575/587/652).
//
// The audit-side half (apicoverNewPackageGraduationDefault, audit.go:248) landed
// 2026-07-07; this encodes the missing build-entry half: a deterministic
// post-build check that FAILS the build phase — explicit abort_reason, unlike
// buildSelfCheck's WARN-only contract — when a changed go/internal/<pkg> is new
// this cycle and absent from go/.apicover-enforce.
//
// Contract under test (Builder implements; tests must not be modified):
//
//	buildGraduationCheck(ctx context.Context, worktree string) string
//
// returns "" when nothing is ungraduated (or the check cannot apply: empty
// worktree, no enforce file — fail-open mirroring the audit default), else a
// non-empty abort reason naming each ungraduated package and the
// .apicover-enforce graduation obligation. Detection reuses
// ciparity.NewUngraduatedPackages over the worktree's changed set; a package
// whose directory no longer exists in the worktree (delete/rename) is NOT new
// and must never be flagged (AC3).

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gradWrite writes a file under the worktree, creating parent dirs.
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

// gradGit runs a git command in the worktree, failing the test on error.
func gradGit(t *testing.T, wt string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = wt
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// gradCommitAll stages and commits everything currently in the worktree, so a
// case can distinguish "committed baseline" from "this cycle's pending diff".
func gradCommitAll(t *testing.T, wt string) {
	t.Helper()
	gradGit(t, wt, "add", "-A")
	gradGit(t, wt, "commit", "-q", "-m", "seed")
}

// TestBuildGraduationCheck is the AC1/AC3 table: the guard fires exactly on a
// NEW ungraduated go/internal/<pkg> and stays silent on enrolled, out-of-scope,
// deleted, and renamed-but-reenrolled changes.
func TestBuildGraduationCheck(t *testing.T) {
	ctx := context.Background()

	if got := buildGraduationCheck(ctx, ""); got != "" {
		t.Fatalf("buildGraduationCheck(empty worktree) = %q, want \"\" (no-op)", got)
	}

	cases := []struct {
		name  string
		setup func(t *testing.T, wt string)
		// wantContains non-empty → the abort reason must contain every entry;
		// empty → the guard must return "" (no false positive).
		wantContains []string
	}{
		{
			// AC1 positive: reproduces cycle-652 — a brand-new internal package
			// with no .apicover-enforce entry must fail the build phase, and the
			// reason must name the package AND the graduation obligation.
			name: "new-ungraduated-package-fails",
			setup: func(t *testing.T, wt string) {
				gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n")
				gradCommitAll(t, wt)
				gradWrite(t, wt, "go/internal/brandnew/x.go", "package brandnew\n")
			},
			wantContains: []string{"./internal/brandnew", ".apicover-enforce"},
		},
		{
			// AC1 negative (the anti-no-op arm): the same new package enrolled in
			// the SAME diff (self-graduation) must pass — a guard that flags every
			// new package regardless of enrollment is wrong.
			name: "enrolled-same-diff-passes",
			setup: func(t *testing.T, wt string) {
				gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n")
				gradCommitAll(t, wt)
				gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n./internal/brandnew\n")
				gradWrite(t, wt, "go/internal/brandnew/x.go", "package brandnew\n")
			},
		},
		{
			// Scope: go/cmd/... is outside apicover's scope and never flagged.
			name: "cmd-package-out-of-scope",
			setup: func(t *testing.T, wt string) {
				gradWrite(t, wt, "go/.apicover-enforce", "")
				gradCommitAll(t, wt)
				gradWrite(t, wt, "go/cmd/evolve/newcmd.go", "package main\n")
			},
		},
		{
			// Fail-open parity with apicoverNewPackageGraduationDefault: no
			// enforce list → nothing to graduate against.
			name: "missing-enforce-file-fail-open",
			setup: func(t *testing.T, wt string) {
				gradWrite(t, wt, "go/internal/brandnew/x.go", "package brandnew\n")
			},
		},
		{
			// AC3: a package DELETED this cycle (its enforce entry removed in the
			// same diff) appears in the changed set but is not NEW — flagging it
			// would make graduation hygiene un-shippable.
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
			// AC3: a RENAME whose destination is enrolled in the same diff — the
			// old side is deleted (not new), the new side is enrolled.
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
			// Non-Go changes never trip a Go graduation gate.
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

// gradStubSelfCheckRunner makes the (separate, WARN-only) unit-test self-check
// pass without spawning `go test`, so the wiring tests isolate the graduation
// guard's contribution to recordAndBranch's outcome.
func gradStubSelfCheckRunner(t *testing.T) {
	t.Helper()
	old := buildSelfCheckRunner
	t.Cleanup(func() { buildSelfCheckRunner = old })
	buildSelfCheckRunner = func(context.Context, string, string) (string, bool) { return "", true }
}

// gradCycleRun builds a minimal cycleRun positioned to complete the build phase
// over wt, with fake ledger/storage (mirrors debuggerGateHarness).
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

// TestRecordAndBranch_BuildGraduationGuardAborts is the wiring half of AC1: at
// the post-build seam (recordAndBranch(PhaseBuild)), an ungraduated new package
// must FAIL the phase — a returned error naming the package and obligation, NOT
// loopNext — and the abort_reason must land on the recorded phase outcome
// (phasetiming AbortReason), unlike the WARN-only unit-test self-check.
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

// TestRecordAndBranch_BuildGraduationGuardEnrolledProceeds is the wiring
// negative: the same new package enrolled in the same diff must leave the
// post-build path untouched — loopNext, nil error, no graduation abort_reason.
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
