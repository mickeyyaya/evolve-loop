package phasecmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestObserverSignalCenter_PublishesAsThePhaseObserverAndTheCloserDrainsIt(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	signals, closeSignals := observerSignalCenter(io.Discard)

	signals.Emit(signalcenter.Event{
		Cycle: 7, Phase: "build", Module: signalcenter.ModuleObserver, Origin: "Test.emit", Kind: signalcenter.KindObserverWarning,
		Severity: signalcenter.SeverityWarn, Code: CodePolicyLoadFailed, Reason: "a stall",
	})
	closeSignals()

	for _, name := range []string{"errors", "signals"} {
		log, err := channel.New(paths.EventChannelsRoot(filepath.Join(root, ".evolve")), name, channel.Config{SegmentBytes: 1 << 20, LockDeadline: time.Minute})
		if err != nil {
			t.Fatal(err)
		}
		batch, err := log.Read(0)
		if err != nil || len(batch.Records) != 1 || batch.Records[0].Source != "phase-observer" || batch.Records[0].Signal.Cycle != 7 {
			t.Errorf("channel %s = %+v, %v, want the WARN of cycle 7 from the role phase-observer", name, batch.Records, err)
		}
	}
}

func TestObserverSignalCenter_AnUnsetProjectRootPublishesNothingAndLeavesTheWorkingDirectoryAlone(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	cwd := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	var stderr bytes.Buffer
	signals, closeSignals := observerSignalCenter(&stderr)

	signals.Emit(signalcenter.Event{
		Cycle: 7, Phase: "build", Module: signalcenter.ModuleObserver, Origin: "Test.emit", Kind: signalcenter.KindObserverWarning,
		Severity: signalcenter.SeverityWarn, Code: CodePolicyLoadFailed, Reason: "a stall",
	})
	closeSignals()

	if entries, rerr := os.ReadDir(cwd); rerr != nil || len(entries) != 0 {
		t.Errorf("the working directory holds %v (%v), want it empty: an unset EVOLVE_PROJECT_ROOT makes the evolve directory relative, and a relative path must not put channel logs under the cwd", entries, rerr)
	}
	if got := strings.Count(stderr.String(), "[events] WARN: role phase-observer publishes no events"); got != 1 {
		t.Errorf("stderr = %q, want one WARN that the role publishes no events", stderr.String())
	}
	if !strings.Contains(stderr.String(), "[observer] observer.warning WARN") {
		t.Errorf("stderr = %q, want the Center to keep rendering the observer WARN on the console", stderr.String())
	}
}
