//go:build integration

package main

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
)

func TestFleetLaunch_EachOfFourLaneProcessesGetsAThreeCPUBudgetOnFourteenCPUs(t *testing.T) {
	t.Parallel()
	bin := fakeCycleBinary(t, "echo \"width=$EVOLVE_FLEET_WIDTH\"\n")
	var stdout bytes.Buffer
	s := &fleet.Supervisor{Concurrency: 4, Launch: execCycleLaunch(bin, true, "/proj", "hash", "goal", &stdout, &bytes.Buffer{})}
	specs := []fleet.CycleSpec{{Scope: []string{"a"}}, {Scope: []string{"b"}}, {Scope: []string{"c"}}, {Scope: []string{"d"}}}

	for _, r := range s.Run(context.Background(), specs) {
		if r.Status() != fleet.LaneOK {
			t.Fatalf("lane %d = %+v, want ok", r.Index, r)
		}
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != len(specs) {
		t.Fatalf("lane output %q, want one line for each of %d lanes", stdout.String(), len(specs))
	}
	for _, line := range lines {
		_, value, _ := strings.Cut(line, "width=")
		width, err := strconv.Atoi(value)
		if err != nil || lanebudget.Procs(14, width) != 3 {
			t.Errorf("lane line %q gives width %d (%v), want the width 4 that gives a budget of 3", line, width, err)
		}
	}
}
