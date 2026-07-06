package dossier

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// interceptExec wraps the real production runner and forces every `git
// commit` invocation naming blockBase to fail permanently, while every other
// call (add, diff, status, and commits for other bases) passes through to
// real git unchanged. Lets a SweepOrphans test exercise a genuinely
// unrecoverable per-pair failure deterministically, without racing a real
// stuck git index.lock.
type interceptExec struct {
	blockBase string
}

func (i *interceptExec) run(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(args) > 0 && args[0] == "commit" {
		for _, a := range args {
			if strings.Contains(a, i.blockBase) {
				if stderr != nil {
					_, _ = stderr.Write([]byte("fatal: forced unrecoverable failure for " + i.blockBase + "\n"))
				}
				return 128, nil
			}
		}
	}
	return sysexec.DefaultRunner(ctx, name, dir, args, env, stdin, stdout, stderr)
}

// initSweepRepo makes dir a git working tree with a commit identity
// configured, mirroring write_test.go's initGitRepo (this file adds its own
// copy to keep sweep_test.go independently readable).
func initSweepRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func writePairFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func sweepGitStatus(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain", "-uall")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestSweepOrphans_RecommitsUntrackedPairs is the scout-mandated verification
// anchor (verifiableBy) for sweep-orphaned-dossier-pairs-and-harden-commit:
// untracked, COMPLETE cycle-N.{json,md} pairs sitting in the main tree (the 35
// live orphans confirmed by cycle-564 scout) must be detected and recommitted.
func TestSweepOrphans_RecommitsUntrackedPairs(t *testing.T) {
	dir := t.TempDir()
	initSweepRepo(t, dir)
	writePairFile(t, dir, "cycle-10.json", `{"cycle":10}`)
	writePairFile(t, dir, "cycle-10.md", "# cycle 10")
	writePairFile(t, dir, "cycle-20.json", `{"cycle":20}`)
	writePairFile(t, dir, "cycle-20.md", "# cycle 20")

	res, err := SweepOrphans(gitexec.Default(dir), io.Discard)
	if err != nil {
		t.Fatalf("SweepOrphans: %v", err)
	}
	want := map[int]bool{10: true, 20: true}
	if len(res.Recommitted) != len(want) {
		t.Fatalf("Recommitted = %v, want cycles %v", res.Recommitted, want)
	}
	for _, c := range res.Recommitted {
		if !want[c] {
			t.Errorf("unexpected cycle %d in Recommitted", c)
		}
	}
	if len(res.Failed) != 0 {
		t.Errorf("Failed = %v, want none", res.Failed)
	}
	if s := sweepGitStatus(t, dir); s != "" {
		t.Fatalf("tree not clean after sweep (orphans not fully cleared):\n%s", s)
	}
}

// TestSweepOrphans_SkipsIncompleteOrMismatchedPairs is the edge-case
// regression for AC2's "skipping incomplete/mismatched pairs" clause: a lone
// cycle-N.json with no matching .md (a partial write, e.g. from a killed
// dossier writer) must be left alone, not force-committed as a half pair.
func TestSweepOrphans_SkipsIncompleteOrMismatchedPairs(t *testing.T) {
	dir := t.TempDir()
	initSweepRepo(t, dir)
	writePairFile(t, dir, "cycle-30.json", `{"cycle":30}`) // .md deliberately missing

	res, err := SweepOrphans(gitexec.Default(dir), io.Discard)
	if err != nil {
		t.Fatalf("SweepOrphans: %v", err)
	}
	if len(res.Recommitted) != 0 {
		t.Errorf("Recommitted = %v, want none (incomplete pair must not be committed)", res.Recommitted)
	}
	found := false
	for _, c := range res.Skipped {
		if c == 30 {
			found = true
		}
	}
	if !found {
		t.Errorf("Skipped = %v, want it to contain cycle 30", res.Skipped)
	}
	// The lone file must still be untracked afterward — proof SweepOrphans
	// never attempted git add/commit on the incomplete pair.
	if s := sweepGitStatus(t, dir); !strings.Contains(s, "cycle-30.json") {
		t.Errorf("cycle-30.json missing from git status (want it left untracked): %q", s)
	}
}

// TestSweepOrphans_LogsUnrecoverableFailureLoudly is the RED anchor for AC3:
// an unrecoverable per-pair commit failure must be logged loudly with the
// cycle number and underlying error (never silently swallowed), and the sweep
// must continue recommitting the REMAINING orphans rather than aborting the
// whole batch on one bad pair.
func TestSweepOrphans_LogsUnrecoverableFailureLoudly(t *testing.T) {
	dir := t.TempDir()
	initSweepRepo(t, dir)
	writePairFile(t, dir, "cycle-40.json", `{"cycle":40}`) // this pair's commit is forced to fail
	writePairFile(t, dir, "cycle-40.md", "# cycle 40")
	writePairFile(t, dir, "cycle-50.json", `{"cycle":50}`) // this pair must still succeed
	writePairFile(t, dir, "cycle-50.md", "# cycle 50")

	g := gitexec.Git{Dir: dir, Exec: (&interceptExec{blockBase: "cycle-40"}).run}
	var log bytes.Buffer
	res, err := SweepOrphans(g, &log)
	if err != nil {
		t.Fatalf("SweepOrphans: %v, want nil (a single unrecoverable pair must not abort the whole sweep)", err)
	}

	if _, ok := res.Failed[40]; !ok {
		t.Fatalf("Failed = %v, want an entry for cycle 40", res.Failed)
	}
	found50 := false
	for _, c := range res.Recommitted {
		if c == 50 {
			found50 = true
		}
	}
	if !found50 {
		t.Errorf("Recommitted = %v, want it to contain cycle 50 (sweep must continue past the cycle-40 failure)", res.Recommitted)
	}

	logged := log.String()
	if !strings.Contains(logged, "40") {
		t.Errorf("log output = %q, want it to name the failing cycle (40)", logged)
	}
	if !strings.Contains(strings.ToLower(logged), "forced unrecoverable failure") {
		t.Errorf("log output = %q, want it to carry the underlying git error, not swallow it", logged)
	}
}
