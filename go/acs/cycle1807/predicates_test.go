//go:build acs

package cycle1807

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

func TestC1807_001_BackupsVerifyExitsZeroWhenEveryHeadAndPatchIsElsewhere(t *testing.T) {
	f := newBackupFixture(t)
	bundleOf(t, f.repo, f.dir, "main.bundle", "main")
	other := f.orphanCommit(t, "b.txt")
	f.repo.Git("update-ref", "refs/remotes/origin/feat", other)
	f.repo.Git("branch", "-D", "scratch-b.txt")
	bundleOf(t, f.repo, f.dir, "feat.bundle", "refs/remotes/origin/feat")
	f.landOnOriginMain(t, "z.txt", "zed\n", "land z")
	if err := os.WriteFile(filepath.Join(f.dir, "z.patch"), []byte(newestPatchText("z.txt", "zed")), 0o644); err != nil {
		t.Fatal(err)
	}
	before := dirSnapshot(t, f.dir)
	r := f.verify(t)
	if r.code != exitOK {
		t.Fatalf("want exit 0, got %s", r)
	}
	for _, tok := range []string{"BUNDLE", "PATCH", "IN_MAIN", "OTHER_BRANCH", "APPLIED", "main.bundle", "feat.bundle", "z.patch"} {
		if !strings.Contains(r.stdout, tok) {
			t.Errorf("stdout missing %q:\n%s", tok, r.stdout)
		}
	}
	if after := dirSnapshot(t, f.dir); after != before {
		t.Errorf("backups verify mutated the backup dir")
	}
}

func TestC1807_002_BackupsVerifyRefusesHeadOnlyInBackupAndNamesTheSHA(t *testing.T) {
	f := newBackupFixture(t)
	bundleOf(t, f.repo, f.dir, "main.bundle", "main")
	orphan := f.orphanCommit(t, "c.txt")
	f.repo.Git("branch", "side", orphan)
	bundleOf(t, f.repo, f.dir, "side.bundle", "side")
	f.repo.Git("branch", "-D", "side", "scratch-c.txt")
	r := f.verify(t)
	if r.code != exitRefused {
		t.Fatalf("want exit 1, got %s", r)
	}
	if !strings.Contains(r.stdout, "ONLY_IN_BACKUP") {
		t.Errorf("stdout lacks ONLY_IN_BACKUP row:\n%s", r.stdout)
	}
	if !strings.Contains(r.stderr, orphan) {
		t.Errorf("stderr does not name offending head %s:\n%s", orphan, r.stderr)
	}
}

func TestC1807_003_BackupsVerifyRefusesUnappliedPatchAndNamesTheFile(t *testing.T) {
	f := newBackupFixture(t)
	if err := os.WriteFile(filepath.Join(f.dir, "pending.patch"), []byte(newestPatchText("never-landed.txt", "x")), 0o644); err != nil {
		t.Fatal(err)
	}
	r := f.verify(t)
	if r.code != exitRefused {
		t.Fatalf("want exit 1, got %s", r)
	}
	if !strings.Contains(r.stdout, "UNAPPLIED") || !strings.Contains(r.stderr, "pending.patch") {
		t.Errorf("want UNAPPLIED row and pending.patch on stderr, got %s", r)
	}
}

func TestC1807_004_BackupsVerifyOneUnsafeItemAmongSafeOnesStillRefuses(t *testing.T) {
	f := newBackupFixture(t)
	bundleOf(t, f.repo, f.dir, "main.bundle", "main")
	if err := os.WriteFile(filepath.Join(f.dir, "a-pending.patch"), []byte(newestPatchText("nope.txt", "n")), 0o644); err != nil {
		t.Fatal(err)
	}
	r := f.verify(t)
	if r.code != exitRefused {
		t.Fatalf("want exit 1 despite a safe bundle, got %s", r)
	}
	if !strings.Contains(r.stdout, "IN_MAIN") || !strings.Contains(r.stdout, "UNAPPLIED") {
		t.Errorf("every item must be reported, got:\n%s", r.stdout)
	}
}

func TestC1807_005_BackupsVerifyEmptyDirIsSafe(t *testing.T) {
	f := newBackupFixture(t)
	if r := f.verify(t); r.code != exitOK {
		t.Fatalf("empty dir want exit 0, got %s", r)
	}
}

func TestC1807_006_BackupsVerifyIOErrorsExitTwoNotOne(t *testing.T) {
	f := newBackupFixture(t)
	missing := filepath.Join(f.dir, "does-not-exist")
	r := runEvolve(t, f.repo.Dir, isolatedEnv("EVOLVE_PROJECT_ROOT="+f.repo.Dir), "backups", "verify", "--dir", missing)
	if r.code != exitIO || strings.Contains(r.stderr, "unknown command") {
		t.Errorf("unreadable dir want verb-handled exit 2, got %s", r)
	}
	r = runEvolve(t, f.repo.Dir, isolatedEnv("EVOLVE_PROJECT_ROOT="+f.repo.Dir), "backups", "verify", "--no-such-flag")
	if r.code != exitIO || strings.Contains(r.stderr, "unknown command") {
		t.Errorf("bad flag want verb-handled exit 2, got %s", r)
	}
	if err := os.WriteFile(filepath.Join(f.dir, "corrupt.bundle"), []byte("not a bundle"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := f.verify(t); r.code != exitIO || strings.Contains(r.stderr, "unknown command") {
		t.Errorf("corrupt bundle (git failure) want verb-handled exit 2, got %s", r)
	}
}

func TestC1807_007_GitexecClassifiesBundleHeadsAndPatches(t *testing.T) {
	f := newBackupFixture(t)
	bundleOf(t, f.repo, f.dir, "main.bundle", "main")
	g := gitexec.Default(f.repo.Dir)
	ctx := context.Background()
	heads, err := g.BundleHeads(ctx, filepath.Join(f.dir, "main.bundle"))
	if err != nil || len(heads) == 0 || heads[0].SHA != f.base {
		t.Fatalf("BundleHeads = %v, %v; want head %s", heads, err, f.base)
	}
	if place, err := g.ClassifyHead(ctx, f.base); err != nil || place != gitexec.HeadInMain {
		t.Errorf("ClassifyHead(main tip) = %v, %v; want HeadInMain", place, err)
	}
	orphan := f.orphanCommit(t, "d.txt")
	if place, err := g.ClassifyHead(ctx, orphan); err != nil || place != gitexec.HeadOnlyInBundle {
		t.Errorf("ClassifyHead(orphan) = %v, %v; want HeadOnlyInBundle", place, err)
	}
	f.repo.Git("update-ref", "refs/remotes/origin/other", orphan)
	if place, err := g.ClassifyHead(ctx, orphan); err != nil || place != gitexec.HeadOnOtherRemoteBranch {
		t.Errorf("ClassifyHead(other-branch) = %v, %v; want HeadOnOtherRemoteBranch", place, err)
	}
	patch := filepath.Join(f.dir, "p.patch")
	if err := os.WriteFile(patch, []byte(newestPatchText("zz.txt", "zz")), 0o644); err != nil {
		t.Fatal(err)
	}
	if applied, err := g.PatchApplied(ctx, patch); err != nil || applied {
		t.Errorf("PatchApplied before landing = %v, %v; want false", applied, err)
	}
	f.landOnOriginMain(t, "zz.txt", "zz\n", "land zz")
	if applied, err := g.PatchApplied(ctx, patch); err != nil || !applied {
		t.Errorf("PatchApplied after landing = %v, %v; want true", applied, err)
	}
}

func TestC1807_008_ReleasePromoteClearsPrereleaseOnlyAfterGreenRunAndAllAssets(t *testing.T) {
	r, calls := promote(t, promoteOpts{conclusion: "success", assets: expectedAssets(t), args: []string{releaseTag}})
	if r.code != exitOK {
		t.Fatalf("want exit 0, got %s\ncalls: %v", r, calls)
	}
	patches := patchCalls(calls)
	if len(patches) != 1 {
		t.Fatalf("want exactly one PATCH call, got %v", calls)
	}
	for _, tok := range []string{"releases/" + releaseID, "prerelease=false", "mickeyyaya/evolve-loop"} {
		if !strings.Contains(patches[0], tok) {
			t.Errorf("PATCH call %q lacks %q", patches[0], tok)
		}
	}
	if last := calls[len(calls)-1]; !strings.Contains(last, "PATCH") {
		t.Errorf("PATCH must follow every check, but calls ended with %q", last)
	}
}

func TestC1807_009_ReleasePromoteRefusesRedWorkflowWithoutPatching(t *testing.T) {
	r, calls := promote(t, promoteOpts{conclusion: "failure", assets: expectedAssets(t), args: []string{releaseTag}})
	if r.code != exitRefused {
		t.Fatalf("want exit 1, got %s", r)
	}
	if n := len(patchCalls(calls)); n != 0 {
		t.Errorf("red workflow issued %d PATCH call(s): %v", n, calls)
	}
	if !strings.Contains(r.combined(), runID) {
		t.Errorf("refusal does not name run %s:\n%s", runID, r.combined())
	}
}

func TestC1807_010_ReleasePromoteRefusesMissingAssetsAndListsThem(t *testing.T) {
	all := expectedAssets(t)
	missing := all[0]
	r, calls := promote(t, promoteOpts{conclusion: "success", assets: all[1:], args: []string{releaseTag}})
	if r.code != exitRefused {
		t.Fatalf("want exit 1, got %s", r)
	}
	if n := len(patchCalls(calls)); n != 0 {
		t.Errorf("missing asset issued %d PATCH call(s): %v", n, calls)
	}
	if !strings.Contains(r.combined(), missing) {
		t.Errorf("refusal does not list missing asset %s:\n%s", missing, r.combined())
	}
}

func TestC1807_011_ReleasePromoteMapsGhIOFailureToExitTwoNotOne(t *testing.T) {
	r, calls := promote(t, promoteOpts{conclusion: "success", assets: expectedAssets(t), fail: true, args: []string{releaseTag}})
	if r.code != exitIO || strings.Contains(r.stderr, "unknown command") || len(calls) == 0 {
		t.Fatalf("gh failure want verb-handled exit 2 after reaching gh, got %s calls=%v", r, calls)
	}
	if n := len(patchCalls(calls)); n != 0 {
		t.Errorf("gh failure issued %d PATCH call(s)", n)
	}
}

func TestC1807_012_ReleasePromoteUsageErrorsExitTwoAndCallNothing(t *testing.T) {
	for _, args := range [][]string{nil, {releaseTag, "--no-such-flag"}} {
		r, calls := promote(t, promoteOpts{conclusion: "success", assets: expectedAssets(t), args: args})
		if r.code != exitIO || strings.Contains(r.stderr, "unknown command") || !strings.Contains(r.stderr, "release-promote") {
			t.Errorf("args %v want verb-handled exit 2 naming release-promote, got %s", args, r)
		}
		if len(calls) != 0 {
			t.Errorf("args %v reached gh: %v", args, calls)
		}
	}
}

func TestC1807_013_ReleasePromoteRerunHappensBeforeTheGreenCheckAndThePatch(t *testing.T) {
	r, calls := promote(t, promoteOpts{conclusion: "success", assets: expectedAssets(t), args: []string{releaseTag, "--rerun"}})
	if r.code != exitOK {
		t.Fatalf("want exit 0, got %s\ncalls: %v", r, calls)
	}
	rerun, check, patch := -1, -1, -1
	for i, c := range calls {
		switch {
		case strings.HasPrefix(c, "run rerun") && rerun < 0:
			rerun = i
		case strings.HasPrefix(c, "run view") || strings.HasPrefix(c, "run list"):
			check = i
		case strings.Contains(c, "PATCH"):
			patch = i
		}
	}
	if rerun < 0 || !strings.Contains(calls[rerun], "--failed") {
		t.Fatalf("no `gh run rerun --failed` call: %v", calls)
	}
	if !(rerun < patch) || check < rerun {
		t.Errorf("order wrong: rerun=%d lastCheck=%d patch=%d calls=%v", rerun, check, patch, calls)
	}
}

func TestC1807_014_ReleasePromoteRerunStillRedRefusesWithoutPatching(t *testing.T) {
	r, calls := promote(t, promoteOpts{conclusion: "failure", assets: expectedAssets(t), args: []string{releaseTag, "--rerun"}})
	if r.code != exitRefused {
		t.Fatalf("want exit 1, got %s", r)
	}
	if n := len(patchCalls(calls)); n != 0 {
		t.Errorf("still-red rerun issued PATCH: %v", calls)
	}
	reran := false
	for _, c := range calls {
		reran = reran || strings.HasPrefix(c, "run rerun")
	}
	if !reran {
		t.Errorf("--rerun never invoked gh run rerun: %v", calls)
	}
}

func TestC1807_015_ReleasePromotePassesTheRepoOnlyToRunCallsBecauseGhApiHasNoRepoFlag(t *testing.T) {
	repo := releaseRepo(t)
	r, calls := promote(t, promoteOpts{conclusion: "success", assets: expectedAssets(t), args: []string{releaseTag, "--rerun"}})
	if r.code != exitOK {
		t.Fatalf("want exit 0 against gh's real flag grammar, got %s\ncalls: %v", r, calls)
	}
	readRelease, patch := -1, -1
	for i, c := range calls {
		fields := strings.Fields(c)
		if len(fields) == 0 || fields[0] != "api" {
			continue
		}
		for _, f := range fields {
			if f == "-R" || f == "--repo" || strings.HasPrefix(f, "--repo=") {
				t.Errorf("gh api call carries %q, which real gh rejects with exit 1: %q", f, c)
			}
		}
		if !strings.Contains(c, "repos/"+repo+"/") {
			t.Errorf("gh api call does not name %s in its endpoint: %q", repo, c)
		}
		switch {
		case strings.Contains(c, "releases/tags/"+releaseTag):
			readRelease = i
		case strings.Contains(c, "PATCH"):
			patch = i
		}
	}
	if readRelease < 0 || patch < 0 || readRelease > patch {
		t.Errorf("want the release read (assets checked) before the PATCH: read=%d patch=%d calls=%v", readRelease, patch, calls)
	}
}

func TestC1807_016_BackupsVerifyPatchPresentOnlyInTheCheckoutIsUnapplied(t *testing.T) {
	for _, staged := range []bool{false, true} {
		f := newBackupFixture(t)
		writeFile(t, filepath.Join(f.dir, "wip.patch"), newestPatchText("wip.txt", "wip"))
		writeFile(t, filepath.Join(f.repo.Dir, "wip.txt"), "wip\n")
		if staged {
			f.repo.Git("add", "wip.txt")
		}
		before := repoState(f.repo)
		r := f.verify(t)
		if r.code != exitRefused {
			t.Errorf("staged=%v: a change origin/main lacks must refuse with exit 1, got %s", staged, r)
			continue
		}
		if got := rowStatus(t, r.stdout, "wip.patch"); got != "UNAPPLIED" {
			t.Errorf("staged=%v: wip.patch status = %s, want UNAPPLIED", staged, got)
		}
		if !strings.Contains(r.stderr, "wip.patch") {
			t.Errorf("staged=%v: stderr does not name wip.patch:\n%s", staged, r.stderr)
		}
		if after := repoState(f.repo); after != before {
			t.Errorf("staged=%v: backups verify changed the repo:\nbefore:\n%s\nafter:\n%s", staged, before, after)
		}
	}
}

func TestC1807_017_BackupsVerifyPatchCommittedOnlyOnTheLocalBranchIsUnapplied(t *testing.T) {
	f := newBackupFixture(t)
	commitFile(t, f.repo, "local.txt", "local\n", "commit not yet pushed")
	writeFile(t, filepath.Join(f.dir, "local.patch"), newestPatchText("local.txt", "local"))
	r := f.verify(t)
	if r.code != exitRefused {
		t.Fatalf("a commit origin/main lacks must refuse with exit 1, got %s", r)
	}
	if got := rowStatus(t, r.stdout, "local.patch"); got != "UNAPPLIED" {
		t.Errorf("local.patch status = %s, want UNAPPLIED", got)
	}
}

func TestC1807_018_BackupsVerifyPatchOnOriginMainIsAppliedWhileTheCheckoutIsBehind(t *testing.T) {
	f := newBackupFixture(t)
	f.repo.Git("checkout", "-q", "-b", "upstream", f.base)
	commitFile(t, f.repo, "a.txt", "two\n", "upstream edits a")
	upstream := commitFile(t, f.repo, "b.txt", "bee\n", "upstream adds b")
	f.repo.Git("checkout", "-q", "main")
	f.repo.Git("update-ref", "refs/remotes/origin/main", upstream)
	f.repo.Git("branch", "-D", "upstream")
	writeFile(t, filepath.Join(f.dir, "edit.patch"), editPatchText("a.txt", "one", "two"))
	writeFile(t, filepath.Join(f.dir, "new.patch"), newestPatchText("b.txt", "bee"))
	before := repoState(f.repo)
	r := f.verify(t)
	if r.code != exitOK {
		t.Fatalf("patches origin/main contains must be safe (exit 0) even when the checkout is behind, got %s", r)
	}
	for _, name := range []string{"edit.patch", "new.patch"} {
		if got := rowStatus(t, r.stdout, name); got != "APPLIED" {
			t.Errorf("%s status = %s, want APPLIED", name, got)
		}
	}
	if after := repoState(f.repo); after != before {
		t.Errorf("backups verify changed the repo:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if data, err := os.ReadFile(filepath.Join(f.repo.Dir, "a.txt")); err != nil || string(data) != "one\n" {
		t.Errorf("checkout a.txt = %q, %v; want it untouched at %q", data, err, "one\n")
	}
}

func TestC1807_019_BackupsVerifyPatchWithoutOriginMainIsAnIOErrorNotASafeVerdict(t *testing.T) {
	f := newBackupFixture(t)
	f.repo.Git("update-ref", "-d", "refs/remotes/origin/main")
	writeFile(t, filepath.Join(f.dir, "a.patch"), editPatchText("a.txt", "zero", "one"))
	r := f.verify(t)
	if r.code != exitIO || strings.Contains(r.stderr, "unknown command") {
		t.Fatalf("no origin/main to check against want verb-handled exit 2, got %s", r)
	}
	if strings.Contains(r.stdout, "backups verify: every head and patch exists elsewhere") {
		t.Errorf("reported safe to delete without origin/main:\n%s", r.stdout)
	}
}

func TestC1807_020_GitexecPatchAppliedAnswersForOriginMainNotTheCheckout(t *testing.T) {
	f := newBackupFixture(t)
	g := gitexec.Default(f.repo.Dir)
	ctx := context.Background()
	staged := filepath.Join(f.dir, "staged.patch")
	writeFile(t, staged, newestPatchText("staged.txt", "s"))
	writeFile(t, filepath.Join(f.repo.Dir, "staged.txt"), "s\n")
	f.repo.Git("add", "staged.txt")
	if applied, err := g.PatchApplied(ctx, staged); err != nil || applied {
		t.Errorf("PatchApplied(staged, absent from origin/main) = %v, %v; want false", applied, err)
	}
	f.repo.Git("rm", "-q", "--cached", "staged.txt")
	if err := os.Remove(filepath.Join(f.repo.Dir, "staged.txt")); err != nil {
		t.Fatal(err)
	}
	f.repo.Git("checkout", "-q", "-b", "upstream", f.base)
	upstream := commitFile(t, f.repo, "up.txt", "up\n", "upstream adds up")
	f.repo.Git("checkout", "-q", "main")
	f.repo.Git("update-ref", "refs/remotes/origin/main", upstream)
	landed := filepath.Join(f.dir, "landed.patch")
	writeFile(t, landed, newestPatchText("up.txt", "up"))
	if applied, err := g.PatchApplied(ctx, landed); err != nil || !applied {
		t.Errorf("PatchApplied(on origin/main, absent from checkout) = %v, %v; want true", applied, err)
	}
	f.repo.Git("update-ref", "-d", "refs/remotes/origin/main")
	if applied, err := g.PatchApplied(ctx, landed); err == nil {
		t.Errorf("PatchApplied without origin/main = %v, nil; want an error, not a verdict", applied)
	}
}
