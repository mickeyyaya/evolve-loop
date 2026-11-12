package baseattr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

var (
	kA = Key{"example.com/m/p", "TestA"}
	kB = Key{"example.com/m/p", "TestB"}
)

type script struct {
	mu    sync.Mutex
	steps [][]Key
	asked [][]Key
	dirs  []string
	err   error
}

func (s *script) run(_ context.Context, dir string, r Reds) ([]Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.asked)
	s.asked = append(s.asked, r.Keys)
	s.dirs = append(s.dirs, dir)
	if !hasGitEntry(dir) && !hasGitEntry(filepath.Dir(dir)) {
		return nil, errors.New("the module has no .git above it")
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.steps[n], nil
}

func hasGitEntry(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func twoCommits(t *testing.T) (r *gittest.Repo, base, tip string) {
	t.Helper()
	r = gittest.Fixture(t)
	base = commitFiles(t, r, "base", map[string]string{"a.txt": "1"})
	tip = commitFiles(t, r, "tip", map[string]string{"a.txt": "2"})
	return r, base, tip
}

func TestAttribute_ANotAttributableRedBlamesTheLaneWithoutARun(t *testing.T) {
	t.Parallel()
	r, base, _ := twoCommits(t)
	s := &script{}
	a := attributorOn(t, r, s.run)
	v, err := a.Attribute(context.Background(), base, Reds{Keys: []Key{kA}, Unnamed: true})
	if err != nil || !reflect.DeepEqual(v, Verdict{LaneOnly: []Key{kA}, Unjudged: true}) || len(s.asked) != 0 {
		t.Errorf("verdict %+v, err %v, runs %d: want the lane blamed and no run", v, err, len(s.asked))
	}
	unnamed, err := a.Attribute(context.Background(), base, Reds{Unnamed: true})
	if err != nil || !unnamed.Blamed() || len(s.asked) != 0 {
		t.Errorf("verdict %+v, err %v: a red with no named test must blame the lane", unnamed, err)
	}
}

func TestAttribute_AGreenBaseBlamesTheLaneAndStopsAfterOneRun(t *testing.T) {
	t.Parallel()
	r, base, _ := twoCommits(t)
	s := &script{steps: [][]Key{nil}}
	var taken, released int
	a := attributorOn(t, r, s.run)
	a.Serialize = func(context.Context) func() { taken++; return func() { released++ } }
	v, err := a.Attribute(context.Background(), base, Reds{Keys: []Key{kA, kB}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(v.LaneOnly, []Key{kA, kB}) || len(v.BaseRed) != 0 || !v.Blamed() || v.Base != base || len(s.asked) != 1 {
		t.Errorf("verdict %+v, runs %d: want both keys lane-only after one run on %s", v, len(s.asked), base)
	}
	if taken != 1 || released != 1 {
		t.Errorf("serialize taken %d released %d, want 1 and 1", taken, released)
	}
}

func TestAttribute_PartitionsBaseRedFromLaneOnlyAndRerunsOnlyTheBaseRedOnMain(t *testing.T) {
	t.Parallel()
	r, base, tip := twoCommits(t)
	s := &script{steps: [][]Key{{kB}, nil}}
	v, err := attributorOn(t, r, s.run).Attribute(context.Background(), base, Reds{Keys: []Key{kA, kB}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(v.LaneOnly, []Key{kA}) || !reflect.DeepEqual(v.BaseRed, []Key{kB}) || len(v.MainRed) != 0 {
		t.Errorf("verdict %+v, want lane-only A, base-red B, main green", v)
	}
	if v.Main != tip || !v.MainGreen() || !reflect.DeepEqual(s.asked, [][]Key{{kA, kB}, {kB}}) {
		t.Errorf("main %q (tip %s), asked %v: the main run must carry only the base-red key", v.Main, tip, s.asked)
	}
}

func TestAttribute_RunsTheRedKeysOnTheRedMainAsMainRed(t *testing.T) {
	t.Parallel()
	r, base, _ := twoCommits(t)
	s := &script{steps: [][]Key{{kA}, {kA}}}
	v, err := attributorOn(t, r, s.run).Attribute(context.Background(), base, Reds{Keys: []Key{kA}})
	if err != nil || !reflect.DeepEqual(v.MainRed, []Key{kA}) || v.MainGreen() || v.Took < 0 {
		t.Errorf("verdict %+v, err %v, want main red A", v, err)
	}
}

func TestAttribute_SkipsTheMainRunWhenMainIsTheBase(t *testing.T) {
	t.Parallel()
	r, _, tip := twoCommits(t)
	s := &script{steps: [][]Key{{kA}}}
	v, err := attributorOn(t, r, s.run).Attribute(context.Background(), tip, Reds{Keys: []Key{kA}})
	if err != nil || len(s.asked) != 1 || !reflect.DeepEqual(v.MainRed, []Key{kA}) || v.Main != tip {
		t.Errorf("verdict %+v, err %v, runs %d: want one run and main red = base red", v, err, len(s.asked))
	}
}

func TestAttribute_AnUnreadableMainRefReturnsTheBaseRedWithNoMain(t *testing.T) {
	t.Parallel()
	r, base, _ := twoCommits(t)
	s := &script{steps: [][]Key{{kA}}}
	a := attributorOn(t, r, s.run)
	a.MainRef = "no-such-ref"
	v, err := a.Attribute(context.Background(), base, Reds{Keys: []Key{kA}})
	if err != nil || v.Main != "" || !reflect.DeepEqual(v.BaseRed, []Key{kA}) || v.MainGreen() {
		t.Errorf("verdict %+v, err %v, want base red A, no main and no error", v, err)
	}
}

func TestAttribute_AnUnknownBaseIsAnError(t *testing.T) {
	t.Parallel()
	r, _, _ := twoCommits(t)
	s := &script{}
	_, err := attributorOn(t, r, s.run).Attribute(context.Background(), "no-such-rev", Reds{Keys: []Key{kA}})
	if err == nil || len(s.asked) != 0 {
		t.Errorf("err %v, runs %d, want an error and no run", err, len(s.asked))
	}
}

func TestAttribute_AFailedRunIsAnErrorOnTheBaseAndOnMain(t *testing.T) {
	t.Parallel()
	boom := errors.New("go test cannot start")
	r, base, _ := twoCommits(t)
	s := &script{err: boom}
	if _, err := attributorOn(t, r, s.run).Attribute(context.Background(), base, Reds{Keys: []Key{kA}}); !errors.Is(err, boom) {
		t.Errorf("base run: err = %v, want %v", err, boom)
	}
	calls := 0
	failSecond := func(ctx context.Context, dir string, rd Reds) ([]Key, error) {
		calls++
		if calls == 2 {
			return nil, boom
		}
		return []Key{kA}, nil
	}
	v, err := attributorOn(t, r, failSecond).Attribute(context.Background(), base, Reds{Keys: []Key{kA}})
	if !errors.Is(err, boom) || !reflect.DeepEqual(v.BaseRed, []Key{kA}) {
		t.Errorf("main run: verdict %+v, err %v, want the error and the base red kept", v, err)
	}
}

func TestAttribute_ATimeoutIsAnErrorThatNamesTheDeadline(t *testing.T) {
	t.Parallel()
	r, base, _ := twoCommits(t)
	hang := func(ctx context.Context, _ string, _ Reds) ([]Key, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	a := attributorOn(t, r, hang)
	a.Timeout = 20 * time.Millisecond
	v, err := a.Attribute(context.Background(), base, Reds{Keys: []Key{kA}})
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "20ms") || v.Took <= 0 {
		t.Errorf("verdict %+v, err %v, want a deadline error with the limit and a Took", v, err)
	}
	if got := worktreeCount(r); got != 1 {
		t.Errorf("%d worktrees left, want 1", got)
	}
}

func TestAttribute_AFailedCheckoutIsAnErrorAndRemovesTheWorktree(t *testing.T) {
	r, base, _ := twoCommits(t)
	gittest.InstallFault(t, gittest.GitFault{Verb: "worktree", Args: []string{"add"}, Code: 128, Stderr: "fatal: already exists\n"})
	s := &script{}
	a := attributorOn(t, r, s.run)
	_, err := a.Attribute(context.Background(), base, Reds{Keys: []Key{kA}})
	if err == nil || !strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "%!") || len(s.asked) != 0 {
		t.Fatalf("err %v, runs %d, want the git error and no run", err, len(s.asked))
	}
	if got := worktreeCount(r); got != 1 {
		t.Errorf("%d worktrees listed, want only the main one", got)
	}
	if entries, _ := os.ReadDir(a.Scratch); len(entries) != 0 {
		t.Errorf("scratch holds %d entries after the failure, want none", len(entries))
	}
}

func TestRunAt_AFailedWorktreeRemoveIsPrunedAway(t *testing.T) {
	r, base, _ := twoCommits(t)
	gittest.InstallFault(t, gittest.GitFault{Verb: "worktree", Args: []string{"remove"}, Code: 128, Stderr: "fatal: locked\n"})
	s := &script{steps: [][]Key{nil}}
	if _, err := attributorOn(t, r, s.run).RunAt(context.Background(), base, Reds{Keys: []Key{kA}}); err != nil {
		t.Fatal(err)
	}
	if got := worktreeCount(r); got != 1 {
		t.Errorf("%d worktrees left after a failed remove, want 1", got)
	}
}

func TestRunAt_RemovesTheWorktreeAndPicksTheGoModuleDirectory(t *testing.T) {
	t.Parallel()
	r := gittest.Fixture(t)
	rev := commitFiles(t, r, "m", map[string]string{"go/go.mod": "module x\n"})
	plain := gittest.Fixture(t)
	plainRev := commitFiles(t, plain, "p", map[string]string{"a.txt": "1"})
	for _, tc := range []struct {
		repo   *gittest.Repo
		rev    string
		suffix string
	}{{r, rev, string(filepath.Separator) + "go"}, {plain, plainRev, ""}} {
		s := &script{steps: [][]Key{{kA}}}
		a := attributorOn(t, tc.repo, s.run)
		got, err := a.RunAt(context.Background(), tc.rev, Reds{Keys: []Key{kA}})
		if err != nil || !reflect.DeepEqual(got, []Key{kA}) {
			t.Fatalf("RunAt = %v, %v", got, err)
		}
		if !strings.HasSuffix(s.dirs[0], tc.suffix) || (tc.suffix != "" && filepath.Base(s.dirs[0]) != "go") {
			t.Errorf("module dir %q, want suffix %q", s.dirs[0], tc.suffix)
		}
		if worktreeCount(tc.repo) != 1 {
			t.Errorf("a worktree is left after RunAt")
		}
		if _, statErr := os.Stat(s.dirs[0]); !os.IsNotExist(statErr) {
			t.Errorf("the scratch directory %s is left", s.dirs[0])
		}
	}
}

func TestAttribute_AnUnwritableScratchIsAnError(t *testing.T) {
	t.Parallel()
	r, base, _ := twoCommits(t)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &script{}
	a := attributorOn(t, r, s.run)
	a.Scratch = filepath.Join(blocker, "sub")
	if _, err := a.Attribute(context.Background(), base, Reds{Keys: []Key{kA}}); err == nil || len(s.asked) != 0 {
		t.Errorf("err %v, runs %d, want a scratch error and no run", err, len(s.asked))
	}
}

func TestRunAt_AScratchRelativeToTheRepoIsJoinedToIt(t *testing.T) {
	t.Parallel()
	r, base, _ := twoCommits(t)
	s := &script{steps: [][]Key{nil}}
	a := attributorOn(t, r, s.run)
	a.Scratch = ".evolve/ephemeral/base-red"
	if _, err := a.RunAt(context.Background(), base, Reds{Keys: []Key{kA}}); err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(r.Dir, ".evolve", "ephemeral", "base-red"); !strings.HasPrefix(s.dirs[0], want) {
		t.Errorf("module dir %q, want it under %q", s.dirs[0], want)
	}
}

const (
	fixtureMod  = "module example.com/fx\n\ngo 1.23\n"
	fixturePkg  = "example.com/fx/p"
	fixtureTest = "package p\n\nimport (\n\t\"os/exec\"\n\t\"strings\"\n\t\"testing\"\n)\n\nconst bin = \"gi\" + \"t\"\n\nfunc TestListsTrackedFiles(t *testing.T) {\n\tout, err := exec.Command(bin, \"ls-files\").Output()\n\tif err != nil || !strings.Contains(string(out), \"p_test.go\") {\n\t\tt.Fatalf(\"no tracked files: %v %q\", err, out)\n\t}\n}\n\nfunc TestRedOnTheBase(t *testing.T) { t.Fatal(\"red\") }\n\nfunc TestNeverAsked(t *testing.T) { t.Fatal(\"red\") }\n"
)

func TestAttribute_RunsOnlyTheRedKeysInADetachedBaseCheckout(t *testing.T) {
	t.Parallel()
	r := gittest.Fixture(t)
	base := commitFiles(t, r, "fx", map[string]string{"go/go.mod": fixtureMod, "go/p/p_test.go": fixtureTest})
	listing, red := Key{fixturePkg, "TestListsTrackedFiles"}, Key{fixturePkg, "TestRedOnTheBase"}
	real := GoTestRunner(ipcenv.GoTestEnv())
	var asked [][]Key
	a := attributorOn(t, r, func(ctx context.Context, dir string, rd Reds) ([]Key, error) {
		asked = append(asked, rd.Keys)
		return real(ctx, dir, rd)
	})
	v, err := a.Attribute(context.Background(), base, Reds{Keys: []Key{listing, red}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(v.LaneOnly, []Key{listing}) || !reflect.DeepEqual(v.BaseRed, []Key{red}) || !reflect.DeepEqual(v.MainRed, []Key{red}) {
		t.Errorf("verdict %+v: a test that reads git must be green in the checkout, and TestRedOnTheBase red on base and main", v)
	}
	if !reflect.DeepEqual(asked, [][]Key{{listing, red}}) || worktreeCount(r) != 1 {
		t.Errorf("asked %v, worktrees %d, want one run of the two keys and no worktree left", asked, worktreeCount(r))
	}
}

func TestAttribute_HoldsTheSerializeLockDuringEachRun(t *testing.T) {
	t.Parallel()
	r, base, _ := twoCommits(t)
	var held bool
	var seen []bool
	run := func(context.Context, string, Reds) ([]Key, error) {
		seen = append(seen, held)
		return []Key{kA}, nil
	}
	a := attributorOn(t, r, run)
	a.Serialize = func(context.Context) func() { held = true; return func() { held = false } }
	if _, err := a.Attribute(context.Background(), base, Reds{Keys: []Key{kA}}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seen, []bool{true, true}) || held {
		t.Errorf("lock held during the runs %v, held after %v: want held in the base and main runs and free after", seen, held)
	}
}

func TestRunAt_ACancelledCallerStillRemovesTheWorktree(t *testing.T) {
	t.Parallel()
	r, base, _ := twoCommits(t)
	ctx, cancel := context.WithCancel(context.Background())
	run := func(context.Context, string, Reds) ([]Key, error) {
		cancel()
		return nil, errors.New("stopped")
	}
	if _, err := attributorOn(t, r, run).RunAt(ctx, base, Reds{Keys: []Key{kA}}); err == nil {
		t.Fatal("RunAt returned no error for a run that failed")
	}
	if got := worktreeCount(r); got != 1 {
		t.Errorf("%d worktrees left after a cancelled caller, want 1", got)
	}
}

func TestRunIn_RunsTheKeysInTheGivenTreeWithNoCheckout(t *testing.T) {
	t.Parallel()
	r := gittest.Fixture(t)
	commitFiles(t, r, "m", map[string]string{"go/go.mod": "module x\n"})
	s := &script{steps: [][]Key{{kA}}}
	a := attributorOn(t, r, s.run)
	got, err := a.RunIn(context.Background(), r.Dir, Reds{Keys: []Key{kA, kB}})
	if err != nil || !reflect.DeepEqual(got, []Key{kA}) {
		t.Fatalf("RunIn = %v, %v", got, err)
	}
	if s.dirs[0] != filepath.Join(r.Dir, "go") || worktreeCount(r) != 1 {
		t.Errorf("ran in %q with %d worktrees, want the module of the given tree and no checkout", s.dirs[0], worktreeCount(r))
	}
}

func TestRunAt_ARunErrorNamesTheRevisionAndTheLimit(t *testing.T) {
	t.Parallel()
	r, base, _ := twoCommits(t)
	s := &script{err: errors.New("the go tool is gone")}
	a := attributorOn(t, r, s.run)
	a.Timeout = 3 * time.Minute
	_, err := a.RunAt(context.Background(), base, Reds{Keys: []Key{kA}})
	if err == nil || !strings.Contains(err.Error(), "run at "+base+" (limit 3m0s)") || !strings.Contains(err.Error(), "the go tool is gone") {
		t.Errorf("err = %v, want the revision, the limit and the cause", err)
	}
}

func TestRunIn_ARunErrorNamesTheTreeAndTheLimit(t *testing.T) {
	t.Parallel()
	r := gittest.Fixture(t)
	s := &script{err: errors.New("the go tool is gone")}
	a := attributorOn(t, r, s.run)
	a.Timeout = 3 * time.Minute
	_, err := a.RunIn(context.Background(), r.Dir, Reds{Keys: []Key{kA}})
	if err == nil || !strings.Contains(err.Error(), "run at "+r.Dir+" (limit 3m0s)") || !strings.Contains(err.Error(), "the go tool is gone") {
		t.Errorf("err = %v, want the tree, the limit and the cause", err)
	}
}

func TestResolve_NamesTheCommitOfARevAndRefusesAnUnknownRev(t *testing.T) {
	t.Parallel()
	r, base, tip := twoCommits(t)
	a := attributorOn(t, r, (&script{}).run)
	for rev, want := range map[string]string{base: base, tip: tip, "HEAD": tip, "HEAD~1": base} {
		if got, err := a.Resolve(context.Background(), rev); err != nil || got != want {
			t.Errorf("Resolve(%q) = %q, %v, want %q", rev, got, err, want)
		}
	}
	if got, err := a.Resolve(context.Background(), "no-such-ref"); err == nil {
		t.Errorf("Resolve of an unknown rev = %q, nil error, want an error", got)
	}
}
