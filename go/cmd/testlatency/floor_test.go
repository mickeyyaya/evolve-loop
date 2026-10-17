package main

import (
	"fmt"
	"strings"
	"testing"
)

func floorStream(pkg string, serial, parallel map[string]float64, order []string) string {
	var lines []string
	for _, name := range order {
		if secs, ok := parallel[name]; ok {
			lines = append(lines,
				fmt.Sprintf(`{"Action":"run","Package":%q,"Test":%q}`, pkg, name),
				fmt.Sprintf(`{"Action":"pause","Package":%q,"Test":%q}`, pkg, name),
				fmt.Sprintf(`{"Action":"cont","Package":%q,"Test":%q}`, pkg, name),
				fmt.Sprintf(`{"Action":"pass","Package":%q,"Test":%q,"Elapsed":%v}`, pkg, name, secs))
			continue
		}
		lines = append(lines,
			fmt.Sprintf(`{"Action":"run","Package":%q,"Test":%q}`, pkg, name),
			fmt.Sprintf(`{"Action":"pass","Package":%q,"Test":%q,"Elapsed":%v}`, pkg, name, serial[name]))
	}
	lines = append(lines, fmt.Sprintf(`{"Action":"pass","Package":%q,"Elapsed":99}`, pkg))
	return strings.Join(lines, "\n")
}

func TestParse_SerialParallelFloor(t *testing.T) {
	tests := []struct {
		name         string
		serial       map[string]float64
		parallel     map[string]float64
		order        []string
		wantSerial   float64
		wantParallel float64
		wantFloor    float64
		wantDominant string
	}{
		{
			name:         "parallel dominates",
			serial:       map[string]float64{"TestS1": 3, "TestS2": 4},
			parallel:     map[string]float64{"TestP1": 5, "TestP2": 10},
			order:        []string{"TestS1", "TestP1", "TestS2", "TestP2"},
			wantSerial:   7,
			wantParallel: 10,
			wantFloor:    17,
			wantDominant: "parallel",
		},
		{
			name:         "serial dominates",
			serial:       map[string]float64{"TestS1": 30, "TestS2": 4},
			parallel:     map[string]float64{"TestP1": 5, "TestP2": 10},
			order:        []string{"TestS1", "TestP1", "TestS2", "TestP2"},
			wantSerial:   34,
			wantParallel: 10,
			wantFloor:    44,
			wantDominant: "serial",
		},
		{
			name:         "tie reports serial",
			serial:       map[string]float64{"TestS1": 4, "TestS2": 6},
			parallel:     map[string]float64{"TestP1": 10},
			order:        []string{"TestS1", "TestS2", "TestP1"},
			wantSerial:   10,
			wantParallel: 10,
			wantFloor:    20,
			wantDominant: "serial",
		},
		{
			name:         "all serial has no parallel part",
			serial:       map[string]float64{"TestS1": 2},
			order:        []string{"TestS1"},
			wantSerial:   2,
			wantFloor:    2,
			wantDominant: "serial",
		},
		{
			name:         "all parallel has no serial part",
			parallel:     map[string]float64{"TestP1": 6},
			order:        []string{"TestP1"},
			wantParallel: 6,
			wantFloor:    6,
			wantDominant: "parallel",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rep, err := Parse(strings.NewReader(floorStream("x/p", tc.serial, tc.parallel, tc.order)))
			if err != nil {
				t.Fatal(err)
			}
			p := rep.Packages[0]
			if p.SerialOnlySum != tc.wantSerial || p.ParallelLongest != tc.wantParallel {
				t.Errorf("serial/parallel = %v/%v, want %v/%v", p.SerialOnlySum, p.ParallelLongest, tc.wantSerial, tc.wantParallel)
			}
			if p.Floor() != tc.wantFloor || p.Dominant() != tc.wantDominant {
				t.Errorf("floor/dominant = %v/%s, want %v/%s", p.Floor(), p.Dominant(), tc.wantFloor, tc.wantDominant)
			}
		})
	}
}

func TestParse_SubtestPauseDoesNotMakeParentParallel(t *testing.T) {
	stream := strings.Join([]string{
		`{"Action":"run","Package":"x/p","Test":"TestA"}`,
		`{"Action":"pause","Package":"x/p","Test":"TestA/sub"}`,
		`{"Action":"pass","Package":"x/p","Test":"TestA/sub","Elapsed":4}`,
		`{"Action":"pass","Package":"x/p","Test":"TestA","Elapsed":4}`,
		`{"Action":"pass","Package":"x/p","Elapsed":4}`,
	}, "\n")
	rep, err := Parse(strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	if p := rep.Packages[0]; p.SerialOnlySum != 4 || p.ParallelLongest != 0 {
		t.Errorf("serial/parallel = %v/%v, want 4/0", p.SerialOnlySum, p.ParallelLongest)
	}
}

func TestMarkdown_SlowPackagesShowsFloorColumns(t *testing.T) {
	stream := floorStream("x/p", map[string]float64{"TestS1": 3, "TestS2": 4}, map[string]float64{"TestP1": 5, "TestP2": 10}, []string{"TestS1", "TestP1", "TestS2", "TestP2"})
	rep, err := Parse(strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	md := rep.Markdown(MarkdownOptions{Title: "T", Top: 1, ThresholdPkg: 5, ThresholdTst: 1})
	for _, want := range []string{
		"| Package | Wall (s) | Tests | Σall tests (s) | Serial Σ (s) | Parallel max (s) | Floor (s) | Dominant | Slowest test | Slowest (s) |",
		"| x/p | 99.00 | 4 | 22.00 | 7.00 | 10.00 | 17.00 | parallel | TestP2 | 10.00 |",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}

func TestParse_PausedTestWithoutTerminalEventStaysIncomplete(t *testing.T) {
	stream := strings.Join([]string{
		`{"Action":"run","Package":"x/p","Test":"TestHang"}`,
		`{"Action":"pause","Package":"x/p","Test":"TestHang"}`,
		`{"Action":"pass","Package":"x/p","Test":"TestDone","Elapsed":2}`,
	}, "\n")
	rep, err := Parse(strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	p := rep.Packages[0]
	if p.ParallelLongest != 0 || p.SerialOnlySum != 2 {
		t.Errorf("serial/parallel = %v/%v, want 2/0", p.SerialOnlySum, p.ParallelLongest)
	}
	if len(rep.Incomplete) != 1 || rep.Incomplete[0] != "x/p" {
		t.Errorf("Incomplete = %v, want [x/p]", rep.Incomplete)
	}
}

func TestParse_PauseIsScopedToItsPackage(t *testing.T) {
	stream := strings.Join([]string{
		`{"Action":"pause","Package":"x/a","Test":"TestSame"}`,
		`{"Action":"pass","Package":"x/a","Test":"TestSame","Elapsed":5}`,
		`{"Action":"pass","Package":"x/b","Test":"TestSame","Elapsed":3}`,
		`{"Action":"pass","Package":"x/a","Elapsed":5}`,
		`{"Action":"pass","Package":"x/b","Elapsed":3}`,
	}, "\n")
	rep, err := Parse(strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][2]float64{}
	for _, p := range rep.Packages {
		got[p.Pkg] = [2]float64{p.SerialOnlySum, p.ParallelLongest}
	}
	if got["x/a"] != [2]float64{0, 5} || got["x/b"] != [2]float64{3, 0} {
		t.Errorf("per-package serial/parallel = %v, want a=[0 5] b=[3 0]", got)
	}
}

func TestParse_PauseDoesNotLeakIntoRepeatedRun(t *testing.T) {
	stream := strings.Join([]string{
		`{"Action":"pause","Package":"x/p","Test":"TestRep"}`,
		`{"Action":"pass","Package":"x/p","Test":"TestRep","Elapsed":5}`,
		`{"Action":"pass","Package":"x/p","Test":"TestRep","Elapsed":3}`,
		`{"Action":"pass","Package":"x/p","Elapsed":8}`,
	}, "\n")
	rep, err := Parse(strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	if p := rep.Packages[0]; p.SerialOnlySum != 3 || p.ParallelLongest != 5 {
		t.Errorf("serial/parallel = %v/%v, want 3/5", p.SerialOnlySum, p.ParallelLongest)
	}
}
