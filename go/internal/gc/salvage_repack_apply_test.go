package gc

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestSalvageRepack_ABundleThatChangedSincethePlanIsRefused(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/race", "race.txt")
	e.fullLeaf("cycle-aaaa-71", "lane/race", tip)
	items := e.plan()
	f, err := os.OpenFile(e.bundlePath("cycle-aaaa-71"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("x"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{items["cycle-aaaa-71"]})
	if !errors.Is(err, errSalvageChanged) || freed != 0 {
		t.Errorf("freed = %d, err = %v", freed, err)
	}
}

func TestSalvageRepack_AFailedAuditWriteLeavesTheBundle(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/audit", "audit.txt")
	e.fullLeaf("cycle-aaaa-92", "lane/audit", tip)
	items := e.plan()
	if err := os.WriteFile(filepath.Join(e.opts.EvolveDir, "archive"), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := treeDigest(t, e.leafDir("cycle-aaaa-92"))

	freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{items["cycle-aaaa-92"]})

	if err == nil || freed != 0 {
		t.Fatalf("freed = %d, err = %v, want a failure", freed, err)
	}
	assertSameDigest(t, before, treeDigest(t, e.leafDir("cycle-aaaa-92")))
}

func TestSalvageRepack_TheFinalRecheckOfARepackRunsUnderTheShipLock(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/lock", "lock.txt")
	e.fullLeaf("cycle-aaaa-130", "lane/lock", tip)
	seen := e.observeShipLock()
	item := e.planned("cycle-aaaa-130")
	if item.Action != SalvageRepack {
		t.Fatalf("setup: item = %+v", item)
	}

	if _, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{item}); err != nil {
		t.Fatal(err)
	}

	got := *seen
	if len(got) < 3 || got[0] || got[len(got)-1] != true {
		t.Errorf("lock held at each recheck = %v, want free at the plan and held at the last recheck", got)
	}
}

func TestSalvageRepack_TheRecheckOfADropRunsUnderTheShipLock(t *testing.T) {
	e := newSalvageRepackEnv(t)
	seen := e.observeShipLock()
	item := e.landedLeaf("cycle-aaaa-130", "lane/lockdrop")
	if item.Action != SalvageDropRedundant {
		t.Fatalf("setup: item = %+v", item)
	}

	if _, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{item}); err != nil {
		t.Fatal(err)
	}

	got := *seen
	if len(got) < 2 || got[0] || got[len(got)-1] != true {
		t.Errorf("lock held at each check = %v, want free at the plan and held at the recheck", got)
	}
}

func TestSalvageRepack_ABundleThatChangesWhileTheNewOneIsBuiltIsRefused(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/mid", "mid.txt")
	e.fullLeaf("cycle-aaaa-101", "lane/mid", tip)
	item := e.planned("cycle-aaaa-101")
	inner := e.opts.Exec
	e.opts.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		code, err := inner(ctx, name, dir, args, env, in, out, errw)
		if len(args) > 2 && args[1] == "list-heads" && strings.HasSuffix(args[2], salvageRepackSuffix) {
			f, openErr := os.OpenFile(e.bundlePath("cycle-aaaa-101"), os.O_APPEND|os.O_WRONLY, 0)
			if openErr != nil {
				t.Error(openErr)
				return code, err
			}
			_, _ = f.WriteString("x")
			_ = f.Close()
		}
		return code, err
	}

	freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{item})

	if !errors.Is(err, errSalvageChanged) || freed != 0 {
		t.Fatalf("freed = %d, err = %v, want a refusal for the changed bundle", freed, err)
	}
	if got := fileSize(t, e.bundlePath("cycle-aaaa-101")); got != item.Bytes+1 {
		t.Errorf("bundle is %d bytes, want the writer's %d", got, item.Bytes+1)
	}
}

func TestSalvageRepack_ALeafNoteThatChangedSinceThePlanIsRefused(t *testing.T) {
	e := newSalvageRepackEnv(t)
	item := e.landedLeaf("cycle-aaaa-102", "lane/note")
	if err := os.WriteFile(filepath.Join(e.leafDir("cycle-aaaa-102"), salvageHeadFile), []byte("other y\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{item}); !errors.Is(err, errSalvageChanged) {
		t.Errorf("err = %v", err)
	}
}

func TestSalvageRepack_ALockThatCannotBeTakenKeepsEachBundle(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/nolock", "nolock.txt")
	e.fullLeaf("cycle-aaaa-103", "lane/nolock", tip)
	drop := e.landedLeaf("cycle-aaaa-104", "lane/nolockdrop")
	repack := e.planned("cycle-aaaa-103")
	if err := os.MkdirAll(e.lockPath(), 0o755); err != nil {
		t.Fatal(err)
	}
	before := treeDigest(t, OperatorSalvageDir(e.opts.EvolveDir))

	freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{repack, drop})

	if err == nil || freed != 0 {
		t.Fatalf("freed = %d, err = %v", freed, err)
	}
	assertSameDigest(t, before, treeDigest(t, OperatorSalvageDir(e.opts.EvolveDir)))
}

func TestSalvageRepack_ALeafThatVanishedSinceThePlanIsRefused(t *testing.T) {
	e := newSalvageRepackEnv(t)
	gone := e.landedLeaf("cycle-aaaa-105", "lane/gone")
	nobundle := e.landedLeaf("cycle-aaaa-106", "lane/nobundle")
	if err := os.RemoveAll(e.leafDir("cycle-aaaa-105")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(e.bundlePath("cycle-aaaa-106")); err != nil {
		t.Fatal(err)
	}

	freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{gone, nobundle})

	if !errors.Is(err, os.ErrNotExist) || freed != 0 {
		t.Errorf("freed = %d, err = %v, want two refusals", freed, err)
	}
}

func TestSalvageRepack_AHeldLeafIsRefusedAtTheApplyEvenWhenThePlanSawItFree(t *testing.T) {
	e := newSalvageRepackEnv(t)
	item := e.landedLeaf("cycle-aaaa-107", "lane/held")
	runDir := filepath.Join(e.opts.EvolveDir, "runs", "cycle-107")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "cycle-107", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}

	freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{item})

	if !errors.Is(err, errSalvageHeld) || freed != 0 {
		t.Errorf("freed = %d, err = %v", freed, err)
	}
	if _, statErr := os.Stat(e.bundlePath("cycle-aaaa-107")); statErr != nil {
		t.Errorf("bundle gone: %v", statErr)
	}
}

func TestSalvageRepack_ARepackOrDropFailsWhenOriginMainIsGoneAtTheApply(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/nomain2", "nomain2.txt")
	e.fullLeaf("cycle-aaaa-108", "lane/nomain2", tip)
	repack := e.planned("cycle-aaaa-108")
	drop := e.landedLeaf("cycle-aaaa-109", "lane/nomain3")
	e.repo.Git("update-ref", "-d", "refs/remotes/origin/main")

	freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{repack, drop})

	if !errors.Is(err, errSalvageNoOrigin) || freed != 0 {
		t.Errorf("freed = %d, err = %v", freed, err)
	}
}

func TestSalvageRepack_ADropFailsWhenTheCountOfCommitsAheadCannotBeRead(t *testing.T) {
	e := newSalvageRepackEnv(t)
	drop := e.landedLeaf("cycle-aaaa-110", "lane/count")
	e.failing(func(a []string) bool { return a[0] == "rev-list" })

	freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{drop})

	if err == nil || freed != 0 {
		t.Errorf("freed = %d, err = %v", freed, err)
	}
	if _, statErr := os.Stat(e.bundlePath("cycle-aaaa-110")); statErr != nil {
		t.Errorf("bundle gone: %v", statErr)
	}
}

func TestSalvageRepack_ADropThatCannotRemoveTheBundleReportsIt(t *testing.T) {
	e := newSalvageRepackEnv(t)
	drop := e.landedLeaf("cycle-aaaa-111", "lane/rm")
	dir := e.leafDir("cycle-aaaa-111")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{drop})

	if err == nil || freed != 0 {
		t.Errorf("freed = %d, err = %v", freed, err)
	}
}

func TestSalvageRepack_AnItemThatIsKeepOrUnknownIsSkipped(t *testing.T) {
	e := newSalvageRepackEnv(t)

	freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{{Leaf: "x", Action: SalvageKeep}, {Leaf: "y", Action: SalvageRepackAction("other")}})

	if err != nil || freed != 0 {
		t.Errorf("freed = %d, err = %v", freed, err)
	}
}

func TestSalvageRepack_ADropWhoseAuditRowCannotBeWrittenKeepsTheBundle(t *testing.T) {
	e := newSalvageRepackEnv(t)
	drop := e.landedLeaf("cycle-aaaa-141", "lane/auditdrop")
	if err := os.WriteFile(filepath.Join(e.opts.EvolveDir, "archive"), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}

	freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{drop})

	if err == nil || freed != 0 {
		t.Fatalf("freed = %d, err = %v", freed, err)
	}
	if _, statErr := os.Stat(e.bundlePath("cycle-aaaa-141")); statErr != nil {
		t.Errorf("bundle gone: %v", statErr)
	}
}

func TestSalvageRepack_ABundleRewrittenWithTheSameSizeAtAnotherTimeIsRefused(t *testing.T) {
	e := newSalvageRepackEnv(t)
	item := e.landedLeaf("cycle-aaaa-120", "lane/retime")
	later := item.ModTime.Add(time.Minute)
	if err := os.Chtimes(e.bundlePath("cycle-aaaa-120"), later, later); err != nil {
		t.Fatal(err)
	}

	freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{item})
	if !errors.Is(err, errSalvageChanged) || freed != 0 {
		t.Errorf("freed = %d, err = %v", freed, err)
	}
}
