package core

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func stampedSnapshotRepo(t *testing.T, snapshotA string) (*gittest.Repo, continuation.Continuation) {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("a.txt", "base\n")
	repo.Git("add", "a.txt")
	repo.Git("commit", "-q", "-m", "base")
	base := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	repo.Git("checkout", "-q", "-b", "cycle-2035")
	repo.Write("half_built.go", "package wip\n")
	repo.Write("a.txt", snapshotA)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", salvageSnapshotSubject)
	snapshot := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	repo.Git("checkout", "-q", "main")
	repo.Write("a.txt", "main moved\n")
	repo.Write("landed.go", "package landed\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "main moves on")
	ws := filepath.Join(t.TempDir(), "runs", "cycle-2035")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	findings := filepath.Join(ws, "audit-fail-reason.json")
	if err := os.WriteFile(findings, []byte(`{"schema_version":1,"phase":"audit","reasons":["the protected-surface test is red"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo, continuation.Continuation{Branch: "cycle-2035", SnapshotSHA: snapshot, BaseSHA: base, FindingsPath: findings, Cycle: 2035}
}

func TestAdoptContinuationForConsole_GivesTheSnapshotPlusMainAndAFindingsFile(t *testing.T) {
	t.Parallel()
	repo, c := stampedSnapshotRepo(t, "base\n")
	wt := filepath.Join(t.TempDir(), "adopted")

	got, err := AdoptContinuationForConsole(context.Background(), ConsoleAdoptRequest{ProjectRoot: repo.Dir, Worktree: wt, ItemID: "retro-minting", Binding: c, Warn: io.Discard})

	if err != nil {
		t.Fatalf("adopt a Clean snapshot: %v", err)
	}
	for _, f := range []string{"half_built.go", "landed.go"} {
		if _, err := os.Stat(filepath.Join(wt, f)); err != nil {
			t.Errorf("the adopted worktree must hold the snapshot work and main: %s missing (%v)", f, err)
		}
	}
	mainTip := strings.TrimSpace(repo.Git("rev-parse", "main"))
	if got.BaseSHA != mainTip || got.Worktree != wt || got.Branch != "adopt/retro-minting" {
		t.Errorf("adoption = %+v; want base %s, worktree %s, branch adopt/retro-minting", got, mainTip, wt)
	}
	if status := strings.TrimSpace(repo.Git("-C", wt, "status", "--porcelain")); status != "" {
		t.Errorf("the findings file must sit next to the worktree, not inside it; status:\n%s", status)
	}
	body, err := os.ReadFile(got.FindingsFile)
	if err != nil || filepath.Dir(got.FindingsFile) != filepath.Dir(wt) {
		t.Fatalf("findings file %s next to the worktree: %v", got.FindingsFile, err)
	}
	for _, want := range []string{"cycle 2035", c.SnapshotSHA, "the protected-surface test is red", "Red predicates"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("findings file must name %q:\n%s", want, body)
		}
	}
}

func TestAdoptContinuationForConsole_RefusesAConflictingSnapshotWithItsFiles(t *testing.T) {
	t.Parallel()
	repo, c := stampedSnapshotRepo(t, "lane edit\n")
	wt := filepath.Join(t.TempDir(), "adopted")

	_, err := AdoptContinuationForConsole(context.Background(), ConsoleAdoptRequest{ProjectRoot: repo.Dir, Worktree: wt, ItemID: "retro-minting", Binding: c, Warn: io.Discard})

	if !errors.Is(err, ErrContinuationConflict) || !strings.HasSuffix(err.Error(), ": a.txt") {
		t.Fatalf("err = %v; want ErrContinuationConflict naming exactly a.txt", err)
	}
	if strings.Contains(err.Error(), "half_built.go") {
		t.Errorf("only the conflicting files are named: %v", err)
	}
	if _, serr := os.Stat(wt); !os.IsNotExist(serr) {
		t.Errorf("a refused adoption must create no worktree: %v", serr)
	}
}

func TestAdoptContinuationForConsole_RefusesAnInvalidBinding(t *testing.T) {
	t.Parallel()
	repo, c := stampedSnapshotRepo(t, "base\n")
	missing := c
	missing.SnapshotSHA = strings.Repeat("0", 40)

	_, err := AdoptContinuationForConsole(context.Background(), ConsoleAdoptRequest{ProjectRoot: repo.Dir, Worktree: filepath.Join(t.TempDir(), "w"), ItemID: "retro-minting", Binding: missing, Warn: io.Discard})

	if err == nil || errors.Is(err, ErrContinuationConflict) || !strings.Contains(err.Error(), "not in repo") {
		t.Errorf("err = %v; a snapshot absent from the repo is an invalid binding, not a conflict", err)
	}
}

func TestAdoptContinuationForConsole_RefusesABaseThatIsNotAnAncestor(t *testing.T) {
	t.Parallel()
	repo, c := stampedSnapshotRepo(t, "base\n")
	c.BaseSHA = strings.TrimSpace(repo.Git("rev-parse", "main"))

	_, err := AdoptContinuationForConsole(context.Background(), ConsoleAdoptRequest{ProjectRoot: repo.Dir, Worktree: filepath.Join(t.TempDir(), "w"), ItemID: "retro-minting", Binding: c, Warn: io.Discard})

	if err == nil || errors.Is(err, ErrContinuationConflict) || !strings.Contains(err.Error(), "is not an ancestor") {
		t.Errorf("err = %v; a base outside the snapshot history is an invalid binding", err)
	}
}

func TestAdoptContinuationForConsole_AWorktreeAddFailureLeavesNoFindings(t *testing.T) {
	t.Parallel()
	repo, c := stampedSnapshotRepo(t, "base\n")
	repo.Git("branch", "adopt/retro-minting")
	wt := filepath.Join(t.TempDir(), "adopted")

	_, err := AdoptContinuationForConsole(context.Background(), ConsoleAdoptRequest{ProjectRoot: repo.Dir, Worktree: wt, ItemID: "retro-minting", Binding: c, Warn: io.Discard})

	if err == nil || !strings.Contains(err.Error(), "adopt/retro-minting") {
		t.Fatalf("err = %v; an existing branch must refuse the adoption by name", err)
	}
	if _, serr := os.Stat(wt + ".findings.md"); !os.IsNotExist(serr) {
		t.Errorf("no findings file without a worktree: %v", serr)
	}
}

func TestAdoptContinuationForConsole_AnUnwritableFindingsFileIsAnError(t *testing.T) {
	t.Parallel()
	repo, c := stampedSnapshotRepo(t, "base\n")
	wt := filepath.Join(t.TempDir(), "adopted")
	if err := os.Mkdir(wt+".findings.md", 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := AdoptContinuationForConsole(context.Background(), ConsoleAdoptRequest{ProjectRoot: repo.Dir, Worktree: wt, ItemID: "retro-minting", Binding: c, Warn: io.Discard})

	if err == nil || !strings.Contains(err.Error(), wt+".findings.md") {
		t.Errorf("err = %v; a findings file that cannot be written must fail the adoption by path", err)
	}
	if _, serr := os.Stat(wt); !os.IsNotExist(serr) {
		t.Errorf("a failed adoption must remove the worktree it added: %v", serr)
	}
	if out := strings.TrimSpace(repo.Git("branch", "--list", "adopt/retro-minting")); out != "" {
		t.Errorf("a failed adoption must delete the branch it added: %q", out)
	}
	if _, err := AdoptContinuationForConsole(context.Background(), ConsoleAdoptRequest{ProjectRoot: repo.Dir, Worktree: wt + "/", ItemID: "retro-minting", Binding: c, Warn: io.Discard}); err == nil || !strings.Contains(err.Error(), wt+".findings.md") {
		t.Errorf("a trailing slash names the same worktree and the same findings file: %v", err)
	}
}

func TestAdoptContinuationForConsole_AFailedBaseAdvanceIsAnError(t *testing.T) {
	c := continuation.Continuation{SnapshotSHA: "5a5a5a5a5a5a5a5a", BaseSHA: "bbbb", Cycle: 2035}
	undone := false
	useScriptedGit(t, func(_ int, args []string) scriptedReply {
		switch {
		case hasArg(args, "worktree") && hasArg(args, "remove"):
			undone = true
			return scriptedReply{}
		case hasArg(args, "merge-base"):
			if hasArg(args, c.BaseSHA) {
				return scriptedReply{}
			}
			return scriptedReply{exit: 1}
		case hasArg(args, "cherry"):
			return scriptedReply{stdout: "+ 5a5a5a5a\n"}
		case hasArg(args, "rev-parse"):
			return scriptedReply{stdout: "c0ffee\n"}
		case hasArg(args, "merge") && hasArg(args, "--no-edit"):
			return scriptedReply{exit: 1}
		case hasArg(args, "diff"):
			return scriptedReply{stdout: "a.txt\n"}
		}
		return scriptedReply{}
	})
	wt := filepath.Join(t.TempDir(), "adopted")

	_, err := AdoptContinuationForConsole(context.Background(), ConsoleAdoptRequest{ProjectRoot: t.TempDir(), Worktree: wt, ItemID: "retro-minting", Binding: c, Warn: io.Discard})

	if err == nil || !strings.Contains(err.Error(), "base-advance merge c0ffee failed") || !strings.Contains(err.Error(), "a.txt") {
		t.Errorf("err = %v; a merge of main that fails must name the merge and its conflicts", err)
	}
	if _, serr := os.Stat(wt + ".findings.md"); !os.IsNotExist(serr) {
		t.Errorf("no findings file after a failed base advance: %v", serr)
	}
	if !undone {
		t.Error("a failed base advance must remove the worktree it added")
	}
	if strings.Contains(err.Error(), "undo:") {
		t.Errorf("a clean undo adds no undo error: %v", err)
	}
}

func TestUndoConsoleWorktree_NamesEachStepThatFails(t *testing.T) {
	useScriptedGit(t, func(_ int, args []string) scriptedReply { return scriptedReply{exit: 1} })

	err := undoConsoleWorktree(context.Background(), gitexec.Git{Dir: t.TempDir(), Exec: gitRunner}, ConsoleAdoption{Worktree: "/w", Branch: "adopt/x"})

	if err == nil || !strings.Contains(err.Error(), "worktree remove --force /w") || !strings.Contains(err.Error(), "branch -D adopt/x") {
		t.Errorf("err = %v; each failed undo step is named", err)
	}
}

func TestAdoptContinuationForConsole_TheFindingsFileNamesTheRedPredicatesOfThePriorTry(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Write("a.txt", "base\n")
	repo.Git("add", "a.txt")
	repo.Git("commit", "-q", "-m", "base")
	base := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	repo.Git("checkout", "-q", "-b", "cycle-2035")
	repo.Write("go/acs/c2035/pred_test.go", "package c2035\n\nfunc TestPredRed(t *testing.T) {}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", salvageSnapshotSubject)
	snapshot := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	repo.Git("checkout", "-q", "main")
	ws := filepath.Join(t.TempDir(), "cycle-2035")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "acs-verdict.json"), []byte(`{"results":[{"ac_id":"c2035/TestPredRed","result":"red"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	c := continuation.Continuation{SnapshotSHA: snapshot, BaseSHA: base, FindingsPath: filepath.Join(ws, "audit-fail-reason.json"), Cycle: 2035}

	a, err := AdoptContinuationForConsole(context.Background(), ConsoleAdoptRequest{ProjectRoot: repo.Dir, Worktree: filepath.Join(t.TempDir(), "w"), ItemID: "x", Binding: c, Warn: &warn})

	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(a.FindingsFile)
	if !strings.Contains(string(body), "go/acs/c2035/pred_test.go: TestPredRed") {
		t.Errorf("findings file misses the red predicate:\n%s", body)
	}
	if !strings.Contains(warn.String(), "audit-fail-reason.json") {
		t.Errorf("an unreadable findings artifact is warned on the caller's writer, got %q", warn.String())
	}
}

func TestAdoptContinuationForConsole_ARelativeWorktreeIsUnderTheProjectRoot(t *testing.T) {
	t.Parallel()
	repo, c := stampedSnapshotRepo(t, "base\n")

	a, err := AdoptContinuationForConsole(context.Background(), ConsoleAdoptRequest{ProjectRoot: repo.Dir, Worktree: "adopted", ItemID: "rel", Binding: c, Warn: io.Discard})

	if err != nil || a.Worktree != filepath.Join(repo.Dir, "adopted") || a.FindingsFile != filepath.Join(repo.Dir, "adopted.findings.md") {
		t.Fatalf("adoption = %+v, %v; a relative worktree resolves under the project root", a, err)
	}
	if _, err := os.Stat(a.FindingsFile); err != nil {
		t.Errorf("findings file: %v", err)
	}
}

func TestUndoConsoleWorktree_RunsUnderACancelledContext(t *testing.T) {
	t.Parallel()
	repo, c := stampedSnapshotRepo(t, "base\n")
	wt := filepath.Join(t.TempDir(), "w")
	repo.Git("worktree", "add", "-q", "-b", "adopt/p", wt, c.SnapshotSHA)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := undoConsoleWorktree(ctx, gitexec.Git{Dir: repo.Dir, Exec: gitRunner}, ConsoleAdoption{Worktree: wt, Branch: "adopt/p"}); err != nil {
		t.Fatalf("an undo after a cancel must still run: %v", err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("worktree kept after the undo: %v", err)
	}
	if out := strings.TrimSpace(repo.Git("branch", "--list", "adopt/p")); out != "" {
		t.Errorf("branch kept after the undo: %q", out)
	}
}

func TestAdoptContinuationForConsole_ANilWarnWriterIsDiscarded(t *testing.T) {
	t.Parallel()
	repo, c := stampedSnapshotRepo(t, "base\n")
	c.FindingsPath = filepath.Join(t.TempDir(), "absent.json")

	if _, err := AdoptContinuationForConsole(context.Background(), ConsoleAdoptRequest{ProjectRoot: repo.Dir, Worktree: filepath.Join(t.TempDir(), "w"), ItemID: "nil-warn", Binding: c}); err != nil {
		t.Errorf("a nil Warn must not panic or fail: %v", err)
	}
}

func TestAdoptContinuationForConsole_ABadBaseOnAConflictingSnapshotIsNotReportedAsAConflict(t *testing.T) {
	t.Parallel()
	repo, c := stampedSnapshotRepo(t, "lane edit\n")
	c.BaseSHA = strings.TrimSpace(repo.Git("rev-parse", "main"))

	_, err := AdoptContinuationForConsole(context.Background(), ConsoleAdoptRequest{ProjectRoot: repo.Dir, Worktree: filepath.Join(t.TempDir(), "w"), ItemID: "bad-base", Binding: c})

	if err == nil || errors.Is(err, ErrContinuationConflict) || !strings.Contains(err.Error(), "is not an ancestor") {
		t.Errorf("err = %v; the first failed check is the cause, not the conflict", err)
	}
}

func TestAdoptContinuationForConsole_AnAlreadyLandedSnapshotIsNotAConflict(t *testing.T) {
	t.Parallel()
	repo, c := stampedSnapshotRepo(t, "base\n")
	repo.Git("merge", "-q", "--no-edit", c.SnapshotSHA)

	_, err := AdoptContinuationForConsole(context.Background(), ConsoleAdoptRequest{ProjectRoot: repo.Dir, Worktree: filepath.Join(t.TempDir(), "w"), ItemID: "landed", Binding: c})

	if err == nil || errors.Is(err, ErrContinuationConflict) || !strings.Contains(err.Error(), "no longer Clean against main (verdict=0)") {
		t.Errorf("err = %v; landed work has nothing to adopt and is no conflict", err)
	}
}
