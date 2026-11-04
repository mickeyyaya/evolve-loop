package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type rebaseScript struct {
	unmerged     []string
	cachedCode   int
	cachedErr    error
	failPrefix   string
	derivedFails bool
}

func (rs rebaseScript) git(t *testing.T) *scriptedGit {
	t.Helper()
	blobs := map[string]string{"1": baseItem, "2": stampItem, "3": laneItem}
	seen := 0
	g := &scriptedGit{}
	g.respond = func(args []string) (string, int, error) {
		line := joined(args)
		switch {
		case rs.failPrefix != "" && strings.HasPrefix(line, rs.failPrefix):
			return "", 1, nil
		case line == "rebase main":
			return "", 1, nil
		case strings.HasPrefix(line, "diff --name-only --diff-filter=U"):
			i := min(seen, len(rs.unmerged)-1)
			seen++
			return rs.unmerged[i], 0, nil
		case line == "diff --cached --quiet":
			return "", rs.cachedCode, rs.cachedErr
		case strings.HasPrefix(line, "merge-tree --write-tree"):
			return "tree1\n", 0, nil
		case strings.HasPrefix(line, "diff --name-only -z tree1 HEAD"):
			return "", 0, nil
		case line == "rev-parse HEAD":
			return "orig\n", 0, nil
		case args[0] == "ls-files" && args[1] == "-u":
			return stageListing("123"), 0, nil
		case args[0] == "show":
			return blobs[args[1][1:2]], 0, nil
		}
		return "", 0, nil
	}
	return g
}

func newScriptedRebase(t *testing.T, g *scriptedGit, regenErr error) *fleetRebase {
	t.Helper()
	worktree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(worktree, ".evolve/inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &fleetRebase{
		worktree: worktree, git: g.capture,
		regen:    func(context.Context, string, string) error { return regenErr },
		classify: func(p string) (string, bool) { return "flag-index", p == cflags },
		inbox:    newInboxStamps([]string{ownItemID}),
	}
}

func TestFleetRebase_AnAbortAfterAResolveLeavesNoStampedPath(t *testing.T) {
	t.Parallel()

	both := ownItemPath + "\x00" + cflags + "\x00"
	g := rebaseScript{unmerged: []string{both, both}}.git(t)
	f := newScriptedRebase(t, g, errors.New("regen boom"))

	ok, conflicts := f.run(context.Background())

	if ok || len(conflicts) != 0 || len(f.stamped) != 0 || !g.ran("rebase --abort") {
		t.Fatalf("run = (%v, %v) stamped %v abort=%v, want a refused landing with the record rolled back", ok, conflicts, f.stamped, g.ran("rebase --abort"))
	}
}

func TestFleetRebase_AGitStepFailingDuringTheResolveAbortsTheRebase(t *testing.T) {
	t.Parallel()

	g := rebaseScript{unmerged: []string{ownItemPath + "\x00"}, failPrefix: "show :3:"}.git(t)
	f := newScriptedRebase(t, g, nil)

	ok, conflicts := f.run(context.Background())

	if ok || len(conflicts) != 0 || len(f.stamped) != 0 || !g.ran("rebase --abort") {
		t.Fatalf("run = (%v, %v) stamped %v, want ok=false with the rebase aborted and no conflict claimed", ok, conflicts, f.stamped)
	}
}

func TestFleetRebase_ALostAuditedCodeAfterAResolveRestoresTheLane(t *testing.T) {
	t.Parallel()

	g := rebaseScript{unmerged: []string{ownItemPath + "\x00"}}.git(t)
	g.respond = wrapRespond(g.respond, func(line string) (string, int, bool) {
		if strings.HasPrefix(line, "diff --name-only -z tree1 HEAD") {
			return "go/lane.go\x00", 0, true
		}
		return "", 0, false
	})
	f := newScriptedRebase(t, g, nil)

	ok, conflicts := f.run(context.Background())

	if ok || len(f.stamped) != 0 || !g.ran("reset --hard orig") {
		t.Fatalf("run ok=%v stamped=%v reset=%v, want the audited commit restored", ok, f.stamped, g.ran("reset --hard orig"))
	}
	if want := []string{"go/lane.go", ownItemPath}; !reflect.DeepEqual(conflicts, want) {
		t.Errorf("conflicts = %v, want %v: a refused proof routes to the debugger with the drift and the stamped item", conflicts, want)
	}
}

func TestFleetRebase_AResolveThatHoldsTheAuditedCodeLands(t *testing.T) {
	t.Parallel()

	g := rebaseScript{unmerged: []string{ownItemPath + "\x00"}}.git(t)
	f := newScriptedRebase(t, g, nil)

	ok, _ := f.run(context.Background())

	if !ok || len(f.stamped) != 1 || g.ran("reset --hard") {
		t.Fatalf("run ok=%v stamped=%v, want the landing with the item stamped", ok, f.stamped)
	}
}

func TestFleetRebase_AnUnreadableDeltaAfterAResolveRestoresTheLane(t *testing.T) {
	t.Parallel()

	g := rebaseScript{unmerged: []string{ownItemPath + "\x00"}}.git(t)
	g.respond = wrapRespond(g.respond, func(line string) (string, int, bool) {
		if strings.HasPrefix(line, "diff --name-only -z tree1 HEAD") {
			return "", 128, true
		}
		return "", 0, false
	})
	f := newScriptedRebase(t, g, nil)

	ok, _ := f.run(context.Background())

	if ok || !g.ran("reset --hard orig") {
		t.Fatalf("run ok=%v reset=%v, want the audited commit restored", ok, g.ran("reset --hard orig"))
	}
}

func TestFleetRebase_AnUnreadableMergedTreeRestoresTheLane(t *testing.T) {
	t.Parallel()

	g := rebaseScript{unmerged: []string{ownItemPath + "\x00"}}.git(t)
	g.respond = wrapRespond(g.respond, func(line string) (string, int, bool) {
		if strings.HasPrefix(line, "merge-tree --write-tree") {
			return "", 128, true
		}
		return "", 0, false
	})
	f := newScriptedRebase(t, g, nil)

	ok, _ := f.run(context.Background())

	if ok || !g.ran("reset --hard orig") {
		t.Fatalf("run ok=%v reset=%v, want the audited commit restored", ok, g.ran("reset --hard orig"))
	}
}

func TestFleetRebase_ARestoreThatFailsStillRefusesTheLanding(t *testing.T) {
	t.Parallel()

	g := rebaseScript{unmerged: []string{ownItemPath + "\x00"}, failPrefix: "reset --hard"}.git(t)
	g.respond = wrapRespond(g.respond, func(line string) (string, int, bool) {
		if strings.HasPrefix(line, "diff --name-only -z tree1 HEAD") {
			return "go/lane.go\x00", 0, true
		}
		return "", 0, false
	})
	f := newScriptedRebase(t, g, nil)

	ok, _ := f.run(context.Background())

	if ok || len(f.stamped) != 0 {
		t.Fatalf("run ok=%v stamped=%v, want a refused landing even when the restore fails", ok, f.stamped)
	}
}

func TestFleetRebase_TheAuditedCommitMustBeReadableBeforeTheRebaseStarts(t *testing.T) {
	t.Parallel()

	for _, fail := range []string{"rev-parse HEAD"} {
		g := rebaseScript{unmerged: []string{ownItemPath + "\x00"}, failPrefix: fail}.git(t)
		f := newScriptedRebase(t, g, nil)

		ok, _ := f.run(context.Background())

		if ok || g.ran("rebase main") {
			t.Errorf("%s failing: ok=%v rebase ran=%v, want no rebase", fail, ok, g.ran("rebase main"))
		}
	}
}

func TestFleetRebase_ARunWithNoLaneItemsNeverReadsTheAuditedCommit(t *testing.T) {
	t.Parallel()

	g := rebaseScript{unmerged: []string{cflags + "\x00"}}.git(t)
	f := newScriptedRebase(t, g, nil)
	f.inbox = inboxStamps{}

	ok, _ := f.run(context.Background())

	if !ok || g.ran("rev-parse") || g.ran("merge-base") {
		t.Fatalf("run ok=%v calls %v, want a plain derived rebase", ok, g.calls)
	}
}

func TestFleetRebase_AStagedChangeIsNeverSkipped(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		code     int
		err      error
		wantSkip bool
	}{
		{"a staged change", 1, nil, false},
		{"a failed delta read", 128, nil, false},
		{"a delta read error", 0, errors.New("boom"), false},
		{"nothing staged", 0, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := rebaseScript{unmerged: []string{cflags + "\x00", ""}, cachedCode: tc.code, cachedErr: tc.err}.git(t)
			f := newScriptedRebase(t, g, nil)
			f.inbox = inboxStamps{}

			ok, _ := f.run(context.Background())

			if g.ran("rebase --skip") != tc.wantSkip || ok != tc.wantSkip || g.ran("rebase --abort") == tc.wantSkip {
				t.Fatalf("run ok=%v skip=%v abort=%v, want skip=%v", ok, g.ran("rebase --skip"), g.ran("rebase --abort"), tc.wantSkip)
			}
		})
	}
}

func TestFleetRebase_ASkipThatFailsLeavesTheLoopRunning(t *testing.T) {
	t.Parallel()

	g := rebaseScript{unmerged: []string{cflags + "\x00", ""}, failPrefix: "rebase --skip"}.git(t)
	f := newScriptedRebase(t, g, nil)
	f.inbox = inboxStamps{}

	ok, _ := f.run(context.Background())

	if ok || !g.ran("rebase --abort") {
		t.Fatalf("run ok=%v, want the loop to end in an abort after %d steps", ok, maxRebaseContinueSteps)
	}
}

func TestRebaseLane_NoStampedPathSurvivesARefusedLanding(t *testing.T) {
	t.Parallel()

	ok, _, stamped := rebaseLane(context.Background(), "/p", laneRebase{worktree: "/p", own: []string{ownItemID}})

	if ok || stamped != nil {
		t.Fatalf("rebaseLane = %v, %v", ok, stamped)
	}
}

func wrapRespond(inner func([]string) (string, int, error), over func(line string) (string, int, bool)) func([]string) (string, int, error) {
	return func(args []string) (string, int, error) {
		if out, code, hit := over(joined(args)); hit {
			return out, code, nil
		}
		return inner(args)
	}
}

func TestFleetRebase_AMergeTreeWithNoTreeIsAnInfraRefusalNotAConflict(t *testing.T) {
	t.Parallel()

	for _, reply := range []struct {
		out  string
		code int
	}{{"", 0}, {"tree1\n", 128}} {
		g := rebaseScript{unmerged: []string{ownItemPath + "\x00"}}.git(t)
		g.respond = wrapRespond(g.respond, func(line string) (string, int, bool) {
			if strings.HasPrefix(line, "merge-tree --write-tree") {
				return reply.out, reply.code, true
			}
			return "", 0, false
		})
		f := newScriptedRebase(t, g, nil)

		ok, conflicts := f.run(context.Background())

		if ok || len(conflicts) != 0 || !g.ran("reset --hard orig") {
			t.Errorf("merge-tree %q exit %d: run = (%v, %v), want an infra refusal with the audited commit restored", reply.out, reply.code, ok, conflicts)
		}
	}
}

func TestFleetRebase_AMergeTreeThatReportsAConflictStillYieldsItsTree(t *testing.T) {
	t.Parallel()

	g := rebaseScript{unmerged: []string{ownItemPath + "\x00"}}.git(t)
	g.respond = wrapRespond(g.respond, func(line string) (string, int, bool) {
		if strings.HasPrefix(line, "merge-tree --write-tree") {
			return "tree1\n\n.evolve/inbox/x.json\n", 1, true
		}
		return "", 0, false
	})
	f := newScriptedRebase(t, g, nil)

	ok, _ := f.run(context.Background())

	if !ok || g.ran("reset --hard") {
		t.Fatalf("run ok=%v: merge-tree exit 1 only reports the inbox conflict of the proof merge", ok)
	}
}
