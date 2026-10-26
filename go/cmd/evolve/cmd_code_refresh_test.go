package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
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
	status  string
	heads   string
	behind  string
	blockOn string
}

func (r *refreshRunner) answer(dir string, args []string) string {
	switch strings.Join(args, " ") {
	case "rev-parse --show-toplevel":
		return cmpOr(r.top, dir) + "\n"
	case "status --porcelain --untracked-files=normal":
		return r.status
	case "rev-parse HEAD origin/main":
		return cmpOr(r.heads, "1111111111111111\n2222222222222222\n")
	case "rev-list --count HEAD..origin/main":
		return cmpOr(r.behind, "3\n")
	}
	return ""
}

func cmpOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (r *refreshRunner) run(ctx context.Context, name, dir string, args, _ []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, refreshCall{name: name, dir: dir, args: args})
	verb := strings.Join(append([]string{name}, args...), " ")
	if r.blockOn != "" && strings.Contains(verb, r.blockOn) {
		r.mu.Unlock()
		<-ctx.Done()
		r.mu.Lock()
		return 0, ctx.Err()
	}
	if r.failOn != "" && strings.Contains(verb, r.failOn) {
		if !r.quiet {
			_, _ = io.WriteString(stderr, cmpOr(r.msg, "fatal: could not read from remote\nmore\n"))
		}
		return r.failRC, r.failErr
	}
	_, _ = io.WriteString(stdout, r.answer(dir, args))
	return 0, nil
}

func (r *refreshRunner) argvs() []string {
	var out []string
	for _, c := range r.calls {
		out = append(out, c.name+" "+strings.Join(c.args, " "))
	}
	return out
}

func presentStat(string) (os.FileInfo, error) { return nil, nil }

func absentStat(string) (os.FileInfo, error) { return nil, fs.ErrNotExist }

func refreshHost(run *refreshRunner) codeIndexHost {
	return codeIndexHost{run: run.run, env: []string{"PATH=/usr/bin"}, stat: presentStat}
}

func noEnv(string) string { return "" }

func refreshWith(t *testing.T, host codeIndexHost, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := codeRefreshVerb(args, &stdout, &stderr, host, noEnv, codeIndexRefreshTimeout)
	return rc, stdout.String(), stderr.String()
}

var fullRefresh = []string{
	"git rev-parse --show-toplevel",
	"git status --porcelain --untracked-files=normal",
	"git fetch --quiet origin main",
	"git rev-parse HEAD origin/main",
	"git rev-list --count HEAD..origin/main",
	"git checkout --quiet --detach origin/main",
	"/abs/trail build -- /abs/index",
}

func TestCodeRefresh_MovesTheIndexWorktreeToOriginMainDetachedThenRebuilds(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{}

	rc, stdout, stderr := refreshWith(t, refreshHost(rr), "--project-root", root)

	if rc != 0 || !slices.Equal(rr.argvs(), fullRefresh) {
		t.Fatalf("rc=%d argvs=%q, want 0 and %q; stderr:\n%s", rc, rr.argvs(), fullRefresh, stderr)
	}
	for _, c := range rr.calls[:6] {
		if c.dir != "/abs/index" {
			t.Errorf("%s %v ran in %q, want the index worktree /abs/index", c.name, c.args, c.dir)
		}
	}
	want := "code refresh: /abs/index is at origin/main (222222222222), 3 commit(s) after 111111111111\n"
	if stdout != want || stderr != "" {
		t.Errorf("stdout=%q stderr=%q, want %q and no message", stdout, stderr, want)
	}
}

func decodeCodeEnvelope(t *testing.T, raw string) map[string]any {
	t.Helper()
	var e map[string]any
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatalf("stdout %q is not one JSON object: %v", raw, err)
	}
	return e
}

func TestCodeRefresh_JSONPrintsOneEnvelopeWithTheMove(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{}

	rc, stdout, _ := refreshWith(t, refreshHost(rr), "--json", "--project-root", root)

	e := decodeCodeEnvelope(t, stdout)
	data, _ := e["data"].(map[string]any)
	if rc != 0 || e["schema"] != codeRefreshSchema || e["ok"] != true || e["error"] != nil || fmt.Sprint(e["warnings"]) != "[]" {
		t.Fatalf("rc=%d envelope=%v", rc, e)
	}
	if data["index_dir"] != "/abs/index" || data["from"] != "1111111111111111" || data["to"] != "2222222222222222" || data["behind"] != 3.0 || fmt.Sprint(data["rebuilt"]) != "[trail]" {
		t.Errorf("data = %v", data)
	}
}

func TestCodeRefresh_AnIndexAtOriginMainSkipsTheCountAndStillRebuilds(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{heads: "abc\nabc\n"}

	rc, stdout, stderr := refreshWith(t, refreshHost(rr), "--project-root", root)

	if rc != 0 || slices.Contains(rr.argvs(), "git rev-list --count HEAD..origin/main") || !strings.Contains(stdout, "0 commit(s) after abc") {
		t.Errorf("rc=%d argvs=%q stdout=%q stderr=%q", rc, rr.argvs(), stdout, stderr)
	}
}

func TestCodeRefresh_NeverNamesTheProjectRootTreeInAnyCommand(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{}

	refreshWith(t, refreshHost(rr), "--project-root", root)

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

func TestCodeRefresh_ResolvesTheDefaultIndexAgainstTheHubRoot(t *testing.T) {
	t.Parallel()
	runtime, _ := devHubFixture(t)
	hub, err := filepath.EvalSymlinks(filepath.Dir(runtime))
	if err != nil {
		t.Fatal(err)
	}
	rr := &refreshRunner{}

	rc, _, stderr := refreshWith(t, refreshHost(rr), "--project-root", runtime)

	wantIndex := filepath.Join(hub, "dev", "code-index")
	last := rr.calls[len(rr.calls)-1]
	if rc != 0 || len(rr.calls) != len(fullRefresh) || rr.calls[0].dir != wantIndex || last.args[len(last.args)-1] != wantIndex {
		t.Errorf("rc=%d calls=%v, want the index worktree %s; stderr:\n%s", rc, rr.argvs(), wantIndex, stderr)
	}
}

func TestCodeRefresh_AnAbsentIndexExitsTwoAndRunsNothing(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{}
	host := refreshHost(rr)
	host.stat = absentStat

	rc, stdout, stderr := refreshWith(t, host, "--json", "--project-root", root)

	e := decodeCodeEnvelope(t, stdout)
	failure, _ := e["error"].(map[string]any)
	if rc != codeExitAbsent || len(rr.calls) != 0 || !strings.Contains(stderr, "the code index is absent: /abs/index does not exist") {
		t.Errorf("rc=%d calls=%v stderr=%q, want exit 2, no call and the absent path", rc, rr.argvs(), stderr)
	}
	if e["ok"] != false || failure["code"] != 2.0 || failure["kind"] != "absent" || e["data"] != nil {
		t.Errorf("envelope = %v, want an absent error", e)
	}
}

func TestCodeRefresh_ARelativeIndexOutsideAHubIsAbsent(t *testing.T) {
	t.Parallel()
	rr := &refreshRunner{}

	rc, _, stderr := refreshWith(t, refreshHost(rr), "--project-root", t.TempDir())

	if rc != codeExitAbsent || len(rr.calls) != 0 || !strings.Contains(stderr, "not a hub worktree") {
		t.Errorf("rc=%d calls=%v stderr=%q, want exit 2 and a hub message", rc, rr.argvs(), stderr)
	}
}

func TestCodeRefresh_AForeignChangeInTheIndexExitsTwentyTwoAndResetsNothing(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{status: " M .gitignore\n?? .ignore\n M go/main.go\n?? notes/\n"}

	rc, stdout, stderr := refreshWith(t, refreshHost(rr), "--project-root", root)

	want := []string{"git rev-parse --show-toplevel", "git status --porcelain --untracked-files=normal"}
	if rc != codeExitIndexDirty || !slices.Equal(rr.argvs(), want) || stdout != "" {
		t.Fatalf("rc=%d argvs=%q stdout=%q, want exit 22 and no fetch, checkout or reset", rc, rr.argvs(), stdout)
	}
	if !strings.Contains(stderr, "not the index tool's own: go/main.go, notes/;") || strings.Contains(stderr, ".gitignore") || !strings.Contains(stderr, "nothing was reset") {
		t.Errorf("stderr = %q, want the two foreign paths only and the no-reset remedy", stderr)
	}
}

func TestCodeRefresh_TheIndexToolsOwnEditsDoNotBlockTheMove(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{status: " M .gitignore\n?? .ignore\n"}

	rc, _, stderr := refreshWith(t, refreshHost(rr), "--project-root", root)

	if rc != 0 || !slices.Equal(rr.argvs(), fullRefresh) {
		t.Errorf("rc=%d argvs=%q stderr=%q, want the full refresh", rc, rr.argvs(), stderr)
	}
}

func TestCodeRefresh_AFetchFailureExitsTwentyThreeBeforeTheCheckout(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		runner *refreshRunner
		want   string
	}{
		"fetch exits non-zero": {&refreshRunner{failOn: "fetch", failRC: 128}, "could not read from remote"},
		"fetch exits silently": {&refreshRunner{failOn: "fetch", failRC: 9, quiet: true}, "exit 9"},
		"fetch cannot start":   {&refreshRunner{failOn: "fetch", failErr: errors.New("spawn git")}, "spawn git"},
	} {
		root := codePolicyRoot(t, nil)

		rc, _, stderr := refreshWith(t, refreshHost(tc.runner), "--project-root", root)

		if rc != codeExitIndexFetch || len(tc.runner.calls) != 3 || !strings.Contains(stderr, "git fetch origin main failed") || !strings.Contains(stderr, tc.want) {
			t.Errorf("%s: rc=%d calls=%q stderr=%q, want exit 23 holding %q", name, rc, tc.runner.argvs(), stderr, tc.want)
		}
	}
}

func TestCodeRefresh_AnotherFailureExitsFourAndStopsAtTheFailedStep(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		runner    *refreshRunner
		wantCalls int
		want      string
	}{
		"rev-parse cannot start":    {&refreshRunner{failOn: "show-toplevel", failErr: errors.New("spawn git")}, 1, "spawn git"},
		"status fails":              {&refreshRunner{failOn: "status", failRC: 128}, 2, "git status in /abs/index: exit 128"},
		"the heads fail":            {&refreshRunner{failOn: "HEAD origin/main", failRC: 128}, 4, "git rev-parse in /abs/index"},
		"one head only":             {&refreshRunner{heads: "abc\n"}, 4, `want two object names, got "abc"`},
		"the count fails":           {&refreshRunner{failOn: "rev-list", failRC: 1}, 5, "git rev-list in /abs/index"},
		"the count is not a number": {&refreshRunner{behind: "many\n"}, 5, "git rev-list --count in /abs/index"},
		"checkout conflicts":        {&refreshRunner{failOn: "checkout", failRC: 1, msg: "error: Your local changes would be overwritten by checkout\n"}, 6, "local changes would be overwritten by checkout; clean the index worktree"},
		"the rebuild exits":         {&refreshRunner{failOn: "/abs/trail", failRC: 1}, 7, "trail"},
		"the rebuild cannot start":  {&refreshRunner{failOn: "/abs/trail", failErr: errors.New("no node")}, 7, "no node"},
	} {
		root := codePolicyRoot(t, nil)

		rc, stdout, stderr := refreshWith(t, refreshHost(tc.runner), "--project-root", root)

		if rc != codeExitFailed || len(tc.runner.calls) != tc.wantCalls || !strings.Contains(stderr, tc.want) || stdout != "" {
			t.Errorf("%s: rc=%d calls=%q stdout=%q stderr=%q, want exit 4 after %d calls holding %q", name, rc, tc.runner.argvs(), stdout, stderr, tc.wantCalls, tc.want)
		}
	}
}

func TestCodeRefresh_AStatErrorThatIsNotAbsenceExitsFour(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{}
	host := refreshHost(rr)
	host.stat = func(string) (os.FileInfo, error) { return nil, fs.ErrPermission }

	rc, _, stderr := refreshWith(t, host, "--project-root", root)

	if rc != codeExitFailed || len(rr.calls) != 0 || !strings.Contains(stderr, "code index /abs/index: permission denied") {
		t.Errorf("rc=%d calls=%v stderr=%q", rc, rr.argvs(), stderr)
	}
}

func TestCodeRefresh_AnIndexDirThatIsNotItsOwnWorktreeExitsFourWithNoFetch(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{top: "/abs"}

	rc, _, stderr := refreshWith(t, refreshHost(rr), "--project-root", root)

	if rc != codeExitFailed || !slices.Equal(rr.argvs(), []string{"git rev-parse --show-toplevel"}) || !strings.Contains(stderr, "not its own git worktree") {
		t.Errorf("rc=%d argvs=%q stderr=%q, want exit 4 and only the rev-parse", rc, rr.argvs(), stderr)
	}
}

func TestCodeRefresh_ABrokenPolicyOrAnUnknownBackendExitsFourAndRunsNothing(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"no backend":      `{"code_index":{"backends":[]}}`,
		"unknown backend": `{"code_index":{"backends":["nope"],"trail_bin":"/abs/trail","index_dir":"/abs/index"}}`,
	} {
		root := t.TempDir()
		writeCodePolicy(t, root, body)
		rr := &refreshRunner{}

		rc, _, stderr := refreshWith(t, refreshHost(rr), "--project-root", root)

		if rc != codeExitFailed || len(rr.calls) != 0 || !strings.Contains(stderr, "evolve code refresh: ") {
			t.Errorf("%s: rc=%d calls=%v stderr=%q", name, rc, rr.argvs(), stderr)
		}
	}
}

func TestCodeRefresh_AHungGitCallEndsAtTheDeadline(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{blockOn: "fetch"}
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)

	go func() {
		done <- codeRefreshVerb([]string{"--project-root", root}, &stdout, &stderr, refreshHost(rr), noEnv, 50*time.Millisecond)
	}()

	select {
	case rc := <-done:
		if rc != codeExitIndexFetch || !strings.Contains(stderr.String(), "deadline exceeded") {
			t.Errorf("rc=%d stderr=%q, want exit 23 and a deadline message", rc, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the refresh outlived its deadline")
	}
}

func TestCodeRefresh_AnIndexDirInsideTheProjectIsRefusedAndRunsNothing(t *testing.T) {
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
		writeCodePolicy(t, root, fmt.Sprintf(`{"code_index":{"trail_bin":"/abs/trail","index_dir":%q}}`, filepath.Join(real, sub)))
		rr := &refreshRunner{}

		rc, _, stderr := refreshWith(t, refreshHost(rr), "--project-root", root)

		if rc != codeExitFailed || len(rr.calls) != 0 || !strings.Contains(stderr, "separate worktree") {
			t.Errorf("%s: rc=%d calls=%v stderr=%q, want exit 4 and no call", name, rc, rr.argvs(), stderr)
		}
	}
}

func TestCodeRefresh_AnIndexDirThroughASymlinkIntoTheRootIsRefused(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	writeCodePolicy(t, root, fmt.Sprintf(`{"code_index":{"trail_bin":"/abs/trail","index_dir":%q}}`, link))
	rr := &refreshRunner{}

	rc, _, stderr := refreshWith(t, refreshHost(rr), "--project-root", root)

	if rc != codeExitFailed || len(rr.calls) != 0 || !strings.Contains(stderr, "is the project root") {
		t.Errorf("rc=%d calls=%v stderr=%q, want the symlink resolved to the root and refused", rc, rr.argvs(), stderr)
	}
}

func TestCodeRefresh_AnIndexDirInTheRuntimePlaneIsRefused(t *testing.T) {
	t.Parallel()
	runtime, _ := devHubFixture(t)
	hub := filepath.Dir(runtime)
	dev := filepath.Join(hub, "dev", "k2")
	brGit(t, filepath.Join(hub, ".repo.git"), "worktree", "add", "-q", "-b", "k2", dev)
	for name, index := range map[string]string{
		"the runtime root":   runtime,
		"inside the runtime": filepath.Join(runtime, "sub"),
	} {
		writeCodePolicy(t, dev, fmt.Sprintf(`{"code_index":{"trail_bin":"/abs/trail","index_dir":%q}}`, index))
		rr := &refreshRunner{}

		rc, _, stderr := refreshWith(t, refreshHost(rr), "--project-root", dev)

		if rc != codeExitFailed || len(rr.calls) != 0 || !strings.Contains(stderr, "runtime plane") {
			t.Errorf("%s: rc=%d calls=%v stderr=%q, want a runtime-plane refusal and no call", name, rc, rr.argvs(), stderr)
		}
	}
}

func TestCodeRefresh_UsageErrorsExitTenWithAnEnvelopeUnderJSON(t *testing.T) {
	t.Parallel()
	for name, args := range map[string][]string{
		"a stray word":    {"now"},
		"an unknown flag": {"--bogus"},
		"json and a word": {"--json", "now"},
		"a missing value": {"--project-root"},
	} {
		rr := &refreshRunner{}

		rc, stdout, stderr := refreshWith(t, refreshHost(rr), args...)

		if rc != codeExitUsage || len(rr.calls) != 0 || !strings.Contains(stderr, "usage: evolve code") {
			t.Errorf("%s: rc=%d calls=%v stderr=%q", name, rc, rr.argvs(), stderr)
		}
		if slices.Contains(args, "--json") {
			e := decodeCodeEnvelope(t, stdout)
			if failure, _ := e["error"].(map[string]any); failure["kind"] != "usage" || failure["message"] != "evolve code refresh: want no argument" {
				t.Errorf("%s: envelope = %v", name, e)
			}
		} else if stdout != "" {
			t.Errorf("%s: stdout = %q, want none without --json", name, stdout)
		}
	}
}

func TestCodeRefresh_InsideAPhaseItRefusesWithExitThreeAndRunsNothing(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{}
	var stdout, stderr bytes.Buffer
	inPhase := func(key string) string {
		if key == "EVOLVE_DISPATCH_DEPTH" {
			return "1"
		}
		return ""
	}

	rc := codeRefreshVerb([]string{"--project-root", root}, &stdout, &stderr, refreshHost(rr), inPhase, codeIndexRefreshTimeout)

	if rc != codeExitRefused || len(rr.calls) != 0 || !strings.Contains(stderr.String(), "refused inside a phase") {
		t.Errorf("rc=%d calls=%v stderr=%q", rc, rr.argvs(), stderr.String())
	}
}

func TestRunCodeCommand_RefreshRoutesToTheRefreshVerb(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer

	rc := runCodeCommand([]string{"refresh", "--project-root", t.TempDir()}, nil, &stdout, &stderr)

	if rc != codeExitAbsent || !strings.Contains(stderr.String(), "evolve code refresh: the code index is absent") {
		t.Errorf("rc=%d stderr=%q, want the refresh verb to report a missing hub index", rc, stderr.String())
	}
}

func TestDefaultCodeIndexHost_UsesTheRealRunnerAndTheRealStat(t *testing.T) {
	t.Parallel()
	host := defaultCodeIndexHost()

	_, err := host.stat(filepath.Join(t.TempDir(), "none"))

	if host.run == nil || !errors.Is(err, fs.ErrNotExist) || len(host.env) == 0 {
		t.Errorf("host = %+v, stat err = %v", host, err)
	}
}

func TestWaveNext_HasNoSeparateRefreshStepBecauseSyncMainOwnsIt(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	h := newWaveHarness()

	rc := h.run("next", "--dry-run", "--json", "--project-root", p.root)

	verbs := waveStepVerbs(decodeWaveEnvelope(t, h.stdout.Bytes()))
	if rc != 0 || slices.Contains(verbs, "code-refresh") || !slices.Contains(verbs, "sync-main") {
		t.Errorf("rc=%d steps=%v, want sync-main and no code-refresh step", rc, verbs)
	}
}

func TestForeignIndexChanges_OnlyTheTwoToolFilesAtTheTopAreTheToolsOwn(t *testing.T) {
	t.Parallel()
	porcelain := strings.Join([]string{
		" M .gitignore",
		"?? .ignore",
		"A  .ignore",
		"R  .gitignore -> x",
		`?? "sp ace"`,
		"?? sub/.gitignore",
		"",
		"M",
	}, "\n")

	got := foreignIndexChanges(porcelain)

	want := []string{".gitignore -> x", `"sp ace"`, "sub/.gitignore"}
	if !slices.Equal(got, want) {
		t.Errorf("foreignIndexChanges = %q, want %q", got, want)
	}
}
