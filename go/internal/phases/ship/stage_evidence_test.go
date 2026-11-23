package ship

import (
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

type stageRecord struct {
	key    string
	fields map[string]string
}

type stageSession struct {
	mu      sync.Mutex
	cycle   int
	records []stageRecord
	closed  int
}

func (s *stageSession) Record(key string, fields map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, stageRecord{key: key, fields: fields})
}

func (s *stageSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed++
}

type stageOpenings struct {
	mu       sync.Mutex
	sessions []*stageSession
}

func (o *stageOpenings) open(cycle int) core.StageRecorder {
	o.mu.Lock()
	defer o.mu.Unlock()
	s := &stageSession{cycle: cycle}
	o.sessions = append(o.sessions, s)
	return s
}

func TestHeldProposalShadowWritesEvidenceRecord(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "shadow", "go/internal/guards/role.go")
	var opened stageOpenings
	var opener StageOpener = opened.open
	f.opts.StageEvidence = opener

	if err := f.check(); err != nil {
		t.Fatalf("shadow must land: %v", err)
	}
	if len(opened.sessions) != 1 || opened.sessions[0].cycle != heldCycle || opened.sessions[0].closed != 1 {
		t.Fatalf("sessions = %+v, want one session of cycle %d, closed once", opened.sessions, heldCycle)
	}
	records := opened.sessions[0].records
	if len(records) != 1 || records[0].key != StageKeyHeldProposals {
		t.Fatalf("records = %+v, want one held-proposals decision", records)
	}
	f0 := records[0].fields
	if f0["would_hold"] != "true" || f0["tree"] != heldTree || f0["cycle"] != "41" || !strings.Contains(f0["reasons"], "protected") || f0["record_error"] != "" {
		t.Errorf("fields = %v, want would_hold, the tree, the cycle and the protected reason", f0)
	}
}

func TestHeldProposalShadowEvidenceCarriesAFailedRecordWrite(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "shadow", "go/internal/guards/role.go")
	writeDoc(t, f.opts.ProjectRoot, ".evolve/landing/held", "a file where the directory belongs")
	var opened stageOpenings
	f.opts.StageEvidence = opened.open

	if err := f.check(); err != nil {
		t.Fatal(err)
	}
	if len(opened.sessions) != 1 || len(opened.sessions[0].records) != 1 || !strings.Contains(opened.sessions[0].records[0].fields["record_error"], "heldproposal") {
		t.Errorf("sessions = %+v, want the decision to carry the record error", opened.sessions)
	}
}

func TestHeldProposalWritesNoEvidenceWhereItWouldNotHold(t *testing.T) {
	t.Parallel()
	for name, f := range map[string]*heldFixture{
		"ordinary change":  newHeldFixture(t, "shadow", "go/internal/other/x.go"),
		"enforce a hold":   newHeldFixture(t, "enforce", "go/internal/guards/role.go"),
		"stage off":        newHeldFixture(t, "off", "go/internal/guards/role.go"),
		"no evidence port": newHeldFixture(t, "shadow", "go/internal/guards/role.go"),
	} {
		var opened stageOpenings
		if name != "no evidence port" {
			f.opts.StageEvidence = opened.open
		}
		_ = f.check()
		if len(opened.sessions) != 0 {
			t.Errorf("%s: %d evidence sessions opened, want none", name, len(opened.sessions))
		}
	}
}

func TestPhasePassesItsStageEvidencePortToEveryShip(t *testing.T) {
	t.Parallel()
	var opened stageOpenings
	p := New(Config{StageEvidence: opened.open})
	if !p.StageEvidenceWired() || New(Config{}).StageEvidenceWired() {
		t.Fatal("StageEvidenceWired must be true with a port and false without")
	}
	opts := p.shipOptions(core.PhaseRequest{Cycle: 12, ProjectRoot: t.TempDir()}, "msg")
	if opts.StageEvidence == nil {
		t.Fatal("the ship options lost the stage evidence port")
	}
	opts.recordStage(12, StageKeyHeldProposals, map[string]string{"k": "v"})
	if len(opened.sessions) != 1 || opened.sessions[0].cycle != 12 || opened.sessions[0].closed != 1 || opened.sessions[0].records[0].fields["k"] != "v" {
		t.Errorf("sessions = %+v, want one session of cycle 12 with the record, closed", opened.sessions)
	}
}

func TestHeldProposalShadowEvidenceScoresAsAConflictWhenTheClaimedItemIsRouted(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "shadow", "go/internal/other/x.go")
	f.claim(t, "pipeline-repair")
	var opened stageOpenings
	f.opts.StageEvidence = opened.open
	if err := f.check(); err != nil {
		t.Fatal(err)
	}
	if len(opened.sessions) != 1 || len(opened.sessions[0].records) != 1 {
		t.Fatalf("sessions = %+v, want one held-proposals decision", opened.sessions)
	}
	emitted := stageevidence.Record{Fields: opened.sessions[0].records[0].fields}

	routed := stageevidence.DefaultScorer().Classify(emitted, stageevidence.Outcome{Routed: []string{"item"}})
	unrouted := stageevidence.DefaultScorer().Classify(emitted, stageevidence.Outcome{Routed: []string{"another"}})

	if routed != stageevidence.ClassConflict || unrouted != stageevidence.ClassNone {
		t.Errorf("classes = %q routed, %q unrouted; want %q and %q", routed, unrouted, stageevidence.ClassConflict, stageevidence.ClassNone)
	}
}
