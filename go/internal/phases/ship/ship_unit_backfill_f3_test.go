package ship

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

func scriptedRunnerF3(handler func(args []string, stdout, stderr io.Writer) (int, error)) CmdRunner {
	return func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
		return handler(args, stdout, stderr)
	}
}

func hasArgF3(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestBoundCarryRecord_NamesWhyNoCarryCanBeBound(t *testing.T) {
	t.Parallel()
	t.Run("the audit binding names no artifact", func(t *testing.T) {
		t.Parallel()

		_, found, reason := boundCarryRecord(&Options{ProjectRoot: t.TempDir()})

		if found || reason != "the audit binding names no audit artifact" {
			t.Fatalf("found=%v reason=%q", found, reason)
		}
	})
	t.Run("the ledger cannot be read", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".evolve", "ledger.jsonl"), 0o755); err != nil {
			t.Fatal(err)
		}

		_, found, reason := boundCarryRecord(&Options{ProjectRoot: root, internalAuditArtifactSHA: "abc123"})

		if found || !strings.HasPrefix(reason, "the ledger is unreadable: ") {
			t.Fatalf("found=%v reason=%q", found, reason)
		}
	})
	t.Run("no carry names the artifact", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		mustWrite(t, filepath.Join(root, ".evolve", "ledger.jsonl"), "")

		_, found, reason := boundCarryRecord(&Options{ProjectRoot: root, internalAuditArtifactSHA: "abc123"})

		if found || reason != "no identical-rebase carry names audit abc123" {
			t.Fatalf("found=%v reason=%q", found, reason)
		}
	})
}

func TestVerifyCommitGateAttestation_ATreeShaThatCannotBeComputedIsAGitIOFault(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".commit-gate", "attestation.json"), `{"tree_state_sha":"reviewed","tool":"shasum"}`)
	opts := &Options{ProjectRoot: root, Runner: scriptedRunnerF3(func(args []string, _, _ io.Writer) (int, error) {
		if hasArgF3(args, "diff") {
			return -1, errors.New("git binary vanished")
		}
		return 0, nil
	})}
	res := &RunResult{Provenance: "audited"}

	err := verifyCommitGateAttestation(context.Background(), opts, res)

	se := wantShipErr(t, err, core.CodeGitIO, core.ShipClassTransient, "commit-gate tree SHA")
	if !strings.Contains(se.Message, "git binary vanished") || res.Provenance != "audited" || len(res.Logs) != 0 {
		t.Fatalf("message=%q provenance=%q logs=%v", se.Message, res.Provenance, res.Logs)
	}
}

func rollbackConsumerF3(t *testing.T, runner CmdRunner, prefix []string) (*itemConsumer, *RunResult) {
	t.Helper()
	res := &RunResult{}
	return &itemConsumer{ctx: context.Background(), opts: &Options{Runner: runner, ProjectRoot: t.TempDir()}, res: res, prefix: prefix}, res
}

func TestItemConsumerRollback_RestoresTheRootFileDropsTheRecordAndRestages(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var gotArgs []string
	c, res := rollbackConsumerF3(t, scriptedRunnerF3(func(args []string, _, _ io.Writer) (int, error) {
		gotArgs = args
		return 0, nil
	}), []string{"-C", "/wt"})
	m := consumedMove{id: "task-1", src: filepath.Join(dir, "task-1.json"), dstAbs: filepath.Join(dir, "consumed", "task-1.json"),
		srcRel: ".evolve/inbox/task-1.json", dstRel: ".evolve/inbox/consumed/task-1.json", raw: []byte(`{"id":"task-1"}`)}
	mustWrite(t, m.dstAbs, "the consumed record")

	c.rollback(m)

	if got, err := os.ReadFile(m.src); err != nil || string(got) != `{"id":"task-1"}` {
		t.Fatalf("root file = %q (%v), want the original bytes back", got, err)
	}
	if _, err := os.Stat(m.dstAbs); !os.IsNotExist(err) {
		t.Fatalf("the consumed record must be removed: %v", err)
	}
	if strings.Join(gotArgs, " ") != "-C /wt add -A -- .evolve/inbox/task-1.json .evolve/inbox/consumed/task-1.json" || len(res.Logs) != 0 {
		t.Fatalf("args=%v logs=%v", gotArgs, res.Logs)
	}
}

func TestItemConsumerRollback_EachFailedStepIsLoggedAndTheOthersStillRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	mustWrite(t, blocker, "a file")
	cases := []struct {
		name     string
		src      string
		runner   CmdRunner
		wantLogs []string
	}{
		{name: "the root file cannot be written", src: filepath.Join(blocker, "task.json"),
			runner:   scriptedRunnerF3(func([]string, io.Writer, io.Writer) (int, error) { return 0, nil }),
			wantLogs: []string{`rollback write "task-2"`}},
		{name: "the restage exits non-zero", src: filepath.Join(dir, "ok.json"),
			runner:   scriptedRunnerF3(func([]string, io.Writer, io.Writer) (int, error) { return 128, nil }),
			wantLogs: []string{`rollback restage "task-2" rc=128 err=<nil>`}},
		{name: "the restage cannot start", src: filepath.Join(dir, "ok2.json"),
			runner:   scriptedRunnerF3(func([]string, io.Writer, io.Writer) (int, error) { return -1, errors.New("no git") }),
			wantLogs: []string{`rollback restage "task-2" rc=-1 err=no git`}},
		{name: "both fail", src: filepath.Join(blocker, "both.json"),
			runner:   scriptedRunnerF3(func([]string, io.Writer, io.Writer) (int, error) { return 1, nil }),
			wantLogs: []string{`rollback write "task-2"`, `rollback restage "task-2" rc=1`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, res := rollbackConsumerF3(t, tc.runner, nil)

			c.rollback(consumedMove{id: "task-2", src: tc.src, dstAbs: filepath.Join(dir, "never-existed.json"), srcRel: "a", dstRel: "b", raw: []byte("x")})

			if len(res.Logs) != len(tc.wantLogs) {
				t.Fatalf("logs = %v, want %d lines", res.Logs, len(tc.wantLogs))
			}
			for i, want := range tc.wantLogs {
				if !strings.Contains(res.Logs[i], want) || !strings.HasPrefix(res.Logs[i], "[ship] WARN: unified_commitment ") {
					t.Fatalf("log %d = %q, want it to contain %q", i, res.Logs[i], want)
				}
			}
		})
	}
}

func diffTreeRunnerF3(stdout string, exit int, err error) (CmdRunner, *[]string) {
	var got []string
	return scriptedRunnerF3(func(args []string, out, _ io.Writer) (int, error) {
		got = args
		_, _ = io.WriteString(out, stdout)
		return exit, err
	}), &got
}

func TestTreeDriftExplainedByConsumption_OnlySanctionedPathsExplainTheDrift(t *testing.T) {
	t.Parallel()
	sanctioned := []string{".evolve/inbox/a.json", ".evolve/inbox/consumed/a.json"}
	many := func(n int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "stray/file-%02d.go\n", i)
		}
		return b.String()
	}
	cases := []struct {
		name       string
		consumed   []string
		out        string
		exit       int
		runErr     error
		gitDir     string
		wantOK     bool
		wantSuffix string
		wantRan    bool
	}{
		{name: "nothing was consumed so nothing is explained", consumed: nil, wantOK: false},
		{name: "every drifting path is sanctioned", consumed: sanctioned, out: ".evolve/inbox/a.json\n.evolve/inbox/consumed/a.json\n", wantOK: true, wantRan: true},
		{name: "blank lines are skipped", consumed: sanctioned, out: "\n.evolve/inbox/a.json\n\n   \n.evolve/inbox/consumed/a.json\n", wantOK: true, wantRan: true},
		{name: "an empty diff is explained", consumed: sanctioned, out: "", wantOK: true, wantRan: true},
		{name: "one unsanctioned path is named", consumed: sanctioned, out: ".evolve/inbox/a.json\nsrc/leak.go\n", wantSuffix: " (unsanctioned drift path(s): src/leak.go)", wantRan: true},
		{name: "git fails", consumed: sanctioned, out: "src/leak.go\n", exit: 128, wantRan: true},
		{name: "git cannot start", consumed: sanctioned, runErr: errors.New("no git"), exit: -1, wantRan: true},
		{name: "exactly the cap of unsanctioned paths names them all", consumed: sanctioned, out: many(8), wantSuffix: " (unsanctioned drift path(s): stray/file-00.go, stray/file-01.go, stray/file-02.go, stray/file-03.go, stray/file-04.go, stray/file-05.go, stray/file-06.go, stray/file-07.go)", wantRan: true},
		{name: "one past the cap says how many more", consumed: sanctioned, out: many(9), wantSuffix: ", stray/file-07.go and 1 more)", wantRan: true},
		{name: "a worktree directory is passed to git", consumed: sanctioned, out: ".evolve/inbox/a.json\n", gitDir: "/wt", wantOK: true, wantRan: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runner, args := diffTreeRunnerF3(tc.out, tc.exit, tc.runErr)
			opts := &Options{Runner: runner, internalConsumedPaths: tc.consumed}

			ok, suffix := treeDriftExplainedByConsumption(context.Background(), opts, tc.gitDir, "boundtree", "actualtree")

			if ok != tc.wantOK || (tc.wantSuffix != "" && !strings.HasSuffix(suffix, tc.wantSuffix)) || (tc.wantSuffix == "" && suffix != "") {
				t.Fatalf("ok=%v suffix=%q", ok, suffix)
			}
			if (len(*args) > 0) != tc.wantRan {
				t.Fatalf("git ran=%v, want %v (args %v)", len(*args) > 0, tc.wantRan, *args)
			}
			if tc.gitDir != "" && (*args)[0] != "-C" {
				t.Fatalf("args = %v, want -C %s first", *args, tc.gitDir)
			}
		})
	}
}

func TestExplicitAddFailure_ClassifiesTheRefusalAndBoundsItsStderrTail(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		exit      int
		stderr    string
		runErr    error
		wantClass core.ShipErrorClass
	}{
		{name: "invalid path is deterministic", exit: 128, stderr: "fatal: Invalid path 'x'", wantClass: core.ShipClassPrecondition},
		{name: "outside repository is deterministic", exit: 128, stderr: "fatal: /a/b is outside repository at /r", wantClass: core.ShipClassPrecondition},
		{name: "pathspec that matched nothing is deterministic", exit: 128, stderr: "fatal: pathspec 'z' did not match any files", wantClass: core.ShipClassPrecondition},
		{name: "a pathspec fatal that did match is not", exit: 128, stderr: "fatal: pathspec 'z' is ambiguous", wantClass: core.ShipClassTransient},
		{name: "gitignored paths are deterministic at exit 1", exit: 1, stderr: "The following paths are ignored by one of your .gitignore files:", wantClass: core.ShipClassPrecondition},
		{name: "the ignored message at another exit is not", exit: 128, stderr: "The following paths are ignored by one of your .gitignore files:", wantClass: core.ShipClassTransient},
		{name: "an invalid-path message at exit 1 is not", exit: 1, stderr: "fatal: Invalid path 'x'", wantClass: core.ShipClassTransient},
		{name: "an unexplained refusal is transient", exit: 128, stderr: "fatal: unable to create index.lock", runErr: errors.New("exit 128"), wantClass: core.ShipClassTransient},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := &explicitAdd{opts: &Options{}, exit: tc.exit, runErr: tc.runErr}
			a.errTail.WriteString(tc.stderr)

			err := a.failure("/wt", []string{"a", "b"})

			se := wantShipErr(t, err, core.CodeGitStageFailed, tc.wantClass, fmt.Sprintf("git add failed (rc=%d)", tc.exit))
			if se.Debug["worktree"] != "/wt" || se.Debug["git_rc"] != fmt.Sprintf("%d", tc.exit) {
				t.Fatalf("debug = %v", se.Debug)
			}
		})
	}
	t.Run("a tail past 300 bytes keeps its last 300 behind an ellipsis", func(t *testing.T) {
		t.Parallel()
		a := &explicitAdd{opts: &Options{}, exit: 128}
		a.errTail.WriteString(strings.Repeat("x", 100) + strings.Repeat("y", 300))

		err := a.failure("", nil)

		se := wantShipErr(t, err, core.CodeGitStageFailed, core.ShipClassTransient, "")
		if got := se.Debug["git_stderr"]; got != "…"+strings.Repeat("y", 300) {
			t.Fatalf("git_stderr = %q", got)
		}
	})
	t.Run("a tail of exactly 300 bytes is kept whole", func(t *testing.T) {
		t.Parallel()
		a := &explicitAdd{opts: &Options{}, exit: 128}
		a.errTail.WriteString(strings.Repeat("z", 300))

		se := wantShipErr(t, a.failure("", nil), core.CodeGitStageFailed, core.ShipClassTransient, "")

		if got := se.Debug["git_stderr"]; got != strings.Repeat("z", 300) {
			t.Fatalf("git_stderr = %q", got)
		}
	})
	t.Run("the same pathspec refused twice in a row is a precondition", func(t *testing.T) {
		t.Parallel()
		ws := t.TempDir()
		a := &explicitAdd{opts: &Options{WorkspacePath: ws}, exit: 128}
		a.errTail.WriteString("fatal: unable to create index.lock")

		first := wantShipErr(t, a.failure("", []string{"b", "a"}), core.CodeGitStageFailed, core.ShipClassTransient, "")
		second := wantShipErr(t, a.failure("", []string{"a", "b"}), core.CodeGitStageFailed, core.ShipClassPrecondition, "")

		if first == nil || second == nil {
			t.Fatal("both refusals carry a ship error")
		}
	})
}

func releaseRecorderF3(exit int, err error) (CmdRunner, *[][]string) {
	var calls [][]string
	return scriptedRunnerF3(func(args []string, _, _ io.Writer) (int, error) {
		calls = append(calls, append([]string(nil), args...))
		return exit, err
	}), &calls
}

func TestStageReleaseSet_StagesOnlyWhatExistsOnceEach(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "CHANGELOG.md"), "# changes\n")
	mustWrite(t, filepath.Join(root, "go", "evolve"), "binary")
	mustWrite(t, filepath.Join(root, "bin", "ship"), "ship binary")
	runner, calls := releaseRecorderF3(0, nil)
	opts := &Options{ProjectRoot: root, Runner: runner, ShipBinaryPath: filepath.Join(root, "bin", "ship")}

	if err := stageReleaseSet(context.Background(), opts); err != nil {
		t.Fatal(err)
	}

	if len(*calls) != 1 {
		t.Fatalf("calls = %v", *calls)
	}
	args := (*calls)[0]
	want := []string{"add", "--", "CHANGELOG.md", "go/evolve", "bin/ship"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestStageReleaseSet_ABinaryAlreadyInTheSetIsNotStagedTwice(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go", "evolve"), "binary")
	runner, calls := releaseRecorderF3(0, nil)
	opts := &Options{ProjectRoot: root, Runner: runner, ShipBinaryPath: filepath.Join(root, "go", "evolve")}

	if err := stageReleaseSet(context.Background(), opts); err != nil {
		t.Fatal(err)
	}

	if len(*calls) != 1 || strings.Join((*calls)[0], " ") != "add -- go/evolve" {
		t.Fatalf("calls = %v", *calls)
	}
}

func TestStageReleaseSet_NothingToStageMakesNoGitCall(t *testing.T) {
	t.Parallel()
	runner, calls := releaseRecorderF3(0, nil)
	opts := &Options{ProjectRoot: t.TempDir(), Runner: runner}

	if err := stageReleaseSet(context.Background(), opts); err != nil || len(*calls) != 0 {
		t.Fatalf("err=%v calls=%v", err, *calls)
	}
}

func TestStageReleaseSet_APathThatCannotBeRelativizedIsSkippedNotStaged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "CHANGELOG.md"), "# changes\n")
	mustWrite(t, filepath.Join(root, "go", "evolve"), "binary")
	runner, calls := releaseRecorderF3(0, nil)
	opts := &Options{ProjectRoot: root, Runner: runner, relPath: func(base, target string) (string, error) {
		if strings.HasSuffix(target, "CHANGELOG.md") {
			return "", errors.New("no relative path")
		}
		return filepath.Rel(base, target)
	}}

	if err := stageReleaseSet(context.Background(), opts); err != nil {
		t.Fatal(err)
	}

	if len(*calls) != 1 || strings.Join((*calls)[0], " ") != "add -- go/evolve" {
		t.Fatalf("calls = %v", *calls)
	}
}

func TestStageReleaseSet_AFailingAddIsATransientStageFault(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "CHANGELOG.md"), "# changes\n")
	for name, tc := range map[string]struct {
		exit int
		err  error
	}{"non-zero exit": {exit: 128}, "runner error": {exit: -1, err: errors.New("no git")}} {
		t.Run(name, func(t *testing.T) {
			runner, _ := releaseRecorderF3(tc.exit, tc.err)

			err := stageReleaseSet(context.Background(), &Options{ProjectRoot: root, Runner: runner})

			se := wantShipErr(t, err, core.CodeGitStageFailed, core.ShipClassTransient, "git add (release set) failed")
			if se.Debug["git_rc"] != fmt.Sprintf("%d", tc.exit) {
				t.Fatalf("debug = %v", se.Debug)
			}
		})
	}
}

func TestOptionsRel_UsesTheSeamWhenSetAndFilepathRelOtherwise(t *testing.T) {
	t.Parallel()
	got, err := (&Options{}).rel("/a", "/a/b/c")
	if err != nil || got != filepath.Join("b", "c") {
		t.Fatalf("default = %q %v", got, err)
	}
	seam := &Options{relPath: func(b, tg string) (string, error) { return b + "|" + tg, nil }}
	if got, _ := seam.rel("p", "q"); got != "p|q" {
		t.Fatalf("seam = %q", got)
	}
}

func packEventsF3(events ...string) io.Reader {
	return strings.NewReader(strings.Join(events, "\n") + "\n")
}

func TestClassifyPackEvents_StreamErrorsAndUnattributedBuildFailures(t *testing.T) {
	t.Parallel()
	t.Run("a line past the scanner buffer is reported on the tee", func(t *testing.T) {
		t.Parallel()
		var tee bytes.Buffer
		huge := `{"Action":"output","Output":"` + strings.Repeat("x", 4*1024*1024+16) + `"}` + "\n"

		failed, _ := classifyPackEvents(strings.NewReader(huge), &tee)

		if len(failed) != 0 || !strings.Contains(tee.String(), "event stream read error: "+bufio.ErrTooLong.Error()) {
			t.Fatalf("failed=%v tee tail=%q", failed, tee.String()[max(0, tee.Len()-120):])
		}
	})
	t.Run("a build failure with no package is still a RED", func(t *testing.T) {
		t.Parallel()

		failed, _ := classifyPackEvents(packEventsF3(`{"Action":"build-fail"}`), io.Discard)

		if len(failed) != 1 || failed[0].Package != "[build failed] (package unattributed)" || failed[0].Test != "" {
			t.Fatalf("failed = %+v", failed)
		}
	})
	t.Run("an unattributed failure is dropped when an attributed one already covers it", func(t *testing.T) {
		t.Parallel()

		failed, _ := classifyPackEvents(packEventsF3(`{"Action":"build-fail"}`, `{"Action":"output","Package":"x/p","Output":"FAIL x/p [build failed]\n"}`), io.Discard)

		if len(failed) != 1 || failed[0].Package != "x/p [build failed]" {
			t.Fatalf("failed = %+v", failed)
		}
	})
	t.Run("a setup failure names its package once however often it repeats", func(t *testing.T) {
		t.Parallel()
		ev := `{"Action":"output","Package":"x/q","Output":"FAIL x/q [setup failed]\n"}`

		failed, _ := classifyPackEvents(packEventsF3(ev, ev, `{"Action":"fail","Package":"x/r","Test":"TestA"}`), io.Discard)

		if len(failed) != 2 || failed[0] != (packFailure{Package: "x/r", Test: "TestA"}) || failed[1].Package != "x/q [build failed]" {
			t.Fatalf("failed = %+v", failed)
		}
	})
	t.Run("non-json lines are teed and skipped", func(t *testing.T) {
		t.Parallel()
		var tee bytes.Buffer

		failed, _ := classifyPackEvents(packEventsF3("not json at all", `{"Action":"fail","Package":"p","Test":"TestB"}`), &tee)

		if len(failed) != 1 || !strings.Contains(tee.String(), "not json at all\n") {
			t.Fatalf("failed=%v tee=%q", failed, tee.String())
		}
	})
}

func TestRunClassifiedPackRetrying_AmbiguousExitsAreRetriedOrDeclaredInfra(t *testing.T) {
	t.Parallel()
	ambiguous := packOutcome{err: errors.New("exit status 1")}
	red := packOutcome{err: errors.New("exit status 1"), failures: []packFailure{{Package: "p", Test: "TestX"}}}
	green := packOutcome{}
	sequence := func(outcomes ...packOutcome) (func() packOutcome, *int) {
		i := 0
		return func() packOutcome {
			o := outcomes[min(i, len(outcomes)-1)]
			i++
			return o
		}, &i
	}
	t.Run("not retried: an ambiguous exit is infra at once", func(t *testing.T) {
		t.Parallel()
		run, runs := sequence(ambiguous)
		var out bytes.Buffer

		err := runClassifiedPackRetrying(context.Background(), &out, "", "scanner pack", false, run, nil)

		se := wantShipErr(t, err, shiperr.CodeRepoContractInfra, core.ShipClassPrecondition, "not retried")
		if *runs != 1 || !strings.Contains(se.Message, "stderr only (no run workspace)") || out.Len() != 0 {
			t.Fatalf("runs=%d message=%q out=%q", *runs, se.Message, out.String())
		}
	})
	t.Run("not retried names the scan log of a real workspace", func(t *testing.T) {
		t.Parallel()
		run, _ := sequence(ambiguous)

		err := runClassifiedPackRetrying(context.Background(), io.Discard, "/ws/run", "scanner pack", false, run, nil)

		wantShipErr(t, err, shiperr.CodeRepoContractInfra, core.ShipClassPrecondition, filepath.Join("/ws/run", scanLogName))
	})
	t.Run("a red on the retry is the contract RED", func(t *testing.T) {
		t.Parallel()
		run, runs := sequence(ambiguous, red)
		var out bytes.Buffer

		err := runClassifiedPackRetrying(context.Background(), &out, "", "scanner pack", true, run, nil)

		wantShipErr(t, err, shiperr.CodeRepoContractGate, core.ShipClassPrecondition, "p.TestX")
		if *runs != 2 || !strings.Contains(out.String(), "retrying once") {
			t.Fatalf("runs=%d out=%q", *runs, out.String())
		}
	})
	t.Run("a green retry proceeds", func(t *testing.T) {
		t.Parallel()
		run, runs := sequence(ambiguous, green)
		var out bytes.Buffer

		err := runClassifiedPackRetrying(context.Background(), &out, "", "scanner pack", true, run, nil)

		if err != nil || *runs != 2 || !strings.Contains(out.String(), "retry GREEN") {
			t.Fatalf("err=%v runs=%d out=%q", err, *runs, out.String())
		}
	})
	t.Run("two ambiguous exits are infra with both causes", func(t *testing.T) {
		t.Parallel()
		run, runs := sequence(ambiguous, packOutcome{err: errors.New("second failure")})

		err := runClassifiedPackRetrying(context.Background(), io.Discard, "", "scanner pack", true, run, nil)

		se := wantShipErr(t, err, shiperr.CodeRepoContractInfra, core.ShipClassPrecondition, "exited nonzero TWICE")
		if *runs != 2 || !strings.Contains(se.Message, "second failure") {
			t.Fatalf("runs=%d message=%q", *runs, se.Message)
		}
	})
	t.Run("a first green never retries", func(t *testing.T) {
		t.Parallel()
		run, runs := sequence(green)

		if err := runClassifiedPackRetrying(context.Background(), io.Discard, "", "scanner pack", true, run, nil); err != nil || *runs != 1 {
			t.Fatalf("err=%v runs=%d", err, *runs)
		}
	})
}
