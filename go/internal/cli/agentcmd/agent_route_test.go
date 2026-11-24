package agentcmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func routedDeps(fb *fakeBridge) Deps {
	return Deps{
		Getenv: noEnv, Now: fixedNow, TreeState: (&treeStates{values: []string{"t", "t"}}).state,
		Bridge: func(string, *signalcenter.Center) core.Bridge { return fb },
	}
}

func TestNewRouted_ARouteAdmissionThatRefusesEveryCandidateLaunchesNothing(t *testing.T) {
	t.Parallel()
	root := project(t)
	fb := &fakeBridge{artifact: passReport}
	deny := Route{Admit: func(string, string) bool { return false }}
	var stdout, stderr bytes.Buffer

	code := NewRouted(routedDeps(fb), deny)(runArgs(root), strings.NewReader("diff"), &stdout, &stderr)

	if code == 0 || len(fb.requests) != 0 {
		t.Errorf("exit %d with %d launches, want a refusal and no launch when the admission refuses every route", code, len(fb.requests))
	}
}

func TestNewRouted_AnAdmissionThatAllowsTheRouteLetsTheSeatRun(t *testing.T) {
	t.Parallel()
	root := project(t)
	fb := &fakeBridge{artifact: passReport}
	var stdout, stderr bytes.Buffer

	code := NewRouted(routedDeps(fb), Route{Admit: func(string, string) bool { return true }})(runArgs(root), strings.NewReader("diff"), &stdout, &stderr)

	if code != 0 || len(fb.requests) != 1 {
		t.Errorf("exit %d launches %d stderr %s, want one launch", code, len(fb.requests), stderr.String())
	}
}

func TestAgentRun_TheVerbTakesNoRouteFlagSoNoCellCanChooseARoute(t *testing.T) {
	t.Parallel()
	root := project(t)
	fb := &fakeBridge{artifact: passReport}
	var stdout, stderr bytes.Buffer

	code := New(routedDeps(fb))(append(runArgs(root), "--route", "held-proposal-audit"), strings.NewReader("diff"), &stdout, &stderr)

	if code == 0 || len(fb.requests) != 0 {
		t.Errorf("exit %d launches %d, want the unknown flag refused", code, len(fb.requests))
	}
}
