package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/codeindex/gotypes"
)

type gotypesBuild struct {
	moduleDir, out string
	trailCalls     int
	hasDeadline    bool
}

type gotypesRecorder struct {
	mu     sync.Mutex
	rr     *refreshRunner
	builds []gotypesBuild
	err    error
}

func (g *gotypesRecorder) build(ctx context.Context, moduleDir, out string) error {
	g.rr.mu.Lock()
	seen := len(g.rr.calls)
	g.rr.mu.Unlock()
	g.mu.Lock()
	defer g.mu.Unlock()
	_, hasDeadline := ctx.Deadline()
	g.builds = append(g.builds, gotypesBuild{moduleDir: moduleDir, out: out, trailCalls: seen, hasDeadline: hasDeadline})
	return g.err
}

func gotypesHost(rr *refreshRunner, buildErr error) (codeIndexHost, *gotypesRecorder) {
	rec := &gotypesRecorder{rr: rr, err: buildErr}
	host := refreshHost(rr)
	host.buildGotypes = rec.build
	return host, rec
}

func nonBlankLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestCodeRefresh_Gotypes_BuildsTheIndexFromTheIndexWorktreeAfterTheTrailStep(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{}
	host, rec := gotypesHost(rr, nil)

	rc, stdout, stderr := refreshWith(t, host, "--project-root", root)

	if rc != 0 || len(rec.builds) != 1 {
		t.Fatalf("rc=%d builds=%v stderr=%q, want exit 0 and one gotypes build", rc, rec.builds, stderr)
	}
	got := rec.builds[0]
	if got.moduleDir != "/abs/index/go" || got.out != "/abs/index-gotypes/index.gob" {
		t.Errorf("build(%q, %q), want the go module of the index worktree and gotypes.IndexPath of it", got.moduleDir, got.out)
	}
	if got.trailCalls != len(fullRefresh) {
		t.Errorf("the build ran after %d command(s), want it after all %d, the trail build last", got.trailCalls, len(fullRefresh))
	}
	want := "code refresh: /abs/index is at origin/main (222222222222), 3 commit(s) after 111111111111\n"
	if stdout != want || stderr != "" {
		t.Errorf("stdout=%q stderr=%q, want the unchanged success line and no message", stdout, stderr)
	}
}

func TestCodeRefresh_Gotypes_WritesTheIndexBesideTheWorktreeOutsideEveryGitTree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(parent, "code-index")
	writeCodePolicy(t, root, fmt.Sprintf(`{"code_index":{"trail_bin":"/abs/trail","index_dir":%q}}`, index))
	rr := &refreshRunner{}
	host, rec := gotypesHost(rr, nil)

	rc, _, stderr := refreshWith(t, host, "--project-root", root)

	if rc != 0 || len(rec.builds) != 1 {
		t.Fatalf("rc=%d builds=%v stderr=%q", rc, rec.builds, stderr)
	}
	got := rec.builds[0]
	if got.out != gotypes.IndexPath(index) || got.out != filepath.Join(parent, "code-index-gotypes", "index.gob") {
		t.Errorf("out = %q, want %q", got.out, filepath.Join(parent, "code-index-gotypes", "index.gob"))
	}
	for name, tree := range map[string]string{"the index worktree": index, "the project root": root, "the module dir": got.moduleDir} {
		if pathWithin(tree, got.out) {
			t.Errorf("the index file %s lies inside %s", got.out, name)
		}
	}
	if got.moduleDir != filepath.Join(index, "go") {
		t.Errorf("moduleDir = %q, want the go directory of the index worktree", got.moduleDir)
	}
}

func TestCodeRefresh_Gotypes_AFailedBuildPrintsOneWarnLineAndKeepsTheTrailExit(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{}
	host, rec := gotypesHost(rr, errors.New("gotypes: go list in /abs/index/go: exit 1: boom"))

	rc, stdout, stderr := refreshWith(t, host, "--project-root", root)

	lines := nonBlankLines(stderr)
	if rc != 0 || len(rec.builds) != 1 || !slices.Equal(rr.argvs(), fullRefresh) {
		t.Fatalf("rc=%d builds=%d argvs=%q stderr=%q, want exit 0 after the full refresh", rc, len(rec.builds), rr.argvs(), stderr)
	}
	if len(lines) != 1 || !strings.Contains(lines[0], "WARN") || !strings.Contains(lines[0], "gotypes") || !strings.Contains(lines[0], "exit 1: boom") {
		t.Errorf("stderr = %q, want exactly one WARN line that names gotypes and the cause", stderr)
	}
	if !strings.HasPrefix(stdout, "code refresh: /abs/index is at origin/main") {
		t.Errorf("stdout = %q, want the success line of the move", stdout)
	}
}

func TestCodeRefresh_Gotypes_AFailedBuildUnderJSONIsOneWarningAndNotAnError(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{}
	host, _ := gotypesHost(rr, errors.New("gotypes: go list failed: boom"))

	rc, stdout, stderr := refreshWith(t, host, "--json", "--project-root", root)

	e := decodeCodeEnvelope(t, stdout)
	warnings, _ := e["warnings"].([]any)
	data, _ := e["data"].(map[string]any)
	if rc != 0 || e["ok"] != true || e["error"] != nil {
		t.Fatalf("rc=%d envelope=%v, want ok with no error", rc, e)
	}
	if len(warnings) != 1 || !strings.Contains(fmt.Sprint(warnings[0]), "go list failed: boom") || !strings.Contains(stderr, "WARN") {
		t.Errorf("warnings=%v stderr=%q, want the one warning in the envelope and on stderr", warnings, stderr)
	}
	if fmt.Sprint(data["rebuilt"]) != "[trail]" {
		t.Errorf("rebuilt = %v, want the trail backend only", data["rebuilt"])
	}
}

func TestCodeRefresh_Gotypes_ASuccessfulBuildUnderJSONAddsNoWarning(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	host, rec := gotypesHost(&refreshRunner{}, nil)

	rc, stdout, stderr := refreshWith(t, host, "--json", "--project-root", root)

	e := decodeCodeEnvelope(t, stdout)
	if rc != 0 || len(rec.builds) != 1 || fmt.Sprint(e["warnings"]) != "[]" || stderr != "" {
		t.Errorf("rc=%d builds=%d warnings=%v stderr=%q", rc, len(rec.builds), e["warnings"], stderr)
	}
}

func TestCodeRefresh_Gotypes_AFailedTrailStepStillBuildsAndKeepsItsOwnExit(t *testing.T) {
	t.Parallel()
	for name, buildErr := range map[string]error{"the build passes": nil, "the build fails too": errors.New("gotypes: boom")} {
		root := codePolicyRoot(t, nil)
		rr := &refreshRunner{failOn: "/abs/trail", failRC: 1}
		host, rec := gotypesHost(rr, buildErr)

		rc, stdout, stderr := refreshWith(t, host, "--project-root", root)

		if rc != codeExitFailed || len(rec.builds) != 1 || rec.builds[0].trailCalls != len(fullRefresh) || stdout != "" {
			t.Errorf("%s: rc=%d builds=%v stdout=%q, want exit 4 with one build after the trail step", name, rc, rec.builds, stdout)
		}
		if !strings.Contains(stderr, "trail") {
			t.Errorf("%s: stderr = %q, want the trail failure named", name, stderr)
		}
	}
}

func TestCodeRefresh_Gotypes_NoBuildRunsWhenTheRefreshStopsBeforeTheTrailStep(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		runner *refreshRunner
		stat   func(string) (os.FileInfo, error)
		rc     int
	}{
		"the index is absent":   {&refreshRunner{}, absentStat, codeExitAbsent},
		"a foreign change":      {&refreshRunner{status: " M go/main.go\n"}, presentStat, codeExitIndexDirty},
		"the fetch fails":       {&refreshRunner{failOn: "fetch", failRC: 128}, presentStat, codeExitIndexFetch},
		"the checkout conflict": {&refreshRunner{failOn: "checkout", failRC: 1}, presentStat, codeExitFailed},
	} {
		host, rec := gotypesHost(tc.runner, nil)
		host.stat = tc.stat

		rc, _, _ := refreshWith(t, host, "--project-root", codePolicyRoot(t, nil))

		if rc != tc.rc || len(rec.builds) != 0 {
			t.Errorf("%s: rc=%d builds=%v, want exit %d and no build", name, rc, rec.builds, tc.rc)
		}
	}
}

func TestCodeRefresh_Gotypes_InsideAPhaseNoBuildRuns(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, nil)
	rr := &refreshRunner{}
	host, rec := gotypesHost(rr, nil)
	var stdout, stderr bytes.Buffer
	inPhase := func(key string) string {
		if key == "EVOLVE_DISPATCH_DEPTH" {
			return "1"
		}
		return ""
	}

	rc := codeRefreshVerb([]string{"--project-root", root}, &stdout, &stderr, host, inPhase, codeIndexRefreshTimeout)

	if rc != codeExitRefused || len(rec.builds) != 0 || len(rr.calls) != 0 {
		t.Errorf("rc=%d builds=%v calls=%v, want the refusal and nothing run", rc, rec.builds, rr.argvs())
	}
}

func TestSyncMainCodeIndex_Gotypes_TheMergeBoundaryBuildsTheIndexAfterTheTrailStep(t *testing.T) {
	t.Parallel()
	rr := &refreshRunner{}
	host, rec := gotypesHost(rr, nil)

	rc, stdout, stderr := syncIndexWith(t, host, codePolicyRoot(t, nil))

	if rc != 0 || len(rec.builds) != 1 || rec.builds[0].out != "/abs/index-gotypes/index.gob" || rec.builds[0].trailCalls != len(fullRefresh) {
		t.Fatalf("rc=%d builds=%v stderr=%q, want exit 0 and one build after the trail step", rc, rec.builds, stderr)
	}
	if !strings.HasPrefix(stdout, "sync-main: moved the code index /abs/index to origin/main") || stderr != "" {
		t.Errorf("stdout=%q stderr=%q, want the unchanged moved line and no message", stdout, stderr)
	}
}

func TestSyncMainCodeIndex_Gotypes_AFailedBuildIsOneWarnLineAndSyncMainStillExitsZero(t *testing.T) {
	t.Parallel()
	rr := &refreshRunner{}
	host, rec := gotypesHost(rr, errors.New("gotypes: type check: boom"))

	rc, stdout, stderr := syncIndexWith(t, host, codePolicyRoot(t, nil))

	lines := nonBlankLines(stderr)
	if rc != 0 || len(rec.builds) != 1 || !strings.HasPrefix(stdout, "sync-main: moved the code index") {
		t.Fatalf("rc=%d builds=%d stdout=%q stderr=%q, want exit 0 and the moved line", rc, len(rec.builds), stdout, stderr)
	}
	if len(lines) != 1 || !strings.Contains(lines[0], "WARN") || !strings.Contains(lines[0], "type check: boom") {
		t.Errorf("stderr = %q, want exactly one WARN line with the cause", stderr)
	}
}

func TestDefaultCodeIndexHost_Gotypes_BuildsThroughTheRealGotypesBuild(t *testing.T) {
	t.Parallel()
	host := defaultCodeIndexHost()
	out := filepath.Join(t.TempDir(), "idx-gotypes", "index.gob")

	if host.buildGotypes == nil {
		t.Fatal("the production host has no gotypes build: the refresh cannot reach it")
	}
	err := host.buildGotypes(context.Background(), filepath.Join(t.TempDir(), "no-such-module"), out)

	if err == nil || !strings.Contains(err.Error(), "gotypes: the module dir") {
		t.Errorf("buildGotypes err = %v, want the module-dir error of gotypes.Build", err)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Errorf("a failed build wrote %s", out)
	}
}

func untilTheContextEnds(ctx context.Context, _, _ string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return errors.New("the build never saw the deadline of the refresh")
	}
}

func TestCodeRefresh_Gotypes_TheBuildRunsUnderTheDeadlineOfTheRefresh(t *testing.T) {
	t.Parallel()
	host, rec := gotypesHost(&refreshRunner{}, nil)

	rc, _, stderr := refreshWith(t, host, "--project-root", codePolicyRoot(t, nil))

	if rc != 0 || len(rec.builds) != 1 || !rec.builds[0].hasDeadline {
		t.Errorf("rc=%d builds=%+v stderr=%q, want one build whose context carries the refresh deadline", rc, rec.builds, stderr)
	}
}

func TestCodeRefresh_Gotypes_ABuildThatOutlivesTheDeadlineIsOneWarnLineAndTheRefreshStillExitsZero(t *testing.T) {
	t.Parallel()
	host := refreshHost(&refreshRunner{})
	host.buildGotypes = untilTheContextEnds
	var stdout, stderr bytes.Buffer

	rc := codeRefreshVerb([]string{"--project-root", codePolicyRoot(t, nil)}, &stdout, &stderr, host, noEnv, 200*time.Millisecond)

	lines := nonBlankLines(stderr.String())
	if rc != 0 || len(lines) != 1 || !strings.Contains(lines[0], "WARN") || !strings.Contains(lines[0], "deadline exceeded") {
		t.Errorf("rc=%d stderr=%q, want exit 0 and one WARN line that names the exceeded deadline", rc, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "code refresh: /abs/index is at origin/main") {
		t.Errorf("stdout = %q, want the success line of the move", stdout.String())
	}
}

func TestSyncMainCodeIndex_Gotypes_ABuildThatOutlivesTheDeadlineIsOneWarnLineAndSyncMainStillExitsZero(t *testing.T) {
	t.Parallel()
	host := refreshHost(&refreshRunner{})
	host.buildGotypes = untilTheContextEnds
	var stdout, stderr bytes.Buffer

	rc := syncMainCodeIndex(codePolicyRoot(t, nil), host, 200*time.Millisecond, &stdout, &stderr)

	lines := nonBlankLines(stderr.String())
	if rc != 0 || len(lines) != 1 || !strings.Contains(lines[0], "deadline exceeded") || !strings.HasPrefix(stdout.String(), "sync-main: moved the code index") {
		t.Errorf("rc=%d stdout=%q stderr=%q, want exit 0, the moved line and one WARN line", rc, stdout.String(), stderr.String())
	}
}

const goListFailure = "gotypes: go list in /abs/index/go: exit 1: first problem\nsecond problem\n\tthird problem"

func TestCodeRefresh_Gotypes_AMultiLineCauseIsStillOneWarnLine(t *testing.T) {
	t.Parallel()
	host, _ := gotypesHost(&refreshRunner{}, errors.New(goListFailure))

	rc, _, stderr := refreshWith(t, host, "--project-root", codePolicyRoot(t, nil))

	lines := nonBlankLines(stderr)
	if rc != 0 || len(lines) != 1 || !strings.Contains(lines[0], "WARN") || !strings.Contains(lines[0], "first problem") {
		t.Errorf("rc=%d stderr=%q, want exit 0 and one WARN line that carries the first line of the cause", rc, stderr)
	}
	if strings.Contains(stderr, "second problem") || !strings.HasSuffix(stderr, "\n") || strings.HasSuffix(stderr, "\n\n") {
		t.Errorf("stderr = %q, want the later lines of the cause cut and exactly one line end", stderr)
	}
}

func TestCodeRefresh_Gotypes_AMultiLineCauseUnderJSONIsOneWarningWithoutALineBreak(t *testing.T) {
	t.Parallel()
	host, _ := gotypesHost(&refreshRunner{}, errors.New(goListFailure))

	rc, stdout, _ := refreshWith(t, host, "--json", "--project-root", codePolicyRoot(t, nil))

	e := decodeCodeEnvelope(t, stdout)
	warnings, _ := e["warnings"].([]any)
	if rc != 0 || len(warnings) != 1 || strings.Contains(fmt.Sprint(warnings[0]), "\n") || !strings.Contains(fmt.Sprint(warnings[0]), "first problem") {
		t.Errorf("rc=%d warnings=%q, want one warning on one line", rc, warnings)
	}
}

func TestSyncMainCodeIndex_Gotypes_AMultiLineCauseIsStillOneWarnLine(t *testing.T) {
	t.Parallel()
	host, _ := gotypesHost(&refreshRunner{}, errors.New(goListFailure))

	rc, _, stderr := syncIndexWith(t, host, codePolicyRoot(t, nil))

	lines := nonBlankLines(stderr)
	if rc != 0 || len(lines) != 1 || !strings.Contains(lines[0], "first problem") || strings.Contains(stderr, "second problem") {
		t.Errorf("rc=%d stderr=%q, want exit 0 and one WARN line with the first line of the cause", rc, stderr)
	}
}
