//go:build acs

package cycle2300

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cycleA        = 21
	cycleB        = 22
	exitRefused   = 3
	exitUsage     = 10
	exitInput     = 11
	retireReason  = "abandoned: quota wall hit twice, superseded by a fresh lane"
	claimedTaskID = "task-held-by-cycle-21"
)

var (
	builtBinary string
	buildOnce   sync.Once
	buildFailed string
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
		dir, err := os.MkdirTemp("", "cycle2300-evolve-")
		if err != nil {
			buildFailed = err.Error()
			return
		}
		binaryDir = dir
		moduleDir, err := acsassert.ModuleDir(".")
		if err != nil {
			buildFailed = err.Error()
			return
		}
		out := filepath.Join(dir, "evolve")
		stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", moduleDir, "build", "-o", out, "./cmd/evolve")
		if code != 0 {
			buildFailed = "go build ./cmd/evolve exited " + strconv.Itoa(code) + ": " + acsassert.OutputTail(stdout+stderr, 20)
			return
		}
		builtBinary = out
	})
	if buildFailed != "" {
		t.Fatalf("cannot build the evolve binary: %s", buildFailed)
	}
	return builtBinary
}

type pausedLane struct {
	cycle    int
	runDir   string
	state    string
	worktree string
}

func pauseOnQuota(t *testing.T, root string, cycle int, scopeID string) pausedLane {
	t.Helper()
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pin, err := json.Marshal(core.LaneScope{TodoIDs: []string{scopeID}, GoalHash: "wave-goal"})
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(runDir, core.CycleStateFile)
	seed, err := json.Marshal(map[string]any{"cycle_id": cycle, "phase": "build"})
	if err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string][]byte{filepath.Join(runDir, core.LaneScopeFile): pin, state: seed} {
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	worktree := t.TempDir()
	t.Setenv(ipcenv.CycleStateFileKey, state)
	cs := core.CycleState{CycleID: cycle, Phase: "build", WorkspacePath: runDir, ActiveWorktree: worktree}
	if err := core.QuotaBoundaryCheckpointer(cs, root, time.Now()); err != nil {
		t.Fatalf("quota pause of cycle %d: %v", cycle, err)
	}
	t.Setenv(ipcenv.CycleStateFileKey, "")
	return pausedLane{cycle: cycle, runDir: runDir, state: state, worktree: worktree}
}

func evolve(t *testing.T, root string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	full := append(slices.Clone(args), "--project-root", root)
	stdout, stderr, code, err := acsassert.SubprocessOutput(evolveBinary(t), full...)
	if code == -1 {
		t.Fatalf("evolve %s did not launch: %v", strings.Join(full, " "), err)
	}
	return stdout, stderr, code
}

func retire(t *testing.T, root string, cycle int, reason string) (stdout, stderr string, code int) {
	t.Helper()
	return evolve(t, root, "cycle", "retire", "--cycle", strconv.Itoa(cycle), "--reason", reason)
}

func listedByResumeAll(t *testing.T, root string) []int {
	t.Helper()
	stdout, stderr, code := evolve(t, root, "cycle", "resume", "--all", "--dry-run", "--json")
	if code != 0 {
		t.Fatalf("evolve cycle resume --all --dry-run exited %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	var envelope struct {
		Data struct {
			Cycles []struct {
				Cycle int `json:"cycle"`
			} `json:"cycles"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("resume --all JSON does not parse: %v\n%s", err, stdout)
	}
	var cycles []int
	for _, c := range envelope.Data.Cycles {
		cycles = append(cycles, c.Cycle)
	}
	slices.Sort(cycles)
	return cycles
}

func ledgerLines(t *testing.T, root string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "ledger.jsonl"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

func linesNaming(lines []string, needles ...string) []string {
	var hits []string
	for _, line := range lines {
		lower := strings.ToLower(line)
		matched := true
		for _, n := range needles {
			matched = matched && strings.Contains(lower, strings.ToLower(n))
		}
		if matched {
			hits = append(hits, line)
		}
	}
	return hits
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	handle, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer handle.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(handle); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return buf.Bytes()
}

func twoPausedLanes(t *testing.T) (root string, a, b pausedLane) {
	t.Helper()
	root = t.TempDir()
	a = pauseOnQuota(t, root, cycleA, "item-a")
	b = pauseOnQuota(t, root, cycleB, "item-b")
	return root, a, b
}

func grantResume(t *testing.T, root string, cycle int) {
	t.Helper()
	stdout, stderr, code := evolve(t, root, "cycle", "resume", "--cycle", strconv.Itoa(cycle), "--grant")
	if code != 0 {
		t.Fatalf("grant for cycle %d exited %d\nstdout:\n%s\nstderr:\n%s", cycle, code, stdout, stderr)
	}
}

func TestC2300_001_ARetiredPausedCycleWithAGrantIsNoLongerListedByResumeAll(t *testing.T) {
	root, _, b := twoPausedLanes(t)
	grantResume(t, root, cycleA)
	if got := listedByResumeAll(t, root); !slices.Equal(got, []int{cycleA, cycleB}) {
		t.Fatalf("precondition: resume --all lists %v before the retire, want [%d %d]", got, cycleA, cycleB)
	}
	otherBefore := read(t, b.state)

	stdout, stderr, code := retire(t, root, cycleA, retireReason)

	if code != 0 {
		t.Fatalf("evolve cycle retire --cycle %d exited %d\nstdout:\n%s\nstderr:\n%s", cycleA, code, stdout, stderr)
	}
	if got := listedByResumeAll(t, root); !slices.Equal(got, []int{cycleB}) {
		t.Errorf("resume --all lists %v after retiring cycle %d, want [%d]", got, cycleA, cycleB)
	}
	if !bytes.Equal(otherBefore, read(t, b.state)) {
		t.Errorf("cycle %d changed, but only cycle %d was retired", cycleB, cycleA)
	}
}

func TestC2300_002_ARetiredCycleCannotBeGrantedAResumeAgain(t *testing.T) {
	root, _, _ := twoPausedLanes(t)
	grantResume(t, root, cycleA)
	if _, _, code := retire(t, root, cycleA, retireReason); code != 0 {
		t.Fatalf("retire exited %d", code)
	}

	stdout, stderr, code := evolve(t, root, "cycle", "resume", "--cycle", strconv.Itoa(cycleA), "--grant")

	if code != exitInput {
		t.Errorf("grant for the retired cycle exited %d, want %d (not a paused cycle)\nstdout:\n%s\nstderr:\n%s", code, exitInput, stdout, stderr)
	}
	if got := listedByResumeAll(t, root); slices.Contains(got, cycleA) {
		t.Errorf("resume --all lists the retired cycle again: %v", got)
	}
}

func TestC2300_003_TheSealIsInTheProjectLedgerWithTheReason(t *testing.T) {
	root, _, _ := twoPausedLanes(t)
	if hits := linesNaming(ledgerLines(t, root), retireReason); len(hits) != 0 {
		t.Fatalf("precondition: the ledger already names the reason: %v", hits)
	}

	if stdout, stderr, code := retire(t, root, cycleA, retireReason); code != 0 {
		t.Fatalf("retire exited %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	hits := linesNaming(ledgerLines(t, root), retireReason, "retire")
	if len(hits) != 1 {
		t.Fatalf("the project ledger holds %d retire entries with the reason, want 1: %v", len(hits), hits)
	}
	if !strings.Contains(hits[0], "cycle-"+strconv.Itoa(cycleA)) && !strings.Contains(hits[0], `"cycle":`+strconv.Itoa(cycleA)) {
		t.Errorf("the retire entry does not name cycle %d: %s", cycleA, hits[0])
	}
	if err := ledger.New(filepath.Join(root, ".evolve")).Verify(context.Background()); err != nil {
		t.Errorf("the ledger chain does not verify after the retire: %v", err)
	}
}

func TestC2300_004_TheVerbRefusesACycleWhoseLaneIsLive(t *testing.T) {
	root, a, _ := twoPausedLanes(t)
	grantResume(t, root, cycleA)
	if err := runlease.Write(a.runDir, runlease.Lease{RunID: "live-lane", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	stateBefore := read(t, a.state)

	stdout, stderr, code := retire(t, root, cycleA, retireReason)

	if code != exitRefused {
		t.Errorf("retire of a live lane exited %d, want %d\nstdout:\n%s\nstderr:\n%s", code, exitRefused, stdout, stderr)
	}
	if !bytes.Equal(stateBefore, read(t, a.state)) {
		t.Errorf("the refused retire changed the state of cycle %d", cycleA)
	}
	if hits := linesNaming(ledgerLines(t, root), retireReason); len(hits) != 0 {
		t.Errorf("the refused retire wrote a ledger entry: %v", hits)
	}

	if err := os.Remove(runlease.PathIn(a.runDir)); err != nil {
		t.Fatal(err)
	}
	if stdout, stderr, code := retire(t, root, cycleA, retireReason); code != 0 {
		t.Errorf("retire after the lease ended exited %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
}

func TestC2300_005_ACycleThatIsNotPausedOrNotNamedIsRefusedAndChangesNothing(t *testing.T) {
	root, a, b := twoPausedLanes(t)
	beforeA, beforeB := read(t, a.state), read(t, b.state)

	_, stderrUnknown, codeUnknown := retire(t, root, 99, retireReason)
	_, stderrNone, codeNone := evolve(t, root, "cycle", "retire", "--reason", retireReason)

	if codeUnknown != exitInput {
		t.Errorf("retire of unknown cycle 99 exited %d, want %d\nstderr:\n%s", codeUnknown, exitInput, stderrUnknown)
	}
	if codeNone != exitUsage {
		t.Errorf("retire with no --cycle exited %d, want %d\nstderr:\n%s", codeNone, exitUsage, stderrNone)
	}
	if !bytes.Equal(beforeA, read(t, a.state)) || !bytes.Equal(beforeB, read(t, b.state)) {
		t.Errorf("a refused retire changed a paused lane")
	}
	if hits := linesNaming(ledgerLines(t, root), retireReason); len(hits) != 0 {
		t.Errorf("a refused retire wrote a ledger entry: %v", hits)
	}
	if got := listedByResumeAll(t, root); !slices.Equal(got, []int{cycleA, cycleB}) {
		t.Errorf("resume --all lists %v after the refused retires, want [%d %d]", got, cycleA, cycleB)
	}
}

func TestC2300_006_TheRetireReleasesTheInboxClaimsAndKeepsTheWorktree(t *testing.T) {
	root, a, _ := twoPausedLanes(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	claimDir := filepath.Join(inbox, "processing", "cycle-"+strconv.Itoa(cycleA))
	if err := os.MkdirAll(claimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	item := []byte(`{"id":"` + claimedTaskID + `"}`)
	if err := os.WriteFile(filepath.Join(claimDir, claimedTaskID+".json"), item, 0o644); err != nil {
		t.Fatal(err)
	}

	if stdout, stderr, code := retire(t, root, cycleA, retireReason); code != 0 {
		t.Fatalf("retire exited %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	if _, err := os.Stat(filepath.Join(claimDir, claimedTaskID+".json")); err == nil {
		t.Errorf("the claim of cycle %d still sits in %s after the retire", cycleA, claimDir)
	}
	if _, err := os.Stat(filepath.Join(inbox, claimedTaskID+".json")); err != nil {
		t.Errorf("the released item is not back in the inbox queue: %v", err)
	}
	if info, err := os.Stat(a.worktree); err != nil || !info.IsDir() {
		t.Errorf("the worktree %s of the retired cycle is gone (err=%v); gc owns its removal", a.worktree, err)
	}
}
