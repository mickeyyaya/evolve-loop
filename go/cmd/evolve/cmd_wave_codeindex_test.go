package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type refreshCall struct {
	name, dir string
	args      []string
}

type refreshRunner struct {
	mu      sync.Mutex
	calls   []refreshCall
	failOn  string
	failRC  int
	failErr error
	quiet   bool
	msg     string
	top     string
	blockOn string
}

func (r *refreshRunner) run(ctx context.Context, name, dir string, args, _ []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, refreshCall{name: name, dir: dir, args: args})
	if len(args) > 0 && args[0] == "rev-parse" && r.failOn != "rev-parse" {
		top := r.top
		if top == "" {
			top = dir
		}
		_, _ = io.WriteString(stdout, top+"\n")
		return 0, nil
	}
	if r.blockOn != "" && len(args) > 0 && args[0] == r.blockOn {
		r.mu.Unlock()
		<-ctx.Done()
		r.mu.Lock()
		return 0, ctx.Err()
	}
	if r.failOn != "" && (name == r.failOn || (len(args) > 0 && args[0] == r.failOn)) {
		if !r.quiet {
			msg := r.msg
			if msg == "" {
				msg = "fatal: could not read from remote\nmore\n"
			}
			_, _ = io.WriteString(stderr, msg)
		}
		return r.failRC, r.failErr
	}
	return 0, nil
}

func (r *refreshRunner) argvs() []string {
	var out []string
	for _, c := range r.calls {
		out = append(out, c.name+" "+strings.Join(c.args, " "))
	}
	return out
}

func refreshWith(t *testing.T, run sysexec.RunFunc, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runWaveCodeRefresh(run, waveCodeRefreshTimeout, []string{"PATH=/usr/bin"}, args, &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func TestWaveCodeRefresh_FastForwardsTheIndexWorktreeThenRunsTheIncrementalRebuild(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{}

	rc, stdout, stderr := refreshWith(t, rr.run, "--project-root", root)

	want := []string{"git rev-parse --show-toplevel", "git fetch --quiet origin", "git checkout --quiet --detach origin/main", "/abs/trail build -- /abs/index"}
	if rc != 0 || !slices.Equal(rr.argvs(), want) {
		t.Fatalf("rc=%d argvs=%q, want 0 and %q; stderr:\n%s", rc, rr.argvs(), want, stderr)
	}
	for _, c := range rr.calls[:3] {
		if c.dir != "/abs/index" {
			t.Errorf("%s ran in %q, want the index worktree /abs/index", c.name, c.dir)
		}
	}
	if !strings.Contains(stdout, "code index refreshed") || stderr != "" {
		t.Errorf("stdout=%q stderr=%q, want a refreshed line and no warning", stdout, stderr)
	}
}

func TestWaveCodeRefresh_NeverNamesTheProjectRootTreeInAnyCommand(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{}

	refreshWith(t, rr.run, "--project-root", root)

	if len(rr.calls) == 0 {
		t.Fatal("no command ran")
	}
	for _, c := range rr.calls {
		for _, s := range append([]string{c.name, c.dir}, c.args...) {
			if strings.HasPrefix(s, root) {
				t.Errorf("command %s %v in %q touches the project root %s", c.name, c.args, c.dir, root)
			}
		}
	}
}

func TestWaveCodeRefresh_ResolvesTheDefaultIndexAgainstTheHubRoot(t *testing.T) {
	t.Parallel()
	runtime, _ := devHubFixture(t)
	hub, err := filepath.EvalSymlinks(filepath.Dir(runtime))
	if err != nil {
		t.Fatal(err)
	}
	rr := &refreshRunner{}

	rc, _, stderr := refreshWith(t, rr.run, "--project-root", runtime)

	wantIndex := filepath.Join(hub, "dev", "code-index")
	if rc != 0 || len(rr.calls) != 4 || rr.calls[0].dir != wantIndex || rr.calls[3].args[len(rr.calls[3].args)-1] != wantIndex {
		t.Errorf("rc=%d calls=%v, want the index worktree %s; stderr:\n%s", rc, rr.argvs(), wantIndex, stderr)
	}
}

func TestWaveCodeRefresh_AFailureIsAWarningAndTheStepStillSucceeds(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		runner    *refreshRunner
		wantCalls int
		wantWarn  string
	}{
		"fetch exits non-zero":     {&refreshRunner{failOn: "fetch", failRC: 128}, 2, "could not read from remote"},
		"fetch exits silently":     {&refreshRunner{failOn: "fetch", failRC: 9, quiet: true}, 2, "exit 9"},
		"checkout cannot start":    {&refreshRunner{failOn: "checkout", failErr: errors.New("spawn git")}, 3, "spawn git"},
		"rev-parse cannot start":   {&refreshRunner{failOn: "rev-parse", failErr: errors.New("spawn git")}, 1, "spawn git"},
		"the rebuild exits":        {&refreshRunner{failOn: "/abs/trail", failRC: 1}, 4, "trail"},
		"the rebuild cannot start": {&refreshRunner{failOn: "/abs/trail", failErr: errors.New("no node")}, 4, "no node"},
	} {
		root := codePolicyRoot(t, nil)

		rc, stdout, stderr := refreshWith(t, tc.runner.run, "--project-root", root)

		if rc != 0 || len(tc.runner.calls) != tc.wantCalls || !strings.Contains(stderr, "WARN: code index not refreshed") || !strings.Contains(stderr, tc.wantWarn) || strings.Contains(stdout, "refreshed at") {
			t.Errorf("%s: rc=%d calls=%q stdout=%q stderr=%q, want rc 0, %d calls and a WARN holding %q", name, rc, tc.runner.argvs(), stdout, stderr, tc.wantCalls, tc.wantWarn)
		}
	}
}

func TestWaveCodeRefresh_ARelativeIndexOutsideAHubIsAWarningAndRunsNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rr := &refreshRunner{}

	rc, _, stderr := refreshWith(t, rr.run, "--project-root", root)

	if rc != 0 || len(rr.calls) != 0 || !strings.Contains(stderr, "WARN: code index not refreshed") || !strings.Contains(stderr, "hub") {
		t.Errorf("rc=%d calls=%v stderr=%q, want rc 0, nothing run and a hub WARN", rc, rr.argvs(), stderr)
	}
}

func TestWaveCodeRefresh_ABrokenPolicyIsAWarning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCodePolicy(t, root, `{"code_index":{"backends":[]}}`)
	rr := &refreshRunner{}

	rc, _, stderr := refreshWith(t, rr.run, "--project-root", root)

	if rc != 0 || len(rr.calls) != 0 || !strings.Contains(stderr, "WARN: code index not refreshed") {
		t.Errorf("rc=%d calls=%v stderr=%q, want rc 0, nothing run and a WARN", rc, rr.argvs(), stderr)
	}
}

func TestWaveCodeRefresh_WithoutAProjectRootIsAUsageError(t *testing.T) {
	t.Parallel()
	rr := &refreshRunner{}

	rc, _, stderr := refreshWith(t, rr.run)

	if rc != exitUsage || len(rr.calls) != 0 || !strings.Contains(stderr, waveCodeRefreshVerb+" needs --project-root") {
		t.Errorf("rc=%d calls=%v stderr=%q, want usage and nothing run", rc, rr.argvs(), stderr)
	}
}

func TestWaveCodeRefresh_AFailedRefreshDoesNotStopTheBoundarySteps(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{failOn: "fetch", failRC: 128}
	refresh := dispatchWaveVerb(rr.run)
	var ran []string
	dispatch := func(verb string, args []string, stdout, stderr io.Writer) int {
		ran = append(ran, verb)
		if verb == waveCodeRefreshVerb {
			return refresh(verb, args, stdout, stderr)
		}
		return 0
	}
	steps := []boundaryStep{
		{verb: waveCodeRefreshVerb, args: []string{"--project-root", root}},
		{verb: "loop", args: []string{"--detach"}},
	}
	var stdout, stderr bytes.Buffer

	rc := runBoundarySteps(dispatch, steps, &stdout, &stderr)

	if rc != 0 || !slices.Equal(ran, []string{waveCodeRefreshVerb, "loop"}) || !strings.Contains(stderr.String(), "WARN") {
		t.Errorf("rc=%d ran=%v stderr=%q, want both steps run and a WARN", rc, ran, stderr.String())
	}
}

func TestWaveNext_DryRunListsTheRefreshStepBeforeTheLaunch(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	h := newWaveHarness()

	rc := h.run("next", "--dry-run", "--json", "--project-root", p.root)

	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	at := slices.Index(waveStepVerbs(e), waveCodeRefreshVerb)
	if rc != 0 || at < 0 {
		t.Fatalf("rc=%d steps=%v, want the refresh step", rc, waveStepVerbs(e))
	}
	if s := e.Steps[at]; s.Name != "refresh the code index" || s.Detail != "planned" || e.Steps[at+1].Verb != "loop-stop" {
		t.Errorf("refresh step = %+v next=%s, want its name and the launch next", s, e.Steps[at+1].Verb)
	}
}

func TestWaveCodeRefresh_AnUnknownBackendIsAWarningAndRunsNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCodePolicy(t, root, `{"code_index":{"backends":["nope"],"trail_bin":"/abs/trail","index_dir":"/abs/index"}}`)
	rr := &refreshRunner{}

	rc, _, stderr := refreshWith(t, rr.run, "--project-root", root)

	if rc != 0 || len(rr.calls) != 0 || !strings.Contains(stderr, `unknown backend "nope"`) {
		t.Errorf("rc=%d calls=%v stderr=%q, want rc 0, nothing run and an unknown-backend WARN", rc, rr.argvs(), stderr)
	}
}

func TestWaveCodeRefresh_AnIndexDirThatIsNotItsOwnWorktreeIsAWarningWithNoFetchOrCheckout(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{top: "/abs"}

	rc, stdout, stderr := refreshWith(t, rr.run, "--project-root", root)

	if rc != 0 || !slices.Equal(rr.argvs(), []string{"git rev-parse --show-toplevel"}) || !strings.Contains(stderr, "not its own git worktree") || strings.Contains(stdout, "refreshed at") {
		t.Errorf("rc=%d argvs=%q stderr=%q, want a WARN and only the rev-parse", rc, rr.argvs(), stderr)
	}
}

func TestWaveCodeRefresh_ADirtyIndexWorktreeWarnsAndNeverReachesTheRebuild(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{failOn: "checkout", failRC: 1, msg: "error: Your local changes would be overwritten by checkout\n"}

	rc, _, stderr := refreshWith(t, rr.run, "--project-root", root)

	want := []string{"git rev-parse --show-toplevel", "git fetch --quiet origin", "git checkout --quiet --detach origin/main"}
	if rc != 0 || !slices.Equal(rr.argvs(), want) || !strings.Contains(stderr, "WARN: code index not refreshed") || !strings.Contains(stderr, "local changes would be overwritten") {
		t.Errorf("rc=%d argvs=%q stderr=%q, want rc 0, no trail call and a conflict WARN", rc, rr.argvs(), stderr)
	}
}

func TestWaveCodeRefresh_AHungGitCallEndsAtTheDeadlineWithAWarning(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{blockOn: "fetch"}
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)

	go func() {
		done <- runWaveCodeRefresh(rr.run, 50*time.Millisecond, nil, []string{"--project-root", root}, &stdout, &stderr)
	}()

	select {
	case rc := <-done:
		if rc != 0 || !strings.Contains(stderr.String(), "WARN: code index not refreshed") || !strings.Contains(stderr.String(), "deadline exceeded") {
			t.Errorf("rc=%d stderr=%q, want rc 0 and a deadline WARN", rc, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the step outlived its deadline")
	}
}

func TestWaveCodeRefresh_AnIndexDirInsideTheProjectIsAWarningAndRunsNothing(t *testing.T) {
	t.Parallel()
	for name, sub := range map[string]string{
		"the project root":        "",
		"a directory in the root": "idx",
		"the project worktrees":   ".evolve/worktrees/cycle-1",
		"a dot-dot path to root":  "idx/..",
	} {
		root := t.TempDir()
		real, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatal(err)
		}
		index := filepath.Join(real, sub)
		writeCodePolicy(t, root, fmt.Sprintf(`{"code_index":{"trail_bin":"/abs/trail","index_dir":%q}}`, index))
		rr := &refreshRunner{}

		rc, _, stderr := refreshWith(t, rr.run, "--project-root", root)

		if rc != 0 || len(rr.calls) != 0 || !strings.Contains(stderr, "WARN: code index not refreshed") || !strings.Contains(stderr, "separate worktree") {
			t.Errorf("%s: rc=%d calls=%v stderr=%q, want a WARN and no call", name, rc, rr.argvs(), stderr)
		}
	}
}

func TestWaveCodeRefresh_AnIndexDirThroughASymlinkIntoTheRootIsRefused(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	writeCodePolicy(t, root, fmt.Sprintf(`{"code_index":{"trail_bin":"/abs/trail","index_dir":%q}}`, link))
	rr := &refreshRunner{}

	rc, _, stderr := refreshWith(t, rr.run, "--project-root", root)

	if rc != 0 || len(rr.calls) != 0 || !strings.Contains(stderr, "is the project root") {
		t.Errorf("rc=%d calls=%v stderr=%q, want the symlink resolved to the root and refused", rc, rr.argvs(), stderr)
	}
}

func TestWaveCodeRefresh_AnIndexDirInTheRuntimePlaneIsAWarningAndRunsNothing(t *testing.T) {
	t.Parallel()
	runtime, _ := devHubFixture(t)
	hub := filepath.Dir(runtime)
	store := filepath.Join(hub, ".repo.git")
	dev := filepath.Join(hub, "dev", "k2")
	brGit(t, store, "worktree", "add", "-q", "-b", "k2", dev)
	for name, index := range map[string]string{
		"the runtime root":   runtime,
		"inside the runtime": filepath.Join(runtime, "sub"),
	} {
		writeCodePolicy(t, dev, fmt.Sprintf(`{"code_index":{"trail_bin":"/abs/trail","index_dir":%q}}`, index))
		rr := &refreshRunner{}

		rc, _, stderr := refreshWith(t, rr.run, "--project-root", dev)

		if rc != 0 || len(rr.calls) != 0 || !strings.Contains(stderr, "runtime plane") {
			t.Errorf("%s: rc=%d calls=%v stderr=%q, want a runtime-plane WARN and no call", name, rc, rr.argvs(), stderr)
		}
	}
}
