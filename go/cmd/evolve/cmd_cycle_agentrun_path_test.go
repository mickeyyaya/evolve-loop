package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const hostPath = "/usr/bin:/bin"

func evolveBinaryDirs(t *testing.T) []string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dirs := []string{filepath.Dir(exe)}
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(exe)); err == nil && resolved != dirs[0] {
		dirs = append(dirs, resolved)
	}
	return dirs
}

func firstEntryIsEvolveDir(t *testing.T, pathValue string) bool {
	t.Helper()
	first, _, _ := strings.Cut(pathValue, string(os.PathListSeparator))
	for _, dir := range evolveBinaryDirs(t) {
		if first == dir {
			return true
		}
	}
	return false
}

func occurrencesOfEvolveDir(t *testing.T, pathValue string) int {
	t.Helper()
	count := 0
	for _, entry := range filepath.SplitList(pathValue) {
		for _, dir := range evolveBinaryDirs(t) {
			if entry == dir {
				count++
				break
			}
		}
	}
	return count
}

func TestAgentRunBridge_EvolveBinaryDirOnPath(t *testing.T) {
	t.Setenv("PATH", hostPath)

	agentRunBridge(t.TempDir(), nil)

	got := os.Getenv("PATH")
	if !firstEntryIsEvolveDir(t, got) {
		t.Errorf("PATH = %q, want the evolve binary directory %q first", got, evolveBinaryDirs(t)[0])
	}
	if !strings.HasSuffix(got, string(os.PathListSeparator)+hostPath) {
		t.Errorf("PATH = %q, want the host PATH %q kept after the evolve binary directory", got, hostPath)
	}
}

func TestAgentRunBridge_EvolveBinaryDirOnPathLeavesNoEmptyEntry(t *testing.T) {
	t.Setenv("PATH", "")

	agentRunBridge(t.TempDir(), nil)

	got := os.Getenv("PATH")
	if !firstEntryIsEvolveDir(t, got) {
		t.Errorf("PATH = %q, want the evolve binary directory first", got)
	}
	for _, entry := range strings.Split(got, string(os.PathListSeparator)) {
		if entry == "" {
			t.Errorf("PATH = %q holds an empty entry, which the shell reads as the working directory", got)
		}
	}
}

func TestAgentRunBridge_EvolveBinaryDirOnPathIsAddedOnce(t *testing.T) {
	t.Setenv("PATH", hostPath)
	root := t.TempDir()

	agentRunBridge(root, nil)
	agentRunBridge(root, nil)

	if got := occurrencesOfEvolveDir(t, os.Getenv("PATH")); got != 1 {
		t.Errorf("PATH = %q holds the evolve binary directory %d times, want 1", os.Getenv("PATH"), got)
	}
}

func TestHostPathValue_UnsetPathIsEmpty(t *testing.T) {
	t.Setenv("PATH", hostPath)
	if err := os.Unsetenv("PATH"); err != nil {
		t.Fatal(err)
	}

	if got := hostPathValue(); got != "" {
		t.Errorf("hostPathValue() = %q, want empty when PATH is unset", got)
	}
}

func TestHostPathValue_ReturnsThePathValue(t *testing.T) {
	t.Setenv("PATH", hostPath)

	if got := hostPathValue(); got != hostPath {
		t.Errorf("hostPathValue() = %q, want %q", got, hostPath)
	}
}

func TestPutEvolveBinaryDirOnPath_ExecutableErrorLeavesPathUnchanged(t *testing.T) {
	t.Setenv("PATH", hostPath)

	putEvolveBinaryDirOnPath(func() (string, error) { return "", os.ErrNotExist })

	if got := os.Getenv("PATH"); got != hostPath {
		t.Errorf("PATH = %q, want %q unchanged when the executable path is unknown", got, hostPath)
	}
}

type pathRecordingBridge struct {
	*composingBridge
	pathAtLaunch string
}

func (b *pathRecordingBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.pathAtLaunch = os.Getenv("PATH")
	return b.composingBridge.Launch(ctx, req)
}

func TestAgentRunBridge_EvolveBinaryDirOnPathWhenTheSeatLaunches(t *testing.T) {
	t.Setenv("PATH", hostPath)
	root := diffReviewerProject(t, "code-simplifier")
	rec := &pathRecordingBridge{composingBridge: &composingBridge{}}
	run := agentcmd.New(agentcmd.Deps{
		InstallRouter: rootRouterInstaller,
		Bridge: func(r string, s *signalcenter.Center) core.Bridge {
			rec.real = agentRunBridge(r, s)
			return rec
		},
		Signals:   quietSignals,
		TreeState: fixedTreeState("tree"),
		Getenv:    func(string) string { return "" },
	})
	var stdout, stderr bytes.Buffer

	code := run([]string{"run", "code-simplifier", "--project-root", root}, strings.NewReader("diff"), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !firstEntryIsEvolveDir(t, rec.pathAtLaunch) {
		t.Errorf("PATH at the seat launch = %q, want the evolve binary directory first", rec.pathAtLaunch)
	}
}
