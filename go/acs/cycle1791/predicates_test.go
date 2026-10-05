//go:build acs

package cycle1791

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	diskFloorHaltPolicy = `{"preflight":{"min_free_gib":1000000000},"boot":{"binary_refresh":"off"}}`
	refreshOffPolicy    = `{"boot":{"binary_refresh":"off"}}`
	unfinishedCycleJunk = "{not-json"
	liveStatusRunID     = "run-status-live"
)

var (
	buildOnce sync.Once
	binDir    string
	binPath   string
	buildOut  string
	buildErr  error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if binDir != "" {
		os.RemoveAll(binDir)
	}
	os.Exit(code)
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		if binDir, buildErr = os.MkdirTemp("", "cycle1791-evolve-"); buildErr != nil {
			return
		}
		binPath = filepath.Join(binDir, "evolve")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./cmd/evolve")
		cmd.Dir = goDir(t)
		out, err := cmd.CombinedOutput()
		buildOut, buildErr = string(out), err
	})
	if buildErr != nil {
		t.Fatalf("go build ./cmd/evolve: %v\n%s", buildErr, buildOut)
	}
	return binPath
}

type evolveEnv struct {
	emptyPath  bool
	pluginRoot string
}

func isolatedEnv(t *testing.T, root string, opts evolveEnv) []string {
	t.Helper()
	tmuxDir, err := os.MkdirTemp("/tmp", "c1791tmux")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmuxDir) })
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "EVOLVE_") || name == "TMUX" || name == "TMUX_PANE" || name == "TMUX_TMPDIR" {
			continue
		}
		if name == "PATH" && opts.emptyPath {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "EVOLVE_PROJECT_ROOT="+root, "TMUX_TMPDIR="+tmuxDir)
	if opts.emptyPath {
		env = append(env, "PATH="+t.TempDir())
	}
	if opts.pluginRoot != "" {
		env = append(env, "EVOLVE_PLUGIN_ROOT="+opts.pluginRoot)
	}
	return env
}

type evolveRun struct {
	stdout, stderr string
	code           int
}

func (r evolveRun) String() string {
	return fmt.Sprintf("exit=%d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
}

func runEvolve(t *testing.T, root string, opts evolveEnv, args ...string) evolveRun {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBinary(t), args...)
	cmd.Dir = root
	cmd.Env = isolatedEnv(t, root, opts)
	cmd.WaitDelay = 5 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	run := evolveRun{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		run.code = exitErr.ExitCode()
	default:
		t.Fatalf("run evolve %v: %v", args, err)
	}
	return run
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newProject(t *testing.T, policy string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if policy != "" {
		writeFile(t, filepath.Join(root, ".evolve", "policy.json"), policy)
	}
	return root
}

func requireUndispatched(t *testing.T, root string) {
	t.Helper()
	evolveDir := filepath.Join(root, ".evolve")
	for _, name := range []string{"state.json", "ledger.jsonl", "worktrees"} {
		if _, err := os.Stat(filepath.Join(evolveDir, name)); err == nil {
			t.Errorf("a non-dispatching mode created .evolve/%s", name)
		}
	}
	_ = filepath.WalkDir(evolveDir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == ".lease" {
			t.Errorf("a non-dispatching mode wrote a run lease at %s", path)
		}
		return nil
	})
}

func blockingLines(stderr string) string {
	var out []string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, "blocking") {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

type persistedGate struct {
	OverallLevel string `json:"overall_level"`
	Checks       []struct {
		Name  string `json:"name"`
		Level string `json:"level"`
	} `json:"checks"`
}

func readPersistedGate(t *testing.T, root string) persistedGate {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "loop-preflight.json"))
	if err != nil {
		t.Fatalf("--preflight-only must persist the real gate's result to .evolve/loop-preflight.json: %v", err)
	}
	var g persistedGate
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatalf("loop-preflight.json: %v\n%s", err, raw)
	}
	return g
}

func requireUnitTestsPass(t *testing.T, names ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "-run", "^("+strings.Join(names, "|")+")$", "./cmd/evolve")
	cmd.Dir = goDir(t)
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	for _, name := range names {
		if !regexp.MustCompile(`(?m)^--- PASS: ` + regexp.QuoteMeta(name) + ` \(`).Match(out) {
			t.Errorf("%s did not PASS", name)
		}
	}
	if err != nil || t.Failed() {
		t.Errorf("go test ./cmd/evolve -run %v: %v\n%s", names, err, out)
	}
}

func runtimeReference(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "runtime-reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func launchStep(t *testing.T) string {
	t.Helper()
	for _, line := range strings.Split(runtimeReference(t), "\n") {
		if strings.Contains(line, "**Launch**") {
			return line
		}
	}
	t.Fatalf("runtime-reference.md has no wave-boundary **Launch** step")
	return ""
}

func documentedLoopCommand(step, flag string) []string {
	for _, span := range regexp.MustCompile("`([^`]*)`").FindAllStringSubmatch(step, -1) {
		fields := strings.Fields(span[1])
		if len(fields) < 2 || fields[0] != "evolve" || fields[1] != "loop" || !strings.Contains(span[1], flag) {
			continue
		}
		var args []string
		for _, f := range fields[1:] {
			if strings.HasPrefix(f, "[") || strings.HasSuffix(f, "]") {
				continue
			}
			if f == "N" {
				f = "1"
			}
			args = append(args, f)
		}
		return args
	}
	return nil
}

func TestC1791_001_PreflightOnlyRunsTheRealGateAndExitsOneNamingTheBlockingCheck(t *testing.T) {
	root := newProject(t, diskFloorHaltPolicy)
	run := runEvolve(t, root, evolveEnv{emptyPath: true}, "loop", "--preflight-only", "--skip-preflight-boot", "--project-root", root)
	if strings.Contains(run.stderr, "flag provided but not defined") {
		t.Fatalf("evolve loop does not define --preflight-only:\n%s", run)
	}
	if run.code != 1 {
		t.Fatalf("a halting readiness gate under --preflight-only must exit 1:\n%s", run)
	}
	if !strings.Contains(blockingLines(run.stderr), "disk-space") {
		t.Errorf("stderr must name the halting check disk-space on a blocking line:\n%s", run)
	}
	gate := readPersistedGate(t, root)
	halted := false
	for _, c := range gate.Checks {
		halted = halted || (c.Name == "disk-space" && c.Level == "halt")
	}
	if gate.OverallLevel != "halt" || !halted {
		t.Errorf("the persisted gate must record disk-space at halt (the real gate ran): %+v", gate)
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "cycle-state.json")); err == nil {
		t.Errorf("--preflight-only wrote cycle-state.json")
	}
	requireUndispatched(t, root)
}

func TestC1791_002_PreflightOnlyExitCodeAgreesWithTheRealGatesVerdict(t *testing.T) {
	root := newProject(t, refreshOffPolicy)
	cycleState := filepath.Join(root, ".evolve", "cycle-state.json")
	writeFile(t, cycleState, unfinishedCycleJunk)
	run := runEvolve(t, root, evolveEnv{pluginRoot: acsassert.RepoRoot(t)}, "loop", "--preflight-only", "--skip-preflight-boot", "--project-root", root)
	if run.code != 0 && run.code != 1 {
		t.Fatalf("--preflight-only exits 0 (ready) or 1 (blocked), never a launch code:\n%s", run)
	}
	gate := readPersistedGate(t, root)
	if gate.OverallLevel == "halt" {
		if run.code != 1 || strings.Contains(run.stdout, "READY") {
			t.Errorf("a halted gate must exit 1 without READY:\n%s", run)
		}
		for _, c := range gate.Checks {
			if c.Level == "halt" && !strings.Contains(blockingLines(run.stderr), c.Name) {
				t.Errorf("halting check %q is not named on a blocking stderr line:\n%s", c.Name, run)
			}
		}
	} else {
		if run.code != 0 || !strings.Contains(run.stdout, "READY") {
			t.Errorf("a gate without a halt must exit 0 and print READY:\n%s", run)
		}
		for _, c := range gate.Checks {
			if !strings.Contains(run.stdout, c.Name) {
				t.Errorf("stdout must print check %q's verdict:\n%s", c.Name, run)
			}
		}
	}
	if raw, err := os.ReadFile(cycleState); err != nil || string(raw) != unfinishedCycleJunk {
		t.Errorf("--preflight-only touched cycle-state.json (err=%v): %q", err, raw)
	}
	requireUndispatched(t, root)
}

func TestC1791_003_PreflightOnlyRejectsModesThatCheckNothingOrDispatch(t *testing.T) {
	for name, extra := range map[string][]string{
		"skip-preflight": {"--skip-preflight"},
		"dry-run":        {"--dry-run"},
		"detach":         {"--detach", "--log", "detach.log"},
	} {
		t.Run(name, func(t *testing.T) {
			root := newProject(t, diskFloorHaltPolicy)
			args := append([]string{"loop", "--preflight-only", "--project-root", root, "--goal-text", "g"}, extra...)
			run := runEvolve(t, root, evolveEnv{emptyPath: true}, args...)
			if run.code != 10 || strings.Contains(run.stderr, "flag provided but not defined") || !strings.Contains(run.stderr, "mutually exclusive") {
				t.Fatalf("--preflight-only with %v must exit 10 as mutually exclusive:\n%s", extra, run)
			}
			for _, never := range []string{filepath.Join(root, ".evolve", "loop-preflight.json"), filepath.Join(root, "detach.log")} {
				if _, err := os.Stat(never); err == nil {
					t.Errorf("a rejected combination ran something: %s exists", never)
				}
			}
		})
	}
}

func TestC1791_004_LaunchStepDocumentsPreflightOnlyAsARunnableCheck(t *testing.T) {
	args := documentedLoopCommand(launchStep(t), "--preflight-only")
	if args == nil {
		t.Fatalf("the Launch step names no `evolve loop ... --preflight-only` command:\n%s", launchStep(t))
	}
	root := newProject(t, diskFloorHaltPolicy)
	run := runEvolve(t, root, evolveEnv{emptyPath: true}, append(args, "--skip-preflight-boot", "--project-root", root)...)
	if run.code != 1 || !strings.Contains(blockingLines(run.stderr), "disk-space") {
		t.Errorf("the documented `evolve %s` must run the readiness gate and exit 1 naming the blocking check:\n%s", strings.Join(args, " "), run)
	}
	requireUndispatched(t, root)
}

func TestC1791_005_PreflightOnlyUnitContractPasses(t *testing.T) {
	requireUnitTestsPass(t,
		"TestLoopPreflightOnly_PassingGateExitsZeroReadyWithoutDispatch",
		"TestLoopPreflightOnly_HaltingGateExitsOneNamingEachBlockingCheck",
		"TestLoopPreflightOnly_LeavesTheBoundaryReexecHandoffForTheLoop",
		"TestLoopPreflightOnly_ConflictingModesExitTenWithoutRunningTheGate",
	)
}

func TestC1791_010_DetachFlagCombinationsAreRejectedBeforeAnythingLaunches(t *testing.T) {
	cases := []struct {
		name, mention string
		args          func(root, log string) []string
	}{
		{"detach without log", "--log", func(root, _ string) []string {
			return []string{"loop", "--detach", "--goal-text", "g", "--project-root", root}
		}},
		{"log without detach", "--detach", func(root, log string) []string {
			return []string{"loop", "--log", log, "--goal-text", "g", "--project-root", root}
		}},
		{"detach with dry-run", "mutually exclusive", func(root, log string) []string {
			return []string{"loop", "--detach", "--log", log, "--dry-run", "--goal-text", "g", "--project-root", root}
		}},
		{"detach without a goal", "goal", func(root, log string) []string {
			return []string{"loop", "--detach", "--log", log, "--project-root", root}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newProject(t, refreshOffPolicy)
			log := filepath.Join(root, "detach.log")
			run := runEvolve(t, root, evolveEnv{emptyPath: true}, tc.args(root, log)...)
			if run.code != 10 || strings.Contains(run.stderr, "flag provided but not defined") || !strings.Contains(run.stderr, tc.mention) {
				t.Fatalf("%s must exit 10 mentioning %q:\n%s", tc.name, tc.mention, run)
			}
			if _, err := os.Stat(log); err == nil {
				t.Errorf("a rejected --detach opened its log: nothing may launch")
			}
			requireUndispatched(t, root)
		})
	}
}

func TestC1791_011_DetachUnitContractPasses(t *testing.T) {
	requireUnitTestsPass(t,
		"TestLoopDetach_ChildThatTakesTheRunLeaseExitsZeroNamingItsPidAndLog",
		"TestLoopDetach_ChildExitingDuringBootExitsOneWithThisLaunchsLogTail",
		"TestLoopDetach_RefusesToLaunchBesideALiveRun",
		"TestLoopDetach_BootUnconfirmedWithinThePolicyWaitExitsOneLeavingTheChildRunning",
		"TestLoopDetach_ChildArgvIsTheParentsWithoutTheDetachFlags",
		"TestLoopDetach_ChildOutlivesItsExitedParentInItsOwnSession",
	)
}

func TestC1791_012_LaunchStepDocumentsDetachWithALog(t *testing.T) {
	step := launchStep(t)
	args := documentedLoopCommand(step, "--detach")
	if args == nil || !strings.Contains(strings.Join(args, " "), "--log") {
		t.Fatalf("the Launch step names no `evolve loop ... --detach --log F` command:\n%s", step)
	}
	root := newProject(t, refreshOffPolicy)
	run := runEvolve(t, root, evolveEnv{emptyPath: true}, append(args, "--dry-run", "--project-root", root)...)
	if run.code != 10 || strings.Contains(run.stderr, "flag provided but not defined") || !strings.Contains(run.stderr, "mutually exclusive") {
		t.Errorf("the documented `evolve %s` must be a real --detach command (with --dry-run it is refused as mutually exclusive):\n%s", strings.Join(args, " "), run)
	}
	requireUndispatched(t, root)
}

func liveLoopProject(t *testing.T) string {
	t.Helper()
	root := newProject(t, refreshOffPolicy)
	writeFile(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"cycle_id":7,"phase":"build"}`)
	writeFile(t, filepath.Join(root, ".evolve", "loop-stop"), "2026-10-05T00:00:00Z\n")
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-7")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: liveStatusRunID, OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	return root
}

func treeSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		out[rel] = fmt.Sprintf("%v %d %d", d.IsDir(), info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func requireTreeUnchanged(t *testing.T, before, after map[string]string) {
	t.Helper()
	for path, stamp := range after {
		if before[path] != stamp {
			t.Errorf("a read-only status changed %s (%q -> %q)", path, before[path], stamp)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			t.Errorf("a read-only status removed %s", path)
		}
	}
}

func TestC1791_020_LoopStatusReportsTheLoopAndStartsNothing(t *testing.T) {
	root := liveLoopProject(t)
	before := treeSnapshot(t, root)
	run := runEvolve(t, root, evolveEnv{emptyPath: true}, "loop", "status", "--project-root", root)
	if run.code != 0 {
		t.Fatalf("evolve loop status must exit 0 on a readable plane:\n%s", run)
	}
	lines := strings.Split(run.stdout, "\n")
	if lines[0] != "loop:    running=true brake=true cycle=7 phase=build" {
		t.Errorf("line 1 must be evolve status's loop line for the live cycle 7:\n%s", run)
	}
	if len(lines) < 2 || !strings.HasPrefix(lines[1], "lease:   heartbeat=") || strings.Contains(lines[1], "none") {
		t.Errorf("line 2 must carry the live lease heartbeat:\n%s", run)
	}
	requireTreeUnchanged(t, before, treeSnapshot(t, root))
}

func TestC1791_021_LoopStatusJSONCarriesTheLoopObject(t *testing.T) {
	root := liveLoopProject(t)
	run := runEvolve(t, root, evolveEnv{emptyPath: true}, "loop", "status", "--json", "--project-root", root)
	if run.code != 0 {
		t.Fatalf("evolve loop status --json must exit 0:\n%s", run)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(run.stdout), &doc); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, run)
	}
	var loop struct {
		Running      bool   `json:"running"`
		BrakeEngaged bool   `json:"brake_engaged"`
		CycleID      int    `json:"cycle_id"`
		Phase        string `json:"phase"`
	}
	if err := json.Unmarshal(doc["loop"], &loop); err != nil || !loop.Running || !loop.BrakeEngaged || loop.CycleID != 7 || loop.Phase != "build" {
		t.Errorf(".loop must report the live cycle 7 at build with the brake (err=%v): %+v\n%s", err, loop, run)
	}
	for _, remote := range []string{"prs", "ci"} {
		if _, ok := doc[remote]; ok {
			t.Errorf("evolve loop status reports only the loop; it carries %q", remote)
		}
	}
}

func TestC1791_022_LoopStatusRefusesAMissingSnapshotAndStrayArguments(t *testing.T) {
	root := t.TempDir()
	notADir := filepath.Join(root, ".evolve")
	writeFile(t, notADir, "a file where .evolve should be\n")
	run := runEvolve(t, root, evolveEnv{emptyPath: true}, "loop", "status", "--project-root", root)
	if run.code != 2 || !strings.Contains(run.stderr, "cannot read the snapshot") {
		t.Errorf("evolve loop status without a .evolve directory must exit 2 saying the snapshot is unreadable:\n%s", run)
	}
	if info, err := os.Stat(notADir); err != nil || info.IsDir() {
		t.Errorf("evolve loop status must not create or replace .evolve (err=%v)", err)
	}
	live := liveLoopProject(t)
	run = runEvolve(t, live, evolveEnv{emptyPath: true}, "loop", "status", "--json", "extra")
	if run.code != 10 || !strings.Contains(run.stderr, `unexpected argument "extra"`) {
		t.Errorf("evolve loop status --json extra must exit 10 naming the stray argument:\n%s", run)
	}
}

func TestC1791_023_ReservedWordGoalsExitTenNamingTheIntendedVerb(t *testing.T) {
	cases := []struct {
		goal []string
		verb string
	}{
		{[]string{"stop"}, "evolve loop-stop"},
		{[]string{"3", "STATUS"}, "evolve loop status"},
		{[]string{"help"}, "evolve loop -h"},
		{[]string{"plan"}, "evolve loop --dry-run"},
		{[]string{" Watch "}, "evolve dashboard"},
	}
	for _, tc := range cases {
		root := newProject(t, refreshOffPolicy)
		args := append([]string{"loop", "--dry-run", "--project-root", root}, tc.goal...)
		run := runEvolve(t, root, evolveEnv{emptyPath: true}, args...)
		if run.code != 10 || !strings.Contains(run.stderr, tc.verb) {
			t.Errorf("positional goal %q must exit 10 naming %q:\n%s", tc.goal, tc.verb, run)
		}
	}
	root := newProject(t, refreshOffPolicy)
	writeFile(t, filepath.Join(root, ".evolve", "cycle-state.json"), unfinishedCycleJunk)
	run := runEvolve(t, root, evolveEnv{emptyPath: true}, "loop", "--project-root", root, "stop")
	if run.code != 10 || !strings.Contains(run.stderr, "evolve loop-stop") {
		t.Errorf("evolve loop stop must exit 10 naming evolve loop-stop:\n%s", run)
	}
	requireUndispatched(t, root)
}

func TestC1791_024_MultiWordAndExplicitGoalsStillLaunchAsBefore(t *testing.T) {
	cases := []struct {
		args []string
		goal string
	}{
		{[]string{"status", "of", "the", "fleet"}, "status of the fleet"},
		{[]string{"3", "balanced", "stop", "the", "leak"}, "stop the leak"},
		{[]string{"--goal-text", "status"}, "status"},
	}
	for _, tc := range cases {
		root := newProject(t, refreshOffPolicy)
		args := append([]string{"loop", "--dry-run", "--project-root", root}, tc.args...)
		run := runEvolve(t, root, evolveEnv{emptyPath: true}, args...)
		var out struct {
			Config struct {
				GoalText string `json:"goal_text"`
			} `json:"config"`
		}
		if run.code != 0 || json.Unmarshal([]byte(run.stdout), &out) != nil || out.Config.GoalText != tc.goal {
			t.Errorf("evolve loop --dry-run %q must still resolve goal %q:\n%s", tc.args, tc.goal, run)
		}
	}
}

func TestC1791_025_OperatorCommandsDocumentLoopStatus(t *testing.T) {
	doc := runtimeReference(t)
	start := strings.Index(doc, "## Operator commands")
	if start < 0 {
		t.Fatalf("runtime-reference.md has no ## Operator commands section")
	}
	section := doc[start:]
	if end := strings.Index(section[len("## Operator commands"):], "\n## "); end >= 0 {
		section = section[:len("## Operator commands")+end]
	}
	if !strings.Contains(section, "`evolve loop status") {
		t.Fatalf("the Operator commands section must document `evolve loop status`")
	}
	root := liveLoopProject(t)
	run := runEvolve(t, root, evolveEnv{emptyPath: true}, "loop", "status", "--project-root", root)
	if run.code != 0 || !strings.HasPrefix(run.stdout, "loop:    running=true") {
		t.Errorf("the documented evolve loop status must report the running loop:\n%s", run)
	}
}
