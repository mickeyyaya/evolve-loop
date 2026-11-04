package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type walledCycle struct {
	cycle int
	phase string
	done  []string
}

var walledWave = []walledCycle{
	{cycle: 2137, phase: "audit", done: []string{"scout", "triage", "tdd", "build", "audit", "audit", "audit"}},
	{cycle: 2138, phase: "audit", done: []string{"scout", "triage", "tdd", "build", "audit", "build", "audit"}},
	{cycle: 2139, phase: "build", done: []string{"scout", "triage", "tdd"}},
	{cycle: 2141, phase: "tdd", done: []string{"scout", "triage", "fault-localization", "bug-reproduction"}},
}

func walledTodo(cycle int) string { return "item-" + strconv.Itoa(cycle) }

func seedWalledCycle(t *testing.T, root string, w walledCycle) string {
	t.Helper()
	evolveDir := filepath.Join(root, ".evolve")
	dir := filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(w.cycle))
	inbox := filepath.Join(evolveDir, "inbox")
	for _, d := range []string{dir, inbox} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	scope, _ := json.Marshal(core.LaneScope{TodoIDs: []string{walledTodo(w.cycle)}, GoalHash: "wave-151"})
	state, _ := json.Marshal(map[string]any{"cycle_id": w.cycle, "phase": w.phase, "completed_phases": w.done, "checkpoint": map[string]any{
		"enabled": true, "reason": string(checkpoint.ReasonQuotaLikely), "resumeFromPhase": w.phase, "completedPhases": w.done,
		"worktreePath": t.TempDir(), "savedAt": time.Now().UTC().Format(time.RFC3339),
		"autoResumeAttempts": 0, "autoResumeMaxAttempts": 0, "quotaResetSource": "unknown",
		"operatorAction": "no quota reset time is known: check the usage of the walled CLIs, then run `evolve loop --resume` by hand",
	}})
	item, _ := json.Marshal(map[string]any{"id": walledTodo(w.cycle), "title": "fixture " + walledTodo(w.cycle), "kind": "bug"})
	statePath := filepath.Join(dir, core.CycleStateFile)
	for path, body := range map[string][]byte{
		filepath.Join(dir, core.LaneScopeFile): scope,
		statePath:                              state,
		filepath.Join(inbox, "2026-11-03T10-00-00Z-"+walledTodo(w.cycle)+".json"): item,
	} {
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return statePath
}

type adoption struct {
	lane   int
	resume *core.ResumePoint
	stderr string
}

func adoptConcurrently(root string, signals *signalcenter.Center, scopes [][]string) []adoption {
	evolveDir := filepath.Join(root, ".evolve")
	results := make([]adoption, len(scopes))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, scope := range scopes {
		wg.Add(1)
		go func(i int, scope []string) {
			defer wg.Done()
			var stderr bytes.Buffer
			lane := laneResume{projectRoot: root, evolveDir: evolveDir, scope: scope, record: checkpoint.RecordResume,
				claim: hostInboxClaimer(nil, signals, nil), signals: signals, stderr: &stderr,
				release: laneClaimReleaser(root, &stderr, nil, signals)}
			<-start
			results[i] = adoption{lane: i, resume: lane.adopt(), stderr: stderr.String()}
		}(i, scope)
	}
	close(start)
	wg.Wait()
	return results
}

func walledScopes() [][]string {
	scopes := make([][]string, len(walledWave))
	for i, w := range walledWave {
		scopes[i] = []string{walledTodo(w.cycle)}
	}
	return scopes
}

func TestResumeAll_WithoutTheGrantEachLaneOfAnUnknownResetWallRefusesItsPausedCycle(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	for _, w := range walledWave {
		seedWalledCycle(t, root, w)
	}

	got := adoptConcurrently(root, signalcenter.New(), walledScopes())

	for i, a := range got {
		if a.resume != nil {
			t.Errorf("lane %s adopted cycle %d with no operator grant; an unknown reset leaves no auto-resume budget", walledScopes()[i][0], a.resume.CycleID)
		}
	}
}

func TestResumeAll_EveryPausedCycleResumesConcurrentlyAtItsOwnPhaseAfterOneGrant(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	evolveDir := filepath.Join(root, ".evolve")
	statePaths := map[int]string{}
	for _, w := range walledWave {
		statePaths[w.cycle] = seedWalledCycle(t, root, w)
	}
	var grantOut, grantErr bytes.Buffer
	if rc := runCycle([]string{"resume", "--all", "--project-root", root}, nil, &grantOut, &grantErr); rc != cliExitOK {
		t.Fatalf("resume --all rc = %d stderr = %s", rc, grantErr.String())
	}

	paused, err := core.PausedLaneScopes(root, evolveDir, core.ResumeOptions{Log: &bytes.Buffer{}})
	if err != nil {
		t.Fatal(err)
	}
	backlog := pausedPlan{paused: paused, now: time.Now(), adoptable: inboxAdoptable(evolveDir, nil), itemFiles: inboxItemFiles(evolveDir)}.order([]fleet.Todo{{ID: walledTodo(2139), Files: []string{"go/x.go"}}, {ID: "fresh-item"}})
	var order []string
	for _, td := range backlog {
		order = append(order, td.ID)
	}
	if want := []string{walledTodo(2137), walledTodo(2138), walledTodo(2139), walledTodo(2141), "fresh-item"}; !slices.Equal(order, want) {
		t.Fatalf("backlog = %v, want every paused scope first, once each, then the fresh todo", order)
	}

	signals := signalcenter.New()
	var mu sync.Mutex
	resumed := 0
	signals.Subscribe(func(e signalcenter.Event) {
		if e.Code == core.CodeLaneResumed {
			mu.Lock()
			resumed++
			mu.Unlock()
		}
	})
	got := adoptConcurrently(root, signals, walledScopes())

	for i, w := range walledWave {
		a := got[i]
		if a.resume == nil {
			t.Errorf("lane %s did not adopt cycle %d; it would start a fresh cycle and lose the work up to %s: %s", walledTodo(w.cycle), w.cycle, w.phase, a.stderr)
			continue
		}
		if a.resume.CycleID != w.cycle || a.resume.Phase != w.phase {
			t.Errorf("lane %s adopted cycle %d at %s, want its own cycle %d at its paused phase %s", walledTodo(w.cycle), a.resume.CycleID, a.resume.Phase, w.cycle, w.phase)
		}
		if cp := readCheckpoint(t, statePaths[w.cycle]); cp.AutoResumeAttempts != 1 || cp.AutoResumeMaxAttempts < checkpoint.DefaultAutoResumeAttempts {
			t.Errorf("cycle %d checkpoint = attempts %d of %d, want exactly one resume spent from the granted budget", w.cycle, cp.AutoResumeAttempts, cp.AutoResumeMaxAttempts)
		}
		loc, err := inboxmover.Locate(filepath.Join(evolveDir, "inbox"), walledTodo(w.cycle))
		if err != nil || loc.Cycle != w.cycle {
			t.Errorf("todo %s is held by cycle %d (err %v), want its own paused cycle %d", walledTodo(w.cycle), loc.Cycle, err, w.cycle)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if resumed != len(walledWave) {
		t.Errorf("ORCHESTRATOR_LANE_RESUMED fired %d times, want once for each of the %d paused cycles", resumed, len(walledWave))
	}
}
