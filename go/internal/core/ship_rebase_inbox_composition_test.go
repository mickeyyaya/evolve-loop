package core_test

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	laneItemID   = "wave-orchestrator-o6-wire-oversight-into-the-loop"
	laneItemPath = ".evolve/inbox/2026-10-26T19-22-34Z-" + laneItemID + ".json"
)

func initInboxStampRepoT(t *testing.T, mainLandsLaneFile bool) string {
	t.Helper()
	dir := gittest.Fixture(t).Dir
	write := func(rel, content string) {
		t.Helper()
		if err := writeFileT(filepath.Join(dir, rel), content); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(msg string) {
		runGitT(t, dir, "add", "-A")
		runGitT(t, dir, "commit", "-m", msg)
	}
	write(laneItemPath, `{"id":"`+laneItemID+`","notes":"original"}`)
	commit("base")
	runGitT(t, dir, "checkout", "-b", "cycle")
	write(laneItemPath, `{"id":"`+laneItemID+`","notes":"lane"}`)
	write("lane.txt", "lane change\n")
	commit("lane change")
	runGitT(t, dir, "checkout", "main")
	write(laneItemPath, `{"failure_count":1,"id":"`+laneItemID+`","notes":"original"}`)
	if mainLandsLaneFile {
		write("lane.txt", "lane change\n")
	}
	commit("chore(inbox): the loop's inbox stamps from the plane")
	runGitT(t, dir, "checkout", "cycle")
	return dir
}

func TestRecoverFromShipError_AnInboxStampOnMainIsResolvedAndPublishesItsPaths(t *testing.T) {
	t.Parallel()

	dir := initInboxStampRepoT(t, false)
	var mu sync.Mutex
	var got []signalcenter.Event
	c := signalcenter.New()
	c.Subscribe(func(e signalcenter.Event) {
		mu.Lock()
		defer mu.Unlock()
		if e.Code == core.CodeFleetRebaseInboxResolved {
			got = append(got, e)
		}
	})
	ship := &committingShip{}
	dbg := &resolvingDebugger{writes: "unused\n"}
	o := core.NewOrchestrator(&recStorage{}, &fakeLedger{}, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseShip: ship, core.PhaseAudit: &countingRunner{name: "audit"}, core.PhaseBuild: &explanationWritingRunner{},
		core.PhaseTDD: &countingRunner{name: "tdd"}, core.PhaseDebugger: dbg,
	}), core.WithWorktreeProvisioner(fixedWorktree{dir: dir}), core.WithSignalCenter(c))

	_, err := o.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "merge-rung0-goal",
		Context: map[string]string{"commit_message": "test commit"},
		Env:     map[string]string{ipcenv.FleetScopeKey: laneItemID},
	})
	c.Flush()

	if err != nil || dbg.calls != 0 {
		t.Fatalf("cycle = %v, debugger calls %d: the inbox stamp must resolve without the debugger", err, dbg.calls)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || !strings.Contains(got[0].Fields["paths"], laneItemPath) || got[0].Kind != signalcenter.KindShipWarning {
		t.Fatalf("resolved events = %+v, want one ship warning naming %s", got, laneItemPath)
	}
	if m, ok := signalcenter.IsRegistered(core.CodeFleetRebaseInboxResolved); !ok || m != signalcenter.ModuleOrchestrator {
		t.Errorf("IsRegistered = %q, %v", m, ok)
	}
	if out := runGitT(t, dir, "show", "HEAD:"+laneItemPath); !strings.Contains(out, `"lane"`) || !strings.Contains(out, "failure_count") {
		t.Errorf("landed item = %s, want the lane's notes and main's stamp", out)
	}
}

func TestRecoverFromShipError_APeerThatLandedTheSameFileDoesNotEndACorrectLaneAsLostCode(t *testing.T) {
	t.Parallel()

	dir := initInboxStampRepoT(t, true)
	dbg := &resolvingDebugger{writes: "unused\n"}
	o := core.NewOrchestrator(&recStorage{}, &fakeLedger{}, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseShip: &committingShip{}, core.PhaseAudit: &countingRunner{name: "audit"}, core.PhaseBuild: &explanationWritingRunner{},
		core.PhaseTDD: &countingRunner{name: "tdd"}, core.PhaseDebugger: dbg,
	}), core.WithWorktreeProvisioner(fixedWorktree{dir: dir}))

	_, err := o.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "merge-rung0-goal",
		Context: map[string]string{"commit_message": "test commit"},
		Env:     map[string]string{ipcenv.FleetScopeKey: laneItemID},
	})

	if err != nil || dbg.calls != 0 {
		t.Fatalf("cycle = %v, debugger calls %d: a peer that landed the same file leaves the lane's code intact", err, dbg.calls)
	}
	if out := runGitT(t, dir, "show", "HEAD:"+laneItemPath); !strings.Contains(out, `"lane"`) || !strings.Contains(out, "failure_count") {
		t.Errorf("landed item = %s, want the lane's notes and main's stamp", out)
	}
}

func initInboxAndCodeConflictRepoT(t *testing.T) string {
	t.Helper()
	dir := gittest.Fixture(t).Dir
	write := func(rel, content string) {
		t.Helper()
		if err := writeFileT(filepath.Join(dir, rel), content); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(msg string) {
		runGitT(t, dir, "add", "-A")
		runGitT(t, dir, "commit", "-m", msg)
	}
	write(laneItemPath, `{"id":"`+laneItemID+`","notes":"original"}`)
	write("shared.md", "base line\n")
	commit("base")
	runGitT(t, dir, "checkout", "-b", "cycle")
	write(laneItemPath, `{"id":"`+laneItemID+`","notes":"lane"}`)
	write("shared.md", "lane line\n")
	commit("lane change")
	runGitT(t, dir, "checkout", "main")
	write(laneItemPath, `{"failure_count":1,"id":"`+laneItemID+`","notes":"original"}`)
	write("shared.md", "peer line\n")
	commit("chore(inbox): the loop's inbox stamps from the plane")
	runGitT(t, dir, "checkout", "cycle")
	return dir
}

func TestResumeFleetRebaseAfterDebugger_AnInboxStampOnTheReentryRebasePublishesItsPaths(t *testing.T) {
	t.Parallel()

	dir := initInboxAndCodeConflictRepoT(t)
	var mu sync.Mutex
	var got []signalcenter.Event
	c := signalcenter.New()
	c.Subscribe(func(e signalcenter.Event) {
		mu.Lock()
		defer mu.Unlock()
		if e.Code == core.CodeFleetRebaseInboxResolved {
			got = append(got, e)
		}
	})
	dbg := &resolvingDebugger{writes: "peer line\n"}
	o := core.NewOrchestrator(&recStorage{}, &fakeLedger{}, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseShip: &committingShip{}, core.PhaseAudit: &countingRunner{name: "audit"}, core.PhaseBuild: &explanationWritingRunner{},
		core.PhaseTDD: &countingRunner{name: "tdd"}, core.PhaseDebugger: dbg,
	}), core.WithWorktreeProvisioner(fixedWorktree{dir: dir}), core.WithSignalCenter(c))

	_, err := o.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "merge-rung0-goal",
		Context: map[string]string{"commit_message": "test commit"},
		Env:     map[string]string{ipcenv.FleetScopeKey: laneItemID},
	})
	c.Flush()

	mu.Lock()
	defer mu.Unlock()
	if err != nil || dbg.calls != 1 || len(got) != 1 || !strings.Contains(got[0].Fields["paths"], laneItemPath) {
		t.Fatalf("cycle = %v, debugger calls %d, events = %+v, want one resolved event from the re-entry rebase", err, dbg.calls, got)
	}
}
