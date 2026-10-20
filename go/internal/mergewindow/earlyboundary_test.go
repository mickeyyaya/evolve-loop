package mergewindow

import (
	"strings"
	"testing"
	"time"
)

func TestDecideEarlyBoundary_DueOnlyWhenTheLongestWaitPassesTheBoundAndThePoolIsBelowItsWidth(t *testing.T) {
	waiting := []WaitingPR{{Number: "1012", Wait: 40 * time.Minute}, {Number: "1007", Wait: 95 * time.Minute}, {Number: "1010", Wait: 70 * time.Minute}}
	cases := []struct {
		name    string
		waiting []WaitingPR
		pool    Pool
		due     bool
		pr      string
		line    string
	}{
		{"over the bound and below the width", waiting, Pool{Live: 2, Width: 4}, true, "1007",
			"early boundary: #1007 waited 1h35m0s (bound 1h0m0s); pool 2/4 lanes: run evolve loop-stop --wait, then evolve wave next --merge 1007"},
		{"the pool is full", waiting, Pool{Live: 4, Width: 4}, false, "1007",
			"no early boundary: #1007 waited 1h35m0s (bound 1h0m0s); pool 4/4 lanes is full"},
		{"under the bound", []WaitingPR{{Number: "1012", Wait: 40 * time.Minute}}, Pool{Live: 1, Width: 4}, false, "1012",
			"no early boundary: #1012 waited 40m0s (bound 1h0m0s); pool 1/4 lanes"},
		{"at the bound is not over it", []WaitingPR{{Number: "1012", Wait: time.Hour}}, Pool{Live: 1, Width: 4}, false, "1012",
			"no early boundary: #1012 waited 1h0m0s (bound 1h0m0s); pool 1/4 lanes"},
		{"no green boundary PR waits", nil, Pool{Live: 1, Width: 4}, false, "",
			"no early boundary: no green boundary PR waits; pool 1/4 lanes"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DecideEarlyBoundary(c.waiting, c.pool, time.Hour)
			if got.Due != c.due || got.PR != c.pr || got.Line() != c.line {
				t.Fatalf("DecideEarlyBoundary = %+v, line %q; want due=%v pr=%q line %q", got, got.Line(), c.due, c.pr, c.line)
			}
			if got.Bound != time.Hour || got.Pool != c.pool || strings.Contains(got.Line(), "\n") {
				t.Errorf("the call must carry the bound and the pool on one line: %+v", got)
			}
		})
	}
}

func TestDecideEarlyBoundary_ATieKeepsTheFirstListedPR(t *testing.T) {
	var got EarlyBoundary = DecideEarlyBoundary([]WaitingPR{{Number: "5", Wait: 2 * time.Hour}, {Number: "3", Wait: 2 * time.Hour}}, Pool{Live: 0, Width: 2}, time.Hour)
	if got.PR != "5" || got.Wait != 2*time.Hour || !got.Due {
		t.Fatalf("DecideEarlyBoundary = %+v, want #5 due", got)
	}
}
