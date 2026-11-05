package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestDispatch_AForgedPinFileKeepsTheRecordAndEmitsOneSignalPerCycle(t *testing.T) {
	t.Parallel()
	center, events := capturedSignals(t)
	forge := func() (PhaseResponse, error) { return PhaseResponse{Phase: "scout", Verdict: VerdictPASS}, nil }
	runner := &dispatchScriptRunnerE4{name: "scout", steps: []func() (PhaseResponse, error){func() (PhaseResponse, error) { return forge() }}}
	cr := dispatchRunE4(t, &fakeLedger{}, runner, PhaseScout, 1, WithSignalCenter(center))
	root := t.TempDir()
	cr.req.ProjectRoot = root
	cr.cs.WorkspacePath = filepath.Join(root, ".evolve", "runs", "cycle-5")
	cr.ctxSnap["fleet_scope"] = "a"
	writeLaneScopeFixture(t, cr.cs.WorkspacePath, []string{"a"}, "g")
	forge = func() (PhaseResponse, error) {
		writeLaneScopeFixture(t, cr.cs.WorkspacePath, []string{"b", "c"}, "forged")
		return PhaseResponse{Phase: "scout", Verdict: VerdictPASS}, nil
	}
	for i := 0; i < 2; i++ {
		if _, _, err := cr.dispatch(PhaseScout); err != nil {
			t.Fatal(err)
		}
	}
	if got := committedset.LanePin(cr.cs.WorkspacePath); len(got) != 1 || got[0] != "a" {
		t.Fatalf("LanePin = %v, want [a]", got)
	}
	var drift []signalcenter.Event
	for _, e := range *events {
		if e.Code == CodeLanePinForged {
			drift = append(drift, e)
		}
	}
	if len(drift) != 1 {
		t.Fatalf("got %d forged-pin signals, want one per cycle: %+v", len(drift), drift)
	}
	e := drift[0]
	file := filepath.Join(cr.cs.WorkspacePath, "lane-scope.json")
	if e.Severity != signalcenter.SeverityWarn || e.Kind != signalcenter.KindIsolationViolation || e.Cycle != 5 {
		t.Fatalf("event = %+v", e)
	}
	for _, want := range []string{"a", "b, c", file, "operator", "host record"} {
		if !strings.Contains(e.Reason, want) {
			t.Errorf("reason %q lacks %q", e.Reason, want)
		}
	}
	if e.Fields["record"] != "a" || e.Fields["file"] != "b,c" || e.Fields["path"] != file {
		t.Fatalf("fields = %v", e.Fields)
	}
}

func TestDispatch_ACleanPinFileEmitsNoSignal(t *testing.T) {
	t.Parallel()
	center, events := capturedSignals(t)
	runner := &dispatchScriptRunnerE4{name: "scout", steps: []func() (PhaseResponse, error){
		func() (PhaseResponse, error) { return PhaseResponse{Phase: "scout", Verdict: VerdictPASS}, nil },
	}}
	cr := dispatchRunE4(t, &fakeLedger{}, runner, PhaseScout, 1, WithSignalCenter(center))
	root := t.TempDir()
	cr.req.ProjectRoot = root
	cr.cs.WorkspacePath = filepath.Join(root, ".evolve", "runs", "cycle-5")
	cr.ctxSnap["fleet_scope"] = "a"
	writeLaneScopeFixture(t, cr.cs.WorkspacePath, []string{"a"}, "g")
	if _, _, err := cr.dispatch(PhaseScout); err != nil {
		t.Fatal(err)
	}
	for _, e := range *events {
		if e.Code == CodeLanePinForged {
			t.Fatalf("a pin file equal to the record must not signal: %+v", e)
		}
	}
	if _, err := os.Stat(filepath.Join(cr.cs.WorkspacePath, "lane-scope.json")); err != nil {
		t.Fatal(err)
	}
}

func TestDispatch_AForgedPinFileSignalsOnEachFailingReturn(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		attempts int
		resp     PhaseResponse
		err      error
	}{
		{"quota wall on every family", 2, PhaseResponse{Phase: "scout"}, wrapTransient(85)},
		{"a phase error", 1, PhaseResponse{Phase: "scout"}, errors.New("boom")},
		{"a non-canonical verdict after the last attempt", 1, PhaseResponse{Phase: "scout", Verdict: "MAYBE"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			center, events := capturedSignals(t)
			var cr *cycleRun
			step := func() (PhaseResponse, error) {
				writeLaneScopeFixture(t, cr.cs.WorkspacePath, []string{"b", "c"}, "forged")
				return tc.resp, tc.err
			}
			runner := &dispatchScriptRunnerE4{name: "scout", steps: []func() (PhaseResponse, error){step}}
			cr = dispatchRunE4(t, &fakeLedger{}, runner, PhaseScout, tc.attempts, WithSignalCenter(center))
			root := t.TempDir()
			cr.req.ProjectRoot = root
			cr.cs.WorkspacePath = filepath.Join(root, ".evolve", "runs", "cycle-5")
			cr.ctxSnap["fleet_scope"] = "a"
			writeLaneScopeFixture(t, cr.cs.WorkspacePath, []string{"a"}, "g")
			if _, act, err := cr.dispatch(PhaseScout); err == nil || act != loopAbort {
				t.Fatalf("dispatch = %v, %v, want an aborting failure", act, err)
			}
			if got := committedset.LanePin(cr.cs.WorkspacePath); len(got) != 1 || got[0] != "a" {
				t.Fatalf("LanePin = %v, want [a]", got)
			}
			n := 0
			for _, e := range *events {
				if e.Code == CodeLanePinForged {
					n++
				}
			}
			if n != 1 {
				t.Fatalf("got %d forged-pin signals, want one", n)
			}
		})
	}
}
