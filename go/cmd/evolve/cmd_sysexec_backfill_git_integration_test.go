//go:build integration

package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestMainProjectRoot_ResolvesTheMainCheckoutOfAnyWorktree(t *testing.T) {
	t.Parallel()
	repo := bfCommittedRepo(t)
	linked := filepath.Join(t.TempDir(), "linked")
	repo.Git("worktree", "add", "-q", "-b", "linked", linked)
	main := bfRealDir(t, repo.Dir)

	for name, dir := range map[string]string{"main checkout": repo.Dir, "linked worktree": linked, "subdirectory": filepath.Join(linked, ".git", "..")} {
		if got := mainProjectRoot(dir); got != main {
			t.Fatalf("%s: mainProjectRoot = %q, want %q", name, got, main)
		}
	}
}

func TestMainProjectRoot_FallsBackToTheDirWithoutAnAnswer(t *testing.T) {
	plain := bfNonRepo(t)
	if got := mainProjectRoot(plain); got != plain {
		t.Fatalf("non-repository: mainProjectRoot = %q, want %q", got, plain)
	}
	bfStub(t, "git", "printf '\\n'")
	if got := mainProjectRoot("/some/dir"); got != "/some/dir" {
		t.Fatalf("empty git answer: mainProjectRoot = %q, want the dir", got)
	}
}

func TestHasGitRemote_OnlyAConfiguredRemoteCounts(t *testing.T) {
	repo := bfCommittedRepo(t)
	if hasGitRemote(repo.Dir) {
		t.Fatal("a repo without remotes reported a remote")
	}
	repo.Git("remote", "add", "origin", "https://example.invalid/x.git")
	if !hasGitRemote(repo.Dir) {
		t.Fatal("a configured origin was not seen")
	}
	if hasGitRemote(bfNonRepo(t)) {
		t.Fatal("a non-repository reported a remote")
	}
}

func TestRemoteOpenPR_NoRemoteOrNoGhMeansNoOpenPR(t *testing.T) {
	repo := bfCommittedRepo(t)
	record := bfStub(t, "gh", "echo 5")

	open, err := remoteOpenPR(repo.Dir)("cycle-9")

	if open || err != nil || bfCalls(t, record) != "" {
		t.Fatalf("no remote: remoteOpenPR = %v, %v, gh calls %q; want false, nil, none", open, err, bfCalls(t, record))
	}
	repo.Git("remote", "add", "origin", "https://example.invalid/x.git")
	t.Setenv("PATH", bfGitOnlyPath(t))

	open, err = remoteOpenPR(repo.Dir)("cycle-9")

	if open || err != nil {
		t.Fatalf("no gh: remoteOpenPR = %v, %v; want false, nil", open, err)
	}
}

func TestRemoteOpenPR_CountsOpenPRsForTheHeadRef(t *testing.T) {
	for _, tc := range []struct {
		out  string
		want bool
	}{
		{"3", true},
		{" 1 \n", true},
		{"0", false},
		{"", false},
	} {
		t.Run("out="+tc.out, func(t *testing.T) {
			repo := bfCommittedRepo(t)
			repo.Git("remote", "add", "origin", "https://example.invalid/x.git")
			record := bfStub(t, "gh", "printf '"+tc.out+"'")

			open, err := remoteOpenPR(repo.Dir)("cycle-9")

			if err != nil || open != tc.want {
				t.Fatalf("remoteOpenPR = %v, %v; want %v", open, err, tc.want)
			}
			if calls := bfCalls(t, record); !strings.Contains(calls, "pr list --head cycle-9 --state open --json number -q length") {
				t.Fatalf("gh calls %q, want the open-PR count for cycle-9", calls)
			}
		})
	}
}

func TestRemoteOpenPR_GhFailureIsAnError(t *testing.T) {
	repo := bfCommittedRepo(t)
	repo.Git("remote", "add", "origin", "https://example.invalid/x.git")
	bfStub(t, "gh", "exit 4")

	open, err := remoteOpenPR(repo.Dir)("cycle-9")

	var exitErr *exec.ExitError
	if open || !errors.As(err, &exitErr) || !strings.HasPrefix(err.Error(), "gh pr list --head cycle-9: ") {
		t.Fatalf("remoteOpenPR = %v, %v; want false and the wrapped gh exit", open, err)
	}
}

func TestGitDiffCapture_ReturnsTheDiffOrAWrappedFailure(t *testing.T) {
	repo := bfCommittedRepo(t)
	if err := os.WriteFile(filepath.Join(repo.Dir, "README"), []byte("base\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	diff, err := gitDiffCapture(context.Background(), repo.Dir, "HEAD")
	if err != nil || !strings.Contains(string(diff), "\n+changed\n") || !strings.HasPrefix(string(diff), "diff --git a/README b/README") {
		t.Fatalf("gitDiffCapture = %q, %v; want the README diff", diff, err)
	}
	if empty, err := gitDiffCapture(context.Background(), repo.Dir, "HEAD..HEAD"); err != nil || len(empty) != 0 {
		t.Fatalf("empty range: gitDiffCapture = %q, %v; want no bytes", empty, err)
	}
	for name, tc := range map[string]struct {
		ctx  context.Context
		dir  string
		spec string
	}{
		"unknown revision": {context.Background(), repo.Dir, "no-such-ref"},
		"not a repository": {context.Background(), bfNonRepo(t), "HEAD"},
		"canceled":         {canceledCtx(), repo.Dir, "HEAD"},
	} {
		got, err := gitDiffCapture(tc.ctx, tc.dir, tc.spec)
		if got != nil || err == nil || !strings.HasPrefix(err.Error(), "composition snapshot: git diff "+tc.spec+": ") {
			t.Fatalf("%s: gitDiffCapture = %q, %v; want nil and the wrapped failure", name, got, err)
		}
	}
}

func canceledCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestResolveRepoRoot_AnExplicitRootNeverAsksGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	got, err := resolveRepoRoot("/explicit/root")

	if err != nil || got != "/explicit/root" {
		t.Fatalf("resolveRepoRoot = %q, %v; want the explicit root", got, err)
	}
}

func TestPlaneLiveRuns_TheRootListedAgainIsCountedOnce(t *testing.T) {
	t.Parallel()
	repo := bfCommittedRepo(t)
	runs := filepath.Join(repo.Dir, ".evolve", "runs", "cycle-3")
	if err := os.MkdirAll(runs, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := runlease.Write(runs, runlease.Lease{RunID: "cycle-3", OwnerPID: os.Getpid()}, now); err != nil {
		t.Fatal(err)
	}

	live, err := planeLiveRuns(repo.Dir, now)

	if err != nil || len(live) != 1 || live[0].Lease.RunID != "cycle-3" {
		t.Fatalf("planeLiveRuns = %+v, %v; want the root's cycle-3 exactly once", live, err)
	}
}

func TestRunWorktreeList_BadFlagIsUsageExit(t *testing.T) {
	t.Parallel()
	var stdout, stderr strings.Builder

	if code := runWorktreeList([]string{"--no-such-flag"}, &stdout, &stderr); code != 10 || !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf("runWorktreeList = %d, %q; want 10 and the flag error", code, stderr.String())
	}
}

func TestRemoveCycleWorktree_AnUnknownWorktreeFailsWithGitsReason(t *testing.T) {
	t.Parallel()
	repo := bfCommittedRepo(t)
	var stdout, stderr strings.Builder

	code := removeCycleWorktree(repo.Dir, filepath.Join(repo.Dir, ".evolve", "worktrees"), "lane", 4, &stdout, &stderr)

	if code != 1 || stdout.String() != "" || !strings.HasPrefix(stderr.String(), "evolve worktree cleanup: exit status 128\n") || !strings.Contains(stderr.String(), "is not a working tree") {
		t.Fatalf("removeCycleWorktree = %d, stdout %q, stderr %q; want 1 and git's reason", code, stdout.String(), stderr.String())
	}
}

func TestRemoveCycleWorktree_ALeftoverDirectoryOnlyWarns(t *testing.T) {
	for _, tc := range []struct {
		name     string
		rmErr    error
		wantWarn bool
	}{
		{"removal refused", errors.New("denied"), true},
		{"already gone", fs.ErrNotExist, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := bfCommittedRepo(t)
			base := filepath.Join(repo.Dir, ".evolve", "worktrees")
			var createOut, createErr strings.Builder
			if code := runWorktreeCreate([]string{"--cycle", "4", "--project-root", repo.Dir, "--base", base, "--lane", "lane"}, &createOut, &createErr); code != 0 {
				t.Fatalf("setup: create exit %d: %s", code, createErr.String())
			}
			wt := strings.TrimSpace(createOut.String())
			var removed string
			bfSwap(t, &worktreeRemoveAll, func(p string) error { removed = p; return tc.rmErr })
			var stdout, stderr strings.Builder

			code := removeCycleWorktree(repo.Dir, base, "lane", 4, &stdout, &stderr)

			warned := stderr.String() == "evolve worktree cleanup: rm "+wt+": denied\n"
			if code != 0 || stdout.String() != wt+"\n" || removed != wt || warned != tc.wantWarn || !tc.wantWarn && stderr.String() != "" {
				t.Fatalf("removeCycleWorktree = %d, stdout %q, stderr %q (removed %q); want 0, the path, warn=%v", code, stdout.String(), stderr.String(), removed, tc.wantWarn)
			}
		})
	}
}

func TestPruneWorktrees_NotARepositoryIsExitOne(t *testing.T) {
	var stdout, stderr strings.Builder

	code := pruneWorktrees(bfNonRepo(t), &stdout, &stderr)

	if code != 1 || !strings.Contains(stderr.String(), "evolve worktree cleanup: prune: exit status 128") {
		t.Fatalf("pruneWorktrees = %d, %q; want 1 and the prune failure", code, stderr.String())
	}
}

func TestPruneWorktrees_DropsAWorktreeWhoseDirectoryVanished(t *testing.T) {
	t.Parallel()
	repo := bfCommittedRepo(t)
	gone := filepath.Join(t.TempDir(), "gone")
	repo.Git("worktree", "add", "-q", "-b", "gone", gone)
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder

	code := pruneWorktrees(repo.Dir, &stdout, &stderr)

	if code != 0 || strings.Contains(repo.Git("worktree", "list"), gone) {
		t.Fatalf("pruneWorktrees = %d (stderr %q); the vanished worktree is still registered", code, stderr.String())
	}
}

func TestRunResetSHA_UsageAndEnvironmentFailuresExitOne(t *testing.T) {
	t.Run("bad flag", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := runResetSHA([]string{"--bogus"}, nil, &out, &errb); code != 1 || !strings.Contains(errb.String(), "flag provided but not defined") {
			t.Fatalf("runResetSHA = %d, %q; want 1 and the flag error", code, errb.String())
		}
	})
	t.Run("cwd unknown", func(t *testing.T) {
		t.Setenv("EVOLVE_PROJECT_ROOT", "")
		bfSwap(t, &resetSHAGetwd, func() (string, error) { return "", errors.New("getwd failed") })
		var out, errb bytes.Buffer
		if code := runResetSHA(nil, nil, &out, &errb); code != 1 || errb.String() != "evolve reset-sha: cwd: getwd failed\n" {
			t.Fatalf("runResetSHA = %d, %q; want 1 and the cwd error", code, errb.String())
		}
	})
	t.Run("running binary unreadable", func(t *testing.T) {
		root := seedResetProject(t)
		bfSwap(t, &resetSHARunningFn, func() (string, error) { return "", errors.New("unreadable") })
		var out, errb bytes.Buffer
		if code := runResetSHA([]string{"--operator", "--project-root", root}, nil, &out, &errb); code != 1 || errb.String() != "evolve reset-sha: hash running binary: unreadable\n" {
			t.Fatalf("runResetSHA = %d, %q; want 1 and the hash error", code, errb.String())
		}
	})
}

func TestRunResetSHA_ABuildCommitOnHeadIsProvenanceAuthorized(t *testing.T) {
	repo := bfCommittedRepo(t)
	if err := os.MkdirAll(filepath.Join(repo.Dir, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(repo.Dir, ".evolve", "state.json")
	if err := os.WriteFile(statePath, []byte(`{"expected_ship_sha":"OLDPIN"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	head := repo.Git("rev-parse", "HEAD")
	bfSwap(t, &resetSHACommitFn, func() string { return head })
	var out, errb bytes.Buffer

	code := runResetSHA([]string{"--project-root", repo.Dir}, nil, &out, &errb)

	if code != 0 || !strings.Contains(out.String(), "(authorized: provenance)") {
		t.Fatalf("runResetSHA = %d, out %q, err %q; want a provenance-authorized re-pin", code, out.String(), errb.String())
	}
	if body, _ := os.ReadFile(statePath); strings.Contains(string(body), "OLDPIN") {
		t.Fatalf("state still pinned to OLDPIN: %s", body)
	}
}

func TestRunResetSHA_AForeignBuildCommitIsRefusedWithThePinKept(t *testing.T) {
	repo := bfCommittedRepo(t)
	if err := os.MkdirAll(filepath.Join(repo.Dir, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(repo.Dir, ".evolve", "state.json")
	if err := os.WriteFile(statePath, []byte(`{"expected_ship_sha":"OLDPIN"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	bfSwap(t, &resetSHACommitFn, func() string { return strings.Repeat("d", 40) })
	var out, errb bytes.Buffer

	code := runResetSHA([]string{"--project-root", repo.Dir}, nil, &out, &errb)

	if code != 1 || !strings.Contains(errb.String(), "not provenance-verified") {
		t.Fatalf("runResetSHA = %d, err %q; want the provenance refusal", code, errb.String())
	}
	if body, _ := os.ReadFile(statePath); !strings.Contains(string(body), "OLDPIN") {
		t.Fatalf("pin changed on refusal: %s", body)
	}
}

func TestRunResetSHA_ProvenanceOfAnEmptyCommitIsNeverVerified(t *testing.T) {
	repo := bfCommittedRepo(t)
	head := repo.Git("rev-parse", "HEAD")
	var prov phaseintegrity.ProvenanceVerified
	bfSwap(t, &resetSHARepinFn, func(_, _, _, _ string, p phaseintegrity.ProvenanceVerified, _ bool) (phaseintegrity.RepinResult, error) {
		prov = p
		return phaseintegrity.RepinResult{Authorized: "test"}, nil
	})
	var out, errb bytes.Buffer

	if code := runResetSHA([]string{"--project-root", repo.Dir}, nil, &out, &errb); code != 0 {
		t.Fatalf("runResetSHA = %d, %q", code, errb.String())
	}
	if prov == nil || prov("") || !prov(head) || prov(strings.Repeat("d", 40)) {
		t.Fatalf("provenance predicate: empty=%v head=%v foreign=%v; want false, true, false", prov != nil && prov(""), prov != nil && prov(head), prov != nil && prov(strings.Repeat("d", 40)))
	}
}

func TestRunSyncMain_ABadFlagIsUsageAndAnUnknownCwdIsIO(t *testing.T) {
	t.Run("bad flag", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := runSyncMain([]string{"--bogus"}, nil, &out, &errb); code != cliExitUsage || !strings.Contains(errb.String(), "flag provided but not defined") {
			t.Fatalf("runSyncMain = %d, %q; want 10 and the flag error", code, errb.String())
		}
	})
	t.Run("cwd unknown", func(t *testing.T) {
		t.Setenv("EVOLVE_PROJECT_ROOT", "")
		bfSwap(t, &syncMainGetwd, func() (string, error) { return "", errors.New("getwd failed") })
		var out, errb bytes.Buffer
		if code := runSyncMain(nil, nil, &out, &errb); code != cliExitIO || errb.String() != "evolve sync-main: cwd: getwd failed\n" {
			t.Fatalf("runSyncMain = %d, %q; want 2 and the cwd error", code, errb.String())
		}
	})
}

func TestRunSyncMain_TakesTheRootFromTheEnvThenTheCwd(t *testing.T) {
	unborn := bfRealDir(t, gittest.Fixture(t).Dir)
	plain := bfNonRepo(t)
	bfChdir(t, plain)

	t.Setenv("EVOLVE_PROJECT_ROOT", unborn)
	var out, errb bytes.Buffer
	envCode := runSyncMain(nil, nil, &out, &errb)
	envErr := errb.String()

	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	errb.Reset()
	cwdCode := runSyncMain(nil, nil, &out, &errb)

	if envCode != 1 || !strings.HasPrefix(envErr, "evolve sync-main: cannot resolve current branch: ") {
		t.Fatalf("env root: %d %q; want the unborn branch failure", envCode, envErr)
	}
	if cwdCode != 1 || !strings.HasPrefix(errb.String(), "evolve sync-main: reading the tree's changes failed") {
		t.Fatalf("cwd root: %d %q; want the non-repository failure", cwdCode, errb.String())
	}
}

func TestRunSyncMain_AnUnresolvableRootWarnsAndKeepsTheRawRoot(t *testing.T) {
	plain := bfNonRepo(t)
	var label, raw string
	bfSwap(t, &syncMainAbsoluteRoot, func(l, p string, warn func(string)) string {
		label, raw = l, p
		warn("abs failed")
		return p
	})
	var out, errb bytes.Buffer

	code := runSyncMain([]string{"--project-root", plain}, nil, &out, &errb)

	if label != "--project-root" || raw != plain || code != 1 || !strings.HasPrefix(errb.String(), "evolve sync-main: WARN: abs failed\n") {
		t.Fatalf("runSyncMain = %d, %q (label %q raw %q); want the WARN line first", code, errb.String(), label, raw)
	}
}

func TestResetSHAProjectRoot_AnUnresolvableRootWarnsAndKeepsTheRawRoot(t *testing.T) {
	var label, raw string
	bfSwap(t, &resetSHAAbsRoot, func(l, p string, warn func(string)) string {
		label, raw = l, p
		warn("abs failed")
		return p
	})
	var errb bytes.Buffer

	got, err := resetSHAProjectRoot("relative/root", &errb)

	if err != nil || got != "relative/root" || label != "--project-root" || raw != "relative/root" {
		t.Fatalf("resetSHAProjectRoot = %q, %v (label %q raw %q); want the raw root", got, err, label, raw)
	}
	if errb.String() != "evolve reset-sha: WARN: abs failed\n" {
		t.Fatalf("stderr = %q, want the prefixed WARN line", errb.String())
	}
}

func TestResetSHAProjectRoot_FlagThenEnvThenCwd(t *testing.T) {
	bfSwap(t, &resetSHAGetwd, func() (string, error) { return "/from/cwd", nil })
	t.Setenv("EVOLVE_PROJECT_ROOT", "/from/env")
	var errb bytes.Buffer

	flagRoot, _ := resetSHAProjectRoot("/from/flag", &errb)
	envRoot, _ := resetSHAProjectRoot("", &errb)
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	cwdRoot, err := resetSHAProjectRoot("", &errb)

	if flagRoot != "/from/flag" || envRoot != "/from/env" || cwdRoot != "/from/cwd" || err != nil || errb.Len() != 0 {
		t.Fatalf("roots flag=%q env=%q cwd=%q err=%v stderr=%q; want each source in precedence", flagRoot, envRoot, cwdRoot, err, errb.String())
	}
}
