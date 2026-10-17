package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/goalhash"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type goalSourceDryRun struct {
	DryRun bool `json:"dry_run"`
	Config struct {
		GoalHash   string `json:"goal_hash"`
		GoalText   string `json:"goal_text"`
		GoalSource string `json:"goal_source"`
	} `json:"config"`
}

func installNoopLaunchDeps(t *testing.T) {
	t.Helper()
	prev := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = prev })
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		st := &fixtures.FakeStorage{}
		ld := newFakeLedger()
		runners := map[core.Phase]core.PhaseRunner{
			core.PhaseIntent:       noopRunner{name: "intent"},
			core.PhaseScout:        noopRunner{name: "scout"},
			core.PhaseTriage:       noopRunner{name: "triage"},
			core.PhaseTDD:          noopRunner{name: "tdd"},
			core.PhaseBuildPlanner: noopRunner{name: "build-planner"},
			core.PhaseBuild:        noopRunner{name: "build"},
			core.PhaseAudit:        noopRunner{name: "audit"},
			core.PhaseShip:         noopRunner{name: "ship"},
			core.PhaseRetro:        noopRunner{name: "retro"},
		}
		return orchDeps{Storage: st, Ledger: ld, Orchestrator: core.NewOrchestrator(st, ld, runners, core.WithWorktreeProvisioner(inPlaceStubWorktree{}))}
	}
}

func newGoalSourceProject(t *testing.T) (projectRoot, evolveDir string) {
	t.Helper()
	projectRoot = t.TempDir()
	initLoopContractRepo(t, projectRoot)
	evolveDir = filepath.Join(projectRoot, ".evolve")
	writeDispatchPolicy(t, evolveDir, "off")
	return projectRoot, evolveDir
}

func launchLoopWithGoal(t *testing.T, projectRoot, goal string) {
	t.Helper()
	installNoopLaunchDeps(t)
	var stdout, stderr bytes.Buffer
	runLoop([]string{"--project-root", projectRoot, "--goal-text", goal, "--cycles", "1"}, nil, &stdout, &stderr)
}

func dryRunGoal(t *testing.T, args ...string) goalSourceDryRun {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runLoop(append(args, "--dry-run"), nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("dry-run %v rc=%d want 0; stderr=%q", args, rc, firstLine(stderr.String()))
	}
	var out goalSourceDryRun
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("dry-run output is not JSON: %v\n%s", err, stdout.String())
	}
	return out
}

func readRecordedGoal(t *testing.T, evolveDir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(evolveDir, "last-goal.txt"))
	if err != nil {
		t.Fatalf("launch did not record its goal in .evolve/last-goal.txt: %v", err)
	}
	return strings.TrimSpace(string(b))
}

func TestLoopGoalSource_LaunchRecordsGoalTextThatGoalFromLastReplays(t *testing.T) {
	projectRoot, evolveDir := newGoalSourceProject(t)
	const launchedGoal = "Wave 104. Ship the highest-weight lane items with full phase integrity."

	launchLoopWithGoal(t, projectRoot, launchedGoal)

	if got := readRecordedGoal(t, evolveDir); got != launchedGoal {
		t.Fatalf("recorded goal = %q, want %q", got, launchedGoal)
	}
	replayed := dryRunGoal(t, "--project-root", projectRoot, "--goal-from-last")
	if replayed.Config.GoalHash != goalhash.Compute(launchedGoal) {
		t.Errorf("--goal-from-last goal_hash = %q, want the launched goal's hash %q", replayed.Config.GoalHash, goalhash.Compute(launchedGoal))
	}
	if replayed.Config.GoalText != launchedGoal {
		t.Errorf("--goal-from-last goal_text = %q, want %q", replayed.Config.GoalText, launchedGoal)
	}
	if replayed.Config.GoalSource != "goal-from-last" {
		t.Errorf("goal_source = %q, want %q", replayed.Config.GoalSource, "goal-from-last")
	}
}

func TestLoopGoalSource_LaunchReplacesTheRecordAtomically(t *testing.T) {
	projectRoot, evolveDir := newGoalSourceProject(t)
	record := filepath.Join(evolveDir, "last-goal.txt")
	if err := os.WriteFile(record, []byte("a stale goal from an older wave that is longer than the new one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	staleInfo, err := os.Stat(record)
	if err != nil {
		t.Fatal(err)
	}

	launchLoopWithGoal(t, projectRoot, "new wave goal")

	if got := readRecordedGoal(t, evolveDir); got != "new wave goal" {
		t.Fatalf("recorded goal = %q, want %q (the record must be replaced, never appended or partially overwritten)", got, "new wave goal")
	}
	freshInfo, err := os.Stat(record)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(staleInfo, freshInfo) {
		t.Error("last-goal.txt kept its inode: the record was rewritten in place, not atomically renamed over the old file")
	}
	entries, err := os.ReadDir(evolveDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "last-goal") && e.Name() != "last-goal.txt" {
			t.Errorf("atomic write left a temporary file behind: %s", e.Name())
		}
	}
}

func TestLoopGoalSource_DryRunRecordsNothing(t *testing.T) {
	projectRoot, evolveDir := newGoalSourceProject(t)

	dryRunGoal(t, "--project-root", projectRoot, "--goal-text", "a goal that is only planned")

	if _, err := os.Stat(filepath.Join(evolveDir, "last-goal.txt")); !os.IsNotExist(err) {
		t.Errorf("a --dry-run must not record a goal; stat err = %v", err)
	}
}

func TestLoopGoalSource_GoalFileResolvesTheFileText(t *testing.T) {
	projectRoot, _ := newGoalSourceProject(t)
	goalFile := filepath.Join(t.TempDir(), "goal.txt")
	if err := os.WriteFile(goalFile, []byte("  goal read from a file\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := dryRunGoal(t, "--project-root", projectRoot, "--goal-file", goalFile)

	if out.Config.GoalHash != goalhash.Compute("goal read from a file") {
		t.Errorf("--goal-file goal_hash = %q, want %q", out.Config.GoalHash, goalhash.Compute("goal read from a file"))
	}
	if strings.TrimSpace(out.Config.GoalText) != "goal read from a file" {
		t.Errorf("--goal-file goal_text = %q, want the file text", out.Config.GoalText)
	}
	if out.Config.GoalSource != "goal-file" {
		t.Errorf("goal_source = %q, want %q", out.Config.GoalSource, "goal-file")
	}
}

func TestLoopGoalSource_PrintedConfigNamesEveryGoalSource(t *testing.T) {
	projectRoot, _ := newGoalSourceProject(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"goal text flag", []string{"--goal-text", "typed goal"}, "goal-text"},
		{"goal hash flag", []string{"--goal-hash", goalhash.Compute("hashed goal")}, "goal-hash"},
		{"positional goal", []string{"positional goal words"}, "positional"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"--project-root", projectRoot, "--dry-run"}, tc.args...)
			var stdout, stderr bytes.Buffer
			if rc := runLoop(args, nil, &stdout, &stderr); rc != 0 {
				t.Fatalf("rc=%d want 0; stderr=%q", rc, firstLine(stderr.String()))
			}
			var out goalSourceDryRun
			if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
				t.Fatalf("dry-run output is not JSON: %v", err)
			}
			if out.Config.GoalSource != tc.want {
				t.Errorf("goal_source = %q, want %q", out.Config.GoalSource, tc.want)
			}
		})
	}
}

func TestLoopGoalSource_TwoGoalSourcesExit10(t *testing.T) {
	projectRoot, evolveDir := newGoalSourceProject(t)
	if err := os.WriteFile(filepath.Join(evolveDir, "last-goal.txt"), []byte("recorded goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goalFile := filepath.Join(t.TempDir(), "goal.txt")
	if err := os.WriteFile(goalFile, []byte("file goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
	}{
		{"goal file with goal text", []string{"--goal-file", goalFile, "--goal-text", "typed goal"}},
		{"goal file with goal from last", []string{"--goal-file", goalFile, "--goal-from-last"}},
		{"goal from last with goal text", []string{"--goal-from-last", "--goal-text", "typed goal"}},
		{"goal from last with goal hash", []string{"--goal-from-last", "--goal-hash", goalhash.Compute("x")}},
		{"goal file with goal hash", []string{"--goal-file", goalFile, "--goal-hash", goalhash.Compute("x")}},
		{"goal file with positional goal", []string{"--goal-file", goalFile, "positional goal"}},
		{"goal from last with positional goal", []string{"--goal-from-last", "positional goal"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"--project-root", projectRoot, "--dry-run"}, tc.args...)
			var stdout, stderr bytes.Buffer
			if rc := runLoop(args, nil, &stdout, &stderr); rc != 10 {
				t.Errorf("rc=%d want 10 for two goal sources; stderr=%q", rc, firstLine(stderr.String()))
			}
			requireGoalRefusalNamesTheGoal(t, stderr.String())
			if stdout.Len() != 0 {
				t.Errorf("a refused launch must print no run config; stdout=%q", stdout.String())
			}
		})
	}
}

func TestLoopGoalSource_GoalFromLastWithNothingRecordedExits1(t *testing.T) {
	cases := []struct {
		name   string
		record *string
	}{
		{"no record file", nil},
		{"empty record file", ptrTo("")},
		{"whitespace-only record file", ptrTo("  \n\t\n")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			projectRoot, evolveDir := newGoalSourceProject(t)
			if tc.record != nil {
				if err := os.WriteFile(filepath.Join(evolveDir, "last-goal.txt"), []byte(*tc.record), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var stdout, stderr bytes.Buffer
			rc := runLoop([]string{"--project-root", projectRoot, "--goal-from-last", "--dry-run"}, nil, &stdout, &stderr)
			if rc != 1 {
				t.Errorf("rc=%d want 1 when no goal is recorded; stderr=%q", rc, firstLine(stderr.String()))
			}
			if !strings.Contains(stderr.String(), "last-goal") {
				t.Errorf("stderr must name the missing record; got %q", firstLine(stderr.String()))
			}
		})
	}
}

func TestLoopGoalSource_UnreadableGoalFileExits10(t *testing.T) {
	projectRoot, _ := newGoalSourceProject(t)
	emptyGoalFile := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(emptyGoalFile, []byte("\n  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		path string
	}{
		{"missing goal file", filepath.Join(t.TempDir(), "absent.txt")},
		{"blank goal file", emptyGoalFile},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rc := runLoop([]string{"--project-root", projectRoot, "--goal-file", tc.path, "--dry-run"}, nil, &stdout, &stderr)
			if rc != 10 {
				t.Errorf("rc=%d want 10 for an unusable --goal-file; stderr=%q", rc, firstLine(stderr.String()))
			}
			requireGoalRefusalNamesTheGoal(t, stderr.String())
		})
	}
}

func requireGoalRefusalNamesTheGoal(t *testing.T, stderr string) {
	t.Helper()
	if strings.Contains(stderr, "flag provided but not defined") {
		t.Errorf("refused as an unknown flag, not by goal-source validation; stderr=%q", firstLine(stderr))
	}
	if !strings.Contains(stderr, "goal") {
		t.Errorf("the refusal must name the goal source; stderr=%q", firstLine(stderr))
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func ptrTo(s string) *string { return &s }
