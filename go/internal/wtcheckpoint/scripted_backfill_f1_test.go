package wtcheckpoint

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

type stepF1 struct {
	prefix string
	outs   []string
	code   int
	stderr string
	err    error
	times  int
	used   int
}

type scriptF1 struct {
	mu    sync.Mutex
	steps []*stepF1
	calls []string
}

func newScriptF1(steps ...stepF1) *scriptF1 {
	s := &scriptF1{}
	for i := range steps {
		step := steps[i]
		s.steps = append(s.steps, &step)
	}
	return s
}

func (s *scriptF1) run(_ context.Context, _, _ string, args, _ []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	joined := strings.Join(args, " ")
	s.calls = append(s.calls, joined)
	for _, st := range s.steps {
		if !strings.HasPrefix(joined, st.prefix) || (st.times > 0 && st.used >= st.times) {
			continue
		}
		out := ""
		if len(st.outs) > 0 {
			out = st.outs[min(st.used, len(st.outs)-1)]
		}
		st.used++
		_, _ = io.WriteString(stdout, out)
		_, _ = io.WriteString(stderr, st.stderr)
		return st.code, st.err
	}
	return 1, errors.New("unscripted git call: " + joined)
}

func (s *scriptF1) git(dir string) gitexec.Git { return gitexec.Git{Dir: dir, Exec: s.run} }

func (s *scriptF1) called(prefix string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, c := range s.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func useScriptF1(t *testing.T, s *scriptF1) {
	t.Helper()
	orig := checkpointIsolatedGit
	t.Cleanup(func() { checkpointIsolatedGit = orig })
	checkpointIsolatedGit = s.git
}

const (
	headF1  = "1111111111111111111111111111111111111111"
	treeF1  = "2222222222222222222222222222222222222222"
	commitF = "3333333333333333333333333333333333333333"
)

func indexFileF1(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "index")
	if err := os.WriteFile(p, []byte("idx"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func saveStepsF1(t *testing.T, override ...stepF1) *scriptF1 {
	t.Helper()
	index := indexFileF1(t)
	defaults := []stepF1{
		{prefix: "check-ref-format"},
		{prefix: "rev-parse HEAD", outs: []string{headF1}},
		{prefix: "rev-parse --path-format=absolute --git-path index", outs: []string{index}},
		{prefix: "add -A"},
		{prefix: "write-tree", outs: []string{"aaaa", "bbbb"}},
		{prefix: "rev-parse " + headF1 + "^{tree}", outs: []string{treeF1}},
		{prefix: "for-each-ref"},
		{prefix: "-c commit.gpgsign=false commit-tree", outs: []string{"c1", "c2"}},
		{prefix: "update-ref"},
	}
	return newScriptF1(append(override, defaults...)...)
}

func TestSaveBackfillF1_EachGitStepFailureEndsTheSaveWithThatErrorAndRecordsNoRef(t *testing.T) {
	boom := errors.New("injected git failure")
	cases := []struct {
		name         string
		override     []stepF1
		wantCalled   string
		wantNotCalls string
	}{
		{"HEAD unreadable", []stepF1{{prefix: "rev-parse HEAD", err: boom}}, "rev-parse HEAD", "write-tree"},
		{"staged snapshot", []stepF1{{prefix: "rev-parse --path-format=absolute --git-path index", err: boom}}, "rev-parse --path-format", "write-tree"},
		{"full snapshot", []stepF1{{prefix: "add -A", err: boom}}, "add -A", "-c commit.gpgsign=false commit-tree"},
		{"head tree", []stepF1{{prefix: "rev-parse " + headF1 + "^{tree}", err: boom}}, "rev-parse " + headF1, "for-each-ref"},
		{"existing checkpoint listing", []stepF1{{prefix: "for-each-ref", err: boom}}, "for-each-ref", "-c commit.gpgsign=false commit-tree"},
		{"staged commit", []stepF1{{prefix: "-c commit.gpgsign=false commit-tree", err: boom}}, "-c commit.gpgsign=false commit-tree", "update-ref"},
		{"worktree commit", []stepF1{{prefix: "-c commit.gpgsign=false commit-tree", outs: []string{"c1"}, times: 1}, {prefix: "-c commit.gpgsign=false commit-tree", err: boom}}, "-c commit.gpgsign=false commit-tree", "update-ref"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := saveStepsF1(t, tc.override...)
			useScriptF1(t, s)
			res, err := Save(context.Background(), Worktree{Dir: "/wt/task", Name: "task"}, SaveOptions{Keep: 5, Now: func() time.Time { return time.Unix(0, 0) }})
			if !errors.Is(err, boom) {
				t.Fatalf("err = %v, want the injected failure", err)
			}
			if res.Status != "" || res.Ref != "" || res.Worktree != "task" || res.Dir != "/wt/task" {
				t.Fatalf("result = %+v; a failed save names the worktree and reports no status or ref", res)
			}
			if s.called(tc.wantCalled) == 0 {
				t.Errorf("the failing step %q never ran: %v", tc.wantCalled, s.calls)
			}
			if tc.wantNotCalls != "" && s.called(tc.wantNotCalls) != 0 {
				t.Errorf("%q ran after the failure: %v", tc.wantNotCalls, s.calls)
			}
		})
	}
}

func TestSaveBackfillF1_ARefThatCannotBeWrittenIsAnError(t *testing.T) {
	boom := errors.New("update-ref refused")
	s := saveStepsF1(t, stepF1{prefix: "update-ref", err: boom})
	useScriptF1(t, s)
	res, err := Save(context.Background(), Worktree{Dir: "/wt/task", Name: "task"}, SaveOptions{Keep: 5, Now: func() time.Time { return time.Unix(0, 0) }})
	if !errors.Is(err, boom) || res.Status == StatusSaved {
		t.Fatalf("res=%+v err=%v; a ref that was not written must not read as saved", res, err)
	}
}

func TestSaveBackfillF1_APruneFailureAfterTheRecordIsReportedButTheSaveStands(t *testing.T) {
	boom := errors.New("listing failed")
	s := saveStepsF1(t,
		stepF1{prefix: "for-each-ref", times: 1},
		stepF1{prefix: "for-each-ref", err: boom},
	)
	useScriptF1(t, s)
	res, err := Save(context.Background(), Worktree{Dir: "/wt/task", Name: "task"}, SaveOptions{Keep: 5, Now: func() time.Time { return time.Unix(0, 0) }})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the prune listing failure", err)
	}
	if res.Status != StatusSaved || res.Ref != "refs/checkpoints/task/19700101T000000Z" || res.Pruned != nil {
		t.Fatalf("res = %+v; the checkpoint was recorded before the prune failed", res)
	}
}

func TestSnapshotBackfillF1_AFailingFirstOrSecondTreeIsReportedWithoutAPartialResult(t *testing.T) {
	boom := errors.New("snapshot step failed")
	index := indexFileF1(t)
	steps := func(over ...stepF1) gitexec.Git {
		healthy := []stepF1{
			{prefix: "rev-parse --path-format=absolute --git-path index", outs: []string{index}},
			{prefix: "add -A"},
			{prefix: "write-tree", outs: []string{"aaaa", "bbbb"}},
		}
		return newScriptF1(append(over, healthy...)...).git("/wt")
	}
	if got, err := snapshot(context.Background(), steps(stepF1{prefix: "write-tree", err: boom})); !errors.Is(err, boom) || got != (trees{}) {
		t.Errorf("first tree failure = (%+v, %v)", got, err)
	}
	if got, err := snapshot(context.Background(), steps(stepF1{prefix: "add -A", err: boom})); !errors.Is(err, boom) || got != (trees{}) {
		t.Errorf("second tree failure = (%+v, %v)", got, err)
	}
	if got, err := snapshot(context.Background(), steps()); err != nil || got != (trees{staged: "aaaa", full: "bbbb"}) {
		t.Errorf("healthy snapshot = (%+v, %v)", got, err)
	}
}

func TestHubBackfillF1_ListingFailuresAndEmptyListings(t *testing.T) {
	boom := errors.New("worktree list failed")
	hubWith := func(s *scriptF1) Hub {
		useScriptF1(t, s)
		return Hub{plane.Hub{Root: "/hub", Store: "/hub/.store"}}
	}
	t.Run("linked propagates the git failure", func(t *testing.T) {
		h := hubWith(newScriptF1(stepF1{prefix: "worktree list", err: boom}))
		if got, err := h.linked(context.Background()); !errors.Is(err, boom) || got != nil {
			t.Fatalf("linked = (%v, %v)", got, err)
		}
	})
	t.Run("a listing without any worktree line has no linked worktrees", func(t *testing.T) {
		h := hubWith(newScriptF1(stepF1{prefix: "worktree list", outs: []string{"prunable gitdir file points to non-existent location\n"}}))
		got, err := h.linked(context.Background())
		if err != nil || got != nil {
			t.Fatalf("linked = (%v, %v), want nil, nil", got, err)
		}
	})
	t.Run("the first entry is the main worktree and is never linked", func(t *testing.T) {
		listing := "worktree /hub/main\nHEAD x\n\nworktree /hub/dev/a\nprunable gone\n\nworktree /hub/dev/b\n"
		h := hubWith(newScriptF1(stepF1{prefix: "worktree list", outs: []string{listing}}))
		got, err := h.linked(context.Background())
		if err != nil || len(got) != 2 || got[0].path != "/hub/dev/a" || !got[0].prunable || got[1].path != "/hub/dev/b" || got[1].prunable {
			t.Fatalf("linked = (%+v, %v)", got, err)
		}
	})
	t.Run("Worktree and DevWorktrees surface the listing failure", func(t *testing.T) {
		h := hubWith(newScriptF1(stepF1{prefix: "worktree list", err: boom}))
		if w, err := h.Worktree(context.Background(), "/hub/dev/a"); !errors.Is(err, boom) || w != (Worktree{}) {
			t.Errorf("Worktree = (%+v, %v)", w, err)
		}
		if ws, err := h.DevWorktrees(context.Background()); !errors.Is(err, boom) || ws != nil {
			t.Errorf("DevWorktrees = (%v, %v)", ws, err)
		}
	})
}

func TestRealPathBackfillF1(t *testing.T) {
	t.Run("a path that does not exist keeps its absolute form", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "no", "such", "dir")
		if got := realPath(missing); got != missing {
			t.Fatalf("realPath = %q, want %q", got, missing)
		}
	})
	t.Run("a relative path that does not exist is made absolute", func(t *testing.T) {
		got := realPath(filepath.Join("f1-no-such-dir", "child"))
		if !filepath.IsAbs(got) || !strings.HasSuffix(got, filepath.Join("f1-no-such-dir", "child")) {
			t.Fatalf("realPath = %q, want an absolute path ending in the relative one", got)
		}
	})
	t.Run("a symlink resolves to its target", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "real")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		want, _ := filepath.EvalSymlinks(target)
		if got := realPath(link); got != want {
			t.Fatalf("realPath = %q, want %q", got, want)
		}
	})
	t.Run("a path that cannot be made absolute is only cleaned", func(t *testing.T) {
		orig := checkpointAbs
		t.Cleanup(func() { checkpointAbs = orig })
		checkpointAbs = func(string) (string, error) { return "", errors.New("no cwd") }
		if got := realPath("a/./b//c/../d"); got != "a/b/d" {
			t.Fatalf("realPath = %q, want a/b/d", got)
		}
	})
}
