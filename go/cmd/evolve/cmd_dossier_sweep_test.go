package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const (
	sweepPairCycle = 1705
	sweepHalfCycle = 1707
)

func writeCorpusPair(t *testing.T, root string, cycle int) {
	t.Helper()
	d := &dossier.Dossier{Cycle: cycle, Goal: "an orphaned closeout", FinalVerdict: dossier.VerdictPass,
		Phases: []dossier.PhaseRecord{{Name: "build", Verdict: dossier.VerdictPass}}}
	if err := dossier.Write(d, dossier.CyclesDir(root), false); err != nil {
		t.Fatal(err)
	}
}

func planeWithOrphanPairAndHalfPair(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gitPlane(t, "main")
	writeCorpusPair(t, r.Dir, sweepPairCycle)
	writeCorpusPair(t, r.Dir, sweepHalfCycle)
	if err := os.Remove(filepath.Join(dossier.CyclesDir(r.Dir), "cycle-1707.json")); err != nil {
		t.Fatal(err)
	}
	return r
}

func runSweep(t *testing.T, root string, extra ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	args := append([]string{"sweep", "--project-root", root}, extra...)
	code := runDossier(args, nil, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRunDossierSweep_DryRunListsTheOrphanPairAndCommitsNothing(t *testing.T) {
	t.Parallel()
	r := planeWithOrphanPairAndHalfPair(t)
	head := r.Git("rev-parse", "HEAD")
	statusBefore := r.Git("status", "--porcelain=v1", "--untracked-files=all")

	code, stdout, stderr := runSweep(t, r.Dir, "--dry-run")

	if code != 0 {
		t.Fatalf("a dry run on a plane with one orphan pair must exit 0: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "1705") {
		t.Errorf("the dry run must name the pair it would recommit: %q", stdout)
	}
	if r.Git("rev-parse", "HEAD") != head {
		t.Error("a dry run moved HEAD")
	}
	if got := r.Git("status", "--porcelain=v1", "--untracked-files=all"); got != statusBefore {
		t.Errorf("a dry run changed the tree:\nbefore %q\nafter  %q", statusBefore, got)
	}
}

func TestRunDossierSweep_CommitsTheCompletePairAndLeavesTheHalfPair(t *testing.T) {
	t.Parallel()
	r := planeWithOrphanPairAndHalfPair(t)
	base := r.Git("rev-parse", "HEAD")

	code, stdout, stderr := runSweep(t, r.Dir)

	if code != 0 {
		t.Fatalf("sweep must exit 0: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if got := r.Git("show", "--name-only", "--format=%s", "HEAD"); got != closeoutCommit {
		t.Fatalf("HEAD = %q, want one closeout commit of cycle 1705", got)
	}
	if r.Git("rev-parse", "HEAD~1") != base {
		t.Error("the closeout must be one commit on the base")
	}
	if _, err := os.Stat(filepath.Join(dossier.CyclesDir(r.Dir), "cycle-1707.md")); err != nil {
		t.Errorf("the half pair must stay untouched on disk: %v", err)
	}
	if tracked := r.Git("ls-files", "knowledge-base/cycles/cycle-1707.md"); tracked != "" {
		t.Errorf("the half pair was committed: %q", tracked)
	}
	for _, cycle := range []string{"1705", "1707"} {
		if !strings.Contains(stdout, cycle) {
			t.Errorf("the report must name recommitted 1705 and skipped 1707; %s is missing: %q", cycle, stdout)
		}
	}
}

func TestRunDossierSweep_ALiveRunHoldsTheSweepWithExit1AndCommitsNothing(t *testing.T) {
	t.Parallel()
	r := planeWithOrphanPairAndHalfPair(t)
	leaseARun(t, r.Dir, 1706, os.Getppid())
	head := r.Git("rev-parse", "HEAD")

	code, stdout, stderr := runSweep(t, r.Dir)

	if code != 1 {
		t.Fatalf("a sweep held by a live run must exit 1: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "cycle-1706") {
		t.Errorf("the hold must name the live run: %q", stderr)
	}
	if r.Git("rev-parse", "HEAD") != head {
		t.Error("a held sweep committed")
	}
}

func TestRunDossierSweep_ACleanCorpusExits0AndCommitsNothing(t *testing.T) {
	t.Parallel()
	r := gitPlane(t, "main")
	head := r.Git("rev-parse", "HEAD")

	code, stdout, stderr := runSweep(t, r.Dir)

	if code != 0 {
		t.Fatalf("nothing to sweep must exit 0: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if r.Git("rev-parse", "HEAD") != head {
		t.Error("an empty sweep moved HEAD")
	}
}

func TestRunDossierSweep_RejectsStrayArgumentsWithExit10(t *testing.T) {
	t.Parallel()
	r := gitPlane(t, "main")

	code, _, stderr := runSweep(t, r.Dir, "stray")

	if code != 10 || strings.Contains(stderr, "unknown subcommand") {
		t.Fatalf("a stray argument to a known sweep verb is a usage error: code=%d stderr=%q", code, stderr)
	}
}

func TestRunDossier_UsageNamesSweep(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer

	code := runDossier(nil, nil, &stdout, &stderr)

	if code != 10 || !strings.Contains(stderr.String(), "sweep") {
		t.Fatalf("the usage must name sweep and exit 10: code=%d stderr=%q", code, stderr.String())
	}
}
