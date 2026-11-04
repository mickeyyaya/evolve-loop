package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const (
	ownItemID   = "wave-orchestrator-o6-wire-oversight-into-the-loop"
	ownItemPath = ".evolve/inbox/2026-10-26T19-22-34Z-" + ownItemID + ".json"
	ownConsumed = ".evolve/inbox/consumed/2026-10-26T19-22-34Z-" + ownItemID + ".json"
	peerItemID  = "ci-wall-time-c13-macos-cpu-and-spawn-counts"
	peerPath    = ".evolve/inbox/2026-10-26T19-25-52Z-" + peerItemID + ".json"
)

const ownConsumedRecord = `{"consumed":{"cycle":2144,"commit":"f995309d7","verdict":"PASS","at":"2026-11-04T07:00:00Z","by":"ship","detail":"the cycle landed the oversight wiring and recorded its acceptance proof in the ledger"},"id":"` + ownItemID + `"}`

const (
	baseItem  = `{"id":"` + ownItemID + `","notes":"original","title":"O6","weight":0.6}`
	laneItem  = `{"id":"` + ownItemID + `","notes":"lane notes","title":"O6","weight":0.6}`
	stampItem = `{"failure_count":1,"id":"` + ownItemID + `","last_failure_class":"unknown","notes":"original","title":"O6","weight":0.6}`
)

type inboxLane struct {
	repo      *gittest.Repo
	worktree  string
	workspace string
	codeTree  string
}

func newInboxLane(t *testing.T, laneEdit func(l *inboxLane), mainEdit func(l *inboxLane)) *inboxLane {
	t.Helper()
	repo := gittest.Fixture(t)
	l := &inboxLane{repo: repo, worktree: filepath.Join(t.TempDir(), "wt"), workspace: t.TempDir()}
	writeInboxFile(t, filepath.Join(repo.Dir, ownItemPath), baseItem)
	writeInboxFile(t, filepath.Join(repo.Dir, peerPath), `{"id":"`+peerItemID+`","notes":"original"}`)
	writeInboxFile(t, filepath.Join(repo.Dir, "go/a.go"), "package a\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	repo.Git("worktree", "add", "-q", "-b", "cycle-branch", l.worktree, "main")
	writeInboxFile(t, filepath.Join(l.worktree, "go/lane.go"), "package a\n")
	laneEdit(l)
	repo.Git("-C", l.worktree, "add", "-A")
	repo.Git("-C", l.worktree, "commit", "-q", "-m", "lane")
	l.codeTree = repo.Git("-C", l.worktree, "rev-parse", "HEAD:go")
	mainEdit(l)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "chore(inbox): the loop's inbox stamps from the plane")
	materializeLaneScope(l.workspace, ownItemID, "")
	return l
}

func (l *inboxLane) rebase(t *testing.T) (bool, bool, []string, CycleState) {
	t.Helper()
	cs := CycleState{ActiveWorktree: l.worktree, WorkspacePath: l.workspace}
	ok, conflict := rebaseRecordingConflicts(context.Background(), "", &cs)
	return ok, conflict, cs.ShipRecoveryStamped, cs
}

func TestRebaseRecordingConflicts_TheLaneLandsItsOwnInboxItemWithMainsStampsKept(t *testing.T) {
	t.Parallel()

	l := newInboxLane(t,
		func(l *inboxLane) { writeInboxFile(t, filepath.Join(l.worktree, ownItemPath), laneItem) },
		func(l *inboxLane) { writeInboxFile(t, filepath.Join(l.repo.Dir, ownItemPath), stampItem) })

	ok, conflict, stamped, cs := l.rebase(t)

	if !ok || conflict || len(cs.ShipRecoveryConflicts) != 0 {
		t.Fatalf("rebase = (%v, %v, %v), want the lane to land: only its own inbox item conflicted", ok, conflict, cs.ShipRecoveryConflicts)
	}
	if !reflect.DeepEqual(stamped, []string{ownItemPath}) {
		t.Errorf("stamped = %v, want [%s]", stamped, ownItemPath)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(l.repo.Git("-C", l.worktree, "show", "HEAD:"+ownItemPath)), &got); err != nil {
		t.Fatal(err)
	}
	if got["notes"] != "lane notes" || got["last_failure_class"] != "unknown" || got["failure_count"] != float64(1) {
		t.Errorf("merged item = %v, want the lane's notes and main's stamps", got)
	}
	if tree := l.repo.Git("-C", l.worktree, "rev-parse", "HEAD:go"); tree != l.codeTree {
		t.Errorf("code tree = %s, want the audited %s: the resolve never touches code", tree, l.codeTree)
	}
	if status := l.repo.Git("-C", l.worktree, "status", "--porcelain"); status != "" {
		t.Errorf("worktree not clean after the rebase:\n%s", status)
	}
}

func TestRebaseRecordingConflicts_TheCyclesConsumptionLandsOverMainsStamp(t *testing.T) {
	t.Parallel()

	l := newInboxLane(t,
		func(l *inboxLane) {
			writeInboxFile(t, filepath.Join(l.worktree, ownConsumed), ownConsumedRecord)
			l.repo.Git("-C", l.worktree, "rm", "-q", "--", ownItemPath)
		},
		func(l *inboxLane) { writeInboxFile(t, filepath.Join(l.repo.Dir, ownItemPath), stampItem) })

	ok, conflict, stamped, _ := l.rebase(t)

	if !ok || conflict || !reflect.DeepEqual(stamped, []string{ownItemPath}) {
		t.Fatalf("rebase = (%v, %v, %v), want consumption to win the conflict", ok, conflict, stamped)
	}
	if _, err := os.Stat(filepath.Join(l.worktree, ownItemPath)); !os.IsNotExist(err) {
		t.Errorf("the inbox item still exists after the cycle consumed it: %v", err)
	}
	if _, err := os.Stat(filepath.Join(l.worktree, ownConsumed)); err != nil {
		t.Errorf("the consumed copy is missing: %v", err)
	}
	if tree := l.repo.Git("-C", l.worktree, "rev-parse", "HEAD:go"); tree != l.codeTree {
		t.Errorf("code tree = %s, want the audited %s", tree, l.codeTree)
	}
}

func TestRebaseRecordingConflicts_AConflictOnAnotherItemsInboxFileStillFails(t *testing.T) {
	t.Parallel()

	l := newInboxLane(t,
		func(l *inboxLane) {
			writeInboxFile(t, filepath.Join(l.worktree, peerPath), `{"id":"`+peerItemID+`","notes":"lane"}`)
		},
		func(l *inboxLane) {
			writeInboxFile(t, filepath.Join(l.repo.Dir, peerPath), `{"id":"`+peerItemID+`","notes":"peer","x":1}`)
		})

	ok, conflict, stamped, cs := l.rebase(t)

	if ok || !conflict || len(stamped) != 0 || !reflect.DeepEqual(cs.ShipRecoveryConflicts, []string{peerPath}) {
		t.Fatalf("rebase = (%v, %v, %v, %v), want the foreign item to conflict as today", ok, conflict, stamped, cs.ShipRecoveryConflicts)
	}
}

func TestRebaseRecordingConflicts_ACodeConflictStillFailsAndNamesOnlyTheCode(t *testing.T) {
	t.Parallel()

	l := newInboxLane(t,
		func(l *inboxLane) {
			writeInboxFile(t, filepath.Join(l.worktree, ownItemPath), laneItem)
			writeInboxFile(t, filepath.Join(l.worktree, "go/a.go"), "package lane\n")
		},
		func(l *inboxLane) {
			writeInboxFile(t, filepath.Join(l.repo.Dir, ownItemPath), stampItem)
			writeInboxFile(t, filepath.Join(l.repo.Dir, "go/a.go"), "package peer\n")
		})

	ok, conflict, _, cs := l.rebase(t)

	if ok || !conflict || !reflect.DeepEqual(cs.ShipRecoveryConflicts, []string{"go/a.go"}) {
		t.Fatalf("rebase = (%v, %v, %v), want only go/a.go named", ok, conflict, cs.ShipRecoveryConflicts)
	}
	if status := l.repo.Git("-C", l.worktree, "status", "--porcelain"); status != "" {
		t.Errorf("worktree not clean after the abort:\n%s", status)
	}
}

func stageListing(stages string) string {
	var sb strings.Builder
	for _, st := range stages {
		sb.WriteString("100644 abc " + string(st) + "\t" + ownItemPath + "\x00")
	}
	return sb.String()
}

func TestInboxStamps_Resolve(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		stages    string
		blobs     map[string]string
		consumed  bool
		path      string
		gitFails  string
		wantLeft  bool
		wantErr   bool
		wantRan   string
		wantNoRun string
	}{
		{name: "consumed copies are not stamps", stages: "123", blobs: map[string]string{"1": baseItem, "2": stampItem, "3": laneItem}, path: ownConsumed, wantLeft: true, wantNoRun: "add --"},
		{name: "main deleted the item", stages: "13", path: ownItemPath, wantLeft: true},
		{name: "ours is not json", stages: "23", blobs: map[string]string{"2": "not json"}, path: ownItemPath, wantLeft: true},
		{name: "ours is another item", stages: "23", blobs: map[string]string{"2": `{"id":"other"}`}, path: ownItemPath, wantLeft: true},
		{name: "theirs is not json", stages: "123", blobs: map[string]string{"1": baseItem, "2": baseItem, "3": "{{"}, path: ownItemPath, wantLeft: true},
		{name: "lane deleted it with its consumed copy", stages: "12", blobs: map[string]string{"2": stampItem}, consumed: true, path: ownItemPath, wantRan: "rm -q -- " + ownItemPath},
		{name: "lane deleted it with no consumed copy", stages: "12", blobs: map[string]string{"2": stampItem}, path: ownItemPath, wantLeft: true, wantNoRun: "rm -q"},
		{name: "rm fails", stages: "12", blobs: map[string]string{"2": stampItem}, consumed: true, path: ownItemPath, gitFails: "rm", wantErr: true},
		{name: "stage listing fails", stages: "123", path: ownItemPath, gitFails: "ls-files -u", wantErr: true},
		{name: "consumed listing fails", stages: "12", blobs: map[string]string{"2": stampItem}, path: ownItemPath, gitFails: "ls-files -z", wantErr: true},
		{name: "stage 2 read fails", stages: "123", blobs: map[string]string{"3": laneItem}, path: ownItemPath, gitFails: "show :2:", wantErr: true},
		{name: "stage 3 read fails", stages: "123", blobs: map[string]string{"1": baseItem, "2": stampItem}, path: ownItemPath, gitFails: "show :3:", wantErr: true},
		{name: "stage 1 read fails", stages: "123", blobs: map[string]string{"2": stampItem, "3": laneItem}, path: ownItemPath, gitFails: "show :1:", wantErr: true},
		{name: "no base stage merges against nothing", stages: "23", blobs: map[string]string{"2": stampItem, "3": laneItem}, path: ownItemPath, wantRan: "add -- " + ownItemPath},
		{name: "add fails", stages: "123", blobs: map[string]string{"1": baseItem, "2": stampItem, "3": laneItem}, path: ownItemPath, gitFails: "add", wantErr: true, wantRan: "add -- " + ownItemPath},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			worktree := t.TempDir()
			if err := os.MkdirAll(filepath.Join(worktree, ".evolve/inbox"), 0o755); err != nil {
				t.Fatal(err)
			}
			g := &scriptedGit{respond: func(args []string) (string, int, error) {
				line := joined(args)
				switch {
				case tc.gitFails != "" && strings.HasPrefix(line, tc.gitFails):
					return "", 1, nil
				case args[0] == "ls-files" && args[1] == "-u":
					return stageListing(tc.stages), 0, nil
				case args[0] == "ls-files":
					if tc.consumed {
						return ownConsumed + "\x00", 0, nil
					}
					return "", 0, nil
				case args[0] == "show":
					if !strings.Contains(tc.stages, args[1][1:2]) {
						return "", 128, nil
					}
					return tc.blobs[args[1][1:2]], 0, nil
				}
				return "", 0, nil
			}}
			left, settled, err := newInboxStamps([]string{ownItemID}).resolve(context.Background(), worktree, g.capture, []string{tc.path})
			if tc.wantErr {
				if err == nil || len(left) != 0 || len(settled) != 0 {
					t.Fatalf("resolve = left %v settled %v err %v, want an error and no verdict", left, settled, err)
				}
			} else if err != nil || tc.wantLeft != (len(left) == 1) || tc.wantLeft == (len(settled) == 1) {
				t.Fatalf("resolve = left %v settled %v err %v, wantLeft=%v", left, settled, err, tc.wantLeft)
			}
			if tc.wantRan != "" && !g.ran(tc.wantRan) {
				t.Errorf("git calls %v lack %q", g.calls, tc.wantRan)
			}
			if tc.wantNoRun != "" && g.ran(tc.wantNoRun) {
				t.Errorf("git calls %v hold %q", g.calls, tc.wantNoRun)
			}
		})
	}
}

func TestInboxStamps_ResolveFailsWhenTheMergedFileCannotBeWritten(t *testing.T) {
	t.Parallel()

	blobs := map[string]string{"1": baseItem, "2": stampItem, "3": laneItem}
	g := &scriptedGit{respond: func(args []string) (string, int, error) {
		switch args[0] {
		case "ls-files":
			return stageListing("123"), 0, nil
		case "show":
			return blobs[args[1][1:2]], 0, nil
		}
		t.Fatalf("unexpected git call %q", joined(args))
		return "", 0, nil
	}}
	blocked := t.TempDir()
	if err := os.WriteFile(filepath.Join(blocked, ".evolve"), []byte("a file, not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	left, settled, err := newInboxStamps([]string{ownItemID}).resolve(context.Background(), blocked, g.capture, []string{ownItemPath})

	if err == nil || len(left) != 0 || len(settled) != 0 {
		t.Fatalf("resolve = left %v settled %v err %v, want an error that aborts the rebase", left, settled, err)
	}
}

func TestInboxStamps_OwnsNeedsANonEmptyId(t *testing.T) {
	t.Parallel()

	s := newInboxStamps([]string{""})

	if s.owns(`{"notes":"x"}`) || s.owns(`{"id":""}`) {
		t.Fatal("an item with no id must not be owned by an empty scope id")
	}
}

func TestMergeInboxItem(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, base, ours, theirs, want string
		ok                             bool
	}{
		{"lane field and main stamp both kept", `{"id":"a","a":1}`, `{"id":"a","a":1,"s":2}`, `{"id":"a","a":3}`, `{"a":3,"id":"a","s":2}`, true},
		{"main wins a field both changed", `{"id":"a","a":1}`, `{"id":"a","a":2}`, `{"id":"a","a":3}`, `{"a":2,"id":"a"}`, true},
		{"lane drops a field main left alone", `{"id":"a","a":1,"b":2}`, `{"id":"a","a":1,"b":2}`, `{"id":"a","a":1}`, `{"a":1,"id":"a"}`, true},
		{"lane drops a field main changed", `{"id":"a","a":1,"b":2}`, `{"id":"a","a":1,"b":5}`, `{"id":"a","a":1}`, `{"a":1,"b":5,"id":"a"}`, true},
		{"no common base", ``, `{"id":"a","s":2}`, `{"id":"a","a":3}`, `{"a":3,"id":"a","s":2}`, true},
		{"base is not json", `[`, `{"id":"a"}`, `{"id":"a"}`, ``, false},
		{"ours is not json", `{}`, `[`, `{"id":"a"}`, ``, false},
		{"theirs is not json", `{}`, `{"id":"a"}`, `[`, ``, false},
		{"ours has no id", `{}`, `{"s":2}`, `{"a":3}`, ``, false},
		{"theirs is null", `{"id":"a"}`, `{"id":"a","s":2}`, `null`, ``, false},
		{"ours is null", `{"id":"a"}`, `null`, `{"id":"a"}`, ``, false},
		{"theirs empty drops the id", `{"id":"a"}`, `{"id":"a","s":2}`, `{}`, ``, false},
		{"theirs renames the id", `{"id":"a"}`, `{"id":"a","s":2}`, `{"id":"b"}`, ``, false},
		{"html is not escaped", `{"id":"a"}`, `{"id":"a","u":"a<b&c"}`, `{"id":"a"}`, `{"id":"a","u":"a<b&c"}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := mergeInboxItem(tc.base, tc.ours, tc.theirs)
			if ok != tc.ok || string(got) != tc.want {
				t.Fatalf("mergeInboxItem = (%q, %v), want (%q, %v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestStageReader_ReportsAGitFailureAsAnError(t *testing.T) {
	t.Parallel()

	g := &scriptedGit{respond: func([]string) (string, int, error) { return "", 0, errors.New("boom") }}
	r := stageReader{worktree: "/wt", git: g.capture}

	if _, err := r.blob(context.Background(), 2, "p"); err == nil {
		t.Fatal("a git error must read as an error, not an absent stage")
	}
	if _, err := r.stages(context.Background(), "p"); err == nil {
		t.Fatal("a failed stage listing must be an error")
	}
}

func TestStageReader_StagesReadsTheStageNumbersOfTheListing(t *testing.T) {
	t.Parallel()

	g := &scriptedGit{respond: func([]string) (string, int, error) {
		return "100644 abc 1\tp\x00100644 abc 3\tp\x00\tp\x00junk\tp\x00", 0, nil
	}}

	got, err := (stageReader{worktree: "/wt", git: g.capture}).stages(context.Background(), "p")

	if err != nil || !reflect.DeepEqual(got, map[int]bool{1: true, 3: true}) {
		t.Fatalf("stages = %v, %v, want 1 and 3", got, err)
	}
}

func TestAssembleFailureDigest_AFleetRebaseConflictIsItsOwnClassNotUnknown(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	reason := `{"schema_version":1,"phase":"ship","reasons":["ship error GIT_FLEET_REBASE_CONFLICT: the fleet rebase hit a merge conflict on .evolve/inbox/x.json"]}`
	if err := os.WriteFile(filepath.Join(ws, "audit-fail-reason.json"), []byte(reason), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := AssembleFailureDigest(2144, ws, nil)

	if err != nil || d.PreClass != "ship-rebase-conflict" || dossier.PreClassShipRebaseConflict != "ship-rebase-conflict" {
		t.Fatalf("digest = %+v, err %v, want class ship-rebase-conflict", d, err)
	}
}

func writeInboxFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAssembleFailureDigest_AGuardAbortOutranksAFleetRebaseConflict(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	reason := `{"schema_version":1,"phase":"ship","reasons":["ship error GIT_FLEET_REBASE_CONFLICT: merge conflict","statemap severed on the ship edge"]}`
	if err := os.WriteFile(filepath.Join(ws, "audit-fail-reason.json"), []byte(reason), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := AssembleFailureDigest(2144, ws, nil)

	if err != nil || d.PreClass != dossier.PreClassGuardAbort {
		t.Fatalf("digest = %+v, err %v, want guard-abort: a pipeline fault must not hide behind a conflict", d, err)
	}
}
