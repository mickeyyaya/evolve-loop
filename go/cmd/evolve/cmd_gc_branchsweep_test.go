package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const (
	branchSweepLanded  = "console/x"
	branchSweepUnowned = "feat/old"
	branchSweepFresh   = "console/y"
)

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

func branchSweepOperatorRun(t *testing.T, root string, args ...string) (stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	if rc := runGC(append(args, "--project-root", root), nil, &out, &errOut); rc == cliExitUsage {
		t.Fatalf("runGC rejected its arguments: %s", errOut.String())
	}
	return out.String(), errOut.String()
}

func branchSweepInstallEmptyGH(t *testing.T, binDir, root string) {
	t.Helper()
	script := "#!/bin/sh\necho '[]'\n"
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	origin := gittest.Bare(t)
	gcGit(t, root, "remote", "add", "origin", origin.Dir)
	gcGit(t, root, "push", "-q", "origin", "main")
}

func TestBranchSweep_HookNeverDeletesANonCycleBranch(t *testing.T) {
	for _, mode := range []string{"shadow", "enforce"} {
		t.Run(mode, func(t *testing.T) {
			root, evolveDir, workspace := gcWorktreeEnv(t, mode)
			branchSweepLand(t, root, branchSweepLanded, "x.txt")
			branchSweepLand(t, root, branchSweepUnowned, "old.txt")

			var buf bytes.Buffer
			runGCHook(loopConfig{EvolveDir: evolveDir, ProjectRoot: root}, workspace, &buf)

			m, ok := gcReadWorktreeManifest(t, workspace)
			if !ok {
				t.Fatalf("the hook wrote no workspace manifest; stderr=%q", buf.String())
			}
			if !gcManifestPlansBranchDelete(m, gcOrphanBranch) {
				t.Errorf("the hook stopped sweeping cycle-* branches: %+v", m.Items)
			}
			for _, b := range []string{branchSweepLanded, branchSweepUnowned} {
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
	binDir := gcIsolateHost(t)
	branchSweepInstallEmptyGH(t, binDir, root)
	branchSweepLand(t, root, branchSweepLanded, "x.txt")
	gcGit(t, root, "push", "-q", "origin", "main")
	branchSweepCommit(t, root, branchSweepFresh, "y.txt")

	stdout, _ := branchSweepOperatorRun(t, root, "--dry-run")

	if !strings.Contains(stdout, "WOULD-DELETE "+branchSweepLanded) {
		t.Errorf("--dry-run does not plan to delete %s:\n%s", branchSweepLanded, stdout)
	}
	if !gcBranchExists(t, root, branchSweepLanded) {
		t.Errorf("--dry-run deleted %s", branchSweepLanded)
	}
	if strings.Contains(stdout, "WOULD-DELETE "+branchSweepFresh) {
		t.Errorf("--dry-run plans to delete the fresh unlanded branch %s:\n%s", branchSweepFresh, stdout)
	}
	if tsvs, _ := filepath.Glob(filepath.Join(root, ".evolve", "operator-salvage", "branch-gc", "*.tsv")); len(tsvs) != 0 {
		t.Errorf("--dry-run wrote records: %v", tsvs)
	}
}

func TestBranchSweep_OperatorRunDeletesALandedBranchAfterAVerifiedBundle(t *testing.T) {
	root, evolveDir, _ := gcWorktreeEnv(t, "")
	binDir := gcIsolateHost(t)
	branchSweepInstallEmptyGH(t, binDir, root)
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"gc":{"remote_sweep":{"stage":"enforce"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	tip := branchSweepLand(t, root, branchSweepLanded, "x.txt")
	gcGit(t, root, "push", "-q", "origin", "main")
	mainTip := strings.TrimSpace(gcGit(t, root, "rev-parse", "main"))

	stdout, stderr := branchSweepOperatorRun(t, root)

	if gcBranchExists(t, root, branchSweepLanded) {
		t.Errorf("the operator run left the landed branch %s; stdout=%s stderr=%s", branchSweepLanded, stdout, stderr)
	}
	tsvs, err := filepath.Glob(filepath.Join(evolveDir, "operator-salvage", "branch-gc", "*.tsv"))
	if err != nil || len(tsvs) == 0 {
		t.Fatalf("no branch-gc tsv record written (err=%v)", err)
	}
	raw, err := os.ReadFile(tsvs[0])
	if err != nil || !strings.Contains(string(raw), branchSweepLanded+"\t"+tip) {
		t.Errorf("tsv = %q (err=%v), want row for %s@%s", string(raw), err, branchSweepLanded, tip)
	}
	if got := strings.TrimSpace(gcGit(t, root, "rev-parse", "main")); got != mainTip {
		t.Errorf("main moved from %s to %s", mainTip, got)
	}
}

func TestBranchSweep_OperatorRunKeepsAnUnownedBranch(t *testing.T) {
	root, evolveDir, _ := gcWorktreeEnv(t, "")
	binDir := gcIsolateHost(t)
	branchSweepInstallEmptyGH(t, binDir, root)
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"gc":{"remote_sweep":{"stage":"enforce"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	unownedTip := branchSweepLand(t, root, branchSweepUnowned, "old.txt")
	gcGit(t, root, "push", "-q", "origin", "main")

	branchSweepOperatorRun(t, root)

	if got := strings.TrimSpace(gcGit(t, root, "rev-parse", "refs/heads/"+branchSweepUnowned)); got != unownedTip {
		t.Errorf("unowned branch moved or deleted: got %q, want %s", got, unownedTip)
	}
}

func TestBranchSweep_OperatorRunKeepsAFreshUnlandedBranchAndMain(t *testing.T) {
	root, evolveDir, _ := gcWorktreeEnv(t, "")
	binDir := gcIsolateHost(t)
	branchSweepInstallEmptyGH(t, binDir, root)
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"gc":{"remote_sweep":{"stage":"enforce"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
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
	binDir := gcIsolateHost(t)
	branchSweepInstallEmptyGH(t, binDir, root)
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"gc":{"remote_sweep":{"stage":"enforce"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	landedTip := branchSweepLand(t, root, branchSweepLanded, "x.txt")
	gcGit(t, root, "push", "-q", "origin", "main")
	if err := os.WriteFile(gc.OperatorSalvageDir(evolveDir), []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	branchSweepOperatorRun(t, root)

	if got := strings.TrimSpace(gcGit(t, root, "rev-parse", "refs/heads/"+branchSweepLanded)); got != landedTip {
		t.Errorf("%s was deleted or moved although no record could be written: tip %q, want %s", branchSweepLanded, got, landedTip)
	}
}
