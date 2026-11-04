package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type startedCycle struct {
	resumed   *core.ResumePoint
	req       core.CycleRequest
	stateFile string
}

type recordingStarter struct{ got *startedCycle }

func (s recordingStarter) RunCycle(_ context.Context, req core.CycleRequest) (core.CycleResult, error) {
	*s.got = startedCycle{req: req, stateFile: os.Getenv(ipcenv.CycleStateFileKey)}
	return core.CycleResult{Cycle: 99}, nil
}

func (s recordingStarter) RunCycleFromPhase(_ context.Context, req core.CycleRequest, rp *core.ResumePoint) (core.CycleResult, error) {
	*s.got = startedCycle{resumed: rp, req: req, stateFile: os.Getenv(ipcenv.CycleStateFileKey)}
	return core.CycleResult{Cycle: rp.CycleID}, nil
}

type pausedLane struct {
	root, evolveDir, statePath string
	cycle                      int
}

func seedPausedLane(t *testing.T, todo string, cycle int, attempts ...int) pausedLane {
	return seedPausedScope(t, []string{todo}, cycle, append(attempts, 0)[0])
}

func (p pausedLane) addItem(t *testing.T, id string, fields map[string]any) {
	t.Helper()
	item := map[string]any{"id": id, "title": "fixture " + id, "kind": "bug"}
	maps.Copy(item, fields)
	body, _ := json.Marshal(item)
	if err := os.WriteFile(filepath.Join(p.evolveDir, "inbox", "2026-11-02T00-00-00Z-"+id+".json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func seedPausedScope(t *testing.T, todos []string, cycle, attempts int) pausedLane {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	ws := filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(cycle))
	inbox := filepath.Join(evolveDir, "inbox")
	for _, d := range []string{ws, inbox} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	scope, _ := json.Marshal(core.LaneScope{TodoIDs: todos, GoalHash: "paused-goal"})
	state, _ := json.Marshal(map[string]any{"cycle_id": cycle, "phase": "build", "checkpoint": map[string]any{
		"enabled": true, "reason": string(checkpoint.ReasonQuotaLikely), "resumeFromPhase": "build", "worktreePath": t.TempDir(),
		"savedAt": time.Now().UTC().Format(time.RFC3339), "autoResumeAttempts": attempts, "autoResumeMaxAttempts": 3,
	}})
	statePath := filepath.Join(ws, core.CycleStateFile)
	writes := map[string][]byte{filepath.Join(ws, core.LaneScopeFile): scope, statePath: state}
	for path, body := range writes {
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := pausedLane{root: root, evolveDir: evolveDir, statePath: statePath, cycle: cycle}
	for _, id := range todos {
		p.addItem(t, id, nil)
	}
	return p
}

func (p pausedLane) lane(stderr *bytes.Buffer, signals *signalcenter.Center, scope ...string) laneResume {
	return laneResume{projectRoot: p.root, evolveDir: p.evolveDir, scope: scope, record: checkpoint.RecordResume,
		claim: hostInboxClaimer(nil, signals, nil), signals: signals, stderr: stderr,
		release: laneClaimReleaser(p.root, stderr, nil, signals)}
}

func laneRequest() core.CycleRequest {
	return core.CycleRequest{GoalHash: "next-wave-goal", Context: map[string]string{"goal": "next wave", "commit_message": "evolve-cycle: goal=next-wave-goal"}}
}

func readCheckpoint(t *testing.T, path string) checkpoint.Checkpoint {
	t.Helper()
	var state struct {
		Checkpoint checkpoint.Checkpoint `json:"checkpoint"`
	}
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, &state) != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return state.Checkpoint
}

func TestLaneResume_ALaneWhoseTodoPausedOnQuotaResumesThatCycleAtItsPhase(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2107)
	signals := signalcenter.New()
	var events []signalcenter.Event
	signals.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
	var stderr bytes.Buffer
	var got startedCycle
	req := laneRequest()

	res, err := p.lane(&stderr, signals, "item-a").run(context.Background(), recordingStarter{&got}, req)

	if err != nil || res.Cycle != 2107 || got.resumed == nil || got.resumed.CycleID != 2107 || got.resumed.Phase != "build" {
		t.Fatalf("run = %+v, %v, resumed %+v; want cycle 2107 resumed at build", res, err, got.resumed)
	}
	if got.req.GoalHash != "" || got.req.Context["goal"] != "" || got.req.Context["commit_message"] == "" {
		t.Errorf("resumed request = %+v, want the goal of the checkpoint, not of the wave", got.req)
	}
	if req.Context["goal"] != "next wave" {
		t.Errorf("the caller's request changed: %+v", req.Context)
	}
	if loc, err := inboxmover.Locate(filepath.Join(p.evolveDir, "inbox"), "item-a"); err != nil || loc.Cycle != 2107 {
		t.Errorf("Locate(item-a) = %+v, %v; want it claimed by cycle 2107", loc, err)
	}
	if cp := readCheckpoint(t, p.statePath); cp.AutoResumeAttempts != 1 {
		t.Errorf("checkpoint = %+v, want auto-resume attempt 1 recorded", cp)
	}
	if len(events) != 1 || events[0].Code != core.CodeLaneResumed || events[0].Fields["attempts"] != "1" {
		t.Errorf("events = %+v, want one %s at auto-resume 1", events, core.CodeLaneResumed)
	}
	if !strings.Contains(stderr.String(), "lane item-a resumes cycle 2107 at phase build (auto-resume 1 of 3)") {
		t.Errorf("stderr = %q, want the resume line", stderr.String())
	}
}

func TestLaneResume_TheResumeRunsOnTheStateFileOfThePausedCycle(t *testing.T) {
	p := seedPausedLane(t, "item-a", 7)
	t.Setenv(ipcenv.CycleStateFileKey, "")
	os.Unsetenv(ipcenv.CycleStateFileKey)
	var got startedCycle

	if _, err := p.lane(&bytes.Buffer{}, nil, "item-a").run(context.Background(), recordingStarter{&got}, laneRequest()); err != nil {
		t.Fatal(err)
	}

	if got.stateFile != p.statePath || os.Getenv(ipcenv.CycleStateFileKey) != "" {
		t.Errorf("state file during the run = %q, after = %q; want %q, then unset", got.stateFile, os.Getenv(ipcenv.CycleStateFileKey), p.statePath)
	}
}

func TestLaneResume_StartsAFreshCycleWhenThereIsNothingToResume(t *testing.T) {
	t.Parallel()
	for name, scope := range map[string][]string{"not a fleet lane": nil, "a lane of another todo": {"item-b"}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := seedPausedLane(t, "item-a", 7)
			var got startedCycle

			res, err := p.lane(&bytes.Buffer{}, nil, scope...).run(context.Background(), recordingStarter{&got}, laneRequest())

			if err != nil || res.Cycle != 99 || got.resumed != nil || got.req.GoalHash != "next-wave-goal" {
				t.Fatalf("run = %+v, %v, started %+v; want a fresh cycle with the wave goal", res, err, got)
			}
		})
	}
}

func TestLaneResume_ARefusedResumeStartsAFreshCycle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		attempts int
		record   func(string, bool) (checkpoint.Checkpoint, bool, error)
	}{
		{"the auto-resume budget is spent", 3, checkpoint.RecordResume},
		{"the checkpoint cannot be written", 0, func(string, bool) (checkpoint.Checkpoint, bool, error) {
			return checkpoint.Checkpoint{}, true, errors.New("disk full")
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := seedPausedLane(t, "item-a", 7, tc.attempts)
			var stderr bytes.Buffer
			var got startedCycle
			lane := p.lane(&stderr, nil, "item-a")
			lane.record = tc.record

			if _, err := lane.run(context.Background(), recordingStarter{&got}, laneRequest()); err != nil || got.resumed != nil {
				t.Fatalf("run err %v, resumed %+v; want a fresh cycle", err, got.resumed)
			}
			if !strings.Contains(stderr.String(), "the resume of cycle 7 was refused") {
				t.Errorf("stderr = %q, want the refusal", stderr.String())
			}
		})
	}
}

func TestLaneResume_ATodoTheCycleCannotTakeBackStartsAFreshCycle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		setup func(t *testing.T, p pausedLane)
		want  string
	}{
		{"the todo left the inbox", func(t *testing.T, p pausedLane) {
			if err := os.Remove(filepath.Join(p.evolveDir, "inbox", "2026-11-02T00-00-00Z-item-a.json")); err != nil {
				t.Fatal(err)
			}
		}, "locate item-a"},
		{"another cycle holds the todo", func(t *testing.T, p pausedLane) {
			if err := hostInboxClaimer(nil, nil, nil)(filepath.Join(p.evolveDir, "inbox"), 8, []string{"item-a"}); err != nil {
				t.Fatal(err)
			}
		}, "item-a is not claimed by cycle 7 (cycle 8 holds it)"},
		{"the inbox is unreadable", func(t *testing.T, p pausedLane) {
			inbox := filepath.Join(p.evolveDir, "inbox")
			if err := os.Chmod(inbox, 0o000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })
		}, "cannot take its todo back"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := seedPausedLane(t, "item-a", 7)
			tc.setup(t, p)
			var stderr bytes.Buffer
			var got startedCycle

			if _, err := p.lane(&stderr, nil, "item-a").run(context.Background(), recordingStarter{&got}, laneRequest()); err != nil || got.resumed != nil {
				t.Fatalf("run err %v, resumed %+v; want a fresh cycle", err, got.resumed)
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Errorf("stderr = %q, want %q", stderr.String(), tc.want)
			}
			if cp := readCheckpoint(t, p.statePath); cp.AutoResumeAttempts != 0 {
				t.Errorf("checkpoint = %+v, want no auto-resume spent on a todo that the cycle cannot take back", cp)
			}
		})
	}
}

func TestLaneResume_APartialClaimIsReleasedAndTheLaneStartsAFreshCycle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		claim   deliverable.Claimer
		release error
		want    []string
	}{
		{"a todo the claimer skips", func(inboxDir string, cycle int, ids []string) error {
			return hostInboxClaimer(nil, nil, nil)(inboxDir, cycle, ids[:1])
		}, nil, []string{"item-b is not claimed by cycle 7 (cycle 0 holds it)"}},
		{"a failed claim and a failed release", func(inboxDir string, cycle int, ids []string) error {
			if err := hostInboxClaimer(nil, nil, nil)(inboxDir, cycle, ids[:1]); err != nil {
				return err
			}
			return errors.New("claim failed")
		}, errors.New("release failed"), []string{"claim failed", "release failed"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := seedPausedScope(t, []string{"item-a", "item-b"}, 7, 0)
			var stderr bytes.Buffer
			var got startedCycle
			lane := p.lane(&stderr, nil, "item-a", "item-b")
			lane.claim = tc.claim
			if tc.release != nil {
				release := lane.release
				lane.release = func(cycle int) error { return errors.Join(release(cycle), tc.release) }
			}

			if _, err := lane.run(context.Background(), recordingStarter{&got}, laneRequest()); err != nil || got.resumed != nil {
				t.Fatalf("run err %v, resumed %+v; want a fresh cycle", err, got.resumed)
			}
			for _, w := range tc.want {
				if !strings.Contains(stderr.String(), w) {
					t.Errorf("stderr = %q, want %q", stderr.String(), w)
				}
			}
			if loc, err := inboxmover.Locate(filepath.Join(p.evolveDir, "inbox"), "item-a"); err != nil || loc.Cycle != 0 {
				t.Errorf("Locate(item-a) = %+v, %v; want it pending again for the fresh cycle", loc, err)
			}
		})
	}
}

func TestFleetLaneScope_ReadsTheTrimmedScopeOfAFleetLaneOnly(t *testing.T) {
	t.Setenv(ipcenv.FleetScopeKey, " item-b,item-a ")
	t.Setenv(ipcenv.FleetKey, "")
	if got := fleetLaneScope(); got != nil {
		t.Errorf("fleetLaneScope outside a fleet = %v, want nil", got)
	}
	t.Setenv(ipcenv.FleetKey, "1")
	if got := fleetLaneScope(); strings.Join(got, ",") != "item-a,item-b" {
		t.Errorf("fleetLaneScope = %v, want [item-a item-b]", got)
	}
}
