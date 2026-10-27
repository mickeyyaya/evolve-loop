package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const liveClass = "build|gate-block|class-7098bbb545f3"

type liveSeal struct {
	cycle      int
	layer      string
	recurrence int
}

var floorShortCircuitSeals = []liveSeal{{2043, "pipeline-code", 1}, {2046, "pipeline-code", 2}, {2048, "", 0}, {2055, "pipeline-code", 3}}

func writeLiveRecord(t *testing.T, evolveDir string, s liveSeal) {
	t.Helper()
	body := `{"cycle":` + strconv.Itoa(s.cycle) + `,"fingerprint":"build|gate-block|x` + strconv.Itoa(s.cycle) + `","pre_class":"gate-block"`
	if s.layer != "" {
		body += `,"class_fingerprint":"` + liveClass + `","class_layer":"` + s.layer + `","class_recurrence":` + strconv.Itoa(s.recurrence)
	}
	selfhealFile(t, paths.FailureDigestRecordPath(evolveDir, s.cycle), body+"}")
}

func sealedEvent(cycle int, verdict string) signalcenter.Event {
	return signalcenter.Event{Cycle: cycle, Module: signalcenter.ModuleOrchestrator, Origin: "test", Kind: signalcenter.KindCycleSealed,
		Severity: signalcenter.SeverityWarn, Reason: "final verdict " + verdict, Fields: map[string]string{cyclestate.SealedVerdictField: verdict}}
}

type liveReplay struct {
	evolveDir string
	incidents []signalcenter.Event
	at        []int
	warn      bytes.Buffer
}

func replaySeals(t *testing.T, stage string, seals []liveSeal) *liveReplay {
	t.Helper()
	rp := &liveReplay{evolveDir: paths.EvolveDirOf(t.TempDir())}
	signals := signalcenter.New()
	sealing := 0
	signals.Subscribe(func(e signalcenter.Event) {
		if e.Kind == signalcenter.KindSelfhealIncident {
			rp.incidents, rp.at = append(rp.incidents, e), append(rp.at, sealing)
		}
	})
	newLiveReaction(rp.evolveDir, policy.HealingLiveReactionPolicy{Stage: stage}, &rp.warn).arm(signals)
	for _, s := range seals {
		writeLiveRecord(t, rp.evolveDir, s)
		sealing = s.cycle
		signals.Emit(sealedEvent(s.cycle, cyclestate.VerdictFAIL))
	}
	return rp
}

func TestLiveIncident_TheThirdSealOfTheFloorClassEmitsOneIncidentAtShadow(t *testing.T) {
	t.Parallel()
	rp := replaySeals(t, policy.HealingStageShadow, floorShortCircuitSeals)

	if len(rp.incidents) != 1 || rp.at[0] != 2055 {
		t.Fatalf("incidents = %d at %v; want one, at the seal of cycle 2055", len(rp.incidents), rp.at)
	}
	e := rp.incidents[0]
	if e.Code != selfheal.CodeIncidentOpened || e.Fields["class"] != string(selfheal.ClassPipelineDefect) || e.Fields["reaction"] != liveReactionWouldBrake || e.Fields["stage"] != policy.HealingStageShadow {
		t.Errorf("incident event = %+v; want an opened pipeline-defect incident with reaction would_brake", e)
	}
	if _, err := os.Stat(paths.LoopStopPath(rp.evolveDir)); err == nil {
		t.Error("shadow wrote the loop brake")
	}
	records, err := selfheal.LoadIncidents(selfheal.Dir(rp.evolveDir))
	if err != nil || len(records) != 1 || records[0].Subject != liveClass {
		t.Errorf("incident records = %+v, %v; want one record of the class", records, err)
	}
}

func TestLiveIncident_EnforceWritesTheBrakeOnce(t *testing.T) {
	t.Parallel()
	rp := replaySeals(t, policy.HealingStageEnforce, append(floorShortCircuitSeals[:4:4], liveSeal{2056, "pipeline-code", 4}))
	brake := paths.LoopStopPath(rp.evolveDir)
	first, err := os.ReadFile(brake)
	if err != nil {
		t.Fatalf("enforce did not write the loop brake: %v", err)
	}
	if len(rp.incidents) != 1 || rp.incidents[0].Fields["reaction"] != liveReactionBraked {
		t.Fatalf("incidents = %+v; want one with reaction braked", rp.incidents)
	}

	r := liveReaction{evolveDir: rp.evolveDir, stage: policy.HealingStageEnforce, now: time.Now, warn: io.Discard}
	if got := r.brake(selfheal.Incident{ID: "inc-again"}); got != liveReactionBrakeHeld {
		t.Errorf("a second brake = %q; want %q", got, liveReactionBrakeHeld)
	}
	if again, _ := os.ReadFile(brake); !bytes.Equal(again, first) {
		t.Errorf("the brake changed from %q to %q; want it written once", first, again)
	}
}

func TestLiveIncident_ATaskCodeOrPassingSealEmitsNothing(t *testing.T) {
	t.Parallel()
	rp := replaySeals(t, policy.HealingStageEnforce, []liveSeal{{2048, "", 0}})
	signals := signalcenter.New()
	signals.Subscribe(func(e signalcenter.Event) {
		if e.Kind == signalcenter.KindSelfhealIncident {
			rp.incidents = append(rp.incidents, e)
		}
	})
	newLiveReaction(rp.evolveDir, policy.HealingLiveReactionPolicy{Stage: policy.HealingStageEnforce}, io.Discard).arm(signals)
	writeLiveRecord(t, rp.evolveDir, liveSeal{2060, "pipeline-code", 3})
	signals.Emit(sealedEvent(2060, cyclestate.VerdictPASS))
	signals.Emit(signalcenter.Event{Cycle: 2060, Module: signalcenter.ModuleOrchestrator, Origin: "test", Kind: signalcenter.KindShipLanded, Severity: signalcenter.SeverityInfo, Reason: "landed"})

	if len(rp.incidents) != 0 {
		t.Fatalf("incidents = %+v; want none for a task-code FAIL", rp.incidents)
	}
	if records, _ := selfheal.LoadIncidents(selfheal.Dir(rp.evolveDir)); len(records) != 0 {
		t.Errorf("records = %+v; want none for a PASS seal or a landing", records)
	}
}

func TestLiveIncident_OffArmsNothing(t *testing.T) {
	t.Parallel()
	rp := replaySeals(t, policy.HealingStageOff, floorShortCircuitSeals)

	if len(rp.incidents) != 0 {
		t.Fatalf("incidents = %+v; want none at off", rp.incidents)
	}
}

func TestLiveIncident_FaultsAreWarnings(t *testing.T) {
	t.Parallel()
	t.Run("a corrupt record", func(t *testing.T) {
		rp := replaySeals(t, policy.HealingStageShadow, nil)
		r := liveReaction{evolveDir: rp.evolveDir, stage: policy.HealingStageShadow, now: time.Now, emit: func(signalcenter.Event) {}, warn: &rp.warn}
		selfhealFile(t, paths.FailureDigestRecordPath(rp.evolveDir, 5), "{")
		r.onEvent(sealedEvent(5, cyclestate.VerdictFAIL))
		if !strings.Contains(rp.warn.String(), "live incident of cycle 5") {
			t.Errorf("stderr = %q; want a warning", rp.warn.String())
		}
	})
	t.Run("a corrupt incident store", func(t *testing.T) {
		var warn bytes.Buffer
		evolveDir := paths.EvolveDirOf(t.TempDir())
		selfhealFile(t, filepath.Join(selfheal.Dir(evolveDir), "inc-bad.json"), "{")
		writeLiveRecord(t, evolveDir, liveSeal{2055, "pipeline-code", 3})
		emitted := 0
		r := liveReaction{evolveDir: evolveDir, stage: policy.HealingStageShadow, now: time.Now, emit: func(signalcenter.Event) { emitted++ }, warn: &warn}
		r.onEvent(sealedEvent(2055, cyclestate.VerdictFAIL))
		if emitted != 0 || !strings.Contains(warn.String(), "inc-bad.json") {
			t.Errorf("emitted %d, stderr %q; want no event and a warning that names the record", emitted, warn.String())
		}
	})
	t.Run("an unwritable brake", func(t *testing.T) {
		var warn bytes.Buffer
		r := liveReaction{evolveDir: filepath.Join(t.TempDir(), "absent", ".evolve"), stage: policy.HealingStageEnforce, now: time.Now, warn: &warn}
		if got := r.brake(selfheal.Incident{ID: "inc-x"}); got != liveReactionBrakeFail || !strings.Contains(warn.String(), "inc-x") {
			t.Errorf("brake = %q, stderr %q; want %q and a warning", got, warn.String(), liveReactionBrakeFail)
		}
	})
}

func TestWireOrchestratorDeps_ArmsTheLiveIncidentReaction(t *testing.T) {
	root := t.TempDir()
	evolveDir := paths.EvolveDirOf(root)
	writeLiveRecord(t, evolveDir, liveSeal{2055, "pipeline-code", 3})

	d := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{})
	d.Signals.Emit(sealedEvent(2055, cyclestate.VerdictFAIL))
	d.closeSignals()

	records, err := selfheal.LoadIncidents(selfheal.Dir(evolveDir))
	if err != nil || len(records) != 1 || records[0].Class != selfheal.ClassPipelineDefect {
		t.Fatalf("records = %+v, %v; want the cycle composition root to arm the live reaction at the default stage", records, err)
	}
}
