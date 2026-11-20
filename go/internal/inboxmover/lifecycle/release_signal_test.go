package lifecycle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func releasedEvents(rc *recordingCenter) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range rc.events {
		if e.Kind == signalcenter.KindInboxReleased {
			out = append(out, e)
		}
	}
	return out
}

func TestReleaseOne_EmitsInboxReleased(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 5, "task-a.json"), `{"id":"task-a"}`)
	writeItem(t, procPath(inbox, 5, "task-b.json"), `{"id":"task-b"}`)
	rc := newRecordingCenter()
	m := New(inbox, nil, WithSignals(rc.accessor()))

	res, err := m.Release(5, "cycle-failure-release", nil)
	if err != nil || res.Recovered != 2 {
		t.Fatalf("Release = %+v, %v; want both items released", res, err)
	}

	got := releasedEvents(rc)
	if len(got) != 2 {
		t.Fatalf("inbox.released events = %+v, want one per released item", got)
	}
	ids := map[string]int{}
	for _, e := range got {
		ids[e.Fields["id"]]++
		if e.Cycle != 5 || e.Module != signalcenter.ModuleInbox || e.Severity != signalcenter.SeverityInfo || e.Code != "" {
			t.Errorf("inbox.released = cycle %d module %s severity %s code %q, want an INFO inbox event for cycle 5 with no code", e.Cycle, e.Module, e.Severity, e.Code)
		}
		if !strings.Contains(e.Reason, "cycle-failure-release") || !signalcenter.ValidOrigin(e.Origin) {
			t.Errorf("inbox.released reason %q origin %q, want the release reason and a valid origin", e.Reason, e.Origin)
		}
	}
	if ids["task-a"] != 1 || ids["task-b"] != 1 {
		t.Errorf("inbox.released ids = %v, want exactly one each for task-a and task-b", ids)
	}
}

func TestReleaseOne_DoubleMoveEmitsNoInboxReleased(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "dup.json"), `{"id":"dup-original"}`)
	writeItem(t, procPath(inbox, 5, "dup.json"), `{"id":"dup-claimed"}`)
	rc := newRecordingCenter()
	m := New(inbox, nil, WithSignals(rc.accessor()))

	res, err := m.Release(5, "", nil)
	if err != nil || res.Recovered != 0 {
		t.Fatalf("Release = %+v, %v; want nothing released", res, err)
	}
	if got := releasedEvents(rc); len(got) != 0 {
		t.Errorf("inbox.released events = %+v, want none for an item that did not move", got)
	}
}
