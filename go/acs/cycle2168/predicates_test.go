//go:build acs

package cycle2168

import (
	"bytes"
	"context"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	patchFile       = "fence-reverted.patch"
	writablePath    = "src/writable.go"
	basePackageDocs = 1
)

var baselineFuncs = []string{
	"TakeStaged", "Take", "Restore", "Differing", "partition", "verify", "diff", "removeAdded",
	"pruneEmptyParents", "writeTree", "indexCopyTree", "addArgs", "writeFromTree", "gitEnv", "git",
	"WithWorkspace", "Begin", "retake", "writableSet", "End", "savePatch", "Diagnostics",
	"RetakenAfter", "TakeErr", "Retract", "keepWrite", "keepLink", "keepFile",
}

var baselineBelowFull = []string{"diff", "removeAdded", "writeFromTree"}

func dispatchedRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("src/keep.go", "package src\n")
	repo.Write("src/mat.go", "package src // base\n")
	repo.Write(writablePath, "package src // writable base\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	return repo
}

func gitLines(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(withoutAmbientGit(os.Environ()), "GIT_OPTIONAL_LOCKS=0")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimRight(string(out), "\n")
}

func withoutAmbientGit(environ []string) []string {
	kept := make([]string, 0, len(environ))
	for _, kv := range environ {
		if name, _, _ := strings.Cut(kv, "="); !slices.Contains(gitexec.AmbientGitVars, name) {
			kept = append(kept, kv)
		}
	}
	return kept
}

func requireOneState(t *testing.T, repo *gittest.Repo) {
	t.Helper()
	if status := gitLines(t, repo.Dir, "status", "--porcelain"); status != "" {
		t.Errorf("index and working copy are apart after the fence revert; git status --porcelain:\n%s", status)
	}
	if staged := gitLines(t, repo.Dir, "diff", "--cached", "--name-only"); staged != "" {
		t.Errorf("the fence revert left staged paths behind:\n%s", staged)
	}
}

func fenceFor(t *testing.T, repo *gittest.Repo, writable ...string) (*treefence.Fence, string) {
	t.Helper()
	workspace := t.TempDir()
	fence := treefence.Begin(context.Background(), repo.Dir, true, writable...).WithWorkspace(workspace)
	if err := fence.TakeErr(); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	return fence, workspace
}

func exists(repo *gittest.Repo, rel string) bool {
	_, err := os.Lstat(filepath.Join(repo.Dir, filepath.FromSlash(rel)))
	return err == nil
}

func TestC2168_001_AStagedFileOutsideTheWritableListLeavesOneStateAndTheEnvelopeNamesIt(t *testing.T) {
	repo := dispatchedRepo(t)
	fence, _ := fenceFor(t, repo)
	stagedOutsideTheList := []string{"src/new.go", "docs/deep/new.md", "src/with space.go", "-dash.go"}
	for _, rel := range stagedOutsideTheList {
		repo.Write(rel, "package seat // staged by the seat\n")
		repo.Git("add", "--", rel)
	}

	out := fence.End(context.Background())

	if out.RestoreErr != nil || !out.Verified {
		t.Fatalf("the revert must verify: verified=%v err=%v", out.Verified, out.RestoreErr)
	}
	requireOneState(t, repo)
	for _, rel := range stagedOutsideTheList {
		if exists(repo, rel) {
			t.Errorf("%s must be gone from the working copy", rel)
		}
		if !slices.Contains(out.Restored, rel) {
			t.Errorf("Restored = %v, want %s named", out.Restored, rel)
		}
		named := false
		for _, d := range out.Diagnostics("build") {
			named = named || strings.Contains(d.Message, rel)
		}
		if !named {
			t.Errorf("no envelope diagnostic names %s: %+v", rel, out.Diagnostics("build"))
		}
	}
}

func TestC2168_002_AStagedEditThatTheSeatEditedAgainLeavesNoMMStatus(t *testing.T) {
	repo := dispatchedRepo(t)
	fence, _ := fenceFor(t, repo)
	repo.Write("src/mat.go", "package src // staged edit\n")
	repo.Git("add", "--", "src/mat.go")
	repo.Write("src/mat.go", "package src // edited again after staging\n")

	out := fence.End(context.Background())

	if out.RestoreErr != nil || !out.Verified {
		t.Fatalf("the revert must verify: verified=%v err=%v", out.Verified, out.RestoreErr)
	}
	requireOneState(t, repo)
	if got := repo.Git("show", ":src/mat.go"); got != "package src // base" {
		t.Errorf("index entry of src/mat.go = %q, want the dispatched content", got)
	}
}

func TestC2168_003_AStagedDeletionComesBackInTheIndexAndOnDisk(t *testing.T) {
	repo := dispatchedRepo(t)
	fence, _ := fenceFor(t, repo)
	repo.Git("rm", "-q", "--", "src/keep.go")

	out := fence.End(context.Background())

	if out.RestoreErr != nil || !out.Verified {
		t.Fatalf("the revert must verify: verified=%v err=%v", out.Verified, out.RestoreErr)
	}
	requireOneState(t, repo)
	if !exists(repo, "src/keep.go") {
		t.Error("the deleted file must be back on disk")
	}
}

func TestC2168_004_TheRevertedPatchStillHoldsTheStagedChange(t *testing.T) {
	repo := dispatchedRepo(t)
	fence, workspace := fenceFor(t, repo)
	repo.Write("src/seat_output.go", "package src // distinctive seat line\n")
	repo.Git("add", "--", "src/seat_output.go")
	repo.Write("src/mat.go", "package src // distinctive staged edit\n")
	repo.Git("add", "--", "src/mat.go")

	out := fence.End(context.Background())

	if out.RestoreErr != nil || !out.Verified {
		t.Fatalf("the revert must verify: verified=%v err=%v", out.Verified, out.RestoreErr)
	}
	requireOneState(t, repo)
	patch, err := os.ReadFile(filepath.Join(workspace, patchFile))
	if err != nil {
		t.Fatalf("the reverted patch must exist: %v", err)
	}
	for _, want := range []string{"src/seat_output.go", "+package src // distinctive seat line", "src/mat.go", "+package src // distinctive staged edit"} {
		if !bytes.Contains(patch, []byte(want)) {
			t.Errorf("the reverted patch lacks %q:\n%s", want, patch)
		}
	}
	named := false
	for _, d := range out.Diagnostics("build") {
		named = named || strings.Contains(d.Message, filepath.Join(workspace, patchFile))
	}
	if !named {
		t.Errorf("no diagnostic names the patch path: %+v", out.Diagnostics("build"))
	}
}

func TestC2168_005_AFileInsideTheWritableListStaysStagedAndOnDiskWhileTheFencedOneIsReverted(t *testing.T) {
	repo := dispatchedRepo(t)
	fence, _ := fenceFor(t, repo, writablePath, "docs/out.md")
	repo.Write(writablePath, "package src // seat edit inside the list\n")
	repo.Git("add", "--", writablePath)
	repo.Write("docs/out.md", "# seat output inside the list\n")
	repo.Git("add", "--", "docs/out.md")
	repo.Write("src/mat.go", "package src // staged outside the list\n")
	repo.Git("add", "--", "src/mat.go")
	repo.Write("src/stray.go", "package src // staged outside the list\n")
	repo.Git("add", "--", "src/stray.go")

	out := fence.End(context.Background())

	if out.RestoreErr != nil || !out.Verified {
		t.Fatalf("the revert must verify: verified=%v err=%v", out.Verified, out.RestoreErr)
	}
	if got := gitLines(t, repo.Dir, "status", "--porcelain"); got != "M  "+writablePath+"\nA  docs/out.md" && got != "A  docs/out.md\nM  "+writablePath {
		t.Errorf("only the writable paths stay staged and in step; git status --porcelain:\n%s", got)
	}
	if body, err := os.ReadFile(filepath.Join(repo.Dir, writablePath)); err != nil || string(body) != "package src // seat edit inside the list\n" {
		t.Errorf("the writable file must keep its content on disk: %q %v", body, err)
	}
	if !exists(repo, "docs/out.md") {
		t.Error("the writable new file must stay on disk")
	}
	if exists(repo, "src/stray.go") {
		t.Error("the fenced new file must be gone")
	}
	if !slices.Equal(out.Kept, []string{"docs/out.md", writablePath}) {
		t.Errorf("Kept = %v, want both writable paths", out.Kept)
	}
}

func TestC2168_006_AnEditThatWasUnstagedAtDispatchIsNeverStagedByTheRevert(t *testing.T) {
	repo := dispatchedRepo(t)
	repo.Write("src/mat.go", "package src // dispatch-time unstaged edit\n")
	fence, _ := fenceFor(t, repo)
	repo.Write("src/mat.go", "package src // seat edit\n")
	repo.Git("add", "--", "src/mat.go")

	out := fence.End(context.Background())

	if out.RestoreErr != nil || !out.Verified {
		t.Fatalf("the revert must verify: verified=%v err=%v", out.Verified, out.RestoreErr)
	}
	if staged := gitLines(t, repo.Dir, "diff", "--cached", "--name-only"); staged != "" {
		t.Errorf("the revert staged a path that was unstaged at dispatch:\n%s", staged)
	}
	if status := gitLines(t, repo.Dir, "status", "--porcelain"); status != " M src/mat.go" {
		t.Errorf("git status --porcelain = %q, want the dispatch-time state %q", status, " M src/mat.go")
	}
	if body, err := os.ReadFile(filepath.Join(repo.Dir, "src/mat.go")); err != nil || string(body) != "package src // dispatch-time unstaged edit\n" {
		t.Errorf("the working copy must hold the dispatched content: %q %v", body, err)
	}
}

func TestC2168_007_AnIndexThatCannotBeRestoredIsLoudNeverVerified(t *testing.T) {
	repo := dispatchedRepo(t)
	fence, _ := fenceFor(t, repo)
	repo.Write("src/new.go", "package src // staged by the seat\n")
	repo.Git("add", "--", "src/new.go")
	if err := os.WriteFile(filepath.Join(repo.Dir, ".git", "index.lock"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	out := fence.End(context.Background())

	if out.RestoreErr == nil || out.Verified {
		t.Fatalf("a locked index must fail the restore: verified=%v err=%v", out.Verified, out.RestoreErr)
	}
	loud := false
	for _, d := range out.Diagnostics("build") {
		loud = loud || strings.Contains(d.Message, "restore failed")
	}
	if !loud {
		t.Errorf("no diagnostic reports the failed restore: %+v", out.Diagnostics("build"))
	}
}

func TestC2168_008_ASourceWritingSeatKeepsItsStagedFiles(t *testing.T) {
	repo := dispatchedRepo(t)
	fence := treefence.Begin(context.Background(), repo.Dir, false)
	repo.Write("src/new.go", "package src // staged by the builder\n")
	repo.Git("add", "--", "src/new.go")

	out := fence.End(context.Background())

	if len(out.Restored) != 0 || out.RestoreErr != nil {
		t.Fatalf("a source writer's fence is inert: %+v", out)
	}
	if status := gitLines(t, repo.Dir, "status", "--porcelain"); status != "A  src/new.go" {
		t.Errorf("git status --porcelain = %q, want the builder's staged file untouched", status)
	}
}

func TestC2168_009_TheFixAddsNoCommentToThePackage(t *testing.T) {
	dir := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "treefence")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	groups := 0
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		groups += len(f.Comments)
	}
	if groups > basePackageDocs {
		t.Errorf("treefence production code holds %d comment groups, want at most the %d package note", groups, basePackageDocs)
	}
}

func TestC2168_010_EveryChangedOrNewFunctionOfTheFenceIsFullyCovered(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	scratch := t.TempDir()
	profile := filepath.Join(scratch, "cover.out")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	var coverFunc string
	for _, args := range [][]string{
		{"test", "-count=1", "-tags", "integration", "-coverprofile=" + profile, "./internal/treefence"},
		{"tool", "cover", "-func=" + profile},
	} {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = goDir
		cmd.Env = ipcenv.GoTestEnv()
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("go %s: %v\n%s%s", strings.Join(args, " "), err, acsassert.OutputTail(stdout.String(), 30), acsassert.OutputTail(stderr.String(), 30))
		}
		coverFunc = stdout.String()
	}
	for _, line := range strings.Split(coverFunc, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.Contains(fields[0], "internal/treefence/") {
			continue
		}
		name := fields[1]
		if !slices.Contains(baselineFuncs, name) {
			if pct, err := acsassert.FuncCoverage(coverFunc, name); err != nil || pct != 100 {
				t.Errorf("new function %s has %.1f%% coverage (%v), want 100.0%%", name, pct, err)
			}
			continue
		}
		if slices.Contains(baselineBelowFull, name) {
			continue
		}
		if pct, err := acsassert.FuncCoverage(coverFunc, name); err != nil || pct != 100 {
			t.Errorf("function %s dropped to %.1f%% coverage (%v), want 100.0%%", name, pct, err)
		}
	}
}

func envelopeNames(out treefence.Outcome, rel string) bool {
	for _, d := range out.Diagnostics("build") {
		if strings.Contains(d.Message, rel) {
			return true
		}
	}
	return false
}

func requireVerifiedAndInStep(t *testing.T, repo *gittest.Repo, out treefence.Outcome) {
	t.Helper()
	if out.RestoreErr != nil || !out.Verified {
		t.Fatalf("the revert must verify: verified=%v err=%v", out.Verified, out.RestoreErr)
	}
	requireOneState(t, repo)
}

func TestC2168_011_AStagedFileTheSeatDeletedFromDiskLeavesNoADAndTheEnvelopeNamesIt(t *testing.T) {
	repo := dispatchedRepo(t)
	fence, _ := fenceFor(t, repo)
	repo.Write("src/new.go", "package src // staged then deleted\n")
	repo.Git("add", "--", "src/new.go")
	if err := os.Remove(filepath.Join(repo.Dir, "src/new.go")); err != nil {
		t.Fatal(err)
	}

	out := fence.End(context.Background())

	requireVerifiedAndInStep(t, repo, out)
	if !envelopeNames(out, "src/new.go") {
		t.Errorf("no envelope diagnostic names src/new.go: %+v", out.Diagnostics("build"))
	}
}

func TestC2168_012_AStagedEditThatTheSeatWroteBackOnDiskLeavesNoMMAndTheEnvelopeNamesIt(t *testing.T) {
	repo := dispatchedRepo(t)
	fence, _ := fenceFor(t, repo)
	repo.Write("src/mat.go", "package src // staged edit\n")
	repo.Git("add", "--", "src/mat.go")
	repo.Write("src/mat.go", "package src // base\n")

	out := fence.End(context.Background())

	requireVerifiedAndInStep(t, repo, out)
	if got := repo.Git("show", ":src/mat.go"); got != "package src // base" {
		t.Errorf("index entry of src/mat.go = %q, want the dispatched content", got)
	}
	if !envelopeNames(out, "src/mat.go") {
		t.Errorf("no envelope diagnostic names src/mat.go: %+v", out.Diagnostics("build"))
	}
}

func TestC2168_013_ATrackedFileTheSeatDroppedFromTheIndexOnlyIsTrackedAgainAndNamed(t *testing.T) {
	repo := dispatchedRepo(t)
	fence, _ := fenceFor(t, repo)
	repo.Git("rm", "-q", "--cached", "--", "src/mat.go")

	out := fence.End(context.Background())

	requireVerifiedAndInStep(t, repo, out)
	if !exists(repo, "src/mat.go") {
		t.Error("the file must stay on disk")
	}
	if !envelopeNames(out, "src/mat.go") {
		t.Errorf("no envelope diagnostic names src/mat.go: %+v", out.Diagnostics("build"))
	}
}

func TestC2168_014_AStageAllOverAFileThatWasUntrackedAtDispatchLeavesItUntrackedAndNamed(t *testing.T) {
	repo := dispatchedRepo(t)
	repo.Write("scratch.txt", "untracked at dispatch\n")
	fence, _ := fenceFor(t, repo)
	repo.Git("add", "-A")

	out := fence.End(context.Background())

	if out.RestoreErr != nil || !out.Verified {
		t.Fatalf("the revert must verify: verified=%v err=%v", out.Verified, out.RestoreErr)
	}
	if status := gitLines(t, repo.Dir, "status", "--porcelain"); status != "?? scratch.txt" {
		t.Errorf("git status --porcelain = %q, want the dispatch-time state %q", status, "?? scratch.txt")
	}
	if !exists(repo, "scratch.txt") {
		t.Error("the dispatch-time untracked file must stay on disk")
	}
	if !envelopeNames(out, "scratch.txt") {
		t.Errorf("no envelope diagnostic names scratch.txt: %+v", out.Diagnostics("build"))
	}
}

func TestC2168_015_AnotherRevertedPathDoesNotHideAStagedFileTheSeatDeletedFromDisk(t *testing.T) {
	repo := dispatchedRepo(t)
	fence, _ := fenceFor(t, repo)
	repo.Write("src/new.go", "package src // staged then deleted\n")
	repo.Git("add", "--", "src/new.go")
	if err := os.Remove(filepath.Join(repo.Dir, "src/new.go")); err != nil {
		t.Fatal(err)
	}
	repo.Write("src/other.go", "package src // plain stray write\n")

	out := fence.End(context.Background())

	requireVerifiedAndInStep(t, repo, out)
	for _, rel := range []string{"src/new.go", "src/other.go"} {
		if !envelopeNames(out, rel) {
			t.Errorf("no envelope diagnostic names %s: %+v", rel, out.Diagnostics("build"))
		}
	}
}

func TestC2168_016_AWritablePathStaysStagedWhileAFencedStageThatWasUndoneOnDiskIsCleared(t *testing.T) {
	repo := dispatchedRepo(t)
	fence, _ := fenceFor(t, repo, writablePath)
	repo.Write(writablePath, "package src // seat edit inside the list\n")
	repo.Git("add", "--", writablePath)
	repo.Write("src/new.go", "package src // staged then deleted\n")
	repo.Git("add", "--", "src/new.go")
	if err := os.Remove(filepath.Join(repo.Dir, "src/new.go")); err != nil {
		t.Fatal(err)
	}

	out := fence.End(context.Background())

	if out.RestoreErr != nil || !out.Verified {
		t.Fatalf("the revert must verify: verified=%v err=%v", out.Verified, out.RestoreErr)
	}
	if status := gitLines(t, repo.Dir, "status", "--porcelain"); status != "M  "+writablePath {
		t.Errorf("git status --porcelain = %q, want only the writable path staged", status)
	}
	if !envelopeNames(out, "src/new.go") {
		t.Errorf("no envelope diagnostic names src/new.go: %+v", out.Diagnostics("build"))
	}
}

func TestC2168_017_AnIndexThatCannotBeResetAfterAnUndoneStageIsLoudNeverVerified(t *testing.T) {
	repo := dispatchedRepo(t)
	fence, _ := fenceFor(t, repo)
	repo.Write("src/new.go", "package src // staged then deleted\n")
	repo.Git("add", "--", "src/new.go")
	if err := os.Remove(filepath.Join(repo.Dir, "src/new.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, ".git", "index.lock"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	out := fence.End(context.Background())

	if out.RestoreErr == nil || out.Verified {
		t.Fatalf("a locked index must fail the restore: verified=%v err=%v", out.Verified, out.RestoreErr)
	}
}

func conflictedRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := dispatchedRepo(t)
	repo.Git("checkout", "-q", "-b", "side")
	repo.Write("src/mat.go", "package src // side\n")
	repo.Git("commit", "-q", "-am", "side")
	repo.Git("checkout", "-q", "main")
	repo.Write("src/mat.go", "package src // main\n")
	repo.Git("commit", "-q", "-am", "main")
	merge := exec.Command("git", "-C", repo.Dir, "merge", "-q", "--no-edit", "side")
	merge.Env = withoutAmbientGit(os.Environ())
	if err := merge.Run(); err == nil {
		t.Fatal("the fixture merge must conflict")
	}
	if status := gitLines(t, repo.Dir, "status", "--porcelain"); status != "UU src/mat.go" {
		t.Fatalf("fixture status = %q, want an unmerged src/mat.go", status)
	}
	return repo
}

func TestC2168_018_AnUnmergedIndexAtDispatchDoesNotTurnTheFenceOff(t *testing.T) {
	repo := conflictedRepo(t)
	fence := treefence.Begin(context.Background(), repo.Dir, true)
	if err := fence.TakeErr(); err != nil {
		t.Fatalf("an unmerged index at dispatch must not disable the fence: %v", err)
	}
	repo.Write("src/stray.go", "package src // staged by the seat\n")
	repo.Git("add", "--", "src/stray.go")

	out := fence.End(context.Background())

	if out.RestoreErr != nil || !out.Verified {
		t.Fatalf("the revert must verify: verified=%v err=%v", out.Verified, out.RestoreErr)
	}
	if exists(repo, "src/stray.go") {
		t.Error("the stray write must be gone")
	}
	if !slices.Contains(out.Restored, "src/stray.go") {
		t.Errorf("Restored = %v, want src/stray.go named", out.Restored)
	}
	if status := gitLines(t, repo.Dir, "status", "--porcelain"); status != "UU src/mat.go" {
		t.Errorf("git status --porcelain = %q, want the dispatch-time unmerged state", status)
	}
}

func TestC2168_019_ARetractedTestWritingPhaseLeavesIndexAndWorkingCopyInStep(t *testing.T) {
	repo := dispatchedRepo(t)
	snap, err := treefence.Take(context.Background(), repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	repo.Write("src/new_test.go", "package src // staged by the tdd phase\n")
	repo.Write("src/mat.go", "package src // staged edit\n")
	repo.Git("add", "--", "src/new_test.go", "src/mat.go")
	repo.Write("src/mat.go", "package src // edited again\n")
	keepDir := t.TempDir()

	res, err := snap.Retract(context.Background(), keepDir)

	if err != nil {
		t.Fatalf("Retract: %v", err)
	}
	requireOneState(t, repo)
	for _, rel := range []string{"src/new_test.go", "src/mat.go"} {
		if !slices.Contains(res.Restored, rel) {
			t.Errorf("Restored = %v, want %s named", res.Restored, rel)
		}
		if _, err := os.Stat(filepath.Join(keepDir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("the retraction must keep %s as evidence: %v", rel, err)
		}
	}
}

func TestC2168_020_ARetractionNeverStagesAnEditThatWasUnstagedAtDispatch(t *testing.T) {
	repo := dispatchedRepo(t)
	repo.Write("src/mat.go", "package src // dispatch-time unstaged edit\n")
	snap, err := treefence.Take(context.Background(), repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	repo.Write("src/mat.go", "package src // phase edit\n")
	repo.Git("add", "--", "src/mat.go")

	if _, err := snap.Retract(context.Background(), t.TempDir()); err != nil {
		t.Fatalf("Retract: %v", err)
	}

	if status := gitLines(t, repo.Dir, "status", "--porcelain"); status != " M src/mat.go" {
		t.Errorf("git status --porcelain = %q, want the dispatch-time state %q", status, " M src/mat.go")
	}
	if body, err := os.ReadFile(filepath.Join(repo.Dir, "src/mat.go")); err != nil || string(body) != "package src // dispatch-time unstaged edit\n" {
		t.Errorf("the working copy must hold the dispatched content: %q %v", body, err)
	}
}

func TestC2168_021_AStagedFileARetractedPhaseDeletedFromDiskLeavesNoAD(t *testing.T) {
	repo := dispatchedRepo(t)
	snap, err := treefence.Take(context.Background(), repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	repo.Write("src/new_test.go", "package src // staged then deleted\n")
	repo.Git("add", "--", "src/new_test.go")
	if err := os.Remove(filepath.Join(repo.Dir, "src/new_test.go")); err != nil {
		t.Fatal(err)
	}

	if _, err := snap.Retract(context.Background(), t.TempDir()); err != nil {
		t.Fatalf("Retract: %v", err)
	}

	requireOneState(t, repo)
}

func goTestOutput(t *testing.T, pkg, run string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-run", run, pkg)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	cmd.Env = ipcenv.GoTestEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return acsassert.OutputTail(stdout.String(), 40) + acsassert.OutputTail(stderr.String(), 40), err
}

func TestC2168_022_AGateWriteTheFenceRevertedNeverLetsTheIdentityCarryShipWithoutASecondAudit(t *testing.T) {
	run := "^(TestIdentityCarry_|TestRouteRebasedExplanation_ACarryThatCannotBeProvenReturnsToAudit$|TestRouteRebasedExplanation_AByteIdenticalRebaseShipsOnTheCarriedVerdict$)"
	out, err := goTestOutput(t, "./internal/core", run)
	if err != nil {
		t.Errorf("the identity carry must re-audit when a gate changed the tree, and still carry when none did: %v\n%s", err, out)
	}
}

func TestC2168_023_AStageTheSeatUndidOnDiskGivesNoPatchAndBothDocumentsSayIt(t *testing.T) {
	repo := dispatchedRepo(t)
	fence, workspace := fenceFor(t, repo)
	repo.Write("src/new.go", "package src // staged then deleted\n")
	repo.Git("add", "--", "src/new.go")
	if err := os.Remove(filepath.Join(repo.Dir, "src/new.go")); err != nil {
		t.Fatal(err)
	}

	out := fence.End(context.Background())

	requireVerifiedAndInStep(t, repo, out)
	if !envelopeNames(out, "src/new.go") {
		t.Errorf("no envelope diagnostic names src/new.go: %+v", out.Diagnostics("build"))
	}
	if _, err := os.Stat(filepath.Join(workspace, patchFile)); err == nil {
		t.Errorf("a stage undone on disk has no content to save, yet %s exists; the documents must then say a patch is saved for it", patchFile)
	}
	root := acsassert.RepoRoot(t)
	undoneStageHasNoPatch := regexp.MustCompile(`(?i)(undid|undone)[^.\n]*on disk[^.\n]*\bno (reverted )?patch|\bno (reverted )?patch[^.\n]*(undid|undone)[^.\n]*on disk`)
	for _, rel := range []string{"docs/architecture/agent-run.md", "docs/architecture/packages/internal-treefence.md"} {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if !undoneStageHasNoPatch.Match(body) {
			t.Errorf("%s must state in one sentence that a stage the seat undid on disk gives no patch", rel)
		}
		if bytes.Contains(body, []byte("The reverted patch still holds the change.")) {
			t.Errorf("%s still claims without condition that the reverted patch holds the change", rel)
		}
	}
}

func TestC2168_024_ARepositoryOfTheSha256FormatLeavesOneStateAfterTheFenceRevert(t *testing.T) {
	t.Setenv("GIT_DEFAULT_HASH", "sha256")
	dir := gittest.Fixture(t).Dir
	if format := gitLines(t, dir, "rev-parse", "--show-object-format"); format != "sha256" {
		t.Fatalf("fixture object format = %q, want sha256", format)
	}
	gitLines(t, dir, "config", "user.email", "acs@example.invalid")
	gitLines(t, dir, "config", "user.name", "acs")
	if err := os.WriteFile(filepath.Join(dir, "keep.go"), []byte("package src\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitLines(t, dir, "add", "-A")
	gitLines(t, dir, "commit", "-q", "-m", "base")
	fence := treefence.Begin(context.Background(), dir, true)
	if err := fence.TakeErr(); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.go"), []byte("package src // staged by the seat\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitLines(t, dir, "add", "--", "new.go")
	if err := os.WriteFile(filepath.Join(dir, "keep.go"), []byte("package src // staged edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitLines(t, dir, "add", "--", "keep.go")
	if err := os.WriteFile(filepath.Join(dir, "keep.go"), []byte("package src // edited again\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := fence.End(context.Background())

	if out.RestoreErr != nil || !out.Verified {
		t.Fatalf("the revert must verify in a sha256 repository: verified=%v err=%v", out.Verified, out.RestoreErr)
	}
	if status := gitLines(t, dir, "status", "--porcelain"); status != "" {
		t.Errorf("index and working copy are apart in a sha256 repository; git status --porcelain:\n%s", status)
	}
	if !envelopeNames(out, "new.go") || !envelopeNames(out, "keep.go") {
		t.Errorf("the envelope must name both paths: %+v", out.Diagnostics("build"))
	}
}
