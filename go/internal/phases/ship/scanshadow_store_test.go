package ship

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/testverdict"
)

var enforceLimits = policy.ScanCacheConfig{Gate: "enforce", MaxBytes: 1 << 30, Budget: time.Minute}

func floorTestFn(ctx context.Context, moduleDir string, out packLog) packOutcome {
	return runRepoContractPackages(ctx, moduleDir, out, []string{"./internal/user", "./internal/xonly"})
}

func floorPack(t *testing.T, repo, store string) (reds []string, err error) {
	t.Helper()
	rec := newFloorRecorder(testverdict.NewStore(store, time.Now), sysexec.DefaultRunner, testLimits)
	var notes strings.Builder
	reds, _, err = runRecordedPack(context.Background(), rec, repo, floorTestFn, &notes)
	if !strings.Contains(notes.String(), "[build-floor] verdict store:") {
		t.Fatalf("the floor names what it recorded; got %q", notes.String())
	}
	return reds, err
}

func shipAt(store string, center *signalcenter.Center, limits policy.ScanCacheConfig) *scanShadow {
	return newScanShadow(testverdict.NewStore(store, time.Now), sysexec.DefaultRunner, center, shadowCycle, limits)
}

func storeStats(t *testing.T, store string) testverdict.Stats {
	t.Helper()
	st, err := testverdict.NewStore(store, time.Now).Stats()
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestVerdictStore_TheFloorRecordsItsGreensUnderItsOwnGate(t *testing.T) {
	t.Parallel()
	repo := shadowFixture(t)
	store := filepath.Join(t.TempDir(), shadowStoreDir)
	if reds, err := floorPack(t, repo, store); err != nil || len(reds) != 0 {
		t.Fatalf("a green floor: reds=%v err=%v", reds, err)
	}
	if st := storeStats(t, store); st.ByGate["build-floor"] != 2 || st.Entries != 2 {
		t.Fatalf("the floor stores both green packages under gate build-floor; got %+v", st)
	}
}

func TestVerdictStore_AtEnforceTheShipSkipsAPackageTheFloorStoredGreenOnTheSameKey(t *testing.T) {
	t.Parallel()
	repo := shadowFixture(t)
	store := filepath.Join(t.TempDir(), shadowStoreDir)
	floorPack(t, repo, store)

	center := signalcenter.New()
	run := scanOnce(t, repo, shipAt(store, center, enforceLimits), center)
	if run.err != nil {
		t.Fatalf("a green tree ships green at enforce: %v\n%s", run.err, run.notes)
	}
	for _, pkg := range []string{userPackage, xonlyPackage} {
		if strings.Contains(run.scanLog, "ok  \t"+pkg+"\t") {
			t.Errorf("%s has a green from the floor on the same key and must not run at enforce\n%s", pkg, run.scanLog)
		}
	}
	if !strings.Contains(run.scanLog, "[ship] verdict store: skipped 2 package(s)") {
		t.Fatalf("the ship names each skip in the scan log; got %q", run.scanLog)
	}
	if sum := run.summary(t); sum["skipped"] != "2" || sum["stage"] != "enforce" || sum["escapes"] != "0" {
		t.Fatalf("the summary counts the skips at stage enforce; got %v", sum)
	}
}

func TestVerdictStore_AChangedInputRunsThePackageAtEnforce(t *testing.T) {
	t.Parallel()
	repo := shadowFixture(t)
	store := filepath.Join(t.TempDir(), shadowStoreDir)
	floorPack(t, repo, store)
	mustWrite(t, filepath.Join(repo, "go", "internal", "other", "note.txt"), "a new input\n")

	center := signalcenter.New()
	run := scanOnce(t, repo, shipAt(store, center, enforceLimits), center)
	if run.err != nil {
		t.Fatalf("scan: %v", run.err)
	}
	if !strings.Contains(run.scanLog, "ok  \t"+userPackage+"\t") {
		t.Fatalf("a changed tree is a new key, so the package runs\n%s", run.scanLog)
	}
	if sum := run.summary(t); sum["skipped"] != "0" {
		t.Fatalf("nothing is skipped on a new key; got %v", sum)
	}
}

func TestVerdictStore_AtShadowEveryPackageRunsAndAMismatchNamesTheWritingGate(t *testing.T) {
	t.Parallel()
	repo := shadowFixture(t)
	store := filepath.Join(t.TempDir(), shadowStoreDir)
	floorPack(t, repo, store)
	switchOn(t, repo)

	center := signalcenter.New()
	run := scanOnce(t, repo, shipAt(store, center, testLimits), center)
	if run.err == nil {
		t.Fatal("the real run still decides at shadow: the red package fails the ship")
	}
	if !strings.Contains(run.scanLog, userPackage) || !strings.Contains(run.scanLog, "ok  \t"+xonlyPackage+"\t") {
		t.Fatalf("at shadow every package runs\n%s", run.scanLog)
	}
	escapes := run.escapes()
	if len(escapes) != 1 || escapes[0].Fields["gate"] != "build-floor" || escapes[0].Fields["package"] != userPackage {
		t.Fatalf("the mismatch is one escape that names the gate that stored the green; got %v", escapes)
	}
	if strings.Contains(run.notes, "[ship] verdict store:") {
		t.Fatalf("shadow writes no new note; got %q", run.notes)
	}
}

func TestVerdictStore_AFloorRedIsNeverStored(t *testing.T) {
	t.Parallel()
	repo := shadowFixture(t)
	switchOn(t, repo)
	store := filepath.Join(t.TempDir(), shadowStoreDir)
	if reds, err := floorPack(t, repo, store); err == nil || len(reds) == 0 {
		t.Fatalf("the floor names the red; reds=%v err=%v", reds, err)
	}
	if st := storeStats(t, store); st.ByGate["build-floor"] != 1 {
		t.Fatalf("only the green package is stored, the red never; got %+v", st)
	}
}

type listingTools struct {
	toolRun
	list    string
	listErr error
}

func (l listingTools) run(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if name == "go" && len(args) > 0 && args[0] == "list" {
		if l.listErr != nil {
			return -1, l.listErr
		}
		_, _ = io.WriteString(stdout, l.list)
		return 0, nil
	}
	return l.toolRun.run(ctx, name, dir, args, env, stdin, stdout, stderr)
}

func enforcingShadow(t *testing.T, store, root string, tools listingTools) *scanShadow {
	t.Helper()
	s := newScanShadow(testverdict.NewStore(store, time.Now), tools.run, signalcenter.New(), shadowCycle, enforceLimits)
	s.begin(context.Background(), root)
	return s
}

func seedGreen(t *testing.T, store, root string, tools listingTools, r pkgResult) {
	t.Helper()
	rec := newFloorRecorder(testverdict.NewStore(store, time.Now), tools.run, testLimits)
	rec.begin(context.Background(), root)
	rec.observe(packOutcome{pkgs: []pkgResult{r}})
	if rec.recorded != 1 {
		t.Fatalf("seed: recorded=%d storeErrors=%d unkeyed=%d degraded=%q", rec.recorded, rec.storeErrors, rec.unkeyed, rec.degraded)
	}
}

func TestScanShadowPrune_AChangedToolchainOrFlagRunsThePackage(t *testing.T) {
	t.Parallel()
	store, root := filepath.Join(t.TempDir(), shadowStoreDir), t.TempDir()
	tools := listingTools{toolRun: healthyTools(), list: "p/a\np/b\n"}
	args := repoContractTestArgs([]string{"./..."}, nil)
	flags, _ := splitTestArgs(args)
	seedGreen(t, store, root, tools, pkgResult{Package: "p/a", Passed: true, Flags: flags})

	same := enforcingShadow(t, store, root, tools)
	run, skipped := same.prune(context.Background(), root, args)
	if !slices.Equal(skipped, []string{"p/a"}) || !slices.Equal(run, append(slices.Clone(args[:len(args)-1]), "p/b")) {
		t.Fatalf("same key: p/a is skipped and p/b runs by its import path; run=%q skipped=%q", run, skipped)
	}

	newer := tools
	newer.goEnv = strings.Replace(arm64GoEnv, "go1.99", "go1.100", 1)
	if _, skipped := enforcingShadow(t, store, root, newer).prune(context.Background(), root, args); len(skipped) != 0 {
		t.Fatalf("another toolchain is another key; skipped %q", skipped)
	}
	byName := append(repoContractTestArgs(nil, nil), "-run", "^(TestX)$", "./...")
	if _, skipped := enforcingShadow(t, store, root, tools).prune(context.Background(), root, byName); len(skipped) != 0 {
		t.Fatalf("another flag is another key; skipped %q", skipped)
	}
}

func TestScanShadowPrune_AllSkippedRunsNothing(t *testing.T) {
	t.Parallel()
	store, root := filepath.Join(t.TempDir(), shadowStoreDir), t.TempDir()
	tools := listingTools{toolRun: healthyTools(), list: "p/a\n"}
	args := repoContractTestArgs([]string{"./internal/a"}, nil)
	flags, _ := splitTestArgs(args)
	seedGreen(t, store, root, tools, pkgResult{Package: "p/a", Passed: true, Flags: flags})
	s := enforcingShadow(t, store, root, tools)
	var notes strings.Builder
	o := runGoTestJSON(withScanShadow(context.Background(), s), root, packLog{notes: &notes, raw: io.Discard}, args)
	if !o.green() || len(o.pkgs) != 0 {
		t.Fatalf("a run whose every package is skipped is green and runs no go test; got %+v", o)
	}
	if !strings.Contains(notes.String(), "skipped 1 package(s)") || !strings.Contains(notes.String(), "p/a") {
		t.Fatalf("the skip is named; got %q", notes.String())
	}
}

func TestScanShadowPrune_RunsEverythingWhenItCannotDecide(t *testing.T) {
	t.Parallel()
	store, root := filepath.Join(t.TempDir(), shadowStoreDir), t.TempDir()
	args := repoContractTestArgs([]string{"./..."}, nil)
	cases := map[string]struct {
		s    *scanShadow
		args []string
	}{
		"a nil shadow":         {nil, args},
		"stage shadow":         {shadowFor(store, signalcenter.New()), args},
		"a degraded toolchain": {enforcingShadow(t, store, root, listingTools{toolRun: toolRun{goEnv: "not json"}}), args},
		"go list fails":        {enforcingShadow(t, store, root, listingTools{toolRun: healthyTools(), listErr: errors.New("no go")}), args},
		"a tagged tier":        {enforcingShadow(t, store, root, listingTools{toolRun: healthyTools(), list: "p/a\n"}), repoContractTestArgs([]string{"./..."}, []string{"integration"})},
		"no package":           {enforcingShadow(t, store, root, listingTools{toolRun: healthyTools()}), []string{"test", "-json"}},
	}
	for name, c := range cases {
		run, skipped := c.s.prune(context.Background(), root, c.args)
		if !slices.Equal(run, c.args) || skipped != nil {
			t.Errorf("%s: the run must keep its args; run=%q skipped=%q", name, run, skipped)
		}
	}
	failing := enforcingShadow(t, store, root, listingTools{toolRun: healthyTools(), listErr: errors.New("no go")})
	failing.prune(context.Background(), root, args)
	if !strings.Contains(failing.degraded, "no go") {
		t.Fatalf("a failed listing degrades the scan with its cause; got %q", failing.degraded)
	}
}

func TestScanShadowPrune_APanicKeepsTheRun(t *testing.T) {
	t.Parallel()
	center := signalcenter.New()
	root := t.TempDir()
	tools := listingTools{toolRun: healthyTools(), list: "p/a\n"}
	s := newScanShadow(testverdict.NewStore(filepath.Join(t.TempDir(), shadowStoreDir), panickingClock), tools.run, center, shadowCycle, enforceLimits)
	s.begin(context.Background(), root)
	s.store = nil
	args := repoContractTestArgs([]string{"./..."}, nil)
	run, skipped := s.prune(context.Background(), root, args)
	if !slices.Equal(run, args) || skipped != nil || !strings.Contains(s.degraded, "panic in prune") {
		t.Fatalf("a panic in prune runs every package and degrades; run=%q skipped=%q degraded=%q", run, skipped, s.degraded)
	}
}

func TestScanShadow_ATaggedTierOrATreeReadingPackageIsNeverCacheable(t *testing.T) {
	t.Parallel()
	s, center, dir := shadowWith(t, healthyTools())
	treeReader := "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	s.observe(packOutcome{pkgs: []pkgResult{
		{Package: "p/a", Passed: true, Tags: []string{"integration"}},
		{Package: "p/b", Passed: true, Tags: []string{"acs"}},
		{Package: treeReader, Passed: true},
		green("p/c"),
	}})
	s.finish()
	if sum := summaryFields(t, center); sum["uncacheable"] != "3" || sum["recorded"] != "1" || sum["packages"] != "4" {
		t.Fatalf("a tagged tier and a tree-reading package are counted and never stored; got %v", sum)
	}
	if st := storeStats(t, dir); st.Entries != 1 || st.ByGate["ship"] != 1 {
		t.Fatalf("only the untagged unit package is stored, under gate ship; got %+v", st)
	}
}

func TestFloorRecorderFor_FollowsTheDialAndNeedsAProjectRoot(t *testing.T) {
	t.Parallel()
	if floorRecorderFor("") != nil {
		t.Fatal("without a project root there is no store: no recorder")
	}
	root := t.TempDir()
	rec := floorRecorderFor(root)
	if rec == nil || rec.gate != floorGateID || rec.enforce {
		t.Fatalf("the default stage records as the floor and never skips; got %+v", rec)
	}
	mustWrite(t, filepath.Join(root, ".evolve", "policy.json"), `{"gates":{"scan_cache_gate":"enforce"}}`)
	if rec := floorRecorderFor(root); rec == nil || rec.enforce {
		t.Fatal("the floor runs every package at every stage")
	}
	mustWrite(t, filepath.Join(root, ".evolve", "policy.json"), `{"gates":{"scan_cache_gate":"off"}}`)
	if floorRecorderFor(root) != nil {
		t.Fatal("gates.scan_cache_gate=off turns the floor writer off")
	}
}

func TestRunRecordedPack_ANilRecorderRunsThePackUnchanged(t *testing.T) {
	t.Parallel()
	var notes strings.Builder
	reds, _, err := runRecordedPack(context.Background(), nil, "/lane", func(context.Context, string, packLog) packOutcome { return greenPack() }, &notes)
	if err != nil || len(reds) != 0 || notes.Len() != 0 {
		t.Fatalf("no recorder: the pack runs as before and prints nothing; reds=%v err=%v notes=%q", reds, err, notes.String())
	}
}

func TestFloorRecorder_AStoreItCannotWriteIsLoud(t *testing.T) {
	t.Parallel()
	blocked := filepath.Join(t.TempDir(), "blocked")
	mustWrite(t, blocked, "a file where the store directory should be\n")
	rec := newFloorRecorder(testverdict.NewStore(blocked, time.Now), healthyTools().run, testLimits)
	var notes strings.Builder
	_, _, err := runRecordedPack(context.Background(), rec, t.TempDir(), func(context.Context, string, packLog) packOutcome {
		return packOutcome{pkgs: []pkgResult{green("p/a")}}
	}, &notes)
	if err != nil || !strings.Contains(notes.String(), "1 store error(s)") {
		t.Fatalf("a failed write never fails the floor, and the floor names it; err=%v notes=%q", err, notes.String())
	}
}

func TestFloorRecorder_ADegradedKeyIsNamed(t *testing.T) {
	t.Parallel()
	rec := newFloorRecorder(testverdict.NewStore(filepath.Join(t.TempDir(), shadowStoreDir), time.Now), toolRun{goEnv: "not json"}.run, testLimits)
	var notes strings.Builder
	_, _, _ = runRecordedPack(context.Background(), rec, t.TempDir(), func(context.Context, string, packLog) packOutcome {
		return packOutcome{pkgs: []pkgResult{green("p/a")}}
	}, &notes)
	if !strings.Contains(notes.String(), "degraded") || !strings.Contains(notes.String(), "1 unkeyed") {
		t.Fatalf("the floor names a degraded key; got %q", notes.String())
	}
}

func TestScanShadow_TheShadowSummaryNamesItsStage(t *testing.T) {
	t.Parallel()
	s, center, _ := shadowWith(t, healthyTools())
	s.finish()
	if sum := summaryFields(t, center); sum["stage"] != "shadow" || sum["skipped"] != "0" || sum["uncacheable"] != "0" {
		t.Fatalf("got %v", sum)
	}
}

func TestScanShadowPrune_AnUncacheableOrUnkeyablePackageAlwaysRuns(t *testing.T) {
	t.Parallel()
	store, root := filepath.Join(t.TempDir(), shadowStoreDir), t.TempDir()
	treeReader := "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	tools := listingTools{toolRun: healthyTools(), list: treeReader + "\n"}
	args := repoContractTestArgs([]string{"./cmd/evolve"}, nil)
	if _, skipped := enforcingShadow(t, store, root, tools).prune(context.Background(), root, args); skipped != nil {
		t.Fatalf("a package with a tree-reading test is never skipped; skipped %q", skipped)
	}
	shuffled := append(repoContractTestArgs(nil, nil), "-shuffle=on", "./...")
	if _, skipped := enforcingShadow(t, store, root, listingTools{toolRun: healthyTools(), list: "p/a\n"}).prune(context.Background(), root, shuffled); skipped != nil {
		t.Fatalf("a run that has no key is never skipped; skipped %q", skipped)
	}
}

func TestSplitPackages_NeverWritesIntoTheArgsOfTheCaller(t *testing.T) {
	t.Parallel()
	args := []string{"test", "./a", "-json", "-run", "^(X)$", "./b", "-count=1"}
	before := slices.Clone(args)
	flagArgs, patterns := splitPackages(args)
	if !slices.Equal(args, before) {
		t.Fatalf("the args of the run must stay as they were: %q, want %q", args, before)
	}
	if !slices.Equal(flagArgs, []string{"test", "-json", "-run", "^(X)$", "-count=1"}) || !slices.Equal(patterns, []string{"./a", "./b"}) {
		t.Fatalf("flags %q patterns %q", flagArgs, patterns)
	}
}

func TestScanShadow_AFailedSecondBeginForgetsTheOldTree(t *testing.T) {
	t.Parallel()
	store, root := filepath.Join(t.TempDir(), shadowStoreDir), t.TempDir()
	tools := listingTools{toolRun: healthyTools(), list: "p/a\n"}
	args := repoContractTestArgs([]string{"./..."}, nil)
	flags, _ := splitTestArgs(args)
	seedGreen(t, store, root, tools, pkgResult{Package: "p/a", Passed: true, Flags: flags})
	s := enforcingShadow(t, store, root, tools)
	s.run = listingTools{toolRun: toolRun{gitError: errors.New("git is gone")}, list: "p/a\n"}.run
	s.begin(context.Background(), t.TempDir())
	if s.treeID != "" || s.goVersion != "" {
		t.Fatalf("a failed begin must not keep the key of the earlier tree; tree=%q go=%q", s.treeID, s.goVersion)
	}
	if _, skipped := s.prune(context.Background(), root, args); skipped != nil {
		t.Fatalf("a tree that was never hashed skips nothing; skipped %q", skipped)
	}
}

func TestScanShadow_TheFirstStoreErrorKeepsItsCause(t *testing.T) {
	t.Parallel()
	blocked := filepath.Join(t.TempDir(), "blocked")
	mustWrite(t, blocked, "a file where the store directory should be\n")
	rec := newFloorRecorder(testverdict.NewStore(blocked, time.Now), healthyTools().run, testLimits)
	rec.begin(context.Background(), t.TempDir())
	rec.observe(packOutcome{pkgs: []pkgResult{green("p/a"), green("p/b")}})
	if rec.storeErrors != 2 || strings.Count(rec.degraded, "store: ") != 1 {
		t.Fatalf("two failed writes, one named cause; errors=%d degraded=%q", rec.storeErrors, rec.degraded)
	}
}

func TestScanShadowPrune_AnUnkeyableRunNamesItsCause(t *testing.T) {
	t.Parallel()
	store, root := filepath.Join(t.TempDir(), shadowStoreDir), t.TempDir()
	s := enforcingShadow(t, store, root, listingTools{toolRun: healthyTools(), list: "p/a\n"})
	s.prune(context.Background(), root, append(repoContractTestArgs(nil, nil), "-shuffle=on", "./..."))
	if !strings.Contains(s.degraded, "key for the skip:") {
		t.Fatalf("got %q", s.degraded)
	}
}
