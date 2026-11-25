package gc

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func writeEmptyTarGz(t *testing.T, p string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeNonEmptyTarGz(t *testing.T, p string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("untracked\n")
	if err := tw.WriteHeader(&tar.Header{Name: "u.txt", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writePaddedLeaf(t *testing.T, e *salvageRepackEnv, leaf, branch, tip string, age time.Duration, padBytes int, extra map[string][]byte) {
	t.Helper()
	dir := e.leafDir(leaf)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pad := make([]byte, padBytes)
	if _, err := rand.Read(pad); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, salvageHeadFile), []byte(tip+" "+branch+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, salvageBundleFile), pad, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range extra {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	stampLeaf(t, dir, age)
}

func stampLeaf(t *testing.T, dir string, age time.Duration) {
	t.Helper()
	stamp := time.Now().Add(-age)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if err := os.Chtimes(filepath.Join(dir, entry.Name()), stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(dir, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}

func TestSalvageRepack_MigratesFullBundleWhenCommitsLeftHub(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/pruned", "pruned.txt")
	e.fullLeaf("cycle-aaaa-150", "lane/pruned", tip)
	headsBefore := e.heads("cycle-aaaa-150")
	e.repo.Git("branch", "-D", "lane/pruned")
	e.repo.Git("reflog", "expire", "--expire=now", "--all")
	e.repo.Git("gc", "-q", "--prune=now")
	refsBefore := e.refs()

	item := e.planned("cycle-aaaa-150")
	if item.Action != SalvageRepack {
		t.Fatalf("action = %q (%s), want repack", item.Action, item.Reason)
	}
	if _, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{item}); err != nil {
		t.Fatalf("ApplySalvageRepack: %v", err)
	}

	e.repo.Git("bundle", "verify", e.bundlePath("cycle-aaaa-150"))
	if got := e.heads("cycle-aaaa-150"); got != headsBefore {
		t.Errorf("heads = %q, want %q", got, headsBefore)
	}
	if got := fileSize(t, e.bundlePath("cycle-aaaa-150")); got >= thinBundleLimit {
		t.Errorf("bundle size = %d, want under %d", got, thinBundleLimit)
	}
	if e.refs() != refsBefore {
		t.Errorf("hub refs changed")
	}
}

func TestPlanSalvageRetention_ZeroCapAndMissingHomeReturnEmptyPlan(t *testing.T) {
	e := newSalvageRepackEnv(t)

	p, err := PlanSalvageRetention(e.opts, 0)
	if err != nil || p.CapBytes != 0 || len(p.Items) != 0 {
		t.Fatalf("zero cap = %+v, %v", p, err)
	}

	p, err = PlanSalvageRetention(e.opts, 1<<20)
	if err != nil || p.CapBytes != 1<<20 || len(p.Items) != 0 {
		t.Fatalf("missing home = %+v, %v", p, err)
	}
}

func TestPlanSalvageRetention_RequiresOriginMainAndHonorsEmptyAndNonEmptyUntracked(t *testing.T) {
	e := newSalvageRepackEnv(t)
	landedA := e.land("lane/a", "a.txt")
	landedB := e.land("lane/b", "b.txt")
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")

	writePaddedLeaf(t, e, "cycle-aaaa-201", "lane/a", landedA, 10*24*time.Hour, 2<<20, nil)
	writeEmptyTarGz(t, filepath.Join(e.leafDir("cycle-aaaa-201"), salvageUntrackedFile))
	stampLeaf(t, e.leafDir("cycle-aaaa-201"), 10*24*time.Hour)
	writePaddedLeaf(t, e, "cycle-aaaa-202", "lane/b", landedB, 15*24*time.Hour, 2<<20, nil)
	writeNonEmptyTarGz(t, filepath.Join(e.leafDir("cycle-aaaa-202"), salvageUntrackedFile))
	stampLeaf(t, e.leafDir("cycle-aaaa-202"), 15*24*time.Hour)
	writePaddedLeaf(t, e, "cycle-aaaa-203", "lane/c", landedB, 20*24*time.Hour, 2<<20, map[string][]byte{salvageUntrackedFile: []byte("corrupt")})

	plan, err := PlanSalvageRetention(e.opts, 1<<20)
	if err != nil {
		t.Fatalf("PlanSalvageRetention: %v", err)
	}
	actions := map[string]SalvageRetentionItem{}
	for _, it := range plan.Items {
		actions[it.Leaf] = it
	}
	if actions["cycle-aaaa-201"].Action != SalvageEvict {
		t.Errorf("empty untracked leaf: action=%q (%s), want evict", actions["cycle-aaaa-201"].Action, actions["cycle-aaaa-201"].Reason)
	}
	if actions["cycle-aaaa-202"].Action != SalvageRetain {
		t.Errorf("non-empty untracked leaf: action=%q, want keep", actions["cycle-aaaa-202"].Action)
	}
	if actions["cycle-aaaa-203"].Action != SalvageRetain {
		t.Errorf("corrupt untracked leaf: action=%q, want keep", actions["cycle-aaaa-203"].Action)
	}

	e.repo.Git("update-ref", "-d", "refs/remotes/origin/main")
	if _, err := PlanSalvageRetention(e.opts, 1<<20); !errors.Is(err, errSalvageNoOrigin) {
		t.Errorf("missing origin/main: err = %v", err)
	}
}

func TestApplySalvageRetention_EvictsOldestLandedAndRemovesMatchingArchiveRef(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tipOld := e.land("lane/old", "old.txt")
	tipNew := e.land("lane/new", "new.txt")
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	tipUnlanded := e.commitOn("lane/unlanded", "u.txt")

	writePaddedLeaf(t, e, "cycle-aaaa-211", "lane/old", tipOld, 20*24*time.Hour, 2<<20, nil)
	writePaddedLeaf(t, e, "cycle-aaaa-212", "lane/new", tipNew, 10*24*time.Hour, 2<<20, nil)
	writePaddedLeaf(t, e, "cycle-aaaa-213", "lane/unlanded", tipUnlanded, 25*24*time.Hour, 2<<20, nil)
	e.repo.Git("update-ref", archiveRef("cycle-aaaa-211"), tipOld)

	plan, err := PlanSalvageRetention(e.opts, 5<<20)
	if err != nil {
		t.Fatalf("PlanSalvageRetention: %v", err)
	}
	if plan.TotalBytes <= 5<<20 || plan.RemainingBytes > 5<<20 {
		t.Fatalf("unexpected plan sizes: total=%d remaining=%d cap=%d", plan.TotalBytes, plan.RemainingBytes, plan.CapBytes)
	}
	freed, err := ApplySalvageRetention(e.opts, plan)
	if err != nil || freed < 2<<20 {
		t.Fatalf("ApplySalvageRetention: freed=%d err=%v", freed, err)
	}
	if _, err := os.Stat(e.leafDir("cycle-aaaa-211")); !os.IsNotExist(err) {
		t.Errorf("oldest landed leaf still present")
	}
	if _, ok := e.opts.refValue(archiveRef("cycle-aaaa-211")); ok {
		t.Errorf("archive ref for evicted leaf still exists")
	}
	if _, err := os.Stat(e.leafDir("cycle-aaaa-212")); err != nil {
		t.Errorf("newer landed leaf missing: %v", err)
	}
	if _, err := os.Stat(e.leafDir("cycle-aaaa-213")); err != nil {
		t.Errorf("unlanded leaf missing: %v", err)
	}
}

func TestApplySalvageRetention_RecheckRefusesWhenLeafChangesOrBecomesHeld(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.land("lane/race", "race.txt")
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	writePaddedLeaf(t, e, "cycle-aaaa-221", "lane/race", tip, 10*24*time.Hour, 2<<20, nil)
	writePaddedLeaf(t, e, "cycle-aaaa-222", "lane/race", tip, 9*24*time.Hour, 2<<20, nil)
	writePaddedLeaf(t, e, "cycle-aaaa-223", "lane/race", tip, 8*24*time.Hour, 2<<20, nil)

	plan, err := PlanSalvageRetention(e.opts, 1<<20)
	if err != nil {
		t.Fatalf("PlanSalvageRetention: %v", err)
	}

	if err := os.WriteFile(filepath.Join(e.leafDir("cycle-aaaa-221"), salvagePatchFile), []byte("diff\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := continuation.WriteRegistryEntry(e.opts.ProjectRoot, "item-x", continuation.Continuation{Branch: "cycle-aaaa-222", Cycle: 222}); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(e.opts.EvolveDir, "runs", "cycle-223")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "cycle-223", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}

	freed, err := ApplySalvageRetention(e.opts, plan)
	if !errors.Is(err, errSalvageEvictRefused) || freed != 0 {
		t.Fatalf("freed=%d err=%v, want refusal", freed, err)
	}
	for _, leaf := range []string{"cycle-aaaa-221", "cycle-aaaa-222", "cycle-aaaa-223"} {
		if _, statErr := os.Stat(e.leafDir(leaf)); statErr != nil {
			t.Errorf("%s was evicted despite recheck hold: %v", leaf, statErr)
		}
	}
}

func TestPlanSalvageRetention_SparesBadHeadAndSettleWindow(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.land("lane/ok", "ok.txt")
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	writePaddedLeaf(t, e, "cycle-aaaa-231", "lane/ok", "not-a-sha", 10*24*time.Hour, 2<<20, nil)
	writePaddedLeaf(t, e, "cycle-aaaa-232", "lane/ok", tip, 5*time.Minute, 2<<20, nil)
	writePaddedLeaf(t, e, "cycle-aaaa-233", "lane/ok", tip, 10*24*time.Hour, 2<<20, nil)

	plan, err := PlanSalvageRetention(e.opts, 1<<20)
	if err != nil {
		t.Fatalf("PlanSalvageRetention: %v", err)
	}
	byLeaf := map[string]SalvageRetentionItem{}
	for _, it := range plan.Items {
		byLeaf[it.Leaf] = it
	}
	if byLeaf["cycle-aaaa-231"].Action != SalvageRetain || !strings.Contains(byLeaf["cycle-aaaa-231"].Reason, "HEAD") {
		t.Errorf("bad HEAD leaf = %+v", byLeaf["cycle-aaaa-231"])
	}
	if byLeaf["cycle-aaaa-232"].Action != SalvageRetain || !strings.Contains(byLeaf["cycle-aaaa-232"].Reason, "within the last hour") {
		t.Errorf("fresh leaf = %+v", byLeaf["cycle-aaaa-232"])
	}
	if byLeaf["cycle-aaaa-233"].Action != SalvageEvict {
		t.Errorf("evictable leaf = %+v", byLeaf["cycle-aaaa-233"])
	}
}

func TestSalvageRepack_ThinBundleWithPrunedHeadIsKept(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/pruned-thin", "pruned-thin.txt")
	e.thinLeaf("cycle-aaaa-240", "lane/pruned-thin", tip)
	e.repo.Git("branch", "-D", "lane/pruned-thin")
	e.repo.Git("reflog", "expire", "--expire=now", "--all")
	e.repo.Git("gc", "-q", "--prune=now")

	it := e.planned("cycle-aaaa-240")
	if it.Action != SalvageKeep || !strings.Contains(it.Reason, "not in the repository") {
		t.Errorf("item = %+v, want keep with head not in repository", it)
	}
}

func TestSalvageRepack_FullBundleMigrationKeepCasesWhenScratchFails(t *testing.T) {
	cases := map[string]func(args []string) bool{
		"scratch init fails":     func(a []string) bool { return a[0] == "init" },
		"scratch fetch fails":    func(a []string) bool { return a[0] == "fetch" },
		"scratch rev-list fails": func(a []string) bool { return a[0] == "rev-list" },
	}
	for name, match := range cases {
		t.Run(name, func(t *testing.T) {
			e := newSalvageRepackEnv(t)
			tip := e.commitOn("lane/scratch-err", "scratch-err.txt")
			e.fullLeaf("cycle-aaaa-241", "lane/scratch-err", tip)
			e.repo.Git("branch", "-D", "lane/scratch-err")
			e.repo.Git("reflog", "expire", "--expire=now", "--all")
			e.repo.Git("gc", "-q", "--prune=now")
			e.failing(match)

			it := e.planned("cycle-aaaa-241")
			if it.Action != SalvageKeep || !strings.Contains(it.Reason, "not in the repository") {
				t.Errorf("item = %+v, want keep", it)
			}
		})
	}
}

func TestSalvageRepack_BuildThinBundleErrorCases(t *testing.T) {
	cases := map[string]func(args []string) bool{
		"fetch fails": func(a []string) bool { return a[0] == "fetch" },
		"update-ref no-deref fails": func(a []string) bool {
			return len(a) > 1 && a[0] == "update-ref" && a[1] == "--no-deref"
		},
	}
	for name, match := range cases {
		t.Run(name, func(t *testing.T) {
			e := newSalvageRepackEnv(t)
			tip := e.commitOn("lane/build-err", "build-err.txt")
			e.fullLeaf("cycle-aaaa-242", "lane/build-err", tip)
			e.repo.Git("branch", "-D", "lane/build-err")
			e.repo.Git("reflog", "expire", "--expire=now", "--all")
			e.repo.Git("gc", "-q", "--prune=now")
			item := e.planned("cycle-aaaa-242")
			if item.Action != SalvageRepack {
				t.Fatalf("setup: action = %q (%s)", item.Action, item.Reason)
			}
			before := treeDigest(t, e.leafDir("cycle-aaaa-242"))
			e.failing(match)

			freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{item})
			if err == nil || freed != 0 {
				t.Fatalf("freed=%d err=%v, want error", freed, err)
			}
			assertSameDigest(t, before, treeDigest(t, e.leafDir("cycle-aaaa-242")))
		})
	}
}

func TestPlanSalvageRetention_UnreadableHomeAndWalkErrorsAndTieBreak(t *testing.T) {
	e := newSalvageRepackEnv(t)
	if err := os.MkdirAll(e.opts.EvolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(OperatorSalvageDir(e.opts.EvolveDir), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanSalvageRetention(e.opts, 1<<20); err == nil {
		t.Fatalf("expected error when salvage home is a file")
	}
	if err := os.Remove(OperatorSalvageDir(e.opts.EvolveDir)); err != nil {
		t.Fatal(err)
	}

	tip := e.land("lane/tie", "tie.txt")
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	writePaddedLeaf(t, e, "cycle-aaaa-251", "lane/tie", tip, 10*24*time.Hour, 2<<20, nil)
	writePaddedLeaf(t, e, "cycle-aaaa-252", "lane/tie", tip, 10*24*time.Hour, 2<<20, nil)
	sameTime := time.Now().Add(-10 * 24 * time.Hour)
	for _, leaf := range []string{"cycle-aaaa-251", "cycle-aaaa-252"} {
		stampLeafAt(t, e.leafDir(leaf), sameTime)
	}
	if err := os.WriteFile(filepath.Join(OperatorSalvageDir(e.opts.EvolveDir), "stray.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	unreadableDir := filepath.Join(e.leafDir("cycle-aaaa-253"), "locked")
	if err := os.MkdirAll(unreadableDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadableDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadableDir, 0o755) })

	var plan SalvageRetentionPlan
	plan, err := PlanSalvageRetention(e.opts, 3<<20)
	if err != nil {
		t.Fatalf("PlanSalvageRetention: %v", err)
	}
	actions := map[string]SalvageRetentionAction{}
	reasons := map[string]string{}
	for _, it := range plan.Items {
		actions[it.Leaf] = it.Action
		reasons[it.Leaf] = it.Reason
	}
	if actions["cycle-aaaa-251"] != SalvageEvict || actions["cycle-aaaa-252"] != SalvageRetain {
		t.Errorf("tie-break actions = %+v, want 251 evict and 252 keep", actions)
	}
	if actions["cycle-aaaa-253"] != SalvageRetain || !strings.Contains(reasons["cycle-aaaa-253"], "unreadable") {
		t.Errorf("unreadable leaf = %q %q", actions["cycle-aaaa-253"], reasons["cycle-aaaa-253"])
	}
}

func stampLeafAt(t *testing.T, dir string, stamp time.Time) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if err := os.Chtimes(filepath.Join(dir, entry.Name()), stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(dir, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}

func TestPlanSalvageRetention_HoldAndContentAndBasisEdgeCases(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.land("lane/edges", "edges.txt")
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")

	writePaddedLeaf(t, e, "cycle-aaaa-261", "lane/edges", tip, 10*24*time.Hour, 1<<20, nil)
	if err := os.WriteFile(filepath.Join(e.opts.EvolveDir, "continuation-registry.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanSalvageRetention(e.opts, 1<<10)
	if err != nil {
		t.Fatalf("PlanSalvageRetention corrupt registry: %v", err)
	}
	if len(plan.Items) != 1 || plan.Items[0].Action != SalvageRetain || !strings.Contains(plan.Items[0].Reason, "hold state is unreadable") {
		t.Errorf("corrupt hold item = %+v", plan.Items)
	}
	if err := os.Remove(filepath.Join(e.opts.EvolveDir, "continuation-registry.json")); err != nil {
		t.Fatal(err)
	}

	if err := continuation.WriteRegistryEntry(e.opts.ProjectRoot, "item-bound", continuation.Continuation{Branch: "lane/bound", Cycle: 999}); err != nil {
		t.Fatal(err)
	}
	writePaddedLeaf(t, e, "cycle-aaaa-262", "lane/bound", tip, 10*24*time.Hour, 1<<20, nil)
	writePaddedLeaf(t, e, "cycle-aaaa-263", "lane/edges", tip, 10*24*time.Hour, 1<<20, map[string][]byte{salvagePatchFile: []byte("diff\n")})
	writePaddedLeaf(t, e, "cycle-aaaa-264", "lane/edges", tip, 10*24*time.Hour, 1<<20, nil)
	if err := os.Symlink(salvagePatchFile, filepath.Join(e.leafDir("cycle-aaaa-264"), salvagePatchFile)); err != nil {
		t.Fatal(err)
	}
	e.opts.Now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	writePaddedLeaf(t, e, "cycle-aaaa-265", "lane/missing", strings.Repeat("a", 40), 10*24*time.Hour, 1<<20, nil)

	plan, err = PlanSalvageRetention(e.opts, 1<<10)
	if err != nil {
		t.Fatalf("PlanSalvageRetention edges: %v", err)
	}
	byLeaf := map[string]SalvageRetentionItem{}
	for _, it := range plan.Items {
		byLeaf[it.Leaf] = it
	}
	wantReasons := map[string]string{
		"cycle-aaaa-262": "bound to a continuation",
		"cycle-aaaa-263": "uncommitted patch",
		"cycle-aaaa-264": "patch is unreadable",
		"cycle-aaaa-265": "not proven in origin/main",
	}
	for leaf, want := range wantReasons {
		it := byLeaf[leaf]
		if it.Action != SalvageRetain || !strings.Contains(it.Reason, want) {
			t.Errorf("%s: got %+v, want keep containing %q", leaf, it, want)
		}
	}
}

func TestApplySalvageRetention_EvictErrorPaths(t *testing.T) {
	cases := map[string]struct {
		arm    func(t *testing.T, e *salvageRepackEnv, tip, otherTip, leaf string)
		wantIs error
	}{
		"ship lock blocked": {
			arm: func(t *testing.T, e *salvageRepackEnv, _, _, _ string) {
				if err := os.MkdirAll(e.lockPath(), 0o755); err != nil {
					t.Fatal(err)
				}
			},
		},
		"leaf vanished": {
			arm: func(t *testing.T, e *salvageRepackEnv, _, _, leaf string) {
				if err := os.RemoveAll(e.leafDir(leaf)); err != nil {
					t.Fatal(err)
				}
			},
			wantIs: os.ErrNotExist,
		},
		"origin main deleted": {
			arm: func(_ *testing.T, e *salvageRepackEnv, _, _, _ string) {
				e.repo.Git("update-ref", "-d", "refs/remotes/origin/main")
			},
			wantIs: errSalvageNoOrigin,
		},
		"HEAD changed": {
			arm: func(t *testing.T, e *salvageRepackEnv, _, otherTip, leaf string) {
				if err := os.WriteFile(filepath.Join(e.leafDir(leaf), salvageHeadFile), []byte(otherTip+" lane/other\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				stampLeaf(t, e.leafDir(leaf), 10*24*time.Hour)
			},
			wantIs: errSalvageChanged,
		},
		"archive path is file": {
			arm: func(t *testing.T, e *salvageRepackEnv, _, _, _ string) {
				if err := os.WriteFile(filepath.Join(e.opts.EvolveDir, "archive"), []byte("file"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		"archive ref delete fails": {
			arm: func(_ *testing.T, e *salvageRepackEnv, tip, _, leaf string) {
				e.repo.Git("update-ref", archiveRef(leaf), tip)
				e.failing(func(a []string) bool { return len(a) > 1 && a[0] == "update-ref" && a[1] == "-d" })
			},
		},
		"remove leaf dir fails": {
			arm: func(t *testing.T, e *salvageRepackEnv, _, _, _ string) {
				home := OperatorSalvageDir(e.opts.EvolveDir)
				if err := os.Chmod(home, 0o555); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(home, 0o755) })
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := newSalvageRepackEnv(t)
			tip := e.land("lane/evict-a", "evict-a.txt")
			otherTip := e.land("lane/evict-b", "evict-b.txt")
			e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
			const leaf = "cycle-aaaa-271"
			writePaddedLeaf(t, e, leaf, "lane/evict-a", tip, 10*24*time.Hour, 2<<20, nil)
			plan, err := PlanSalvageRetention(e.opts, 1<<20)
			if err != nil || len(plan.Items) != 1 || plan.Items[0].Action != SalvageEvict {
				t.Fatalf("setup plan=%+v err=%v", plan, err)
			}
			tc.arm(t, e, tip, otherTip, leaf)

			freed, err := ApplySalvageRetention(e.opts, plan)
			if err == nil || freed != 0 {
				t.Fatalf("freed=%d err=%v, want error", freed, err)
			}
			if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
				t.Errorf("err = %v, want %v", err, tc.wantIs)
			}
		})
	}
}
