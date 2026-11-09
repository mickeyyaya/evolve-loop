package observer

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestNew_DefaultsTheShellLimitLongerThanThePlainLimit(t *testing.T) {
	o := New(Config{}, io.Discard)
	if o.cfg.ShellStallS != DefaultShellStallS || DefaultShellStallS <= DefaultStallS {
		t.Errorf("ShellStallS=%v default=%v plain=%v", o.cfg.ShellStallS, DefaultShellStallS, DefaultStallS)
	}
}

func TestWatch_ShellPaneStallsAtTheShellLimitAndNamesTheShells(t *testing.T) {
	t.Parallel()
	frozen := time.Now()
	snap := busySnapshot(frozen)
	snap.Busy, snap.Shells = false, 2
	cfg := Config{
		StallS: 10 * time.Millisecond, ShellStallS: 80 * time.Millisecond, PollS: 5 * time.Millisecond, Cycle: 9, Phase: "router", Agent: "router",
		PaneWatch: func() (panewatch.Snapshot, bool, error) { return snap, true, nil },
	}
	var stall *Event
	for _, e := range watchFor(t, cfg, 500*time.Millisecond) {
		if e.Type == eventStallNoProgress {
			e := e
			stall = &e
			break
		}
	}
	if stall == nil || stall.Shells != 2 || !strings.Contains(stall.Reason, "2 running shell") || !strings.Contains(stall.Reason, "idle at its prompt") {
		t.Fatalf("stall = %+v, want a shell-naming stall with shells=2", stall)
	}
}

func TestPhaseStallSignal_AShellStallCarriesTheShellLimitAndCount(t *testing.T) {
	center := signalcenter.New()
	a := &CoreAdapter{Signals: func() *signalcenter.Center { return center }}
	hook := a.phaseStallSignal(context.Background(), phaseWatch{phase: "build", stall: time.Minute, shellStall: time.Hour})
	hook(Event{Type: eventStallNoProgress, Source: sourcePane, Session: "s", Shells: 3, Reason: "r"})
	evs := center.Recent()
	if len(evs) != 1 || evs[0].Fields["shells"] != "3" || evs[0].Fields["stall_s"] != "3600" {
		t.Fatalf("events = %+v, want shells=3 stall_s=3600", evs)
	}
}
