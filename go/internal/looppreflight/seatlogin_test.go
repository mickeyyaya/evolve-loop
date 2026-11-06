package looppreflight

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/seatlogin"
)

func inspectorOf(state seatlogin.State, msg string) func() seatlogin.Health {
	return func() seatlogin.Health { return seatlogin.Health{State: state, Message: msg} }
}

func TestCheckSeatLogin_MapsEachStateToALevel(t *testing.T) {
	cases := map[seatlogin.State]CheckLevel{
		seatlogin.StateNone:     LevelPass,
		seatlogin.StateIsolated: LevelPass,
		seatlogin.StateBad:      LevelHalt,
		seatlogin.StateOrphan:   LevelWarn,
		seatlogin.StateFault:    LevelWarn,
	}
	for state, want := range cases {
		got := CheckSeatLogin(inspectorOf(state, "the message"))
		if got.Name != "seat-login" || got.Level != want {
			t.Errorf("state %v: %s/%s, want seat-login/%s", state, got.Name, got.Level, want)
		}
		if !strings.Contains(got.Message+got.Detail, "the message") {
			t.Errorf("state %v: the inspector message is lost: %+v", state, got)
		}
	}
}

func TestCheckSeatLogin_NoInspectorPassesAndSaysItDidNotMeasure(t *testing.T) {
	got := CheckSeatLogin(nil)
	if got.Level != LevelPass || !strings.Contains(got.Message, "not measured") {
		t.Fatalf("got %+v", got)
	}
}

func TestRun_SeatLoginCheckRunsAndAHaltingHolderHaltsTheLoop(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.SeatInspect = inspectorOf(seatlogin.StateBad, "bridge: the seat token at /h is empty, so no claude seat can start.")
	r, err := Run(opts)
	if err != nil {
		t.Fatal(err)
	}
	c := findCheck(t, r, "seat-login")
	if c.Level != LevelHalt || !r.Halted() {
		t.Fatalf("check %+v halted=%v", c, r.Halted())
	}
}
