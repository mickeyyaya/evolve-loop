package gc

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

type salvageRepackEnv struct {
	*branchSweepEnv
}

func newSalvageRepackEnv(t *testing.T) *salvageRepackEnv {
	t.Helper()
	e := &salvageRepackEnv{newThinBundleEnv(t)}
	exclude := filepath.Join(e.repo.Dir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exclude, []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *salvageRepackEnv) leafDir(leaf string) string {
	return filepath.Join(OperatorSalvageDir(e.opts.EvolveDir), leaf)
}

func (e *salvageRepackEnv) bundlePath(leaf string) string {
	return filepath.Join(e.leafDir(leaf), salvageBundleFile)
}

func (e *salvageRepackEnv) fullLeaf(leaf, branch, tip string) {
	e.t.Helper()
	e.writeLeaf(leaf, branch, tip, "refs/heads/"+branch)
}

func (e *salvageRepackEnv) thinLeaf(leaf, branch, tip string) {
	e.t.Helper()
	e.writeLeaf(leaf, branch, tip, "refs/heads/"+branch, "^refs/remotes/origin/main")
}

func (e *salvageRepackEnv) writeLeaf(leaf, branch, tip string, revs ...string) {
	e.t.Helper()
	dir := e.leafDir(leaf)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		e.t.Fatal(err)
	}
	e.repo.Git(append([]string{"bundle", "create", e.bundlePath(leaf)}, revs...)...)
	if err := os.WriteFile(filepath.Join(dir, salvageHeadFile), []byte(tip+" "+branch+"\n"), 0o644); err != nil {
		e.t.Fatal(err)
	}
	e.age(leaf)
}

func (e *salvageRepackEnv) age(leaf string) {
	e.t.Helper()
	old := time.Now().Add(-3 * time.Hour)
	dir := e.leafDir(leaf)
	for _, p := range []string{filepath.Join(dir, salvageBundleFile), filepath.Join(dir, salvageHeadFile), dir} {
		if err := os.Chtimes(p, old, old); err != nil && !os.IsNotExist(err) {
			e.t.Fatal(err)
		}
	}
}

func (e *salvageRepackEnv) plan() map[string]SalvageRepackItem {
	e.t.Helper()
	items, err := PlanSalvageRepack(e.opts)
	if err != nil {
		e.t.Fatalf("PlanSalvageRepack: %v", err)
	}
	byLeaf := map[string]SalvageRepackItem{}
	for _, it := range items {
		byLeaf[it.Leaf] = it
	}
	return byLeaf
}

func (e *salvageRepackEnv) heads(leaf string) string {
	e.t.Helper()
	return e.repo.Git("bundle", "list-heads", e.bundlePath(leaf))
}

func (e *salvageRepackEnv) refs() string {
	e.t.Helper()
	return e.repo.Git("for-each-ref", "--format=%(refname) %(objectname)")
}

func (e *salvageRepackEnv) audit() []SalvageRepackRecord {
	e.t.Helper()
	raw, err := os.ReadFile(SalvageRepackAuditPath(e.opts.EvolveDir))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		e.t.Fatal(err)
	}
	var rows []SalvageRepackRecord
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var row SalvageRepackRecord
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			e.t.Fatalf("audit line %q: %v", line, err)
		}
		rows = append(rows, row)
	}
	return rows
}

func treeDigest(t *testing.T, root string) map[string]string {
	t.Helper()
	digest := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		sum := ""
		if d.Type().IsRegular() {
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			h := sha256.Sum256(raw)
			sum = string(h[:])
		}
		digest[p] = sum + "|" + info.ModTime().String() + "|" + info.Mode().String()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func assertSameDigest(t *testing.T, before, after map[string]string) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf("entries changed: %d before, %d after", len(before), len(after))
	}
	for p, d := range before {
		if after[p] != d {
			t.Errorf("%s changed", p)
		}
	}
}

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

func TestSalvageRepack_ARepackKeepsTheOldBundleOnEachFailedStep(t *testing.T) {
	steps := map[string]func(args []string) bool{
		"scratch init": func(a []string) bool { return a[0] == "init" },
		"set head ref": func(a []string) bool { return a[0] == "update-ref" },
		"create":       func(a []string) bool { return len(a) > 1 && a[0] == "bundle" && a[1] == "create" },
		"verify the new": func(a []string) bool {
			return len(a) > 2 && a[1] == "verify" && strings.HasSuffix(a[2], salvageRepackSuffix)
		},
		"list the new": func(a []string) bool {
			return len(a) > 2 && a[1] == "list-heads" && strings.HasSuffix(a[2], salvageRepackSuffix)
		},
		"locate the store": func(a []string) bool { return a[0] == "rev-parse" && len(a) > 1 && a[1] == "--path-format=absolute" },
	}
	for name, match := range steps {
		t.Run(name, func(t *testing.T) {
			e := newSalvageRepackEnv(t)
			tip := e.commitOn("lane/step", "step.txt")
			e.fullLeaf("cycle-aaaa-81", "lane/step", tip)
			items := e.plan()
			before := treeDigest(t, e.leafDir("cycle-aaaa-81"))
			e.failing(match)

			freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{items["cycle-aaaa-81"]})

			if err == nil || freed != 0 {
				t.Fatalf("freed = %d, err = %v, want a failure", freed, err)
			}
			assertSameDigest(t, before, treeDigest(t, e.leafDir("cycle-aaaa-81")))
		})
	}
}

func TestSalvageRepack_ANewBundleWithOtherHeadsIsRefused(t *testing.T) {
	e := newSalvageRepackEnv(t)
	tip := e.commitOn("lane/heads", "heads.txt")
	e.fullLeaf("cycle-aaaa-91", "lane/heads", tip)
	items := e.plan()
	before := treeDigest(t, e.leafDir("cycle-aaaa-91"))
	e.rewriting(func(a []string) bool {
		return len(a) > 2 && a[1] == "list-heads" && strings.HasSuffix(a[2], salvageRepackSuffix)
	}, strings.Repeat("0", 40)+" refs/heads/lane/heads\n")

	_, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{items["cycle-aaaa-91"]})

	if !errors.Is(err, errSalvageHeads) {
		t.Fatalf("err = %v, want a heads mismatch", err)
	}
	assertSameDigest(t, before, treeDigest(t, e.leafDir("cycle-aaaa-91")))
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

func sortedLines(s string) []string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	slices.Sort(lines)
	return lines
}

func fileSize(t *testing.T, p string) int64 {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

func (e *salvageRepackEnv) planned(leaf string) SalvageRepackItem {
	e.t.Helper()
	return e.plan()[leaf]
}

func (e *salvageRepackEnv) landedLeaf(leaf, branch string) SalvageRepackItem {
	e.t.Helper()
	tip := e.land(branch, branch+".txt")
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	e.fullLeaf(leaf, branch, tip)
	return e.planned(leaf)
}

func (e *salvageRepackEnv) lockPath() string {
	return filepath.Join(e.opts.ProjectRoot, ".evolve", "ship.lock")
}

func (e *salvageRepackEnv) observeShipLock() *[]bool {
	e.t.Helper()
	runDir := filepath.Join(e.opts.EvolveDir, "runs", "cycle-130")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "cycle-130", OwnerPID: 1}, time.Now()); err != nil {
		e.t.Fatal(err)
	}
	seen := &[]bool{}
	e.opts.PidAlive = func(int) bool {
		release, held, err := flock.TryLock(e.lockPath())
		if err != nil {
			e.t.Error(err)
		}
		if release != nil {
			release()
		}
		*seen = append(*seen, held)
		return false
	}
	return seen
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
	landed := e.land("lane/mix-a", "mix-a.txt")
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	open := e.commitOn("lane/mix-b", "mix-b.txt")
	_ = landed
	_ = open
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

func TestSalvageRepack_ARepackThatCannotBuildNamesTheFailedStepAndKeepsTheBundle(t *testing.T) {
	steps := map[string]func(t *testing.T, e *salvageRepackEnv){
		"no scratch dir": func(t *testing.T, e *salvageRepackEnv) {
			t.Setenv("TMPDIR", filepath.Join(e.opts.EvolveDir, "missing"))
		},
		"no object store link": func(t *testing.T, e *salvageRepackEnv) {
			e.rewriting(func(a []string) bool { return a[0] == "init" }, "")
		},
		"no basis ref": func(t *testing.T, e *salvageRepackEnv) {
			e.failing(func(a []string) bool { return a[0] == "update-ref" && a[1] == salvageOriginMainRef })
		},
		"tmp bundle vanished": func(t *testing.T, e *salvageRepackEnv) {
			inner := e.opts.Exec
			e.opts.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
				code, err := inner(ctx, name, dir, args, env, in, out, errw)
				if len(args) > 2 && args[1] == "list-heads" && strings.HasSuffix(args[2], salvageRepackSuffix) {
					_ = os.Remove(args[2])
				}
				return code, err
			}
		},
	}
	for name, arm := range steps {
		t.Run(name, func(t *testing.T) {
			e := newSalvageRepackEnv(t)
			tip := e.commitOn("lane/build", "build.txt")
			e.fullLeaf("cycle-aaaa-120", "lane/build", tip)
			item := e.planned("cycle-aaaa-120")
			before := treeDigest(t, e.leafDir("cycle-aaaa-120"))
			arm(t, e)

			freed, err := ApplySalvageRepack(e.opts, []SalvageRepackItem{item})

			if err == nil || freed != 0 {
				t.Fatalf("freed = %d, err = %v, want a failure", freed, err)
			}
			assertSameDigest(t, before, treeDigest(t, e.leafDir("cycle-aaaa-120")))
		})
	}
}

func TestSalvageRepack_ABundleWhoseHeadsCannotBeListedKeepsTheLeaf(t *testing.T) {
	cases := map[string]struct {
		arm  func(e *salvageRepackEnv)
		want string
	}{
		"list-heads fails": {func(e *salvageRepackEnv) {
			e.failing(func(a []string) bool { return len(a) > 1 && a[1] == "list-heads" })
		}, "does not verify"},
		"list-heads is empty": {func(e *salvageRepackEnv) {
			e.rewriting(func(a []string) bool { return len(a) > 1 && a[1] == "list-heads" }, "\n")
		}, "no readable head"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := newSalvageRepackEnv(t)
			tip := e.commitOn("lane/lh", "lh.txt")
			e.fullLeaf("cycle-aaaa-121", "lane/lh", tip)
			tc.arm(e)

			if it := e.planned("cycle-aaaa-121"); it.Action != SalvageKeep || !strings.Contains(it.Reason, tc.want) {
				t.Errorf("item = %+v", it)
			}
		})
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

func TestSalvageRepack_ACommitCountThatIsNotANumberKeepsTheBundle(t *testing.T) {
	for name, out := range map[string]string{"text": "many\n", "negative": "-3\n"} {
		t.Run(name, func(t *testing.T) {
			e := newSalvageRepackEnv(t)
			e.landedLeaf("cycle-aaaa-140", "lane/count"+name)
			e.rewriting(func(a []string) bool { return a[0] == "rev-list" }, out)

			it := e.planned("cycle-aaaa-140")

			if it.Action != SalvageKeep {
				t.Errorf("item = %+v, want keep", it)
			}
		})
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
