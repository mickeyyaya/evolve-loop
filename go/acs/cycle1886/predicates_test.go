//go:build acs

package cycle1886

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/goalhash"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const preCycleHead = "cdc0a4083bb2788518b371f4ed7b85721bdc0b85"

var (
	buildOnce sync.Once
	builtBin  string
	buildOut  string
	buildErr  error
)

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "evolve-acs-1886-")
		if err != nil {
			buildErr = err
			return
		}
		builtBin = filepath.Join(dir, "evolve")
		cmd := exec.Command("go", "build", "-o", builtBin, "./cmd/evolve")
		cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
		out, err := cmd.CombinedOutput()
		buildOut, buildErr = string(out), err
	})
	if buildErr != nil {
		t.Fatalf("go build ./cmd/evolve failed: %v\n%s", buildErr, buildOut)
	}
	return builtBin
}

type result struct {
	stdout string
	stderr string
	code   int
}

func runLoopCLI(t *testing.T, args ...string) result {
	t.Helper()
	cmd := exec.Command(evolveBinary(t), append([]string{"loop"}, args...)...)
	cmd.Dir = t.TempDir()
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "EVOLVE_") {
			continue
		}
		cmd.Env = append(cmd.Env, kv)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("evolve loop did not run: %v", err)
		}
		code = ee.ExitCode()
	}
	return result{stdout.String(), stderr.String(), code}
}

type dryRunConfig struct {
	Config struct {
		GoalHash   string `json:"goal_hash"`
		GoalText   string `json:"goal_text"`
		GoalSource string `json:"goal_source"`
	} `json:"config"`
}

func parseDryRun(t *testing.T, r result) dryRunConfig {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("dry-run exit=%d want 0; stderr=%q", r.code, firstLine(r.stderr))
	}
	var out dryRunConfig
	if err := json.Unmarshal([]byte(r.stdout), &out); err != nil {
		t.Fatalf("dry-run stdout is not the JSON run config: %v\n%s", err, r.stdout)
	}
	return out
}

func projectWithRecordedGoal(t *testing.T, recorded *string) string {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if recorded != nil {
		if err := os.WriteFile(filepath.Join(evolveDir, "last-goal.txt"), []byte(*recorded), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func requireNotAnUnknownFlag(t *testing.T, r result) {
	t.Helper()
	if strings.Contains(r.stderr, "flag provided but not defined") {
		t.Errorf("refused as an unknown flag, not by goal-source validation: %q", firstLine(r.stderr))
	}
}

func TestC1886_001_LaunchRecordsTheGoalAtomicallyAndGoalFromLastReplaysIt(t *testing.T) {
	cmd := exec.Command("go", "test", "-count=1", "-v", "-run", "^TestLoopGoalSource_", "./cmd/evolve")
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	raw, err := cmd.CombinedOutput()
	out := string(raw)
	if err != nil {
		t.Errorf("go test -run ^TestLoopGoalSource_ ./cmd/evolve failed: %v", err)
	}
	for _, name := range []string{
		"TestLoopGoalSource_LaunchRecordsGoalTextThatGoalFromLastReplays",
		"TestLoopGoalSource_LaunchReplacesTheRecordAtomically",
		"TestLoopGoalSource_DryRunRecordsNothing",
		"TestLoopGoalSource_GoalFileResolvesTheFileText",
		"TestLoopGoalSource_PrintedConfigNamesEveryGoalSource",
		"TestLoopGoalSource_TwoGoalSourcesExit10",
		"TestLoopGoalSource_GoalFromLastWithNothingRecordedExits1",
		"TestLoopGoalSource_UnreadableGoalFileExits10",
	} {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass", name)
		}
	}
}

func TestC1886_002_GoalFromLastResolvesTheRecordedGoalHashThroughTheCLI(t *testing.T) {
	recorded := "Wave 104. Work the highest-weight lane items.\n"
	root := projectWithRecordedGoal(t, &recorded)

	out := parseDryRun(t, runLoopCLI(t, "--project-root", root, "--goal-from-last", "--dry-run"))

	want := goalhash.Compute(strings.TrimSpace(recorded))
	if out.Config.GoalHash != want {
		t.Errorf("goal_hash=%q want the recorded goal's hash %q", out.Config.GoalHash, want)
	}
	if out.Config.GoalSource != "goal-from-last" {
		t.Errorf("goal_source=%q want goal-from-last", out.Config.GoalSource)
	}
}

func TestC1886_003_GoalFileResolvesThroughTheCLIAndNamesItsSource(t *testing.T) {
	root := projectWithRecordedGoal(t, nil)
	goalFile := filepath.Join(t.TempDir(), "goal.txt")
	if err := os.WriteFile(goalFile, []byte("goal kept in a file\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := parseDryRun(t, runLoopCLI(t, "--project-root", root, "--goal-file", goalFile, "--dry-run"))

	if out.Config.GoalHash != goalhash.Compute("goal kept in a file") {
		t.Errorf("goal_hash=%q want %q", out.Config.GoalHash, goalhash.Compute("goal kept in a file"))
	}
	if out.Config.GoalSource != "goal-file" {
		t.Errorf("goal_source=%q want goal-file", out.Config.GoalSource)
	}
	typed := parseDryRun(t, runLoopCLI(t, "--project-root", root, "--goal-text", "typed goal", "--dry-run"))
	if typed.Config.GoalSource != "goal-text" {
		t.Errorf("--goal-text goal_source=%q want goal-text", typed.Config.GoalSource)
	}
}

func TestC1886_004_TwoGoalSourcesTogetherExit10(t *testing.T) {
	recorded := "recorded goal\n"
	root := projectWithRecordedGoal(t, &recorded)
	goalFile := filepath.Join(t.TempDir(), "goal.txt")
	if err := os.WriteFile(goalFile, []byte("file goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]string{
		"goal file with goal text":       {"--goal-file", goalFile, "--goal-text", "typed goal"},
		"goal from last with goal text":  {"--goal-from-last", "--goal-text", "typed goal"},
		"goal file with goal from last":  {"--goal-file", goalFile, "--goal-from-last"},
		"goal from last with positional": {"--goal-from-last", "positional goal"},
	}
	for name, extra := range cases {
		t.Run(name, func(t *testing.T) {
			r := runLoopCLI(t, append([]string{"--project-root", root, "--dry-run"}, extra...)...)
			if r.code != 10 {
				t.Errorf("exit=%d want 10; stderr=%q", r.code, firstLine(r.stderr))
			}
			requireNotAnUnknownFlag(t, r)
			if r.stdout != "" {
				t.Errorf("a refused launch printed a run config: %q", r.stdout)
			}
		})
	}
}

func TestC1886_005_GoalFromLastWithNothingRecordedExits1(t *testing.T) {
	blank := " \n"
	for name, recorded := range map[string]*string{"no record": nil, "blank record": &blank} {
		t.Run(name, func(t *testing.T) {
			root := projectWithRecordedGoal(t, recorded)
			r := runLoopCLI(t, "--project-root", root, "--goal-from-last", "--dry-run")
			if r.code != 1 {
				t.Errorf("exit=%d want 1; stderr=%q", r.code, firstLine(r.stderr))
			}
			requireNotAnUnknownFlag(t, r)
		})
	}
}

func TestC1886_006_BothFlagsAreDocumentedInTheLaunchStepAndTheHelp(t *testing.T) {
	r := runLoopCLI(t, "-h")
	for _, flagName := range []string{"-goal-file", "-goal-from-last"} {
		if !strings.Contains(r.stderr, flagName) {
			t.Errorf("evolve loop -h does not list %s", flagName)
		}
	}
	// acs-predicate: config-check
	doc, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "runtime-reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	var launchStep string
	for _, line := range strings.Split(string(doc), "\n") {
		if strings.Contains(line, "4. **Launch**") {
			launchStep = line
		}
	}
	if launchStep == "" {
		t.Fatal("runtime-reference.md has no launch step")
	}
	for _, flagName := range []string{"--goal-file", "--goal-from-last"} {
		if !strings.Contains(launchStep, flagName) {
			t.Errorf("the launch step of runtime-reference.md does not document %s", flagName)
		}
	}
}

func TestC1886_007_ChangedLoopFilesAddNoComments(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("git", "-C", root, "diff", "-U0", preCycleHead, "--",
		"go/cmd/evolve/cmd_loop_args.go", "go/cmd/evolve/cmd_loop.go", "go/cmd/evolve/cmd_loop_goal_source_test.go")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git diff: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		code := strings.TrimSpace(strings.TrimPrefix(line, "+"))
		if strings.HasPrefix(code, "//") || strings.HasPrefix(code, "/*") || strings.Contains(code, " // ") {
			t.Errorf("added line carries a comment: %q", code)
		}
	}
}
