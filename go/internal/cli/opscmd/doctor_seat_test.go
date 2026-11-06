package opscmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/seatlogin"
)

func TestRunDoctor_SeatReportsEachStateAndExitsNonZeroOnlyForABadHolder(t *testing.T) {
	cases := []struct {
		state    seatlogin.State
		code     int
		inStdout string
		inStderr string
	}{
		{seatlogin.StateNone, 0, "OK", ""},
		{seatlogin.StateIsolated, 0, "OK", ""},
		{seatlogin.StateOrphan, 0, "", "WARN"},
		{seatlogin.StateFault, 0, "", "WARN"},
		{seatlogin.StateBad, 1, "", "WARN"},
	}
	for _, c := range cases {
		orig := doctorSeatInspect
		doctorSeatInspect = func() seatlogin.Health { return seatlogin.Health{State: c.state, Message: "the message"} }
		var out, errb bytes.Buffer
		code := RunDoctor([]string{"seat"}, nil, &out, &errb)
		doctorSeatInspect = orig
		if code != c.code || !strings.Contains(out.String(), c.inStdout) || !strings.Contains(errb.String(), c.inStderr) {
			t.Errorf("state %v: code=%d out=%q err=%q", c.state, code, out.String(), errb.String())
		}
		if !strings.Contains(out.String()+errb.String(), "the message") {
			t.Errorf("state %v: the message is lost", c.state)
		}
	}
}
