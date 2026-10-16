package cycleoutcome

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestApplyFailure_ThreadsSignals(t *testing.T) {
	root, inbox, ws := seedProject(t, "poison", []string{"poison"})
	if err := os.WriteFile(filepath.Join(inbox, "quarantine"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	center := signalcenter.New()
	var codes []signalcenter.Code
	center.Subscribe(func(e signalcenter.Event) { codes = append(codes, e.Code) })
	in := FailureInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Ceiling: 1, Stderr: io.Discard}.WithLedger(&recordingLifecycleLedger{}).WithSignals(center)
	if _, err := ApplyFailure(in); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}
	var seen bool
	for _, c := range codes {
		if c == "INBOX_QUARANTINE_FAILED" {
			seen = true
		}
	}
	if !seen {
		t.Errorf("the mover's event must reach the threaded Center; got %v", codes)
	}
	base := FailureInputs{ProjectRoot: "/r", Cycle: 3}
	out := base.WithSignals(center)
	if base.Signals != nil || out.Signals != center || out.ProjectRoot != "/r" || out.Cycle != 3 {
		t.Errorf("WithSignals returns a copy with the Center set: %+v / %+v", base, out)
	}
}
