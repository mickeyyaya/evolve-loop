package gc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestSalvageRepack_AFullHistoryBundleBecomesAThinBundleWithTheSameHeads(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/full", "full.txt")
	e.fullLeaf("cycle-aaaa-11", "lane/full", tip)
	headsBefore := e.heads("cycle-aaaa-11")
	refsBefore := e.refs()
	bytesBefore := fileSize(t, e.bundlePath("cycle-aaaa-11"))

	items := e.plan()
	if got := items["cycle-aaaa-11"].Action; got != SalvageRepack {
		t.Fatalf("action = %q (%s), want repack", got, items["cycle-aaaa-11"].Reason)
	}
	freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{items["cycle-aaaa-11"]})
	if err != nil {
		t.Fatalf("ApplySalvageRepack: %v", err)
	}

	if got := e.heads("cycle-aaaa-11"); got != headsBefore {
		t.Errorf("heads = %q, want %q", got, headsBefore)
	}
	e.repo.Git("bundle", "verify", e.bundlePath("cycle-aaaa-11"))
	after := fileSize(t, e.bundlePath("cycle-aaaa-11"))
	if after >= thinBundleLimit {
		t.Errorf("new bundle is %d bytes, want under %d", after, thinBundleLimit)
	}
	if freed != bytesBefore-after {
		t.Errorf("freed = %d, want %d", freed, bytesBefore-after)
	}
	if e.refs() != refsBefore {
		t.Errorf("the hub refs changed")
	}
	if left, _ := filepath.Glob(filepath.Join(e.leafDir("cycle-aaaa-11"), "*"+salvageRepackSuffix)); len(left) != 0 {
		t.Errorf("temp files left: %v", left)
	}
}

func TestSalvageRepack_TheThinBundleRestoresTheBranchInACloneOfOriginMain(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/restore", "restore.txt")
	e.fullLeaf("cycle-aaaa-12", "lane/restore", tip)
	items := e.plan()
	if _, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{items["cycle-aaaa-12"]}); err != nil {
		t.Fatal(err)
	}

	clone := gittest.Clone(t, e.repo.Dir)
	clone.Git("fetch", "-q", e.bundlePath("cycle-aaaa-12"), "refs/heads/lane/restore:refs/heads/restored")
	if got := clone.Git("rev-parse", "restored"); got != tip {
		t.Errorf("restored tip = %q, want %q", got, tip)
	}
}

func TestSalvageRepack_ARepackKeepsTheLeafDirectoryAgeAndWritesOneAuditRow(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/age", "age.txt")
	e.fullLeaf("cycle-aaaa-13", "lane/age", tip)
	before, err := os.Stat(e.leafDir("cycle-aaaa-13"))
	if err != nil {
		t.Fatal(err)
	}
	items := e.plan()
	if _, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{items["cycle-aaaa-13"]}); err != nil {
		t.Fatal(err)
	}

	after, err := os.Stat(e.leafDir("cycle-aaaa-13"))
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("leaf dir mtime = %v, want %v", after.ModTime(), before.ModTime())
	}
	rows := e.audit()
	if len(rows) != 1 || rows[0].Action != SalvageRepack || rows[0].Leaf != "cycle-aaaa-13" || rows[0].Basis != items["cycle-aaaa-13"].Basis {
		t.Errorf("audit rows = %v", rows)
	}
}

func TestSalvageRepack_ABundleWhoseHeadsAreInOriginMainIsDroppedAndTheLeafStays(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.land("lane/landed", "landed.txt")
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	e.fullLeaf("cycle-aaaa-14", "lane/landed", tip)
	patch := filepath.Join(e.leafDir("cycle-aaaa-14"), salvagePatchFile)
	if err := os.WriteFile(patch, []byte("diff\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.age("cycle-aaaa-14")

	leafBefore, err := os.Stat(e.leafDir("cycle-aaaa-14"))
	if err != nil {
		t.Fatal(err)
	}
	items := e.plan()
	if got := items["cycle-aaaa-14"].Action; got != SalvageDropRedundant {
		t.Fatalf("action = %q (%s), want drop-redundant", got, items["cycle-aaaa-14"].Reason)
	}
	freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{items["cycle-aaaa-14"]})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(e.bundlePath("cycle-aaaa-14")); !os.IsNotExist(err) {
		t.Errorf("refs.bundle still there: %v", err)
	}
	leafAfter, err := os.Stat(e.leafDir("cycle-aaaa-14"))
	if err != nil || !leafAfter.ModTime().Equal(leafBefore.ModTime()) {
		t.Errorf("leaf dir mtime = %v, %v, want %v", leafAfter.ModTime(), err, leafBefore.ModTime())
	}
	for _, keep := range []string{salvageHeadFile, salvagePatchFile} {
		if _, err := os.Stat(filepath.Join(e.leafDir("cycle-aaaa-14"), keep)); err != nil {
			t.Errorf("%s gone: %v", keep, err)
		}
	}
	if freed != items["cycle-aaaa-14"].Bytes {
		t.Errorf("freed = %d, want %d", freed, items["cycle-aaaa-14"].Bytes)
	}
	if rows := e.audit(); len(rows) != 1 || rows[0].Action != SalvageDropRedundant {
		t.Errorf("audit rows = %v", rows)
	}
}

func TestSalvageRepack_ADropIsRefusedWhenOriginMainNoLongerHoldsTheHeads(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.land("lane/rewound", "rewound.txt")
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	e.fullLeaf("cycle-aaaa-15", "lane/rewound", tip)
	items := e.plan()
	if items["cycle-aaaa-15"].Action != SalvageDropRedundant {
		t.Fatalf("setup: action = %q", items["cycle-aaaa-15"].Action)
	}
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main~1")

	freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{items["cycle-aaaa-15"]})
	if !errors.Is(err, errSalvageRewound) || freed != 0 {
		t.Fatalf("freed = %d, err = %v, want a refusal", freed, err)
	}
	if _, statErr := os.Stat(e.bundlePath("cycle-aaaa-15")); statErr != nil {
		t.Errorf("bundle gone: %v", statErr)
	}
	if rows := e.audit(); len(rows) != 0 {
		t.Errorf("audit rows = %v, want none", rows)
	}
}

func TestSalvageRepack_KeepCasesNameTheirReasonAndChangeNothing(t *testing.T) {
	e := newSalvageRepackEnv(t)
	corruptTip := e.commitOn("lane/corrupt", "corrupt.txt")
	thinTip := e.commitOn("lane/thin", "thin.txt")
	freshTip := e.commitOn("lane/fresh", "fresh.txt")
	other := gittest.Fixture(t)
	commitRepoFile(t, other, "other.txt", "other\n")

	e.fullLeaf("cycle-aaaa-21", "lane/corrupt", corruptTip)
	if err := os.WriteFile(e.bundlePath("cycle-aaaa-21"), []byte("not a bundle"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.age("cycle-aaaa-21")
	e.thinLeaf("cycle-aaaa-22", "lane/thin", thinTip)
	if err := os.MkdirAll(e.leafDir("cycle-aaaa-23"), 0o755); err != nil {
		t.Fatal(err)
	}
	other.Git("bundle", "create", e.bundlePath("cycle-aaaa-23"), "HEAD")
	e.age("cycle-aaaa-23")
	e.fullLeaf("cycle-aaaa-24", "lane/fresh", freshTip)
	now := time.Now()
	for _, p := range []string{e.bundlePath("cycle-aaaa-24"), e.leafDir("cycle-aaaa-24")} {
		if err := os.Chtimes(p, now, now); err != nil {
			t.Fatal(err)
		}
	}

	cases := map[string]string{
		"cycle-aaaa-21": "does not verify",
		"cycle-aaaa-22": "already incremental",
		"cycle-aaaa-23": "not in the repository",
		"cycle-aaaa-24": "changed within",
	}
	items := e.plan()
	var list []SalvageRepackItem
	for leaf, want := range cases {
		it := items[leaf]
		if it.Action != SalvageKeep || !strings.Contains(it.Reason, want) {
			t.Errorf("%s: action %q reason %q, want keep containing %q", leaf, it.Action, it.Reason, want)
		}
		list = append(list, it)
	}
	before := treeDigest(t, e.opts.EvolveDir)
	freed, err := ApplySalvageRepack(e.opts, list)
	if err != nil || freed != 0 {
		t.Fatalf("freed = %d, err = %v", freed, err)
	}
	assertSameDigest(t, before, treeDigest(t, e.opts.EvolveDir))
}

func TestSalvageRepack_ALeafWithNoBundleIsNotListed(t *testing.T) {
	e := newSalvageRepackEnv(t)
	dir := e.leafDir("cycle-aaaa-31")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, salvageHeadFile), []byte("x y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loop := e.leafDir("cycle-aaaa-32")
	if err := os.MkdirAll(loop, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(salvageBundleFile, filepath.Join(loop, salvageBundleFile)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(OperatorSalvageDir(e.opts.EvolveDir), "stray.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	items := e.plan()
	if _, listed := items["cycle-aaaa-31"]; listed {
		t.Errorf("a leaf with no bundle is listed")
	}
	if it := items["cycle-aaaa-32"]; it.Action != SalvageKeep || !strings.Contains(it.Reason, "unreadable") {
		t.Errorf("unreadable bundle: %+v", it)
	}
}

func TestSalvageRepack_ARunningCycleIsNeverTouched(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tipA := e.commitOn("lane/livea", "livea.txt")
	tipB := e.commitOn("lane/liveb", "liveb.txt")
	e.fullLeaf("cycle-aaaa-41", "lane/livea", tipA)
	e.fullLeaf("cycle-aaaa-42", "lane/liveb", tipB)
	runDir := filepath.Join(e.opts.EvolveDir, "runs", "cycle-41")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "cycle-41", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	state := `{"active_worktree": "` + filepath.Join(e.opts.WorktreeBase, "cycle-aaaa-42") + `"}`
	if err := os.WriteFile(filepath.Join(e.opts.EvolveDir, "cycle-state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}

	items := e.plan()
	for _, leaf := range []string{"cycle-aaaa-41", "cycle-aaaa-42"} {
		if it := items[leaf]; it.Action != SalvageKeep || !strings.Contains(it.Reason, "live") {
			t.Errorf("%s: %+v, want keep as live", leaf, it)
		}
	}
}

func TestSalvageRepack_APausedCycleIsKept(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/paused", "paused.txt")
	e.fullLeaf("cycle-aaaa-51", "lane/paused", tip)
	state := map[string]any{"cycle_id": 51, "phase": "build", "checkpoint": map[string]any{
		"enabled": true, "reason": "quota-likely", "resumeFromPhase": "build", "worktreePath": e.leafDir("cycle-aaaa-51"),
		"savedAt": time.Now().UTC().Format(time.RFC3339), "autoResumeAttempts": 0, "autoResumeMaxAttempts": 3,
	}}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(e.opts.EvolveDir, "runs", "cycle-51")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "cycle-state.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	if it := e.plan()["cycle-aaaa-51"]; it.Action != SalvageKeep || !strings.Contains(it.Reason, "paused") {
		t.Errorf("item = %+v, want keep as paused", it)
	}
}

func TestSalvageRepack_TheDryRunPlanChangesNoByteOfTheSalvageHome(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/dry", "dry.txt")
	e.fullLeaf("cycle-aaaa-61", "lane/dry", tip)
	before := treeDigest(t, e.opts.EvolveDir)
	refs := e.refs()

	items := e.plan()

	if items["cycle-aaaa-61"].Action != SalvageRepack {
		t.Fatalf("action = %q", items["cycle-aaaa-61"].Action)
	}
	assertSameDigest(t, before, treeDigest(t, e.opts.EvolveDir))
	if e.refs() != refs {
		t.Errorf("hub refs changed")
	}
}

func TestPlanSalvageRepack_RefusesWhenOriginMainIsMissing(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/nomain", "nomain.txt")
	e.fullLeaf("cycle-aaaa-93", "lane/nomain", tip)
	e.repo.Git("update-ref", "-d", "refs/remotes/origin/main")

	if _, err := PlanSalvageRepack(e.opts); !errors.Is(err, errSalvageNoOrigin) {
		t.Errorf("err = %v, want a refusal that names origin/main", err)
	}
}

func TestPlanSalvageRepack_ASalvageHomeThatDoesNotExistIsEmpty(t *testing.T) {
	e := newSalvageRepackEnv(t)

	items, err := PlanSalvageRepack(e.opts)

	if err != nil || len(items) != 0 {
		t.Errorf("items = %v, err = %v", items, err)
	}
}

func TestPlanSalvageRepack_AnUnreadableSalvageHomeIsAnError(t *testing.T) {
	e := newSalvageRepackEnv(t)
	if err := os.MkdirAll(e.opts.EvolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(OperatorSalvageDir(e.opts.EvolveDir), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := PlanSalvageRepack(e.opts); err == nil {
		t.Errorf("want an error for a salvage home that is a file")
	}
}

func TestSalvageRepack_ASalvagedHeadWithTwoRefsKeepsBothInTheNewBundle(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tipA := e.commitOn("lane/two-a", "two-a.txt")
	tipB := e.commitOn("lane/two-b", "two-b.txt")
	if err := os.MkdirAll(e.leafDir("cycle-aaaa-112"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.repo.Git("bundle", "create", e.bundlePath("cycle-aaaa-112"), "refs/heads/lane/two-a", "refs/heads/lane/two-b")
	if err := os.WriteFile(filepath.Join(e.leafDir("cycle-aaaa-112"), salvageHeadFile), []byte(tipA+" lane/two-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.age("cycle-aaaa-112")
	before := e.heads("cycle-aaaa-112")
	item := e.planned("cycle-aaaa-112")
	if _, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{item}); err != nil {
		t.Fatal(err)
	}

	got := e.heads("cycle-aaaa-112")
	if !slices.Equal(sortedLines(got), sortedLines(before)) || !strings.Contains(got, tipB) {
		t.Errorf("heads = %q, want %q", got, before)
	}
}

func TestSalvageRepack_ABundleWithSomeHeadsInOriginMainIsKept(t *testing.T) {
	e := newSalvageRepackEnv(t)
	e.land("lane/mix-a", "mix-a.txt")
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	e.commitOn("lane/mix-b", "mix-b.txt")
	if err := os.MkdirAll(e.leafDir("cycle-aaaa-113"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.repo.Git("bundle", "create", e.bundlePath("cycle-aaaa-113"), "refs/heads/lane/mix-a", "refs/heads/lane/mix-b")
	e.age("cycle-aaaa-113")

	if it := e.planned("cycle-aaaa-113"); it.Action != SalvageKeep || !strings.Contains(it.Reason, "some heads") {
		t.Errorf("item = %+v", it)
	}
}

func TestSalvageRepack_AnAuditRowNamesTheOldAndNewSizes(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/rows", "rows.txt")
	e.fullLeaf("cycle-aaaa-114", "lane/rows", tip)
	item := e.planned("cycle-aaaa-114")
	freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{item})
	if err != nil {
		t.Fatal(err)
	}

	rows := e.audit()
	if len(rows) != 1 || rows[0].BytesBefore != item.Bytes || rows[0].BytesBefore-rows[0].BytesAfter != freed {
		t.Errorf("rows = %v, freed = %d", rows, freed)
	}
}

func TestSalvageRepack_ALeafWithoutACycleNumberIsJudgedByItsFilesAlone(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/plain", "plain.txt")
	e.fullLeaf("branch-lane-plain-abcdef012345", "lane/plain", tip)

	if it := e.planned("branch-lane-plain-abcdef012345"); it.Action != SalvageRepack {
		t.Errorf("item = %+v, want repack", it)
	}
}

func TestSalvageRepack_AnUnreadableHoldStateKeepsTheLeaf(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/hold", "hold.txt")
	e.fullLeaf("cycle-aaaa-122", "lane/hold", tip)
	if err := os.WriteFile(filepath.Join(e.opts.EvolveDir, "continuation-registry.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if it := e.planned("cycle-aaaa-122"); it.Action != SalvageKeep || !strings.Contains(it.Reason, "hold state is unreadable") {
		t.Errorf("item = %+v", it)
	}
}

func TestPlanSalvageRepack_TheBundleVerifyRunsUnderTheCLocale(t *testing.T) {
	t.Setenv("LC_ALL", "fr_FR.UTF-8")
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/locale", "locale.txt")
	e.fullLeaf("cycle-aaaa-121", "lane/locale", tip)
	var verifyEnvs [][]string
	inner := e.opts.Exec
	e.opts.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if slices.Contains(args, "bundle") && slices.Contains(args, "verify") {
			verifyEnvs = append(verifyEnvs, env)
		}
		return inner(ctx, name, dir, args, env, in, out, errw)
	}

	e.plan()

	if len(verifyEnvs) == 0 {
		t.Fatal("no bundle verify call was seen")
	}
	for _, env := range verifyEnvs {
		if !containsEnv(env, "LC_ALL=C") {
			t.Errorf("bundle verify ran with env %v, want LC_ALL=C", env)
		}
	}
}
