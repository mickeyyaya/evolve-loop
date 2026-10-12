package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestBlockerBreakerHalt_AnUnreadableFailurePolicyFallsBackToTheDefaults(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeE2File(t, filepath.Join(evolveDir, "policy.json"), `{"failure_policy":{"categories":{"made-up":{"level":"cosmic","action":"halt-and-diagnose"}}}}`)
	var stderr bytes.Buffer

	rc, halted := blockerBreakerHalt(evolveDir, root, 0, &stderr, nil)

	if halted || rc != 0 {
		t.Fatalf("rc=%d halted=%v, want an empty batch to continue under the defaults", rc, halted)
	}
	want := `[loop] WARN: blocker-breaker: failure policy unreadable (policy: failure_policy category "made-up" has unknown level "cosmic" (want system|task)) — using compiled defaults`
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr=%q, want %q", stderr.String(), want)
	}
}

func TestStartDetachedLoop_EachLaunchFailureIsNamed(t *testing.T) {
	cases := []struct {
		name    string
		stub    func(t *testing.T)
		wantErr string
	}{
		{
			name: "an unstatable log",
			stub: func(t *testing.T) {
				previous := detachLogStat
				detachLogStat = func(*os.File) (os.FileInfo, error) { return nil, errors.New("stat refused") }
				t.Cleanup(func() { detachLogStat = previous })
			},
			wantErr: "stat log ",
		},
		{
			name: "an unresolvable command",
			stub: func(t *testing.T) {
				previous := loopDetachCommandFn
				loopDetachCommandFn = func([]string) (*exec.Cmd, error) { return nil, errors.New("no self") }
				t.Cleanup(func() { loopDetachCommandFn = previous })
			},
			wantErr: "start: no self",
		},
		{
			name: "an unstartable command",
			stub: func(t *testing.T) {
				previous := loopDetachCommandFn
				missing := filepath.Join(t.TempDir(), "absent-binary")
				loopDetachCommandFn = func([]string) (*exec.Cmd, error) { return exec.Command(missing), nil }
				t.Cleanup(func() { loopDetachCommandFn = previous })
			},
			wantErr: "start: fork/exec ",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.stub(t)
			logPath := filepath.Join(t.TempDir(), "loop.log")

			launch, err := startDetachedLoop(loopConfig{LogPath: logPath}, nil)

			if err == nil || !strings.HasPrefix(err.Error(), tc.wantErr) {
				t.Fatalf("err=%v, want prefix %q", err, tc.wantErr)
			}
			if launch.pid != 0 || launch.done != nil {
				t.Fatalf("a failed launch returned %+v", launch)
			}
		})
	}
}

func TestStartDetachedLoop_TheOffsetIsTheLogsSizeBeforeTheLaunch(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "loop.log")
	writeE2File(t, logPath, "earlier run\n")
	previous := loopDetachCommandFn
	loopDetachCommandFn = func([]string) (*exec.Cmd, error) { return exec.Command("/bin/sh", "-c", "echo booted"), nil }
	t.Cleanup(func() { loopDetachCommandFn = previous })

	launch, err := startDetachedLoop(loopConfig{LogPath: logPath}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if werr := <-launch.done; werr != nil {
		t.Fatalf("child: %v", werr)
	}

	if launch.offset != int64(len("earlier run\n")) || launch.pid <= 0 {
		t.Fatalf("launch=%+v, want offset 12 and a pid", launch)
	}
	if got := tailLogSince(logPath, launch.offset, 5); len(got) != 1 || got[0] != "booted" {
		t.Fatalf("tail since the offset = %q, want only this launch's line", got)
	}
}

func TestDetachBootWait_AMalformedPolicyWaitsTheDefault(t *testing.T) {
	evolveDir := t.TempDir()
	writeE2File(t, filepath.Join(evolveDir, "policy.json"), "{")
	var stderr bytes.Buffer

	got := detachBootWait(evolveDir, &stderr)

	if got != policy.DefaultBootDetachWait || !strings.HasPrefix(stderr.String(), "evolve loop: WARN: --detach: ") || !strings.Contains(stderr.String(), "waiting the default "+policy.DefaultBootDetachWait.String()) {
		t.Fatalf("got %v stderr=%q", got, stderr.String())
	}
}

func TestDetachBootWait_AnAbsentPolicyIsTheDefaultWithoutAWarning(t *testing.T) {
	var stderr bytes.Buffer

	if got := detachBootWait(t.TempDir(), &stderr); got != policy.DefaultBootDetachWait || stderr.Len() != 0 {
		t.Fatalf("got %v stderr=%q", got, stderr.String())
	}
}

func TestAwaitDetachedBoot_AChildThatExitsBeforeTheLeaseFailsWithItsExitCode(t *testing.T) {
	previous := loopDetachPoll
	loopDetachPoll = 5 * time.Millisecond
	t.Cleanup(func() { loopDetachPoll = previous })
	logPath := filepath.Join(t.TempDir(), "loop.log")
	writeE2File(t, logPath, "old\nboom\n")
	done := make(chan error, 1)
	done <- exec.Command("/bin/sh", "-c", "exit 7").Run()
	var stdout, stderr bytes.Buffer

	code := awaitDetachedBoot(loopConfig{EvolveDir: t.TempDir(), LogPath: logPath}, detachedLaunch{pid: 4242, offset: 4, done: done}, time.Minute, &stdout, &stderr)

	want := "evolve loop: --detach: the loop exited during boot (exit 7); last lines of " + logPath + ":\n  boom\n"
	if code != 1 || stderr.String() != want || stdout.Len() != 0 {
		t.Fatalf("code=%d stderr=%q, want %q", code, stderr.String(), want)
	}
}

func TestTailLogSince_ReportsAnUnreadableLogInPlaceOfItsLines(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "loop.log")
	writeE2File(t, file, "a\n\n  \nb\nc\n")
	cases := []struct {
		name   string
		path   string
		offset int64
		n      int
		want   []string
		prefix string
	}{
		{name: "the last n non-blank lines", path: file, n: 2, want: []string{"b", "c"}},
		{name: "fewer lines than n", path: file, n: 10, want: []string{"a", "b", "c"}},
		{name: "from an offset", path: file, offset: 2, n: 10, want: []string{"b", "c"}},
		{name: "an offset past the end", path: file, offset: 999, n: 10, want: nil},
		{name: "a missing log", path: filepath.Join(dir, "absent.log"), n: 3, prefix: "(log unreadable: open "},
		{name: "a negative offset", path: file, offset: -1, n: 3, prefix: "(log unreadable: seek "},
		{name: "a directory", path: dir, n: 3, prefix: "(log unreadable: read "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tailLogSince(tc.path, tc.offset, tc.n)

			if tc.prefix != "" {
				if len(got) != 1 || !strings.HasPrefix(got[0], tc.prefix) {
					t.Fatalf("got %q, want one line with prefix %q", got, tc.prefix)
				}
				return
			}
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestApplyEscalationBoundary_FailuresWarnAndApplyNothing(t *testing.T) {
	t.Run("a malformed policy", func(t *testing.T) {
		evolveDir := t.TempDir()
		writeE2File(t, filepath.Join(evolveDir, "policy.json"), "{")
		var stderr bytes.Buffer
		applyEscalationBoundary(evolveDir, 3, &stderr, nil)
		if !strings.HasPrefix(stderr.String(), "[loop] WARN: escalation boundary: policy load: ") {
			t.Fatalf("stderr=%q", stderr.String())
		}
		if _, err := os.Stat(filepath.Join(evolveDir, "escalation-apply-report.json")); !os.IsNotExist(err) {
			t.Fatalf("a policy failure wrote a report: %v", err)
		}
	})
	t.Run("unreadable intents", func(t *testing.T) {
		evolveDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(evolveDir, "escalations", "pending-actions.jsonl"), 0o755); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		applyEscalationBoundary(evolveDir, 3, &stderr, nil)
		if !strings.HasPrefix(stderr.String(), "[loop] WARN: escalation boundary: ") || strings.Contains(stderr.String(), "policy load") {
			t.Fatalf("stderr=%q", stderr.String())
		}
	})
}

func TestApplyEscalationBoundary_NothingStagedIsSilent(t *testing.T) {
	evolveDir := t.TempDir()
	var stderr bytes.Buffer

	applyEscalationBoundary(evolveDir, 1, &stderr, nil)

	if stderr.Len() != 0 {
		t.Fatalf("stderr=%q, want silence", stderr.String())
	}
}

func TestWritePipelineEscalation_BothWritesFailLoudlyAndTheRecordStillNamesThePaths(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, "evolve-as-file")
	writeE2File(t, evolveDir, "x")
	writeE2File(t, filepath.Join(root, ".evolve", "inbox"), "x")
	sf := &cyclestate.SystemFailureSignal{Category: "verdict-incoherence", Level: "system", Evidence: "e", Halt: true}
	var stderr bytes.Buffer

	rec := writePipelineEscalation(evolveDir, root, 12, filepath.Join(root, "ws"), sf, &stderr)

	for _, want := range []string{
		"[loop] WARN: could not write pipeline-escalation.json: ",
		"[loop] WARN: could not auto-file pipeline-repair inbox item: ",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr lacks %q:\n%s", want, stderr.String())
		}
	}
	if rec.DossierPath != filepath.Join(evolveDir, "pipeline-escalation.json") || !strings.HasPrefix(rec.InboxItemPath, filepath.Join(root, ".evolve", "inbox")+string(filepath.Separator)) || rec.NextAction == "" {
		t.Fatalf("record = %+v", rec)
	}
}

func TestPruneBatchState_ACorruptStateReportsEveryStep(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	writeE2File(t, statePath, "{corrupt")
	var stderr bytes.Buffer

	pruneBatchState(statePath, &stderr)

	for _, want := range []string{
		"[loop] auto-prune: failurelog: parse state:",
		"[loop] auto-prune: carryover: ",
		"[loop] auto-prune: carryover backfill: ",
		"[loop] auto-prune: carryover unpicked: ",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr lacks %q:\n%s", want, stderr.String())
		}
	}
	if got, _ := os.ReadFile(statePath); string(got) != "{corrupt" {
		t.Fatalf("a corrupt state was rewritten: %q", got)
	}
}

func TestPruneBatchState_AnAbsentStateIsSilent(t *testing.T) {
	var stderr bytes.Buffer

	pruneBatchState(filepath.Join(t.TempDir(), "state.json"), &stderr)

	if stderr.Len() != 0 {
		t.Fatalf("stderr=%q", stderr.String())
	}
}
