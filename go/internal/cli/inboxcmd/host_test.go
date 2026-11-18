package inboxcmd

import (
	"encoding/json"
	"errors"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxtest"
	"io"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/lanerouting"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/build"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func testDeps() Deps {
	return Deps{
		QueueGate: func(string, string, []string, bool, io.Writer, io.Writer) (int, bool) { return exitOK, false },
		LaneForbidden: func(projectRoot string, warn io.Writer) func(string) bool {
			return lanerouting.LazyForbidden(projectRoot, build.ProfileName, signalcenter.ConsoleSink(warn))
		},
		PrintVerbJSON: func(verb string, v any, stdout, stderr io.Writer) int {
			enc := json.NewEncoder(stdout)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(v); err != nil {
				return exitIO
			}
			return exitOK
		},
		HumanWriter: func(stdout io.Writer, asJSON bool) io.Writer {
			if asJSON {
				return io.Discard
			}
			return stdout
		},
		CheckRootHasEvolve: func(string, string, io.Writer) int { return exitOK },
		NewSignalCenter: func(string, string, string, io.Writer) (*signalcenter.Center, func()) {
			return signalcenter.New(), func() {}
		},
	}
}

func runInboxCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return Run(args, stdin, stdout, stderr, testDeps())
}

const realConsumedByNarrative = "console-2026-08-05: fingerprint ship|unknown|76d0f4fca190 = root cause fixed"

const incidentFingerprint = "ship|unknown|76d0f4fca190"

func contractUnitWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	inboxtest.WriteFile(t, filepath.Join(root, filepath.FromSlash(rel)), body)
}

func e2Ptr(s string) *string { return &s }

type failingWriter struct {
	okWrites int
	writes   int
}

func (w *failingWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > w.okWrites {
		return 0, errors.New("broken pipe")
	}
	return len(p), nil
}

func runMoverCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return RunMover(args, stdin, stdout, stderr, testDeps())
}
