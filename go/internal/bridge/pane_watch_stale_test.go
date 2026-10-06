package bridge

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
)

func TestEngineLaunch_ClearsALeftoverPaneWatchSnapshotBeforeEachAttempt(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-panewatch", "")
	artifact := filepath.Join(ws, "artifact.md")
	if err := panewatch.Write(ws, panewatch.Snapshot{Agent: "build", Session: "dead-pane", WriterPID: os.Getpid()}); err != nil {
		t.Fatal(err)
	}
	sawLeftover := true
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}
	inner := fr.runner()
	runner := func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		_, sawLeftover, _ = panewatch.Read(ws, "build")
		return inner(ctx, name, dir, args, env, in, out, errw)
	}
	eng := NewEngine(Deps{Runner: runner, LookupEnv: mapLookup(nil)})
	if _, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "p", Workspace: ws, ArtifactPath: artifact, Agent: "build",
	}); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if sawLeftover {
		t.Error("the driver ran while a previous attempt's pane-watch snapshot was still on disk; an observer would read a dead pane as live")
	}
}

func TestPaneWatcher_StampsTheWriterPID(t *testing.T) {
	ws := t.TempDir()
	lp := agyLaunchForTest()
	w := newPaneWatcher(&Config{Workspace: ws, Agent: "build"}, lp, paneProfileFor(lp), &bytes.Buffer{}, "[agy-tmux]")
	w.observe(agy1217Frame(t, "idle.txt"), time.Now())
	snap, ok, err := panewatch.Read(ws, "build")
	if err != nil || !ok || snap.WriterPID != os.Getpid() {
		t.Errorf("snapshot writer pid = %d (ok=%v err=%v), want this process %d", snap.WriterPID, ok, err, os.Getpid())
	}
}
