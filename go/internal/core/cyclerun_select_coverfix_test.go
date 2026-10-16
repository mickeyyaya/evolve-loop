package core

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type workspaceRemovingStrategy struct {
	router.StaticPreset
	workspace string
}

func (s workspaceRemovingStrategy) Decide(in router.RouteInput) router.RouterDecision {
	_ = os.RemoveAll(s.workspace)
	return s.StaticPreset.Decide(in)
}

func spineWarnCycleRun(t *testing.T, strategy router.RoutingStrategy, sm *StateMachine, workspace string) *cycleRun {
	t.Helper()
	cfg := config.RoutingConfig{
		Stage:     config.StageAdvisory,
		Mandatory: []string{"scout", "build", "audit", "ship"},
		Order:     []string{"scout", "tdd", "build", "audit", "ship"},
	}
	cfg.SpineFloor = config.StageShadow
	return &cycleRun{
		ctx: context.Background(),
		o: &Orchestrator{
			sm:       sm,
			strategy: strategy,
			now:      time.Now,
			ledger:   &fakeLedger{},
			cfg:      cfg,
		},
		cs:          CycleState{WorkspacePath: workspace},
		current:     PhaseBuildPlanner,
		lastVerdict: VerdictPASS,
	}
}

func TestSelectNext_InvalidTransitionAborts(t *testing.T) {
	t.Parallel()
	cr := spineWarnCycleRun(t, router.StaticPreset{}, NewStateMachine(), t.TempDir())
	cr.current = PhaseEnd

	next, act, err := cr.selectNext()
	if err == nil || act != loopAbort || next != "" {
		t.Fatalf("selectNext() = (%q, %v, %v), want abort with a transition error", next, act, err)
	}
}

func TestSelectNext_RedigestFailureFailsOpenWithReason(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cr := spineWarnCycleRun(t, workspaceRemovingStrategy{workspace: ws}, NewStateMachine(), ws)

	next, act, err := cr.selectNext()
	if err != nil || act != loopNext || next != PhaseBuild {
		t.Fatalf("selectNext() = (%q, %v, %v), want fail-open advance to build", next, act, err)
	}
	if len(cr.result.SpineFailOpens) != 1 {
		t.Fatalf("SpineFailOpens = %d, want 1", len(cr.result.SpineFailOpens))
	}
	if got := cr.result.SpineFailOpens[0].Reason; !strings.HasPrefix(got, "re-digest error: ") {
		t.Errorf("reason = %q, want a re-digest error", got)
	}
}

func TestSelectNext_VanishedAnchorIsRecordedAsUnknown(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	sm := NewStateMachine().WithCatalog(func(p Phase) (phasespec.PhaseSpec, bool) {
		if p != PhaseScout {
			return phasespec.PhaseSpec{}, false
		}
		if calls.Add(1) <= 2 {
			return phasespec.PhaseSpec{Gate: &phasespec.ArtifactGate{RequiresPresent: true}}, true
		}
		return phasespec.PhaseSpec{Gate: &phasespec.ArtifactGate{}}, true
	})
	cr := spineWarnCycleRun(t, router.StaticPreset{}, sm, t.TempDir())

	next, act, err := cr.selectNext()
	if err != nil || act != loopNext || next != PhaseBuild {
		t.Fatalf("selectNext() = (%q, %v, %v), want fail-open advance to build", next, act, err)
	}
	if len(cr.result.SpineFailOpens) != 1 || cr.result.SpineFailOpens[0].MissingArtifact != "unknown" {
		t.Fatalf("SpineFailOpens = %+v, want one record anchored \"unknown\"", cr.result.SpineFailOpens)
	}
}
