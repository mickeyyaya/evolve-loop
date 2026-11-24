package bridge

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var hookEvolveDirArg = regexp.MustCompile(`guard ship --evolve-dir \\"([^"\\]+)\\"`)

func shipHookEvolveDir(t *testing.T, projectDir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := hookEvolveDirArg.FindSubmatch(raw)
	if m == nil {
		t.Fatal("no `guard ship --evolve-dir` hook in .claude/settings.json; the wiring parse is broken")
	}
	return strings.ReplaceAll(string(m[1]), "$CLAUDE_PROJECT_DIR", projectDir)
}

func fixtureDenial(t *testing.T, workingDir, guard, cwd, path string) string {
	t.Helper()
	stream := filepath.Join(shipHookEvolveDir(t, workingDir), signalcenter.StreamFileName)
	return strings.Join([]string{stream, guard, cwd, path}, "|")
}

func writeFixtureDenial(spec string) {
	parts := strings.SplitN(spec, "|", 4)
	if err := os.MkdirAll(filepath.Dir(parts[0]), 0o755); err != nil {
		panic(err)
	}
	c := signalcenter.New()
	c.Subscribe(c.NDJSONSink(func(int) string { return parts[0] }))
	guards.EmitDenial(c, guards.Denial{Guard: parts[1], Path: parts[3], Reason: "denied", CWD: parts[2]})
}

func TestHookEvolveDir_IsTheDirectoryTheShipHookWritesTo(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	if got, want := HookEvolveDir(wd), shipHookEvolveDir(t, wd); got != want {
		t.Fatalf("HookEvolveDir(%q) = %q; the hook in .claude/settings.json writes %q", wd, got, want)
	}
}

func TestResolveWorkingDir_TheWorktreeWinsAndTheProcessCwdIsTheFallback(t *testing.T) {
	t.Parallel()
	cwd, _ := os.Getwd()
	if got := resolveWorkingDir("/w/cell"); got != "/w/cell" {
		t.Errorf("worktree: %q", got)
	}
	if got := resolveWorkingDir(""); got != cwd {
		t.Errorf("fallback: %q, want %q", got, cwd)
	}
}

func openedWindow(t *testing.T, wd string, before ...string) denialWindow {
	t.Helper()
	for _, spec := range before {
		writeFixtureDenial(spec)
	}
	return openDenialWindow(wd)
}

func TestDenialWindow_ReadsOnlyWhatWasWrittenAfterItOpened(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	w := openedWindow(t, wd, fixtureDenial(t, wd, "ship", wd, "old push"))
	if got := w.refusal(launchoutcome.ExitArtifactTimeout); got.Present() {
		t.Fatalf("a denial before the launch must not match: %+v", got)
	}
	writeFixtureDenial(fixtureDenial(t, wd, "ship", wd, "new push"))
	if got := w.refusal(launchoutcome.ExitArtifactTimeout); got != (launchoutcome.Refusal{Guard: "ship", Path: "new push"}) {
		t.Fatalf("a denial after the launch start matches: %+v", got)
	}
}

func TestDenialWindow_OnlyAStallReadsTheStream(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	w := openedWindow(t, wd)
	writeFixtureDenial(fixtureDenial(t, wd, "ship", wd, "push"))
	for _, code := range []int{ExitOK, ExitREPLBootTimeout, 85, 2} {
		if got := w.refusal(code); got.Present() {
			t.Errorf("exit %d must not read a denial: %+v", code, got)
		}
	}
}

func TestDenialWindow_ADenialOfAnotherCellOrAFamilyBoundGuardDoesNotMatch(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	w := openedWindow(t, wd)
	writeFixtureDenial(fixtureDenial(t, wd, "ship", t.TempDir(), "push"))
	writeFixtureDenial(fixtureDenial(t, wd, "role", wd, "/etc/x"))
	if got := w.refusal(launchoutcome.ExitArtifactTimeout); got.Present() {
		t.Fatalf("no match expected: %+v", got)
	}
}

func TestDenialWindow_AMissingStreamOrUnknownCellMatchesNothing(t *testing.T) {
	t.Parallel()
	if got := openDenialWindow(t.TempDir()).refusal(launchoutcome.ExitArtifactTimeout); got.Present() {
		t.Errorf("no stream: %+v", got)
	}
	if got := openDenialWindow("").refusal(launchoutcome.ExitArtifactTimeout); got.Present() {
		t.Errorf("no cell: %+v", got)
	}
}
