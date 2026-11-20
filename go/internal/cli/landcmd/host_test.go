package landcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	cliExitOK      = exitOK
	cliExitRefused = exitRefused
	cliExitIO      = exitIO
	cliExitUsage   = exitUsage
)

var (
	processGetwd      = os.Getwd
	prRefreshWorktree = core.DerivedWorktree
)

func init() {
	d := testDeps()
	current.Store(&d)
}

func testResolveRoot(projectRoot string, stderr io.Writer) (string, error) {
	if projectRoot == "" {
		projectRoot = os.Getenv("EVOLVE_PROJECT_ROOT")
	}
	if projectRoot == "" {
		cwd, err := processGetwd()
		if err != nil {
			return "", err
		}
		projectRoot = cwd
	}
	return paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve: WARN: %s\n", m)
	}), nil
}

func testDeps() Deps {
	return Deps{
		ResolveRoot: testResolveRoot,
		PrintVerbJSON: func(verb string, v any, stdout, stderr io.Writer) int {
			enc := json.NewEncoder(stdout)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(v); err != nil {
				fmt.Fprintf(stderr, "evolve %s: encode the JSON result: %v\n", verb, err)
				return exitIO
			}
			return exitOK
		},
		NewSignalCenter: func(string, string, string, io.Writer) (*signalcenter.Center, func()) {
			return signalcenter.New(), func() {}
		},
		DarwinSet: func(args []string, stdout, stderr io.Writer) int {
			return exitUsage
		},
		RefreshWorktree: func(root string) derived.Worktree {
			return prRefreshWorktree(root)
		},
	}
}

func runPRCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return RunPR(args, stdin, stdout, stderr, testDeps())
}

func runCICmd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return RunCI(args, stdin, stdout, stderr, testDeps())
}

var errGetwdBroken = errors.New("getwd: no such file or directory")

func restoreGetwd(prev func() (string, error)) func() {
	return func() { processGetwd = prev }
}

func breakWorkingDirectory(t *testing.T) {
	t.Helper()
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	t.Cleanup(restoreGetwd(processGetwd))
	processGetwd = func() (string, error) { return "", errGetwdBroken }
}

var errBrokenPipe = errors.New("broken pipe")

type failingWriter struct {
	okWrites int
	writes   int
}

func (w *failingWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes <= w.okWrites {
		return len(p), nil
	}
	return 0, errBrokenPipe
}
