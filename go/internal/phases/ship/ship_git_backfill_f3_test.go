//go:build integration

package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func hookOnF3(sub string, hook func()) CmdRunner {
	var once sync.Once
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if hasArgF3(args, sub) {
			once.Do(hook)
		}
		return execRunner(ctx, name, dir, args, env, stdin, stdout, stderr)
	}
}

func failingGitF3(match func(args []string) bool) CmdRunner {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if match(args) {
			return -1, os.ErrClosed
		}
		return execRunner(ctx, name, dir, args, env, stdin, stdout, stderr)
	}
}

func TestVerifyAuditBinding_AReportThatVanishesBeforeItsAgeIsReadIsATransientIOFault(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")
	report := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	opts := auditOpts(t, repo)
	opts.Runner = hookOnF3("rev-parse", func() {
		if err := os.Remove(report); err != nil {
			t.Error(err)
		}
	})

	err := verifyAuditBinding(context.Background(), opts, &RunResult{})

	se := wantShipErr(t, err, core.CodeStateIO, core.ShipClassTransient, "stat audit-report.md")
	if se.Debug["artifact_path"] != report {
		t.Fatalf("debug = %v", se.Debug)
	}
}

func TestVerifyAuditBinding_AFailingCarryForwardProbeStopsTheBindingWithThatFault(t *testing.T) {
	s := trSetup(t)
	trAppendLedgerLine(t, s.repo, s.entry(t, s.patchID, trPassingGates()))
	opts := auditOpts(t, s.repo)
	opts.Runner = failingGitF3(func(args []string) bool { return len(args) == 2 && args[0] == "diff" && args[1] == "HEAD" })
	res := &RunResult{}

	err := verifyAuditBinding(context.Background(), opts, res)

	se := wantShipErr(t, err, core.CodeGitIO, core.ShipClassTransient, "git diff HEAD")
	if core.CodeAuditBindingHeadMoved == se.Code || strings.Contains(strings.Join(res.Logs, "\n"), "carry-forward — audit verdict follows") {
		t.Fatalf("the probe fault must surface as itself, not as a moved HEAD or a carried verdict: %v / %v", se, res.Logs)
	}
}

func dirtyWorktreeF3(t *testing.T) (repo, wt string) {
	t.Helper()
	repo = makeRepo(t)
	addRemote(t, repo)
	return repo, makeWorktree(t, repo, "cycle-dc")
}

func TestDetectColliders_ADirectoryIsExpandedToItsFilesAndUnreadableSubtreesAreSkipped(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root walks a mode-000 directory")
	}
	repo, wt := dirtyWorktreeF3(t)
	mustWrite(t, filepath.Join(wt, "dirx", "collide.txt"), "wt copy\n")
	mustMkdir(t, filepath.Join(wt, "dirx", "a-locked"))
	mustWrite(t, filepath.Join(repo, "dirx", "collide.txt"), "main copy\n")
	if err := os.Chmod(filepath.Join(wt, "dirx", "a-locked"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(wt, "dirx", "a-locked"), 0o755) })

	got, err := detectColliders(context.Background(), &Options{ProjectRoot: repo, Runner: execRunner}, wt, "main", "cycle-dc")

	if err != nil || strings.Join(got, ",") != "dirx/collide.txt" {
		t.Fatalf("colliders = %v err=%v, want only the readable colliding file", got, err)
	}
}

func TestDetectColliders_ADanglingSymlinkInsideAnIncomingDirectoryIsNotACollider(t *testing.T) {
	repo, wt := dirtyWorktreeF3(t)
	mustMkdir(t, filepath.Join(wt, "dirsl"))
	if err := os.Symlink("missing-target", filepath.Join(wt, "dirsl", "link")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	mustWrite(t, filepath.Join(repo, "dirsl", "link"), "an untracked main file at the same path\n")

	got, err := detectColliders(context.Background(), &Options{ProjectRoot: repo, Runner: execRunner}, wt, "main", "cycle-dc")

	if err != nil || len(got) != 0 {
		t.Fatalf("a link whose target cannot be stat-ed cannot be merged over: colliders=%v err=%v", got, err)
	}
}

func TestDetectColliders_AnIndexProbeThatFailsSkipsThatPathInsteadOfNamingItACollider(t *testing.T) {
	repo, wt := dirtyWorktreeF3(t)
	mustWrite(t, filepath.Join(wt, "probe.txt"), "wt copy\n")
	mustWrite(t, filepath.Join(repo, "probe.txt"), "main copy\n")
	opts := &Options{ProjectRoot: repo, Runner: execRunner}
	if got, err := detectColliders(context.Background(), opts, wt, "main", "cycle-dc"); err != nil || strings.Join(got, ",") != "probe.txt" {
		t.Fatalf("control: colliders=%v err=%v", got, err)
	}
	opts.Runner = failingGitF3(func(args []string) bool { return hasArgF3(args, "ls-files") })

	got, err := detectColliders(context.Background(), opts, wt, "main", "cycle-dc")

	if err != nil || len(got) != 0 {
		t.Fatalf("colliders=%v err=%v", got, err)
	}
}

type repairRigF3 struct {
	repo, wt string
	opts     *Options
	res      *RunResult
}

func newRepairRigF3(t *testing.T, files map[string][2]string) repairRigF3 {
	t.Helper()
	repo, wt := dirtyWorktreeF3(t)
	for p, contents := range files {
		mustWrite(t, filepath.Join(wt, p), contents[0])
		mustWrite(t, filepath.Join(repo, p), contents[1])
	}
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), `{"cycle_id":1,"active_worktree":"`+wt+`"}`)
	return repairRigF3{repo: repo, wt: wt, opts: &Options{ProjectRoot: repo, Runner: execRunner, NowFn: defaultNow}, res: &RunResult{}}
}

func collidersErrF3(t *testing.T, debugPaths string) *core.ShipError {
	t.Helper()
	kv := []string{}
	if debugPaths != "" {
		kv = append(kv, "colliders", debugPaths)
	}
	return mustShipErr(t, shipErr(core.CodeGitFFMergeDiverged, core.ShipClassPrecondition, core.StageAtomicShip, "ff merge diverged", kv...))
}

func TestRepairColliders_EveryUnrepairableStateDeclinesWithoutTouchingAnyFile(t *testing.T) {
	cases := []struct {
		name    string
		debug   string
		prepare func(t *testing.T, r repairRigF3)
	}{
		{name: "the failure names no colliders", debug: "", prepare: func(*testing.T, repairRigF3) {}},
		{name: "no active worktree is recorded", debug: "x", prepare: func(t *testing.T, r repairRigF3) {
			mustWrite(t, filepath.Join(r.repo, ".evolve", "cycle-state.json"), `{"cycle_id":1}`)
		}},
		{name: "the main checkout has a detached HEAD", debug: "x", prepare: func(t *testing.T, r repairRigF3) { runGit(t, r.repo, "checkout", "-q", "--detach") }},
		{name: "the worktree has a detached HEAD", debug: "x", prepare: func(t *testing.T, r repairRigF3) { runGit(t, r.wt, "checkout", "-q", "--detach") }},
		{name: "the worktree path is not a repository", debug: "x", prepare: func(t *testing.T, r repairRigF3) {
			mustWrite(t, filepath.Join(r.repo, ".evolve", "cycle-state.json"), `{"cycle_id":1,"active_worktree":"`+t.TempDir()+`"}`)
		}},
		{name: "cycle-state becomes unreadable while colliders are detected", debug: "x", prepare: func(t *testing.T, r repairRigF3) {
			r.opts.Runner = hookOnF3("ls-files", func() {
				mustWrite(t, filepath.Join(r.repo, ".evolve", "cycle-state.json"), "{torn")
			})
		}},
		{name: "cycle-state loses its cycle id while colliders are detected", debug: "x", prepare: func(t *testing.T, r repairRigF3) {
			r.opts.Runner = hookOnF3("ls-files", func() {
				mustWrite(t, filepath.Join(r.repo, ".evolve", "cycle-state.json"), `{"active_worktree":"`+r.wt+`"}`)
			})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRepairRigF3(t, map[string][2]string{"out/a.txt": {"wt\n", "main\n"}})
			tc.prepare(t, r)

			got := repairColliders(context.Background(), r.opts, r.res, collidersErrF3(t, tc.debug))

			if got != repairNone || r.res.RepairOutcome != "" {
				t.Fatalf("outcome=%v repairOutcome=%q logs=%v", got, r.res.RepairOutcome, r.res.Logs)
			}
			if body, err := os.ReadFile(filepath.Join(r.repo, "out", "a.txt")); err != nil || string(body) != "main\n" {
				t.Fatalf("a declined repair must leave the main file alone: %q %v", body, err)
			}
			if _, err := os.Stat(filepath.Join(r.repo, ".evolve", "quarantine")); !os.IsNotExist(err) {
				t.Fatalf("nothing may be quarantined by a declined repair: %v", err)
			}
		})
	}
}

func TestRepairColliders_WorktreeWithNoCollidingFileDeclines(t *testing.T) {
	r := newRepairRigF3(t, nil)
	mustWrite(t, filepath.Join(r.wt, "only-in-worktree.txt"), "x\n")

	got := repairColliders(context.Background(), r.opts, r.res, collidersErrF3(t, "x"))

	if got != repairNone {
		t.Fatalf("outcome = %v", got)
	}
}

func TestRepairColliders_AnUnprovableColliderDeclinesBeforeAnyMutation(t *testing.T) {
	r := newRepairRigF3(t, map[string][2]string{"out/a-first.txt": {"same\n", "same\n"}, "out/z-second.txt": {"wt\n", "main\n"}})
	r.opts.Runner = hookOnF3("ls-files", func() {
		second := filepath.Join(r.repo, "out", "z-second.txt")
		if err := os.Remove(second); err != nil {
			t.Error(err)
		}
		mustWrite(t, filepath.Join(second, "child"), "now a directory\n")
	})

	got := repairColliders(context.Background(), r.opts, r.res, collidersErrF3(t, "x"))

	if got != repairNone {
		t.Fatalf("outcome = %v", got)
	}
	if body, err := os.ReadFile(filepath.Join(r.repo, "out", "a-first.txt")); err != nil || string(body) != "same\n" {
		t.Fatalf("the identical collider ahead of the unprovable one must not be removed first: %q %v", body, err)
	}
}

func TestRepairColliders_AnExecutionFaultAbortsLoudlyAndNeverReRunsTheStage(t *testing.T) {
	t.Run("an identical collider that cannot be removed", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root removes files from a read-only directory")
		}
		r := newRepairRigF3(t, map[string][2]string{"locked/same.txt": {"same\n", "same\n"}})
		lockedDir := filepath.Join(r.repo, "locked")
		if err := os.Chmod(lockedDir, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(lockedDir, 0o755) })

		got := repairColliders(context.Background(), r.opts, r.res, collidersErrF3(t, "x"))

		if got != repairNone || !containsLog(*r.res, "collider repair aborted at locked/same.txt") || containsLog(*r.res, "collider heal") {
			t.Fatalf("outcome=%v logs=%v", got, r.res.Logs)
		}
	})
	t.Run("the quarantine directory cannot be created", func(t *testing.T) {
		r := newRepairRigF3(t, map[string][2]string{"out/a.txt": {"wt\n", "main\n"}})
		mustWrite(t, filepath.Join(r.repo, ".evolve", "quarantine"), "a file where the directory belongs")

		got := repairColliders(context.Background(), r.opts, r.res, collidersErrF3(t, "x"))

		if got != repairNone || !containsLog(*r.res, "collider repair aborted at out/a.txt (mkdir ") {
			t.Fatalf("the abort must name the failed directory creation, not a later rename: outcome=%v logs=%v", got, r.res.Logs)
		}
		if body, err := os.ReadFile(filepath.Join(r.repo, "out", "a.txt")); err != nil || string(body) != "main\n" {
			t.Fatalf("the differing copy must survive an aborted quarantine: %q %v", body, err)
		}
	})
	t.Run("the quarantine destination is occupied", func(t *testing.T) {
		r := newRepairRigF3(t, map[string][2]string{"out/a.txt": {"wt\n", "main\n"}})
		mustWrite(t, filepath.Join(r.repo, ".evolve", "quarantine", "cycle-1", "out", "a.txt", "squatter"), "x")

		got := repairColliders(context.Background(), r.opts, r.res, collidersErrF3(t, "x"))

		if got != repairNone || !containsLog(*r.res, "collider repair aborted at out/a.txt") {
			t.Fatalf("outcome=%v logs=%v", got, r.res.Logs)
		}
		if body, err := os.ReadFile(filepath.Join(r.repo, "out", "a.txt")); err != nil || string(body) != "main\n" {
			t.Fatalf("the differing copy must survive a refused rename: %q %v", body, err)
		}
	})
}

func TestRepairColliders_AManifestThatCannotBeWrittenIsAWarningAndTheHealStillProceeds(t *testing.T) {
	r := newRepairRigF3(t, map[string][2]string{"out/a.txt": {"wt\n", "main\n"}})
	mustMkdir(t, filepath.Join(r.repo, ".evolve", "quarantine", "cycle-1", "manifest.json"))

	got := repairColliders(context.Background(), r.opts, r.res, collidersErrF3(t, "x"))

	if got != repairRetryStage || !containsLog(*r.res, "quarantine manifest write failed") || r.res.RepairOutcome != "colliders-healed:0-identical-removed,1-quarantined" {
		t.Fatalf("outcome=%v repairOutcome=%q logs=%v", got, r.res.RepairOutcome, r.res.Logs)
	}
	if body, err := os.ReadFile(filepath.Join(r.repo, ".evolve", "quarantine", "cycle-1", "out", "a.txt")); err != nil || string(body) != "main\n" {
		t.Fatalf("the quarantined copy keeps the main bytes: %q %v", body, err)
	}
}
