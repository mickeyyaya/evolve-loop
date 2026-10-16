package ship

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/testkey"
	"github.com/mickeyyaya/evolve-loop/go/internal/testverdict"
)

const arm64GoEnv = `{"GOVERSION":"go1.99","GOOS":"darwin","GOARCH":"arm64","GOFLAGS":"","GOEXPERIMENT":"","CGO_ENABLED":"1","CC":"cc","CXX":"c++","CGO_CFLAGS":"-g","CGO_CXXFLAGS":"-g","CGO_LDFLAGS":"-g","GOARM64":"v8.0","GOWASM":"","GO386":"","GOARM":""}`

type toolRun struct {
	files    string
	goEnv    string
	gitCode  int
	goCode   int
	gitError error
}

func (r toolRun) run(_ context.Context, name, _ string, _, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
	switch name {
	case "git":
		if r.gitError != nil {
			return -1, r.gitError
		}
		_, _ = io.WriteString(stdout, r.files)
		return r.gitCode, nil
	default:
		_, _ = io.WriteString(stdout, r.goEnv)
		return r.goCode, nil
	}
}

func healthyTools() toolRun { return toolRun{goEnv: arm64GoEnv} }

func shadowWith(t *testing.T, tools toolRun) (*scanShadow, *signalcenter.Center, string) {
	t.Helper()
	center := signalcenter.New()
	dir := filepath.Join(t.TempDir(), shadowStoreDir)
	s := newScanShadow(testverdict.NewStore(dir, time.Now), tools.run, center, shadowCycle, testLimits)
	s.begin(context.Background(), t.TempDir())
	return s, center, dir
}

func green(pkg string, flags ...string) pkgResult {
	return pkgResult{Package: pkg, Passed: true, Flags: flags}
}

func TestScanShadow_ANilShadowIsInert(t *testing.T) {
	t.Parallel()
	var s *scanShadow
	s.begin(context.Background(), t.TempDir())
	s.observe(packOutcome{pkgs: []pkgResult{green("p")}})
	s.finish()
	if got := scanShadowFrom(context.Background()); got != nil {
		t.Fatalf("a context without a shadow yields none; got %v", got)
	}
	if got := scanShadowFrom(withScanShadow(context.Background(), nil)); got != nil {
		t.Fatalf("a nil shadow in the context stays inert; got %v", got)
	}
}

func TestScanShadow_ARepeatedKeyInOneScanIsSettledOnce(t *testing.T) {
	t.Parallel()
	s, center, _ := shadowWith(t, healthyTools())
	s.observe(packOutcome{pkgs: []pkgResult{green("p/a"), green("p/a"), green("p/a", "-run", "^(X)$")}})
	s.finish()
	sum := summaryFields(t, center)
	if sum["packages"] != "3" || sum["recorded"] != "2" || sum["hits"] != "0" {
		t.Fatalf("the same run of the same package twice in one scan is one record, never a hit on our own write; a different -run is another key; got %v", sum)
	}
}

func TestScanShadow_ARedWithNoStoredGreenIsNeitherHitNorEscape(t *testing.T) {
	t.Parallel()
	s, center, dir := shadowWith(t, healthyTools())
	s.observe(packOutcome{pkgs: []pkgResult{{Package: "p/red"}}})
	s.finish()
	sum := summaryFields(t, center)
	if sum["recorded"] != "0" || sum["escapes"] != "0" || sum["hits"] != "0" || sum["packages"] != "1" {
		t.Fatalf("a red is never stored and a miss is no escape; got %v", sum)
	}
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a red must leave no file in the store: %v", err)
	}
}

func TestScanShadow_AStoreThatCannotWriteIsCountedAndNeverFailsTheScan(t *testing.T) {
	t.Parallel()
	center := signalcenter.New()
	blocked := filepath.Join(t.TempDir(), "blocked")
	mustWrite(t, blocked, "a file where the store directory should be\n")
	s := newScanShadow(testverdict.NewStore(blocked, time.Now), healthyTools().run, center, shadowCycle, testLimits)
	s.begin(context.Background(), t.TempDir())
	s.observe(packOutcome{pkgs: []pkgResult{green("p/a")}})
	s.finish()
	sum := summaryFields(t, center)
	if sum["store_errors"] != "1" || sum["recorded"] != "0" {
		t.Fatalf("a failed write is counted, not raised; got %v", sum)
	}
}

func TestScanShadow_ADegradedToolchainKeysNothingAndSaysWhy(t *testing.T) {
	t.Parallel()
	cases := map[string]toolRun{
		"git cannot list the tree": {goEnv: arm64GoEnv, gitError: errors.New("git is gone")},
		"go env fails":             {goEnv: "", goCode: 2},
		"go env is not json":       {goEnv: "not json"},
	}
	for name, tools := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s, center, _ := shadowWith(t, tools)
			s.observe(packOutcome{pkgs: []pkgResult{green("p/a")}})
			s.finish()
			sum := summaryFields(t, center)
			if sum["unkeyed"] != "1" || sum["recorded"] != "0" || sum["degraded"] == "" {
				t.Fatalf("an unkeyable scan records nothing, counts the package unkeyed and names the cause; got %v", sum)
			}
		})
	}
}

func summaryFields(t *testing.T, c *signalcenter.Center) map[string]string {
	t.Helper()
	return scanRun{events: c.Recent()}.summary(t)
}

func TestTreeID_FollowsContentAndIgnoresDeletedFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.go"), "package a\n")
	mustWrite(t, filepath.Join(root, "sub", "b.go"), "package b\n")
	if err := os.Symlink("a.go", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, filepath.Join(root, "gitlink"))
	list := "sub/b.go\x00a.go\x00link\x00gitlink\x00gone.go\x00a.go\x00"
	id := func(listing string) string {
		t.Helper()
		got, err := treeID(context.Background(), toolRun{files: listing}.run, root, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	base := id(list)
	if again := id("a.go\x00gone.go\x00link\x00sub/b.go\x00gitlink\x00"); again != base {
		t.Fatalf("the id is a function of the set of files, not of the listing order or repeats")
	}
	if len(base) != 64 {
		t.Fatalf("id = %q, want a sha256 hex digest", base)
	}
	mustWrite(t, filepath.Join(root, "sub", "b.go"), "package b\n\nconst Edited = 1\n")
	if edited := id(list); edited == base {
		t.Fatal("a one-byte edit of an unstaged file must change the id: the scan runs the files on disk")
	}
	if err := os.Chmod(filepath.Join(root, "a.go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if moded := id(list); moded == base {
		t.Fatal("a mode change is part of the tree")
	}
	if empty := id(""); empty != hex.EncodeToString(sha256.New().Sum(nil)) {
		t.Fatalf("an empty tree has the digest of nothing, got %q", empty)
	}
}

func TestTreeID_AnUnstatablePathOrAFailingGitIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "file"), "x\n")
	if _, err := treeID(context.Background(), toolRun{files: "file/under-a-file\x00"}.run, root, 1<<20); err == nil {
		t.Fatal("a path below a regular file is an error, not a skipped file")
	}
	if _, err := treeID(context.Background(), toolRun{gitCode: 128}.run, root, 1<<20); err == nil {
		t.Fatal("a failing git listing is an error")
	}
}

func TestFileID_AFileThatCannotBeReadIsAnError(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads every file")
	}
	path := filepath.Join(t.TempDir(), "locked")
	mustWrite(t, path, "x\n")
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fileID(context.Background(), path, 1<<20); err == nil {
		t.Fatal("a file that cannot be opened is an error")
	}
}

func TestGoEnv_FillsThePlatformVariablesTheToolchainOmits(t *testing.T) {
	t.Parallel()
	version, env, err := goEnv(context.Background(), healthyTools().run, t.TempDir())
	if err != nil || version != "go1.99" {
		t.Fatalf("version = %q err = %v", version, err)
	}
	for _, name := range testkey.BuildEnvNames() {
		if _, ok := env[name]; !ok {
			t.Errorf("%s must be present, the key refuses a build environment with a gap", name)
		}
	}
	if env["GOARCH"] != "arm64" || env["GOAMD64"] != "" {
		t.Fatalf("listed variables keep their value and absent ones are empty; got GOARCH=%q GOAMD64=%q", env["GOARCH"], env["GOAMD64"])
	}
}

func TestPkgResultTap_ReadsOnlyThePackageLevelResults(t *testing.T) {
	t.Parallel()
	tap := newPkgResultTap([]string{"test", "-json", "-count=1", "-timeout", "5m", "-tags", "integration,acs", "./a", "./b"})
	stream := `{"Action":"run","Package":"p/a","Test":"TestX"}
{"Action":"pass","Package":"p/a","Test":"TestX"}
{"Action":"skip","Package":"p/none"}
not json at all
{"Action":"output","Package":"p/a","Output":"ok\n"}
{"Action":"pass","Package":"p/a"}
{"Action":"fail","Package":"p/b","FailedBuild":"p/b"}
{"Action":"pass"}
`
	for _, chunk := range []string{stream[:37], stream[37:120], stream[120:]} {
		if n, err := io.WriteString(tap, chunk); n != len(chunk) || err != nil {
			t.Fatalf("write = %d, %v", n, err)
		}
	}
	want := []pkgResult{
		{Package: "p/a", Passed: true, Tags: []string{"integration", "acs"}, Flags: []string{"-json", "-count=1", "-timeout", "5m"}},
		{Package: "p/b", Tags: []string{"integration", "acs"}, Flags: []string{"-json", "-count=1", "-timeout", "5m"}},
	}
	if len(tap.results) != 2 || !pkgResultsEqual(tap.results, want) {
		t.Fatalf("a result per package-level pass or fail, across split writes; got %+v", tap.results)
	}
}

func pkgResultsEqual(a, b []pkgResult) bool {
	return slices.EqualFunc(a, b, func(x, y pkgResult) bool {
		return x.Package == y.Package && x.Passed == y.Passed && slices.Equal(x.Tags, y.Tags) && slices.Equal(x.Flags, y.Flags)
	})
}

func TestSplitTestArgs_SeparatesFlagsTagsAndPackages(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		args  []string
		flags []string
		tags  []string
	}{
		{"pack", repoContractTestArgs([]string{"./a/...", "./b"}, nil), []string{"-json", "-count=1", "-timeout", repoContractTestTimeout}, nil},
		{"tagged", repoContractTestArgs([]string{"./a"}, []string{"integration", "acs"}), []string{"-json", "-count=1", "-timeout", repoContractTestTimeout}, []string{"integration", "acs"}},
		{"by name", repoContractSelectionArgs(repoContractSelections), []string{"-json", "-count=1", "-timeout", repoContractTestTimeout, "-run", ""}, nil},
		{"dangling valued flags", []string{"test", "-run"}, []string{"-run"}, nil},
		{"dangling tags", []string{"test", "-tags"}, []string{"-tags"}, nil},
	}
	for _, c := range cases {
		flags, tags := splitTestArgs(c.args)
		if c.name == "by name" {
			c.flags[5] = flags[5]
			if !strings.HasPrefix(flags[5], "^(") {
				t.Fatalf("by-name run keeps its anchored -run pattern in the flags: %q", flags[5])
			}
		}
		if !slices.Equal(flags, c.flags) || !slices.Equal(tags, c.tags) {
			t.Errorf("%s: flags %q tags %q; want flags %q tags %q", c.name, flags, tags, c.flags, c.tags)
		}
	}
}

func TestPackOutcomeMerged_KeepsTheResultsOfBothRuns(t *testing.T) {
	t.Parallel()
	a := packOutcome{pkgs: []pkgResult{green("p/a")}}
	b := packOutcome{pkgs: []pkgResult{green("p/b")}}
	if got := a.merged(b).pkgs; len(got) != 2 || got[0].Package != "p/a" || got[1].Package != "p/b" {
		t.Fatalf("both runs' package results reach the shadow, in run order; got %+v", got)
	}
}

func TestScanShadowFor_FollowsTheDialAndNeedsAProjectRoot(t *testing.T) {
	t.Parallel()
	p := &Phase{}
	if p.scanShadowFor(core.PhaseRequest{}) != nil {
		t.Fatal("without a project root there is no store location: no shadow")
	}
	root := t.TempDir()
	if p.scanShadowFor(core.PhaseRequest{ProjectRoot: root, Cycle: 3}) == nil {
		t.Fatal("the default stage is shadow")
	}
	mustWrite(t, filepath.Join(root, ".evolve", "policy.json"), `{"gates":{"scan_cache_gate":"off"}}`)
	if p.scanShadowFor(core.PhaseRequest{ProjectRoot: root}) != nil {
		t.Fatal("gates.scan_cache_gate=off turns the shadow off")
	}
}

func TestScanShadow_RunRepoContractGateAttachesTheShadowFromPolicy(t *testing.T) {
	t.Parallel()
	repo := shadowFixture(t)
	center := signalcenter.New()
	p := &Phase{repoContractGate: "enforce", signals: center}
	req := core.PhaseRequest{ProjectRoot: repo, Cycle: 41, Workspace: t.TempDir()}
	if err := p.runRepoContractGate(context.Background(), req, &Options{Class: ClassCycle, ProjectRoot: repo}); err != nil {
		t.Fatalf("a green tree ships green: %v", err)
	}
	if got := (scanRun{events: center.Recent()}).summary(t); got["packages"] != "2" || got["recorded"] != "2" {
		t.Fatalf("the production entry point wires the shadow; got %v", got)
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "landing", "testcache")); err != nil {
		t.Fatalf("the store lives under the project's .evolve/landing/testcache: %v", err)
	}
}

type laneSnapshot struct {
	status string
	index  []byte
	files  map[string]string
}

func snapshotLane(t *testing.T, repo string) laneSnapshot {
	t.Helper()
	snap := laneSnapshot{status: runGitOut(t, repo, "status", "--porcelain=v1", "--untracked-files=all", "--ignored"), files: map[string]string{}}
	index, err := os.ReadFile(filepath.Join(repo, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	snap.index = index
	err = filepath.WalkDir(repo, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && d.Name() == ".git" {
				return fs.SkipDir
			}
			return err
		}
		data, readErr := os.ReadFile(p)
		sum := sha256.Sum256(data)
		rel, _ := filepath.Rel(repo, p)
		snap.files[rel] = hex.EncodeToString(sum[:])
		return readErr
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func TestScanShadow_IsReadOnlyTowardTheLane(t *testing.T) {
	t.Parallel()
	repo := shadowFixture(t)
	runGit(t, repo, "add", "go/internal/base/base.go")
	mustWrite(t, filepath.Join(repo, "go", "internal", "other", "untracked.go"), "package other\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "inbox", "item.json"), "{}\n")
	before := snapshotLane(t, repo)

	center := signalcenter.New()
	run := scanOnce(t, repo, shadowFor(filepath.Join(t.TempDir(), shadowStoreDir), center), center)
	if run.err != nil {
		t.Fatalf("scan: %v", run.err)
	}
	after := snapshotLane(t, repo)

	if before.status != after.status || !bytes.Equal(before.index, after.index) {
		t.Fatalf("the shadow stages nothing and refreshes no index:\n before %q\n  after %q", before.status, after.status)
	}
	if len(before.files) != len(after.files) {
		t.Fatalf("the shadow adds or removes no lane file: %d -> %d", len(before.files), len(after.files))
	}
	for name, sum := range before.files {
		if after.files[name] != sum {
			t.Errorf("%s changed under the shadow", name)
		}
	}
	if !strings.Contains(after.status, "inbox") {
		t.Fatalf("the unconsumed inbox item must still be there:\n%s", after.status)
	}
}

func TestTreeID_ABudgetThatIsExceededOrACancelledContextIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), strings.Repeat("a", 100))
	mustWrite(t, filepath.Join(root, "b.txt"), strings.Repeat("b", 100))
	list := toolRun{files: "a.txt\x00b.txt\x00"}.run

	if _, err := treeID(context.Background(), list, root, 150); !errors.Is(err, errTreeTooLarge) {
		t.Fatalf("two files of 100 bytes exceed a budget of 150: %v", err)
	}
	if _, err := treeID(context.Background(), list, root, 200); err != nil {
		t.Fatalf("the budget is a cap, equal is fine: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := treeID(ctx, list, root, 1<<20); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled or expired context stops the hash: %v", err)
	}
}

func TestScanShadow_ATreeOverTheBudgetDegradesTheScanWithTheReason(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "big.txt"), strings.Repeat("x", 4096))
	center := signalcenter.New()
	tools := toolRun{files: "big.txt\x00", goEnv: arm64GoEnv}
	limits := policy.ScanCacheConfig{MaxBytes: 1024, Budget: time.Minute}
	s := newScanShadow(testverdict.NewStore(filepath.Join(t.TempDir(), shadowStoreDir), time.Now), tools.run, center, shadowCycle, limits)
	s.begin(context.Background(), root)
	s.observe(packOutcome{pkgs: []pkgResult{green("p/a")}})
	s.finish()
	sum := summaryFields(t, center)
	if sum["unkeyed"] != "1" || !strings.Contains(sum["degraded"], errTreeTooLarge.Error()) {
		t.Fatalf("an oversize tree is unkeyed with the reason in the summary; got %v", sum)
	}
}

func TestScanShadow_AnExpiredBudgetStopsTheSnapshotInsteadOfStallingTheShip(t *testing.T) {
	t.Parallel()
	center := signalcenter.New()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), "a")
	tools := toolRun{files: "a.txt\x00", goEnv: arm64GoEnv}
	limits := policy.ScanCacheConfig{MaxBytes: 1 << 20, Budget: time.Nanosecond}
	s := newScanShadow(testverdict.NewStore(filepath.Join(t.TempDir(), shadowStoreDir), time.Now), tools.run, center, shadowCycle, limits)
	time.Sleep(time.Millisecond)
	s.begin(context.Background(), root)
	s.finish()
	if sum := summaryFields(t, center); !strings.Contains(sum["degraded"], "deadline exceeded") {
		t.Fatalf("an expired deadline degrades with its reason; got %v", sum)
	}
}

func TestPkgResultTap_ALineWithoutANewlinePastTheCapIsDroppedAndCounted(t *testing.T) {
	t.Parallel()
	tap := newPkgResultTap([]string{"test"})
	huge := strings.Repeat("x", maxTapLine+1)
	if _, err := io.WriteString(tap, huge); err != nil {
		t.Fatal(err)
	}
	if tap.dropped != 1 || len(tap.partial) != 0 {
		t.Fatalf("the buffer is dropped and counted; dropped=%d held=%d", tap.dropped, len(tap.partial))
	}
	if _, err := io.WriteString(tap, "tail of the dropped line\n{\"Action\":\"pass\",\"Package\":\"p/a\"}\n"); err != nil {
		t.Fatal(err)
	}
	if len(tap.results) != 1 || tap.results[0].Package != "p/a" {
		t.Fatalf("the stream recovers at the next line; got %+v", tap.results)
	}
	small := newPkgResultTap([]string{"test"})
	if _, err := io.WriteString(small, strings.Repeat("x", 1000)); err != nil || small.dropped != 0 || len(small.partial) != 1000 {
		t.Fatalf("a short partial line is kept; dropped=%d held=%d", small.dropped, len(small.partial))
	}
}

func TestScanShadow_DroppedTapLinesAreCountedInTheSummary(t *testing.T) {
	t.Parallel()
	s, center, _ := shadowWith(t, healthyTools())
	s.observe(packOutcome{droppedLines: 2})
	s.observe(packOutcome{droppedLines: 1})
	s.finish()
	if sum := summaryFields(t, center); sum["dropped_lines"] != "3" {
		t.Fatalf("got %v", sum)
	}
}

func TestPackOutcomeMerged_SumsTheDroppedLines(t *testing.T) {
	t.Parallel()
	if got := (packOutcome{droppedLines: 1}).merged(packOutcome{droppedLines: 2}).droppedLines; got != 3 {
		t.Fatalf("got %d", got)
	}
}
