//go:build acs

package cycle2621

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	exitDecideRefused = 10
	subjectAtFour     = "task-at-four-blocks"
	subjectAtThree    = "task-at-three-blocks"
	subjectRedesigned = "task-after-one-redesign"
)

var allLoops = []convergence.Loop{
	convergence.LoopAuditRepair,
	convergence.LoopCodeReview,
	convergence.LoopExplanationReauthor,
	convergence.LoopConsoleLane,
	convergence.LoopCycle,
	convergence.LoopInboxItem,
	convergence.LoopShipRecovery,
}

var (
	buildOnce   sync.Once
	builtBinary string
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
		dir, err := os.MkdirTemp("", "cycle2621-evolve-")
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
		stdout, stderr, code, runErr := acsassert.SubprocessOutput("go", "-C", moduleDir, "build", "-o", out, "./cmd/evolve")
		if runErr != nil || code != 0 {
			buildFailed = "go build ./cmd/evolve failed: " + stdout + stderr
			return
		}
		builtBinary = out
	})
	if buildFailed != "" {
		t.Fatalf("RED: cannot build the evolve binary: %s", buildFailed)
	}
	return builtBinary
}

func openHighRounds(rounds int) []convergence.Judgment {
	js := make([]convergence.Judgment, rounds)
	for i := range js {
		js[i] = convergence.Judgment{Index: i, Findings: []convergence.Finding{
			{ID: "h1", Severity: convergence.SeverityHigh, Status: convergence.StatusOpen, Class: "correctness"},
		}}
	}
	return js
}

func cleanRound() []convergence.Judgment {
	return []convergence.Judgment{{Index: 0}}
}

func decisionFor(loop convergence.Loop, js []convergence.Judgment, blocks, redesigns int) convergence.Decision {
	return convergence.Decide(convergence.Input{
		Loop: loop, Round: len(js) - 1, Rounds: js,
		Config: policy.Policy{}.ConvergenceConfig(), Blocks: blocks, Redesigns: redesigns,
	})
}

func TestC2621_001_ASubjectPastThreeBlocksReturnsRedesign(t *testing.T) {
	for _, loop := range allLoops {
		for _, rounds := range []int{1, 4} {
			js := openHighRounds(rounds)
			base := decisionFor(loop, js, 0, 0)
			if base.Action == convergence.ActionRedesign || base.Action == convergence.ActionToOperator {
				t.Errorf("RED: %s with %d rounds and no host count already returns %s", loop, rounds, base.Action)
			}
			if got := decisionFor(loop, js, 3, 0); !reflect.DeepEqual(got, base) {
				t.Errorf("RED: %s with %d rounds at three blocks changed the decision: got %+v want %+v", loop, rounds, got, base)
			}
			for _, blocks := range []int{4, 9} {
				got := decisionFor(loop, js, blocks, 0)
				if got.Action != convergence.ActionRedesign {
					t.Errorf("RED: %s with %d rounds at %d blocks returned %s, want %s", loop, rounds, blocks, got.Action, convergence.ActionRedesign)
				}
				if !reflect.DeepEqual(got.Redesign, base.Redesign) {
					t.Errorf("RED: %s at %d blocks changed Decision.Redesign (the component list) from %q to %q", loop, blocks, base.Redesign, got.Redesign)
				}
			}
		}
	}
}

func TestC2621_002_AFixedRoundAtFourBlocksStillLandsAndDoesNotRedesign(t *testing.T) {
	for _, loop := range allLoops {
		base := decisionFor(loop, cleanRound(), 0, 0)
		if base.Action != convergence.ActionLand {
			t.Fatalf("RED: %s with a clean round returned %s, want Land", loop, base.Action)
		}
		for _, blocks := range []int{4, 9} {
			if got := decisionFor(loop, cleanRound(), blocks, 0); !reflect.DeepEqual(got, base) {
				t.Errorf("RED: %s with a clean round at %d blocks changed the decision: got %+v want %+v", loop, blocks, got, base)
			}
		}
		if got := decisionFor(loop, cleanRound(), 4, 1); !reflect.DeepEqual(got, base) {
			t.Errorf("RED: %s with a clean round after one redesign changed the decision: got %+v want %+v", loop, got, base)
		}
	}
}

func TestC2621_003_ASecondCountPastThreeReturnsToOperator(t *testing.T) {
	for _, loop := range allLoops {
		for _, rounds := range []int{1, 4} {
			js := openHighRounds(rounds)
			base := decisionFor(loop, js, 0, 0)
			if got := decisionFor(loop, js, 3, 1); !reflect.DeepEqual(got, base) {
				t.Errorf("RED: %s with %d rounds at three blocks after one redesign changed the decision: got %+v want %+v", loop, rounds, got, base)
			}
			for _, tc := range [][2]int{{4, 1}, {9, 1}, {4, 2}} {
				got := decisionFor(loop, js, tc[0], tc[1])
				if got.Action != convergence.ActionToOperator {
					t.Errorf("RED: %s with %d rounds at %d blocks and %d redesigns returned %s, want %s", loop, rounds, tc[0], tc[1], got.Action, convergence.ActionToOperator)
				}
			}
			if got := decisionFor(loop, js, 4, 0); got.Action != convergence.ActionRedesign {
				t.Errorf("RED: %s with %d rounds at four blocks and no redesign returned %s, want %s", loop, rounds, got.Action, convergence.ActionRedesign)
			}
		}
	}
}

func hostRecord() map[string]cyclestate.BlockCount {
	return map[string]cyclestate.BlockCount{
		subjectAtFour:     {Blocks: 4},
		subjectAtThree:    {Blocks: 3},
		subjectRedesigned: {Blocks: 4, Redesigns: 1},
	}
}

func recordCounts(t *testing.T, evolveDir string) {
	t.Helper()
	_, err := storage.New(evolveDir).UpdateState(context.Background(), func(st *core.State) {
		st.BlockCounts = hostRecord()
	})
	if err != nil {
		t.Fatalf("RED: the production writer UpdateState refused the host record: %v", err)
	}
}

func projectRoot(t *testing.T) (root, evolveDir string) {
	t.Helper()
	t.Setenv(ipcenv.CycleStateFileKey, "")
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, evolveDir
}

func roundsFile(t *testing.T, dir, subject, extraFields string) string {
	t.Helper()
	quotedSubject, err := json.Marshal(subject)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"loop":"console-lane","round":0,"subject":` + string(quotedSubject) + extraFields +
		`,"rounds":[{"index":0,"findings":[{"id":"h1","severity":"HIGH","status":"OPEN","class":"correctness"}]}],` +
		`"fixer":{},"judge":{}}`
	path := filepath.Join(dir, "rounds-"+strings.ReplaceAll(subject, "/", "_")+extraFieldsTag(extraFields)+".json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func extraFieldsTag(extra string) string {
	if extra == "" {
		return ""
	}
	return "-extra"
}

type decideRun struct {
	code   int
	stdout string
	stderr string
}

func runDecide(t *testing.T, root, rounds string) decideRun {
	t.Helper()
	cmd := exec.Command(evolveBinary(t), "convergence", "decide", "--input", rounds, "--json")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "EVOLVE_PROJECT_ROOT="+root)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	run := decideRun{stdout: out.String(), stderr: errOut.String()}
	if exitErr, ok := err.(*exec.ExitError); ok {
		run.code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("RED: cannot run the evolve binary: %v", err)
	}
	return run
}

func actionOf(t *testing.T, run decideRun) convergence.Action {
	t.Helper()
	if run.code != 0 {
		t.Fatalf("RED: evolve convergence decide exit=%d stderr=%q", run.code, run.stderr)
	}
	var d convergence.Decision
	if err := json.Unmarshal([]byte(run.stdout), &d); err != nil {
		t.Fatalf("RED: decision is not JSON: %v: %q", err, run.stdout)
	}
	return d.Action
}

func TestC2621_004_TheCountSurvivesAResumeAndReachesTheDecisionOfTheCli(t *testing.T) {
	root, evolveDir := projectRoot(t)
	recordCounts(t, evolveDir)

	ctx := context.Background()
	if err := storage.New(evolveDir).WriteCycleState(ctx, core.CycleState{CycleID: 7, Phase: "build", WorkspacePath: filepath.Join(evolveDir, "runs", "cycle-7")}); err != nil {
		t.Fatalf("pause checkpoint: %v", err)
	}

	resumed := storage.New(evolveDir)
	if _, err := resumed.UpdateState(ctx, func(st *core.State) {
		st.LastAllocatedCycleNumber = 8
		st.LastCycleNumber = 7
	}); err != nil {
		t.Fatalf("resume bookkeeping UpdateState: %v", err)
	}
	st, err := resumed.ReadState(ctx)
	if err != nil {
		t.Fatalf("resume ReadState: %v", err)
	}
	if err := resumed.WriteState(ctx, st); err != nil {
		t.Fatalf("resume WriteState: %v", err)
	}

	after, err := storage.New(evolveDir).ReadState(ctx)
	if err != nil {
		t.Fatalf("ReadState after resume: %v", err)
	}
	if after.LastCycleNumber != 7 {
		t.Fatalf("the resume bookkeeping did not reach state.json: lastCycleNumber=%d", after.LastCycleNumber)
	}
	if !reflect.DeepEqual(after.BlockCounts, hostRecord()) {
		t.Errorf("RED: the resume lost or changed the host block count: got %+v want %+v", after.BlockCounts, hostRecord())
	}

	scratch := t.TempDir()
	want := map[string]convergence.Action{
		subjectAtFour:     convergence.ActionRedesign,
		subjectRedesigned: convergence.ActionToOperator,
	}
	for subject, action := range want {
		if got := actionOf(t, runDecide(t, root, roundsFile(t, scratch, subject, ""))); got != action {
			t.Errorf("RED: after a resume the CLI decided %s for %s, want %s", got, subject, action)
		}
	}
	got := actionOf(t, runDecide(t, root, roundsFile(t, scratch, subjectAtThree, "")))
	if got == convergence.ActionRedesign || got == convergence.ActionToOperator {
		t.Errorf("RED: after a resume the CLI decided %s for a subject at three blocks", got)
	}
}

func TestC2621_005_ASeatCannotWriteTheCount(t *testing.T) {
	root, evolveDir := projectRoot(t)
	recordCounts(t, evolveDir)
	scratch := t.TempDir()
	statePath := filepath.Join(evolveDir, "state.json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}

	if got := actionOf(t, runDecide(t, root, roundsFile(t, scratch, subjectAtFour, ""))); got != convergence.ActionRedesign {
		t.Fatalf("RED: positive control: the host count of four blocks gave %s, want %s", got, convergence.ActionRedesign)
	}

	for _, forged := range []string{`,"blocks":0`, `,"redesigns":0`, `,"blocks":0,"redesigns":0`} {
		run := runDecide(t, root, roundsFile(t, scratch, subjectAtFour, forged))
		if run.code != exitDecideRefused {
			t.Errorf("RED: a seat-authored input carrying %s exit=%d, want %d (refused): stdout=%q", forged, run.code, exitDecideRefused, run.stdout)
		}
	}
	if got := actionOf(t, runDecide(t, root, roundsFile(t, scratch, subjectAtFour, ""))); got != convergence.ActionRedesign {
		t.Errorf("RED: after forged inputs the count of four blocks gave %s, want %s", got, convergence.ActionRedesign)
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("RED: evolve convergence decide changed the host record in state.json")
	}

	workspace := filepath.Join(evolveDir, "runs", "cycle-7")
	worktree := filepath.Join(evolveDir, "worktrees", "cycle-7")
	checkpoint := core.CycleState{CycleID: 7, Phase: "build", ActiveAgent: "builder", WorkspacePath: workspace, ActiveWorktree: worktree}
	if err := storage.New(evolveDir).WriteCycleState(context.Background(), checkpoint); err != nil {
		t.Fatalf("active cycle checkpoint: %v", err)
	}
	role := guards.NewRole(storage.New(evolveDir), false)
	write := func(tool, path string) core.GuardDecision {
		return role.Decide(context.Background(), core.GuardInput{ToolName: tool, ToolInput: map[string]any{"file_path": path}})
	}
	if d := write("Write", filepath.Join(workspace, "build-report.md")); !d.Allow {
		t.Fatalf("RED: positive control: the role guard denied a write inside the workspace: %s", d.Reason)
	}
	for _, tool := range []string{"Write", "Edit"} {
		if d := write(tool, statePath); d.Allow {
			t.Errorf("RED: the role guard let a seat %s the host record %s", tool, statePath)
		}
	}
}

func TestC2621_006_TheStateKeyAllowlistNamesTheNewBlockCountsKey(t *testing.T) {
	moduleDir, err := acsassert.ModuleDir(".")
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-run", "^TestByteStability_", "./internal/core")
	if code != 0 {
		t.Errorf("RED: the byte-stability tests of internal/core fail (exit %d), so the additive state key blockCounts is not in additiveStateKeys: %s%s", code, stdout, stderr)
	}
}
