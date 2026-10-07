package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestApplyEscalationBoundary_NamesEachRefusedIntentInTheLoopLogAndItsSignal(t *testing.T) {
	evolveDir := t.TempDir()
	fixtures.MustWrite(t, filepath.Join(evolveDir, "policy.json"), `{"failure_disposition":{"stage":"enforce"}}`)
	fixtures.MustWrite(t, filepath.Join(evolveDir, "inbox", ".keep"), "")
	fixtures.MustWrite(t, filepath.Join(evolveDir, "escalations", "pending-actions.jsonl"),
		`{"cycle":1,"pattern":"p","item_id":"legacy-absent","action":"autofile","route":"console","recurrence":3,"weight":0.75}`+"\n")
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
	var stderr bytes.Buffer
	applyEscalationBoundary(evolveDir, 7, &stderr, center)
	if want := "[loop] WARN escalation boundary: refused legacy-absent: no priority_class\n"; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr = %q, want the refused intent named: %q", stderr.String(), want)
	}
	if len(events) != 1 || events[0].Code != CodeLoopEscalationBoundary || events[0].Fields["refused"] != "1" {
		t.Errorf("events = %+v, want one %s signal whose refused field is 1", events, CodeLoopEscalationBoundary)
	}
}
