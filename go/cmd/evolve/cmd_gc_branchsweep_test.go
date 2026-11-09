package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
)

const (
	branchSweepLanded = "feat/x"
	branchSweepEnded  = "feat/old"
	branchSweepFresh  = "feat/y"
)

var branchSweepIdleSince = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

func branchSweepCommit(t *testing.T, root, branch, file string) string {
	t.Helper()
	gcGit(t, root, "checkout", "-b", branch)
	if err := os.WriteFile(filepath.Join(root, file), []byte(branch+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gcGit(t, root, "add", file)
	gcGit(t, root, "commit", "-m", "work on "+branch)
	tip := strings.TrimSpace(gcGit(t, root, "rev-parse", "HEAD"))
	gcGit(t, root, "checkout", "main")
	return tip
}

func branchSweepLand(t *testing.T, root, branch, file string) string {
	t.Helper()
	tip := branchSweepCommit(t, root, branch, file)
	gcGit(t, root, "merge", "--no-ff", "-m", "land "+branch, branch)
	return tip
}

func branchSweepEnd(t *testing.T, root, branch, file string) string {
	t.Helper()
	stamp := branchSweepIdleSince.Format(time.RFC3339)
	t.Setenv("GIT_AUTHOR_DATE", stamp)
	t.Setenv("GIT_COMMITTER_DATE", stamp)
	tip := branchSweepCommit(t, root, branch, file)
	os.Unsetenv("GIT_AUTHOR_DATE")
	os.Unsetenv("GIT_COMMITTER_DATE")
	return tip
}

func branchSweepBundleHolds(t *testing.T, root, branch, tip string) bool {
	t.Helper()
	bundles, err := filepath.Glob(filepath.Join(root, ".evolve", "operator-salvage", "*", "refs.bundle"))
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bundles {
		if exec.Command("git", "-C", root, "bundle", "verify", b).Run() != nil {
			continue
		}
		heads := gcGit(t, root, "bundle", "list-heads", b)
		if strings.Contains(heads, tip+" refs/heads/"+branch) {
			return true
		}
	}
	return false
}

func branchSweepOperatorRun(t *testing.T, root string, args ...string) (stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	if rc := runGC(append(args, "--project-root", root), nil, &out, &errOut); rc == cliExitUsage {
		t.Fatalf("runGC rejected its arguments: %s", errOut.String())
	}
	return out.String(), errOut.String()
}

func TestBranchSweep_HookNeverDeletesANonCycleBranch(t *testing.T) {
	for _, mode := range []string{"shadow", "enforce"} {
		t.Run(mode, func(t *testing.T) {
			root, evolveDir, workspace := gcWorktreeEnv(t, mode)
			branchSweepLand(t, root, branchSweepLanded, "x.txt")
			branchSweepEnd(t, root, branchSweepEnded, "old.txt")

			var buf bytes.Buffer
			runGCHook(loopConfig{EvolveDir: evolveDir, ProjectRoot: root}, workspace, &buf)

			m, ok := gcReadWorktreeManifest(t, workspace)
			if !ok {
				t.Fatalf("the hook wrote no workspace manifest; stderr=%q", buf.String())
			}
			if !gcManifestPlansBranchDelete(m, gcOrphanBranch) {
				t.Errorf("the hook stopped sweeping cycle-* branches: %+v", m.Items)
			}
			for _, b := range []string{branchSweepLanded, branchSweepEnded} {
				if gcManifestPlansBranchDelete(m, b) {
					t.Errorf("the hook planned to delete %s: %+v", b, m.Items)
				}
				if !gcBranchExists(t, root, b) {
					t.Errorf("the hook deleted %s at gc.mode=%s", b, mode)
				}
			}
		})
	}
}

func TestBranchSweep_OperatorDryRunPlansTheDeletesAndMutatesNothing(t *testing.T) {
	root, _, _ := gcWorktreeEnv(t, "")
	branchSweepLand(t, root, branchSweepLanded, "x.txt")
	branchSweepEnd(t, root, branchSweepEnded, "old.txt")
	branchSweepCommit(t, root, branchSweepFresh, "y.txt")

	stdout, _ := branchSweepOperatorRun(t, root, "--dry-run")

	for _, b := range []string{branchSweepLanded, branchSweepEnded} {
		if !strings.Contains(stdout, "WOULD-DELETE-BRANCH branch="+b) {
			t.Errorf("--dry-run does not plan to delete %s:\n%s", b, stdout)
		}
		if !gcBranchExists(t, root, b) {
			t.Errorf("--dry-run deleted %s", b)
		}
	}
	if strings.Contains(stdout, "WOULD-DELETE-BRANCH branch="+branchSweepFresh) {
		t.Errorf("--dry-run plans to delete the fresh unlanded branch %s:\n%s", branchSweepFresh, stdout)
	}
	if bundles, _ := filepath.Glob(filepath.Join(root, ".evolve", "operator-salvage", "*", "refs.bundle")); len(bundles) != 0 {
		t.Errorf("--dry-run wrote bundles: %v", bundles)
	}
}

func TestBranchSweep_OperatorRunDeletesALandedBranchAfterAVerifiedBundle(t *testing.T) {
	root, _, _ := gcWorktreeEnv(t, "")
	tip := branchSweepLand(t, root, branchSweepLanded, "x.txt")
	mainTip := strings.TrimSpace(gcGit(t, root, "rev-parse", "main"))

	stdout, stderr := branchSweepOperatorRun(t, root)

	if gcBranchExists(t, root, branchSweepLanded) {
		t.Errorf("the operator run left the landed branch %s; stdout=%s stderr=%s", branchSweepLanded, stdout, stderr)
	}
	if !branchSweepBundleHolds(t, root, branchSweepLanded, tip) {
		t.Errorf("no verified refs.bundle under operator-salvage holds %s@%s", branchSweepLanded, tip)
	}
	if got := strings.TrimSpace(gcGit(t, root, "rev-parse", "main")); got != mainTip {
		t.Errorf("main moved from %s to %s", mainTip, got)
	}
}

func TestBranchSweep_OperatorRunDeletesAnEndedBranchAfterAVerifiedBundle(t *testing.T) {
	root, _, _ := gcWorktreeEnv(t, "")
	tip := branchSweepEnd(t, root, branchSweepEnded, "old.txt")

	stdout, stderr := branchSweepOperatorRun(t, root)

	if gcBranchExists(t, root, branchSweepEnded) {
		t.Errorf("the operator run left the ended branch %s; stdout=%s stderr=%s", branchSweepEnded, stdout, stderr)
	}
	if !branchSweepBundleHolds(t, root, branchSweepEnded, tip) {
		t.Errorf("no verified refs.bundle under operator-salvage holds %s@%s", branchSweepEnded, tip)
	}
}

func TestBranchSweep_OperatorRunKeepsAFreshUnlandedBranchAndMain(t *testing.T) {
	root, _, _ := gcWorktreeEnv(t, "")
	freshTip := branchSweepCommit(t, root, branchSweepFresh, "y.txt")

	branchSweepOperatorRun(t, root)

	if !gcBranchExists(t, root, branchSweepFresh) {
		t.Errorf("the operator run deleted the fresh unlanded branch %s", branchSweepFresh)
	}
	if got := strings.TrimSpace(gcGit(t, root, "rev-parse", branchSweepFresh)); got != freshTip {
		t.Errorf("%s moved from %s to %s", branchSweepFresh, freshTip, got)
	}
	if !gcBranchExists(t, root, "main") {
		t.Errorf("the operator run deleted main")
	}
}

func TestBranchSweep_OperatorRunKeepsABranchWhenItsBundleCannotBeWritten(t *testing.T) {
	root, evolveDir, _ := gcWorktreeEnv(t, "")
	landedTip := branchSweepLand(t, root, branchSweepLanded, "x.txt")
	endedTip := branchSweepEnd(t, root, branchSweepEnded, "old.txt")
	if err := os.WriteFile(gc.OperatorSalvageDir(evolveDir), []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	branchSweepOperatorRun(t, root)

	for b, tip := range map[string]string{branchSweepLanded: landedTip, branchSweepEnded: endedTip} {
		if got := strings.TrimSpace(gcGit(t, root, "rev-parse", "refs/heads/"+b)); got != tip {
			t.Errorf("%s was deleted or moved although no bundle could be written: tip %q, want %s", b, got, tip)
		}
	}
}
