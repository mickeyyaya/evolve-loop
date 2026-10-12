package ship

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func failingExecutableCoverfix(t *testing.T) {
	t.Helper()
	prior := osExecutable
	osExecutable = func() (string, error) { return "", errors.New("no executable path") }
	t.Cleanup(func() { osExecutable = prior })
}

func TestLandingTreeCoverfix_ATypedWorktreeThatIsAFileIsRefused(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	notADir := filepath.Join(t.TempDir(), "worktree-is-a-file")
	mustWrite(t, notADir, "x")
	opts := &Options{Class: ClassCycle, ProjectRoot: root, ActiveWorktree: notADir}

	tree, fromWorktree, err := landingTree(opts)

	se := wantShipErr(t, err, core.CodeWorktreeResolve, core.ShipClassPrecondition, "is unavailable: not a directory")
	if tree != "" || fromWorktree || se.Debug["worktree"] != notADir {
		t.Fatalf("tree=%q fromWorktree=%v debug=%v", tree, fromWorktree, se.Debug)
	}
}

func TestLandingTreeCoverfix_AnUntypedMissingWorktreeFallsBackToTheProjectRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	missing := filepath.Join(t.TempDir(), "gone")
	mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"active_worktree":"`+missing+`"}`)
	opts := &Options{Class: ClassCycle, ProjectRoot: root}

	tree, fromWorktree, err := landingTree(opts)

	if err != nil || tree != root || fromWorktree {
		t.Fatalf("tree=%q fromWorktree=%v err=%v; want the project root, not the vanished %s", tree, fromWorktree, err, missing)
	}
}

func TestIsLandedCoverfix_RefusesWhatItCannotPlaceOnTheBranch(t *testing.T) {
	t.Parallel()
	t.Run("a blank sha is never landed and git is never asked", func(t *testing.T) {
		t.Parallel()
		asked := false
		opts := &Options{Runner: scriptedRunnerF3(func([]string, io.Writer, io.Writer) (int, error) {
			asked = true
			return 0, nil
		})}

		if isLanded(context.Background(), opts, "  \n") || asked {
			t.Fatalf("blank sha landed or git consulted (asked=%v)", asked)
		}
	})
	t.Run("a detached head cannot be checked against origin", func(t *testing.T) {
		t.Parallel()
		var originAsked bool
		opts := &Options{Runner: scriptedRunnerF3(func(args []string, _, _ io.Writer) (int, error) {
			if hasArgF3(args, "symbolic-ref") {
				return 1, nil
			}
			if hasArgF3(args, "origin/main") {
				originAsked = true
			}
			return 1, nil
		})}

		if isLanded(context.Background(), opts, "abc123") || originAsked {
			t.Fatalf("detached head must be not landed without an origin lookup (originAsked=%v)", originAsked)
		}
	})
	t.Run("a sha only on the origin branch is landed", func(t *testing.T) {
		t.Parallel()
		opts := &Options{Runner: scriptedRunnerF3(func(args []string, stdout, _ io.Writer) (int, error) {
			switch {
			case hasArgF3(args, "symbolic-ref"):
				_, _ = io.WriteString(stdout, "main\n")
				return 0, nil
			case hasArgF3(args, "origin/main"):
				return 0, nil
			}
			return 1, nil
		})}

		if !isLanded(context.Background(), opts, "abc123") {
			t.Fatal("a sha reachable from origin/main must count as landed")
		}
	})
}

func TestIsSyncMainMergeCoverfix_AFailedParentLookupIsNotASyncMerge(t *testing.T) {
	t.Parallel()
	mergeBaseAsked := false
	opts := &Options{Runner: scriptedRunnerF3(func(args []string, _, _ io.Writer) (int, error) {
		if hasArgF3(args, "merge-base") {
			mergeBaseAsked = true
			return 0, nil
		}
		return 128, errors.New("rev-list exploded")
	})}

	if isSyncMainMerge(context.Background(), opts, "deadbeef", "main") || mergeBaseAsked {
		t.Fatalf("an unreadable commit must not be a sync merge (mergeBaseAsked=%v)", mergeBaseAsked)
	}
}

func TestRepinPostCycleCoverfix_AnUnknownRunningBinaryLeavesThePinUntouched(t *testing.T) {
	failingExecutableCoverfix(t)
	root := t.TempDir()
	statePath := filepath.Join(root, ".evolve", "state.json")
	mustWrite(t, statePath, `{"expected_ship_sha":"pinned"}`)
	res := &RunResult{}

	err := repinPostCycle(&Options{ProjectRoot: root}, res)

	st, readErr := readStateMap(statePath)
	if err != nil || readErr != nil || stateString(st, "expected_ship_sha") != "pinned" || len(res.Logs) != 0 {
		t.Fatalf("err=%v readErr=%v state=%v logs=%v", err, readErr, st, res.Logs)
	}
}

func TestRepinPostCycleCoverfix_AnUnreadableStateIsReportedNotOverwritten(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	bin := filepath.Join(root, "ship-bin")
	mustWrite(t, bin, "binary\n")
	statePath := filepath.Join(root, ".evolve", "state.json")
	mustWrite(t, statePath, "{not json")
	res := &RunResult{}
	opts := &Options{ProjectRoot: root, ShipBinaryPath: bin, Runner: scriptedRunnerF3(func([]string, io.Writer, io.Writer) (int, error) {
		return 128, nil
	})}

	err := repinPostCycle(opts, res)

	body, _ := os.ReadFile(statePath)
	if err == nil || string(body) != "{not json" || len(res.Logs) != 0 {
		t.Fatalf("err=%v state=%q logs=%v", err, body, res.Logs)
	}
}

func TestRepinPostCycleCoverfix_AFailedPinWriteIsReportedAndNotLogged(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not bind root")
	}
	root := t.TempDir()
	bin := filepath.Join(root, "ship-bin")
	mustWrite(t, bin, "binary\n")
	stateDir := filepath.Join(root, ".evolve")
	statePath := filepath.Join(stateDir, "state.json")
	mustWrite(t, statePath, `{"expected_ship_sha":"stale"}`)
	mustWrite(t, statePath+".lock", "")
	if err := os.Chmod(stateDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(stateDir, 0o755) })
	res := &RunResult{}
	opts := &Options{ProjectRoot: root, ShipBinaryPath: bin, Runner: scriptedRunnerF3(func([]string, io.Writer, io.Writer) (int, error) {
		return 128, nil
	})}

	err := repinPostCycle(opts, res)

	st, _ := readStateMap(statePath)
	if err == nil || stateString(st, "expected_ship_sha") != "stale" || len(res.Logs) != 0 {
		t.Fatalf("err=%v state=%v logs=%v", err, st, res.Logs)
	}
}

func TestAttemptRepairCoverfix_ADryRunNeverRepairs(t *testing.T) {
	t.Parallel()
	calls := 0
	opts := &Options{DryRun: true, ProjectRoot: t.TempDir(), Runner: scriptedRunnerF3(func([]string, io.Writer, io.Writer) (int, error) {
		calls++
		return 0, nil
	})}
	res := &RunResult{}
	cause := shipErr(core.CodeSelfSHATampered, core.ShipClassIntegrity, core.StageAtomicShip, "tampered")

	out := attemptRepair(context.Background(), opts, res, cause)

	if out != repairNone || calls != 0 || res.RepairAttempted != "" || len(res.Logs) != 0 {
		t.Fatalf("out=%v calls=%d attempted=%q logs=%v", out, calls, res.RepairAttempted, res.Logs)
	}
}

func repairPinOptionsCoverfix(t *testing.T, root, bin, committed string) *Options {
	t.Helper()
	return &Options{ProjectRoot: root, ShipBinaryPath: bin, Runner: scriptedRunnerF3(func(args []string, stdout, _ io.Writer) (int, error) {
		if hasArgF3(args, "show") {
			_, _ = io.WriteString(stdout, committed)
			return 0, nil
		}
		return 1, nil
	})}
}

func TestRepairSelfSHAPinCoverfix_AnUnlocatableExecutableDeclines(t *testing.T) {
	failingExecutableCoverfix(t)

	out := repairSelfSHAPin(context.Background(), &Options{ProjectRoot: t.TempDir()}, &RunResult{}, nil)

	if out != repairNone {
		t.Fatalf("out=%v, want repairNone", out)
	}
}

func TestRepairSelfSHAPinCoverfix_EveryUnverifiableBinaryDeclinesWithoutTouchingThePin(t *testing.T) {
	t.Parallel()
	t.Run("the binary cannot be hashed", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		opts := repairPinOptionsCoverfix(t, root, filepath.Join(root, "absent-bin"), "x")

		if out := repairSelfSHAPin(context.Background(), opts, &RunResult{}, nil); out != repairNone {
			t.Fatalf("out=%v, want repairNone", out)
		}
	})
	t.Run("a binary outside the project root has no committed counterpart", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		outside := filepath.Join(t.TempDir(), "bin")
		mustWrite(t, outside, "binary\n")
		opts := repairPinOptionsCoverfix(t, root, outside, "binary\n")

		if out := repairSelfSHAPin(context.Background(), opts, &RunResult{}, nil); out != repairNone {
			t.Fatalf("out=%v, want repairNone", out)
		}
	})
	t.Run("a verified binary whose state cannot be read is not re-pinned", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		bin := filepath.Join(root, "bin")
		mustWrite(t, bin, "binary\n")
		statePath := filepath.Join(root, ".evolve", "state.json")
		mustWrite(t, statePath, "{not json")
		res := &RunResult{}

		out := repairSelfSHAPin(context.Background(), repairPinOptionsCoverfix(t, root, bin, "binary\n"), res, nil)

		body, _ := os.ReadFile(statePath)
		if out != repairNone || string(body) != "{not json" || res.RepairOutcome != "" {
			t.Fatalf("out=%v state=%q outcome=%q", out, body, res.RepairOutcome)
		}
	})
}

func readManifestCoverfix(t *testing.T, dir string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var entries []map[string]any
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("manifest is not json: %v\n%s", err, raw)
	}
	return entries
}

func TestAppendQuarantineManifestCoverfix_AppendsToWhatIsAlreadyQuarantined(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "manifest.json"), `[{"path":"old.txt","reason":"differing-untracked-collider","cycle":3,"ts":"2026-01-01T00:00:00Z"}]`)

	err := appendQuarantineManifest(&Options{}, dir, 7, []string{"new.txt"})

	entries := readManifestCoverfix(t, dir)
	if err != nil || len(entries) != 2 || entries[0]["path"] != "old.txt" || entries[1]["path"] != "new.txt" || entries[1]["cycle"] != float64(7) {
		t.Fatalf("err=%v entries=%v", err, entries)
	}
	if _, parseErr := time.Parse(time.RFC3339, entries[1]["ts"].(string)); parseErr != nil {
		t.Fatalf("the default clock must stamp an RFC3339 time: %v", parseErr)
	}
}

func TestAppendQuarantineManifestCoverfix_ACorruptManifestIsReplacedNotFatal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "manifest.json"), "garbage")
	opts := &Options{NowFn: func() Now { return Now{RFC3339: "2026-02-02T00:00:00Z"} }}

	err := appendQuarantineManifest(opts, dir, 1, []string{"a.txt"})

	entries := readManifestCoverfix(t, dir)
	if err != nil || len(entries) != 1 || entries[0]["ts"] != "2026-02-02T00:00:00Z" {
		t.Fatalf("err=%v entries=%v", err, entries)
	}
}

func TestAppendQuarantineManifestCoverfix_AnUnwritableDirectoryFailsLoudly(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "missing")

	err := appendQuarantineManifest(&Options{}, dir, 1, []string{"a.txt"})

	if err == nil {
		t.Fatal("writing the manifest into a missing directory must fail")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "manifest.json")); !os.IsNotExist(statErr) {
		t.Fatalf("no manifest may exist after a failed write: %v", statErr)
	}
}

func TestAppendQuarantineManifestCoverfix_AnEncodingFailureLeavesNoManifest(t *testing.T) {
	prior := marshalIndentJSON
	marshalIndentJSON = func(any, string, string) ([]byte, error) { return nil, errors.New("encode refused") }
	t.Cleanup(func() { marshalIndentJSON = prior })
	dir := t.TempDir()

	err := appendQuarantineManifest(&Options{}, dir, 1, []string{"a.txt"})

	entries, _ := os.ReadDir(dir)
	if err == nil || !strings.Contains(err.Error(), "encode refused") || len(entries) != 0 {
		t.Fatalf("err=%v leftovers=%v", err, entries)
	}
}

func stagedRunnerCoverfix() CmdRunner {
	return scriptedRunnerF3(func(args []string, stdout, _ io.Writer) (int, error) {
		if hasArgF3(args, "--quiet") {
			return 1, nil
		}
		return 0, nil
	})
}

func TestVerifyManualConfirmCoverfix_ConfirmationFollowsWhatTheOperatorTypes(t *testing.T) {
	t.Parallel()
	t.Run("yes ships and records the interactive provenance", func(t *testing.T) {
		t.Parallel()
		master, slave := openPTYForCoverfix(t)
		if _, err := master.WriteString("yes\n"); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		res := &RunResult{}
		opts := &Options{ProjectRoot: t.TempDir(), Stdin: slave, Stderr: &stderr, Runner: stagedRunnerCoverfix(), Env: map[string]string{}}

		err := verifyManualConfirm(context.Background(), opts, res)

		if err != nil || res.Provenance != "manual (interactive-confirmed)" || !strings.Contains(stderr.String(), "Confirm manual commit?") {
			t.Fatalf("err=%v provenance=%q stderr=%q", err, res.Provenance, stderr.String())
		}
	})
	t.Run("anything but exactly yes aborts", func(t *testing.T) {
		t.Parallel()
		master, slave := openPTYForCoverfix(t)
		if _, err := master.WriteString("yes please\n"); err != nil {
			t.Fatal(err)
		}
		res := &RunResult{}
		opts := &Options{ProjectRoot: t.TempDir(), Stdin: slave, Stderr: io.Discard, Runner: stagedRunnerCoverfix(), Env: map[string]string{}}

		err := verifyManualConfirm(context.Background(), opts, res)

		wantShipErr(t, err, core.CodeManualDeclined, core.ShipClassConfig, "manual confirmation declined")
		if res.Provenance != "" || !anyContains(res.Logs, "declined — aborting") {
			t.Fatalf("provenance=%q logs=%v", res.Provenance, res.Logs)
		}
	})
	t.Run("a terminal that closes before answering aborts as a read failure", func(t *testing.T) {
		t.Parallel()
		devNull, err := os.Open(os.DevNull)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = devNull.Close() })
		res := &RunResult{}
		opts := &Options{ProjectRoot: t.TempDir(), Stdin: devNull, Stderr: io.Discard, Runner: stagedRunnerCoverfix(), Env: map[string]string{}}

		got := verifyManualConfirm(context.Background(), opts, res)

		wantShipErr(t, got, core.CodeManualDeclined, core.ShipClassConfig, "manual confirmation read failed")
		if res.Provenance != "" {
			t.Fatalf("provenance=%q", res.Provenance)
		}
	})
}
