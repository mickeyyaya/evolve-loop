//go:build acs

package cycle2298

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	pausedCycleA = 11
	pausedCycleB = 12
	exitInput    = 11
	exitUsage    = 10
)

var (
	builtBinary string
	buildOnce   sync.Once
	buildErr    string
	binaryDir   string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if binaryDir != "" {
		os.RemoveAll(binaryDir)
	}
	os.Exit(code)
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "cycle2298-evolve-")
		if err != nil {
			buildErr = err.Error()
			return
		}
		binaryDir = dir
		out := filepath.Join(dir, "evolve")
		moduleDir, err := acsassert.ModuleDir(".")
		if err != nil {
			buildErr = err.Error()
			return
		}
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", moduleDir, "build", "-o", out, "./cmd/evolve")
		if code != 0 || err != nil {
			buildErr = "go build ./cmd/evolve exited " + strconv.Itoa(code) + " (err=" + errText(err) + "): " + stdout + stderr
			return
		}
		builtBinary = out
	})
	if buildErr != "" {
		t.Fatalf("cannot build the evolve binary: %s", buildErr)
	}
	return builtBinary
}

type pausedLane struct {
	cycle     int
	scope     string
	runDir    string
	statePath string
	cs        core.CycleState
}

func pauseLaneOnQuota(t *testing.T, root string, cycle int, scope string) pausedLane {
	t.Helper()
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	laneScope, err := json.Marshal(core.LaneScope{TodoIDs: []string{scope}, GoalHash: "wave-goal"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, core.LaneScopeFile), laneScope, 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(runDir, core.CycleStateFile)
	baseState, err := json.Marshal(map[string]any{"cycle_id": cycle, "phase": "build"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, baseState, 0o644); err != nil {
		t.Fatal(err)
	}
	cs := core.CycleState{CycleID: cycle, Phase: "build", WorkspacePath: runDir, ActiveWorktree: t.TempDir()}
	t.Setenv(ipcenv.CycleStateFileKey, statePath)
	if err := core.QuotaBoundaryCheckpointer(cs, root, time.Now()); err != nil {
		t.Fatalf("quota pause of cycle %d: %v", cycle, err)
	}
	return pausedLane{cycle: cycle, scope: scope, runDir: runDir, statePath: statePath, cs: cs}
}

func errText(err error) string {
	if err == nil {
		return "nil"
	}
	return err.Error()
}

func pausedScopeCycles(t *testing.T, root string) []int {
	t.Helper()
	paused, err := core.PausedLaneScopes(root, filepath.Join(root, ".evolve"), core.ResumeOptions{})
	if err != nil {
		t.Fatalf("PausedLaneScopes: %v", err)
	}
	var cycles []int
	for _, p := range paused {
		cycles = append(cycles, p.Cycle)
	}
	return cycles
}

func stopForegroundResume(t *testing.T, root string, lane pausedLane) {
	t.Helper()
	evolveDir := filepath.Join(root, ".evolve")
	rp, ok := core.FindLaneResume(root, evolveDir, []string{lane.scope}, core.ResumeOptions{})
	if !ok {
		t.Fatalf("precondition: cycle %d paused on quota is not resumable before any resume", lane.cycle)
	}
	restore := core.ActivateResumeStatePath(rp, evolveDir)
	defer restore()
	if _, _, err := checkpoint.RecordResume(rp.StatePath, false); err != nil {
		t.Fatalf("foreground resume of cycle %d: %v", lane.cycle, err)
	}
	if err := core.ResumeBoundaryCheckpointer(lane.cs, root, time.Now()); err != nil {
		t.Fatalf("stop of the resumed cycle %d: %v", lane.cycle, err)
	}
}

func TestC2298_001_ALaneStoppedDuringAForegroundResumeStaysListedAsResumable(t *testing.T) {
	root := t.TempDir()
	lane := pauseLaneOnQuota(t, root, pausedCycleA, "item-a")

	stopForegroundResume(t, root, lane)

	acsassert.JSONFieldEquals(t, lane.statePath, "checkpoint.reason", "operator-requested")
	if got := pausedScopeCycles(t, root); !slices.Equal(got, []int{pausedCycleA}) {
		t.Errorf("the wave lists %v after the stop, want [%d]", got, pausedCycleA)
	}
	rp, ok := core.FindLaneResume(root, filepath.Join(root, ".evolve"), []string{"item-a"}, core.ResumeOptions{})
	if !ok || rp.CycleID != pausedCycleA || rp.Phase != "build" {
		t.Errorf("FindLaneResume after the stop = %+v, %v; want cycle %d at build", rp, ok, pausedCycleA)
	}
}

func TestC2298_002_AStoppedLaneDoesNotHideAnotherPausedLane(t *testing.T) {
	root := t.TempDir()
	stopped := pauseLaneOnQuota(t, root, pausedCycleA, "item-a")
	pauseLaneOnQuota(t, root, pausedCycleB, "item-b")

	stopForegroundResume(t, root, stopped)

	if got := pausedScopeCycles(t, root); !slices.Equal(got, []int{pausedCycleA, pausedCycleB}) {
		t.Errorf("the wave lists %v, want both [%d %d]", got, pausedCycleA, pausedCycleB)
	}
}

func TestC2298_003_ARunWithoutALaneScopeStaysOutOfTheWaveListing(t *testing.T) {
	root := t.TempDir()
	lane := pauseLaneOnQuota(t, root, pausedCycleA, "item-a")
	if err := os.Remove(filepath.Join(lane.runDir, core.LaneScopeFile)); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ipcenv.CycleStateFileKey, lane.statePath)
	if err := core.ResumeBoundaryCheckpointer(lane.cs, root, time.Now()); err != nil {
		t.Fatal(err)
	}

	if got := pausedScopeCycles(t, root); len(got) != 0 {
		t.Errorf("the wave lists %v for a run with no lane scope, want none", got)
	}
}

func resumeCommand(t *testing.T, root string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	t.Setenv(ipcenv.CycleStateFileKey, "")
	t.Setenv("HOME", t.TempDir())
	full := append([]string{"cycle", "resume", "--project-root", root}, args...)
	stdout, stderr, code, err := acsassert.SubprocessOutput(evolveBinary(t), full...)
	if code == -1 {
		t.Fatalf("evolve %s did not launch: %v", strings.Join(full, " "), err)
	}
	return stdout, stderr, code
}

type twoPausedLanes struct {
	root string
	a, b pausedLane
}

func twoPausedCycles(t *testing.T) twoPausedLanes {
	t.Helper()
	root := t.TempDir()
	a := pauseLaneOnQuota(t, root, pausedCycleA, "item-a")
	b := pauseLaneOnQuota(t, root, pausedCycleB, "item-b")
	return twoPausedLanes{root: root, a: a, b: b}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func checkpointBlock(t *testing.T, path string) checkpoint.Checkpoint {
	t.Helper()
	var state struct {
		Checkpoint checkpoint.Checkpoint `json:"checkpoint"`
	}
	if err := json.Unmarshal(readFile(t, path), &state); err != nil {
		t.Fatal(err)
	}
	return state.Checkpoint
}

func TestC2298_004_GrantingOneNamedCycleAmongTwoPausedCyclesGrantsOnlyThatCycle(t *testing.T) {
	lanes := twoPausedCycles(t)
	before := checkpointBlock(t, lanes.a.statePath)
	if checkpoint.BudgetLeft(before.AutoResumeAttempts, before.AutoResumeMaxAttempts) {
		t.Fatalf("precondition: cycle %d must have a spent budget, got %+v", pausedCycleA, before)
	}
	otherBefore := readFile(t, lanes.b.statePath)

	stdout, stderr, code := resumeCommand(t, lanes.root, "--cycle", strconv.Itoa(pausedCycleA), "--grant")

	if code != 0 {
		t.Fatalf("evolve cycle resume --cycle %d --grant exited %d\nstdout:\n%s\nstderr:\n%s", pausedCycleA, code, stdout, stderr)
	}
	after := checkpointBlock(t, lanes.a.statePath)
	if after.QuotaResetSource != "operator-grant" || !checkpoint.BudgetLeft(after.AutoResumeAttempts, after.AutoResumeMaxAttempts) {
		t.Errorf("cycle %d after the grant = %+v, want the operator-grant source and a budget", pausedCycleA, after)
	}
	if !bytes.Equal(otherBefore, readFile(t, lanes.b.statePath)) {
		t.Errorf("cycle %d changed, but only cycle %d was named", pausedCycleB, pausedCycleA)
	}
}

func TestC2298_005_AGrantThatNamesNoPausedCycleOrIsMalformedGrantsNothing(t *testing.T) {
	lanes := twoPausedCycles(t)
	snapshot := []byte{}
	for _, lane := range []pausedLane{lanes.a, lanes.b} {
		snapshot = append(snapshot, readFile(t, lane.statePath)...)
	}
	cases := []struct {
		name     string
		args     []string
		wantExit int
	}{
		{"a cycle that is not paused", []string{"--cycle", "99", "--grant"}, exitInput},
		{"a grant with no named cycle", []string{"--grant"}, exitUsage},
		{"a grant together with --all", []string{"--cycle", strconv.Itoa(pausedCycleA), "--grant", "--all"}, exitUsage},
	}

	for _, c := range cases {
		_, stderr, code := resumeCommand(t, lanes.root, c.args...)
		if code != c.wantExit {
			t.Errorf("%s: exit %d, want %d\nstderr:\n%s", c.name, code, c.wantExit, stderr)
		}
	}

	after := []byte{}
	for _, lane := range []pausedLane{lanes.a, lanes.b} {
		after = append(after, readFile(t, lane.statePath)...)
	}
	if !bytes.Equal(snapshot, after) {
		t.Errorf("a refused grant changed a paused cycle state")
	}
}

func treeListing(t *testing.T, root string) []string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".lock") || strings.HasSuffix(path, ".tmp") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		entries = append(entries, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestC2298_006_ANamedGrantStartsNoCycleRun(t *testing.T) {
	lanes := twoPausedCycles(t)
	listingBefore := treeListing(t, lanes.root)
	phaseBefore := string(readFile(t, lanes.a.statePath))
	var stateBefore struct {
		CycleID int    `json:"cycle_id"`
		Phase   string `json:"phase"`
	}
	if err := json.Unmarshal([]byte(phaseBefore), &stateBefore); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := resumeCommand(t, lanes.root, "--cycle", strconv.Itoa(pausedCycleA), "--grant")

	if code != 0 {
		t.Fatalf("evolve cycle resume --cycle %d --grant exited %d\nstdout:\n%s\nstderr:\n%s", pausedCycleA, code, stdout, stderr)
	}
	if listingAfter := treeListing(t, lanes.root); !slices.Equal(listingBefore, listingAfter) {
		t.Errorf("the grant created or removed files, so a run may have started:\nbefore: %v\nafter:  %v", listingBefore, listingAfter)
	}
	var stateAfter struct {
		CycleID int    `json:"cycle_id"`
		Phase   string `json:"phase"`
	}
	if err := json.Unmarshal(readFile(t, lanes.a.statePath), &stateAfter); err != nil {
		t.Fatal(err)
	}
	if stateAfter != stateBefore {
		t.Errorf("the grant moved the cycle state from %+v to %+v", stateBefore, stateAfter)
	}
	if cp := checkpointBlock(t, lanes.a.statePath); cp.ResumeFromPhase != "build" || !cp.Enabled {
		t.Errorf("the grant changed where the cycle resumes: %+v", cp)
	}
}
