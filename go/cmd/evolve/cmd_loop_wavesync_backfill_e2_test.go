package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxstamps"
)

func e2WavesyncExec(failFirstArg func(string) bool) func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
	return func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		if len(args) > 0 && failFirstArg(args[0]) {
			return -1, errors.New("git " + args[0] + " refused")
		}
		return 0, nil
	}
}

func TestSyncMainFromOriginAtWaveBoundary_AFailedFetchPlansAgainstLocalMain(t *testing.T) {
	origin, runtime := syncFixture(t)
	if err := os.RemoveAll(origin); err != nil {
		t.Fatal(err)
	}
	head := gitrun(t, runtime, "rev-parse", "HEAD")
	var warn bytes.Buffer

	synced, halt := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn)

	if synced || halt != nil {
		t.Fatalf("synced=%v halt=%v, want a quiet skip", synced, halt)
	}
	if !strings.HasPrefix(warn.String(), "[loop] WARN: wave-boundary sync: fetch origin failed (rc=") || !strings.Contains(warn.String(), "planning against the local main") {
		t.Fatalf("warn=%q", warn.String())
	}
	if gitrun(t, runtime, "rev-parse", "HEAD") != head {
		t.Fatal("a failed fetch moved HEAD")
	}
}

func TestSyncMainFromOriginAtWaveBoundary_AnUnbornMainWarnsAndSkips(t *testing.T) {
	origin, _ := syncFixture(t)
	repo := gittest.Fixture(t)
	runtime := repo.Dir
	repo.Git("remote", "add", "origin", origin)
	var warn bytes.Buffer

	synced, halt := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn)

	if synced || halt != nil {
		t.Fatalf("synced=%v halt=%v", synced, halt)
	}
	if !strings.HasPrefix(warn.String(), "[loop] WARN: wave-boundary sync: rev-parse HEAD: rc=") || !strings.HasSuffix(warn.String(), " — planning against the local main\n") {
		t.Fatalf("warn=%q", warn.String())
	}
}

func TestPrepareInboxStampsForFastForward_AnUnreadableTreeKeepsTheStamps(t *testing.T) {
	t.Parallel()
	g := gitexec.Git{Dir: t.TempDir(), Exec: e2WavesyncExec(func(string) bool { return true })}
	var warn bytes.Buffer

	plan := prepareInboxStampsForFastForward(context.Background(), g, &warn)

	if len(plan.Replay)+len(plan.Retired)+len(plan.Superseded)+len(plan.Kept) != 0 {
		t.Fatalf("plan = %+v, want empty", plan)
	}
	if warn.String() != "[loop] WARN: wave-boundary sync: reading the tree's changes failed: git status: git status refused — the loop's inbox stamps stay as they are\n" {
		t.Fatalf("warn=%q", warn.String())
	}
}

func TestReplayInboxStampsAfterFastForward_AFailedReplayWarnsAndStillNamesTheStamps(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stamp := inboxstamps.Stamp{Path: ".evolve/inbox/gone.json", Set: map[string]json.RawMessage{"route": json.RawMessage(`"console-manual"`)}}
	var warn bytes.Buffer

	replayInboxStampsAfterFastForward(root, inboxstamps.Plan{Replay: []inboxstamps.Stamp{stamp}}, &warn)

	lines := strings.Split(strings.TrimSpace(warn.String()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "[loop] WARN: wave-boundary sync: apply the stamp on .evolve/inbox/gone.json: ") {
		t.Fatalf("warn=%q", warn.String())
	}
	if lines[1] != "[loop] wave-boundary sync: replayed 1 plane-side inbox stamp(s) onto origin's edits, uncommitted until evolve sync-main: .evolve/inbox/gone.json" {
		t.Fatalf("second line=%q", lines[1])
	}
}

func TestRestoreInboxStampsAfterBlockedFastForward_AFailedRestoreWarnsInsteadOfClaimingIt(t *testing.T) {
	origin, runtime := syncFixture(t)
	item := filepath.Join(".evolve", "inbox", "routed.json")
	commitOnOrigin(t, origin, item, `{"id":"routed"}`)
	gitrun(t, runtime, "pull", "-q", "--ff-only", "origin", "main")
	writeE2File(t, filepath.Join(runtime, item), `{"id":"routed","route":"console-manual"}`)
	commitOnOrigin(t, origin, item, `{"id":"routed","weight":0.5}`)
	gitrun(t, runtime, "fetch", "-q", "origin", "main")
	g := gitexec.Git{Dir: runtime, Exec: gitexec.Default(runtime).Exec}
	var warn bytes.Buffer
	plan := prepareInboxStampsForFastForward(context.Background(), g, &warn)
	if len(slices.Concat(plan.Retired, plan.Superseded, plan.Replay)) == 0 {
		t.Fatalf("the fixture must prepare a stamp: %+v warn=%q", plan, warn.String())
	}
	if err := os.Remove(filepath.Join(runtime, item)); err != nil {
		t.Fatal(err)
	}
	warn.Reset()

	restoreInboxStampsAfterBlockedFastForward(runtime, plan, &warn)

	if !strings.HasPrefix(warn.String(), "[loop] WARN: wave-boundary sync: restoring the loop's inbox stamps failed: ") || strings.Contains(warn.String(), "restored ") {
		t.Fatalf("warn=%q", warn.String())
	}
}

func TestWarnOnFailedRestore_ReportsOnlyAnError(t *testing.T) {
	t.Parallel()
	var warn bytes.Buffer
	if warnOnFailedRestore(nil, &warn) || warn.Len() != 0 {
		t.Fatalf("a nil error must be silent: %q", warn.String())
	}
	if !warnOnFailedRestore(errors.New("disk full"), &warn) || warn.String() != "[loop] WARN: wave-boundary sync: restoring the loop's inbox stamps failed: disk full\n" {
		t.Fatalf("warn=%q", warn.String())
	}
}

func TestBlockingInboxFiles_AnyUnreadableGitAnswerBlocksNothing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		fail func(string) bool
	}{
		{"status fails", func(a string) bool { return a == "status" }},
		{"the remote diff fails", func(a string) bool { return a != "status" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := gitexec.Git{Dir: t.TempDir(), Exec: e2WavesyncExec(tc.fail)}

			if got := blockingInboxFiles(context.Background(), g); got != nil {
				t.Fatalf("got %v, want nil", got)
			}
		})
	}
}
