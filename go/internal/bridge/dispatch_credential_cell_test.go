package bridge

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func coversCellEvents(events []signalcenter.Event) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range events {
		if e.Fields["rule"] == "launch.credential.covers-cell" {
			out = append(out, e)
		}
	}
	return out
}

func TestSandboxWrap_ACredentialDenyOverTheCellKeepsTheDenyAndIsLoud(t *testing.T) {
	t.Parallel()
	for _, goos := range []string{"darwin", "linux"} {
		l := incidentCellLayout(t)
		c, events := recordingSignals()
		var stderr bytes.Buffer
		profile := wrapCellProfile(t, Deps{Signals: c, Stderr: &stderr}, goos, l)
		parent := filepath.Dir(l.root)
		deny := `(deny file-write* (subpath "` + parent + `"))`
		if goos == "linux" {
			deny = strings.Join([]string{"--ro-bind", parent, parent}, "\x00")
		}
		if !strings.Contains(profile, deny) {
			t.Errorf("%s: the credential deny of %s must stay in the profile:\n%s", goos, parent, profile)
		}
		for _, cell := range []string{l.worktree, l.workspace} {
			if !strings.Contains(stderr.String(), "WARN dispatch credential: the write deny "+parent+" covers the cell path "+cell) {
				t.Errorf("%s: the wrap must warn that the deny covers %s, stderr: %q", goos, cell, stderr.String())
			}
		}
		got := coversCellEvents(*events)
		if len(got) != 2 {
			t.Fatalf("%s: want one isolation.violation per covered cell path (2), got %d: %+v", goos, len(got), *events)
		}
		for _, e := range got {
			if e.Kind != signalcenter.KindIsolationViolation || e.Module != signalcenter.ModuleIsolation || e.Fields["deny"] != parent || e.Fields["invariant"] != "I3" || e.Fields["action"] != "logged" {
				t.Errorf("%s: the signal must be an I3 isolation.violation that names the deny %s: %+v", goos, parent, e)
			}
		}
		if got[0].Fields["cell"] != l.worktree || got[1].Fields["cell"] != l.workspace {
			t.Errorf("%s: the signals must name the worktree, then the workspace: %+v", goos, got)
		}
	}
}

func TestSandboxWrap_ANormalLayoutGetsNoCellWarning(t *testing.T) {
	t.Parallel()
	for _, goos := range []string{"darwin", "linux"} {
		for _, l := range []cellLayout{integrationCellLayout(t), contractCellLayout(t)} {
			c, events := recordingSignals()
			var stderr bytes.Buffer
			wrapCellProfile(t, Deps{Signals: c, Stderr: &stderr}, goos, l)
			if strings.Contains(stderr.String(), "covers the cell path") || len(coversCellEvents(*events)) != 0 {
				t.Errorf("%s on %s: a store away from the cell must give no warning and no signal: %q %+v", l.name, goos, stderr.String(), *events)
			}
		}
	}
}

func TestWarnCredentialCoversCell_SkipsAnEmptyOrUnresolvableCellPath(t *testing.T) {
	t.Parallel()
	dir := contractTempDir(t, "", "cellpath")
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	c, events := recordingSignals()
	var stderr bytes.Buffer
	Deps{Signals: c, Stderr: &stderr}.warnCredentialCoversCell([]string{dir}, SandboxWrapRequest{Workspace: filepath.Join(file, "ws")})
	if stderr.Len() != 0 || len(*events) != 0 {
		t.Fatalf("an empty worktree and a workspace under a file must give no report: %q %+v", stderr.String(), *events)
	}
}

func TestWarnCredentialCoversCell_ADenyEqualToTheCellIsReported(t *testing.T) {
	t.Parallel()
	dir := contractTempDir(t, "", "cellpath")
	c, events := recordingSignals()
	Deps{Signals: c}.warnCredentialCoversCell([]string{dir}, SandboxWrapRequest{Worktree: dir})
	if got := coversCellEvents(*events); len(got) != 1 || got[0].Fields["cell"] != dir {
		t.Fatalf("a deny equal to the worktree covers it: %+v", *events)
	}
}
