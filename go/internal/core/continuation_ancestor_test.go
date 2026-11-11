package core

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const unknownButWellFormedSHA = "1111111111111111111111111111111111111111"

type ancestryGitScript struct {
	exitCode int
	runErr   error
	argv     [][]string
	dirs     []string
}

func (s *ancestryGitScript) run(_ context.Context, _, dir string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
	s.argv = append(s.argv, append([]string(nil), args...))
	s.dirs = append(s.dirs, dir)
	return s.exitCode, s.runErr
}

func (s *ancestryGitScript) git(dir string) gitexec.Git {
	return gitexec.Git{Dir: dir, Exec: sysexec.RunFunc(s.run)}
}

func manifestWorkspace(t *testing.T, c continuation.Continuation) string {
	t.Helper()
	ws := t.TempDir()
	if err := continuation.WriteManifest(ws, c); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return ws
}

func TestContinuationAncestor_SalvageOnProjectHeadReportsTrue(t *testing.T) {
	t.Parallel()

	root, wt := initContinuationRepo(t, 91)
	m := stampedContinuation(t, root, wt, 91)
	gitOut(t, root, "merge", "--ff-only", m.SnapshotSHA)
	ws := filepath.Join(root, ".evolve", "runs", cycleDirName(91))

	got := continuationAncestry(context.Background(), gitexec.Git{Dir: root, Exec: gitRunner}, ws)

	if !strings.Contains(got, "ancestor-of-HEAD=true") {
		t.Errorf("a salvage already merged into the project HEAD must report true, got %q", got)
	}
	if !strings.Contains(got, m.SnapshotSHA) || !strings.Contains(got, "cycle-91") {
		t.Errorf("the line must name the salvage sha and the ancestor cycle, got %q", got)
	}
}

func TestContinuationAncestor_SalvageOffProjectHeadReportsFalse(t *testing.T) {
	t.Parallel()

	root, wt := initContinuationRepo(t, 92)
	m := stampedContinuation(t, root, wt, 92)
	ws := filepath.Join(root, ".evolve", "runs", cycleDirName(92))

	got := continuationAncestry(context.Background(), gitexec.Git{Dir: root, Exec: gitRunner}, ws)

	if !strings.Contains(got, "ancestor-of-HEAD=false") {
		t.Errorf("a salvage not yet on the project HEAD must report false, got %q", got)
	}
	if strings.Contains(got, "ancestor-of-HEAD=true") || strings.Contains(got, "ancestor-of-HEAD=unknown") {
		t.Errorf("false must be the only verdict, got %q", got)
	}
	if !strings.Contains(got, m.SnapshotSHA) {
		t.Errorf("the line must name the salvage sha %s, got %q", m.SnapshotSHA, got)
	}
}

func TestContinuationAncestor_AbsentManifestYieldsNothingAndRunsNoGit(t *testing.T) {
	t.Parallel()

	script := &ancestryGitScript{}

	got := continuationAncestry(context.Background(), script.git(t.TempDir()), t.TempDir())

	if got != "" {
		t.Errorf("a workspace with no manifest must yield the empty string, got %q", got)
	}
	if len(script.argv) != 0 {
		t.Errorf("no manifest means no git call, got %v", script.argv)
	}
}

func TestContinuationAncestor_UnreadableManifestYieldsNothing(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, continuation.ManifestName), []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := &ancestryGitScript{}

	if got := continuationAncestry(context.Background(), script.git(t.TempDir()), ws); got != "" || len(script.argv) != 0 {
		t.Errorf("an unparseable manifest must yield nothing and run no git, got %q and %v", got, script.argv)
	}
}

func TestContinuationAncestor_UnknownSHAReportsUnknownNotFalse(t *testing.T) {
	t.Parallel()

	root, _ := initContinuationRepo(t, 93)
	ws := manifestWorkspace(t, continuation.Continuation{Cycle: 93, SnapshotSHA: unknownButWellFormedSHA})

	got := continuationAncestry(context.Background(), gitexec.Git{Dir: root, Exec: gitRunner}, ws)

	if !strings.Contains(got, "ancestor-of-HEAD=unknown") {
		t.Errorf("a sha git cannot resolve must report unknown, got %q", got)
	}
	if strings.Contains(got, "ancestor-of-HEAD=false") {
		t.Errorf("unknown must never be reported as false: false would tell the scout the salvage is unshipped, got %q", got)
	}
}

func TestContinuationAncestor_ProbeAsksGitExactlyMergeBaseIsAncestorAgainstHead(t *testing.T) {
	t.Parallel()

	ws := manifestWorkspace(t, continuation.Continuation{Cycle: 94, SnapshotSHA: unknownButWellFormedSHA})
	script := &ancestryGitScript{exitCode: 1}
	dir := t.TempDir()

	got := continuationAncestry(context.Background(), script.git(dir), ws)

	if len(script.argv) != 1 {
		t.Fatalf("want exactly one git call, got %v", script.argv)
	}
	want := []string{"merge-base", "--is-ancestor", unknownButWellFormedSHA, "HEAD"}
	if strings.Join(script.argv[0], " ") != strings.Join(want, " ") {
		t.Errorf("git argv = %v, want %v", script.argv[0], want)
	}
	if script.dirs[0] != dir {
		t.Errorf("git ran in %q, want the injected runner's dir %q", script.dirs[0], dir)
	}
	if !strings.Contains(got, "ancestor-of-HEAD=false") {
		t.Errorf("exit 1 from merge-base --is-ancestor means not an ancestor, got %q", got)
	}
}

func TestContinuationAncestor_ExitCodeMapsToVerdict(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		script ancestryGitScript
		want   string
	}{
		{"exit 0 is an ancestor", ancestryGitScript{exitCode: 0}, "ancestor-of-HEAD=true"},
		{"exit 1 is not an ancestor", ancestryGitScript{exitCode: 1}, "ancestor-of-HEAD=false"},
		{"exit 128 is a git failure", ancestryGitScript{exitCode: 128}, "ancestor-of-HEAD=unknown"},
		{"a runner error is a git failure", ancestryGitScript{exitCode: -1, runErr: errors.New("git binary vanished")}, "ancestor-of-HEAD=unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := manifestWorkspace(t, continuation.Continuation{Cycle: 95, SnapshotSHA: unknownButWellFormedSHA})
			script := tc.script

			if got := continuationAncestry(context.Background(), script.git(t.TempDir()), ws); !strings.Contains(got, tc.want) {
				t.Errorf("got %q, want it to contain %q", got, tc.want)
			}
		})
	}
}

func TestContinuationAncestor_MalformedSHAIsUnknownAndNeverReachesGit(t *testing.T) {
	t.Parallel()

	for _, sha := range []string{"", "--output=/tmp/pwned", "-x", "zzzz", "abc123", strings.Repeat("a", 39), unknownButWellFormedSHA + "\n--all"} {
		ws := manifestWorkspace(t, continuation.Continuation{Cycle: 96, SnapshotSHA: sha})
		script := &ancestryGitScript{}

		got := continuationAncestry(context.Background(), script.git(t.TempDir()), ws)

		if !strings.Contains(got, "ancestor-of-HEAD=unknown") {
			t.Errorf("sha %q: want unknown, got %q", sha, got)
		}
		if len(script.argv) != 0 {
			t.Errorf("sha %q reached git as %v: a manifest value must never become a git argument unvalidated", sha, script.argv)
		}
	}
}

func TestContinuationAncestor_AdoptionServesTheLineFromTheProjectRootHead(t *testing.T) {
	t.Parallel()

	req := adoptWithPriorVerdict(t, 97, "")

	got := req.Context[CtxKeyContinuationAncestry]
	if !strings.Contains(got, "cycle-97") || !strings.Contains(got, "ancestor-of-HEAD=false") {
		t.Errorf("the build context of an adopted continuation must carry the ancestry line for cycle-97 measured against the project root HEAD (the snapshot is not on main), got %q", got)
	}
}

func TestContinuationAncestor_FreshCycleCarriesNoLine(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runners := buildRunners(nil)
	buildR := runners[PhaseBuild].(*fakeRunner)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners)

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	if buildR.calls == 0 {
		t.Fatal("build must have dispatched")
	}
	if got, ok := buildR.requests[0].Context[CtxKeyContinuationAncestry]; ok {
		t.Errorf("a fresh cycle has no manifest, so it must carry no ancestry key, got %q", got)
	}
}
