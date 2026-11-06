package main

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func zeroShipHalt(walls haltWalls) haltRecord {
	return haltRecord{
		cycle:     2174,
		workspace: "/ws/cycle-2174",
		sf:        &cyclestate.SystemFailureSignal{Category: "zero-ship", Level: "system", Evidence: "3 waves shipped nothing over cycles 2170,2171,2172,2173,2174; next: evolve dossier show 2174", Halt: true},
		now:       time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC),
		walls:     walls,
	}
}

func TestNarrative_AZeroShipHaltOfOnlyLoginWallsIsALoginNotAPipelineDefect(t *testing.T) {
	t.Parallel()
	h := zeroShipHalt(haltWalls{login: []int{2170, 2171, 2172, 2173, 2174}, counted: 5})

	story := h.narrative()
	_, item := h.inboxItem(story)

	want := haltNarrative{
		nextAction: "Log in to the CLI family that each LOGIN-REQUIRED line names (cycles 2170,2171,2172,2173,2174; evolve dossier show 2174), then resume: evolve loop --resume.",
		reproHint:  "Each counted cycle met a login wall: read the LOGIN-REQUIRED line of cycles 2170,2171,2172,2173,2174. Evidence: 3 waves shipped nothing over cycles 2170,2171,2172,2173,2174; next: evolve dossier show 2174.",
		summary:    "Zero-ship halt at cycle 2174. Each counted cycle (2170,2171,2172,2173,2174) ended at a login wall. This is not a pipeline defect: time alone does not clear a login.",
		rootCause:  "A CLI family needs a login. Evidence: 3 waves shipped nothing over cycles 2170,2171,2172,2173,2174; next: evolve dossier show 2174.",
		fix:        "Log in to each CLI family that the LOGIN-REQUIRED lines name, then resume the loop. No code change is needed.",
	}
	if story != want {
		t.Errorf("narrative = %+v\nwant %+v", story, want)
	}
	if strings.Contains(item["title"].(string), "PIPELINE DEFECT") || item["kind"] == inboxbatch.KindPipelineRepair || item["priority"] == "P0" {
		t.Errorf("a logout must not file as a P0 pipeline repair: %+v", item)
	}
	if item["id"] != "login-required-zero-ship-cycle2174" || item["priority"] != "P1" || item["kind"] != "operator-login" || item["route"] != "console" || item["weight"] != 0.5 {
		t.Errorf("item = %+v", item)
	}
}

func TestNarrative_AZeroShipHaltWithAnotherFailureKeepsTheDefectTextAndNamesTheLoginCycles(t *testing.T) {
	t.Parallel()
	h := zeroShipHalt(haltWalls{login: []int{2171, 2174}, counted: 5})
	plain := zeroShipHalt(haltWalls{})

	story, base := h.narrative(), plain.narrative()

	line := " Cycles 2171,2174 ended at a login wall: read the LOGIN-REQUIRED line of each."
	if story.nextAction != base.nextAction+line || story.summary != base.summary+line {
		t.Errorf("story = %+v\nbase = %+v", story, base)
	}
	if story.reproHint != base.reproHint || story.rootCause != base.rootCause || story.fix != base.fix {
		t.Errorf("only the next action and the summary may change: %+v", story)
	}
	if id, item := h.inboxItem(story); id != "pipeline-defect-zero-ship-cycle2174" || item["kind"] != inboxbatch.KindPipelineRepair || item["priority"] != "P0" || item["weight"] != 0.99 {
		t.Errorf("the defect item stays: %s %+v", id, item)
	}
}

func TestNarrative_ANonZeroShipHaltIgnoresLoginWalls(t *testing.T) {
	t.Parallel()
	h := zeroShipHalt(haltWalls{login: []int{1}, counted: 1})
	h.sf = &cyclestate.SystemFailureSignal{Category: "pipeline-blocker", Evidence: "x", Halt: true}
	plain := h
	plain.walls = haltWalls{}
	if got, want := h.narrative(), plain.narrative(); !reflect.DeepEqual(got, want) {
		t.Errorf("narrative = %+v, want %+v", got, want)
	}
}

func TestHaltWallsOf_EveryCountedCycleMustHoldALogin(t *testing.T) {
	t.Parallel()
	waves := []zeroShipWave{
		{cycles: []int{1, 2}, logins: []int{1, 2}},
		{cycles: []int{3, 4}, logins: []int{4}},
	}
	if got, want := haltWallsOf(waves), (haltWalls{login: []int{1, 2, 4}, counted: 4}); !reflect.DeepEqual(got, want) {
		t.Errorf("haltWallsOf = %+v, want %+v", got, want)
	}
	if !(haltWalls{login: []int{1, 2}, counted: 2}).every() || (haltWalls{login: []int{1}, counted: 2}).every() || (haltWalls{}).every() {
		t.Error("every() must hold only when each counted cycle holds a login")
	}
}
