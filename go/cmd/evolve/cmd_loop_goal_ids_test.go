package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func goalIDsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(filepath.Join(inbox, "consumed"), 0o755); err != nil {
		t.Fatal(err)
	}
	evolveDir := filepath.Join(root, ".evolve")
	writeInboxItem(t, inbox, "a.json", "alpha-ready", 0.5)
	writeInboxFile(t, evolveDir, "b.json", `{"id":"bravo-console","weight":0.9,"route":"console"}`)
	writeInboxFile(t, evolveDir, "c.json", `{"id":"delta-waits","weight":0.4,"deps":["alpha-ready"]}`)
	writeInboxFile(t, evolveDir, "f.json", `{"id":"foxtrot","weight":0.2,"route":"console"}`)
	writeInboxFile(t, evolveDir, "consumed/d.json", `{"id":"charlie-done","consumed":{"at":"2026-10-01T00:00:00Z","via":"pr-1"}}`)
	writeInboxFile(t, evolveDir, "consumed/s.json", `{"id":"cli-status","consumed":{"at":"2026-09-30T17:22:07Z","via":"ship"}}`)
	return root
}

func goalLaunch(root, goal string) loopConfig {
	return loopConfig{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve"), GoalText: goal, GoalSource: "goal-text"}
}

func checkGoal(cfg loopConfig) (int, string) {
	var stderr bytes.Buffer
	return refuseInertGoalItems(cfg, &stderr), stderr.String()
}

func runGoalLoop(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	return runLoop(append(args, "--dry-run"), nil, &stdout, &stderr), stderr.String()
}

func TestRunLoop_RefusesAGoalThatNamesItemsNoLaneCanTake(t *testing.T) {
	root := goalIDsFixture(t)

	code, stderr := runGoalLoop(t, "--project-root", root, "--goal-text", "work `bravo-console` and delta-waits")

	if code != exitGoalNamesInertItems {
		t.Fatalf("code=%d, want %d — a goal naming items no lane can take must fail at launch\nstderr=%s", code, exitGoalNamesInertItems, stderr)
	}
	for _, want := range []string{"GOAL_NAMES_INERT_ITEMS", "bravo-console: route:console", "delta-waits: deps unmet: needs alpha-ready"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}
}

func TestRunLoop_TheChainReExecOfTheSameGoalIsNotRefused(t *testing.T) {
	root := goalIDsFixture(t)
	writeHandoff(t, filepath.Join(root, ".evolve"), os.Getpid(), 0)

	code, stderr := runGoalLoop(t, "--project-root", root, "--goal-text", "work bravo-console")

	if code != 0 || strings.Contains(stderr, "GOAL_NAMES_INERT_ITEMS") {
		t.Fatalf("code=%d stderr=%s — a process that takes the chain re-exec handoff continues its run; its own goal must not refuse it", code, stderr)
	}
}

func TestRunLoop_AGoalFromLastReplayIsNotRefused(t *testing.T) {
	root := goalIDsFixture(t)
	if err := os.WriteFile(lastGoalPath(filepath.Join(root, ".evolve")), []byte("work bravo-console\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stderr := runGoalLoop(t, "--project-root", root, "--goal-from-last")

	if code != 0 || strings.Contains(stderr, "GOAL_NAMES_INERT_ITEMS") {
		t.Fatalf("code=%d stderr=%s — a --goal-from-last replay relaunches an earlier goal and must not be refused", code, stderr)
	}
}

func TestRunLoop_ABoundaryRelaunchIsNotRefused(t *testing.T) {
	root := goalIDsFixture(t)

	code, stderr := runGoalLoop(t, "--project-root", root, "--relaunch", "--goal-text", "work bravo-console")

	if code != 0 || strings.Contains(stderr, "GOAL_NAMES_INERT_ITEMS") {
		t.Fatalf("code=%d stderr=%s — the boundary relaunch of a goal must not be refused", code, stderr)
	}
}

func TestBoundaryLaunchSteps_TheLoopStepIsARelaunch(t *testing.T) {
	t.Parallel()
	steps := boundaryLaunchSteps(boundaryArgs{goalText: "g", runID: "r"}, "/p")

	last := steps[len(steps)-1]
	if last.verb != "loop" || !strings.Contains(strings.Join(last.args, " "), "--relaunch") {
		t.Fatalf("the boundary loop step must pass --relaunch, got %s %v", last.verb, last.args)
	}
}

func TestRefuseInertGoalItems_ProseThatMentionsAConsumedIDOnlyWarns(t *testing.T) {
	t.Parallel()
	root := goalIDsFixture(t)

	code, stderr := checkGoal(goalLaunch(root, "Make the cli-status output shorter and fix the follow-up wording."))

	if code != 0 {
		t.Fatalf("code=%d stderr=%s — ordinary prose that names a consumed id must not refuse the launch", code, stderr)
	}
	if !strings.Contains(stderr, "WARN: the goal names a consumed item, which no lane takes: cli-status: consumed at 2026-09-30T17:22:07Z via ship") {
		t.Fatalf("a consumed id gets a WARN, got %q", stderr)
	}
}

func TestRefuseInertGoalItems_AGoalThatNamesASelectableItemWarnsThatItCannotExcludeIt(t *testing.T) {
	t.Parallel()
	root := goalIDsFixture(t)

	code, stderr := checkGoal(goalLaunch(root, "EXCLUDE alpha-ready; mention bravo-consoles, xcharlie-done and the foxtrot word"))

	if code != 0 {
		t.Fatalf("code=%d stderr=%s — a selectable id, a longer token that only contains an id, or a plain word that equals a one-word id must not refuse the launch", code, stderr)
	}
	if !strings.Contains(stderr, "goal text does not select or exclude inbox items; the lanes can take alpha-ready") || !strings.Contains(stderr, "evolve inbox route-console <id> <reason> <cycle>") {
		t.Fatalf("a named selectable item must WARN and name the verb that keeps lanes off it, got %q", stderr)
	}
	if strings.Contains(stderr, "consumed item") {
		t.Fatalf("a token that only contains a consumed id must not match it: %q", stderr)
	}
}

func TestRefuseInertGoalItems_OnlyAnOperatorLaunchIsChecked(t *testing.T) {
	t.Parallel()
	root := goalIDsFixture(t)
	for name, mutate := range map[string]func(*loopConfig){
		"resume":         func(c *loopConfig) { c.Resume = true },
		"handed off":     func(c *loopConfig) { c.HandedOff = true },
		"relaunch":       func(c *loopConfig) { c.Relaunch = true },
		"goal-from-last": func(c *loopConfig) { c.GoalSource = "goal-from-last" },
		"empty goal":     func(c *loopConfig) { c.GoalText = "" },
	} {
		cfg := goalLaunch(root, "work bravo-console")
		mutate(&cfg)
		if code, stderr := checkGoal(cfg); code != 0 || stderr != "" {
			t.Errorf("%s: code=%d stderr=%q — the check runs only on an operator-initiated fresh launch", name, code, stderr)
		}
	}
}

func TestNamedGoalItems_ReportsAnUnreadableInbox(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(filepath.Dir(inbox), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inbox, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := namedGoalItems(root, filepath.Join(root, ".evolve"), "work alpha-ready", &bytes.Buffer{})

	if err == nil || !strings.Contains(err.Error(), inbox) {
		t.Fatalf("err=%v — an unreadable inbox must be an error that names the inbox, not an empty verdict", err)
	}
}

func TestRefuseInertGoalItems_AnUnreadableInboxWarnsAndLaunches(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stderr := checkGoal(goalLaunch(root, "work alpha-ready"))

	if code != 0 || !strings.Contains(stderr, "cannot check the items the goal names") {
		t.Fatalf("code=%d stderr=%s — an unreadable inbox must warn, not block the launch", code, stderr)
	}
}

func TestRefuseInertGoalItems_ARefiledItemIsJudgedByItsPendingCopy(t *testing.T) {
	t.Parallel()
	root := goalIDsFixture(t)
	evolveDir := filepath.Join(root, ".evolve")
	writeInboxFile(t, evolveDir, "e.json", `{"id":"echo-refiled","weight":0.3,"created_at":"2026-10-02T00:00:00Z"}`)
	writeInboxFile(t, evolveDir, "consumed/e.json", `{"id":"echo-refiled","created_at":"2026-09-01T00:00:00Z","consumed":{"at":"2026-09-02T00:00:00Z","via":"pr-2"}}`)

	code, stderr := checkGoal(goalLaunch(root, "work echo-refiled"))

	if code != 0 || strings.Contains(stderr, "consumed item") {
		t.Fatalf("code=%d stderr=%s — a refiled item is judged by its pending copy, not by its older consumed record", code, stderr)
	}
}

func TestRunLoop_ADetachedLaunchIsCheckedBeforeItDetaches(t *testing.T) {
	root := goalIDsFixture(t)

	var stdout, errBuf bytes.Buffer
	code := runLoop([]string{"--project-root", root, "--preflight-only", "--goal-text", "work bravo-console"}, nil, &stdout, &errBuf)
	stderr := errBuf.String()

	if code != exitGoalNamesInertItems || !strings.Contains(stderr, "GOAL_NAMES_INERT_ITEMS") {
		t.Fatalf("code=%d stderr=%s — the front door (--preflight-only, --detach) checks the goal before it runs", code, stderr)
	}
}

func TestRelaunchArgv_TheChainReExecMarksItselfARelaunch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in, want []string
	}{
		{[]string{"evolve", "loop", "--goal-text", "g", "my goal"}, []string{"evolve", "loop", "--relaunch", "--goal-text", "g", "my goal"}},
		{[]string{"evolve", "loop", "--relaunch", "--goal-text", "g"}, []string{"evolve", "loop", "--relaunch", "--goal-text", "g"}},
		{[]string{"evolve", "cycle", "run"}, []string{"evolve", "cycle", "run"}},
		{[]string{"evolve"}, []string{"evolve"}},
	} {
		if got := relaunchArgv(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("relaunchArgv(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseLoopArgs_TheDetachedChildIsARelaunch(t *testing.T) {
	t.Parallel()
	cfg, rc := parseLoopArgs([]string{"--detach", "--log", "l.log", "--goal-text", "work bravo-console"}, &bytes.Buffer{})

	if rc != 0 || len(cfg.DetachArgv) == 0 || cfg.DetachArgv[0] != "--relaunch" {
		t.Fatalf("rc=%d argv=%q — the parent checked the goal; the detached child must not check it again", rc, cfg.DetachArgv)
	}
}

func TestChainReExecArgv_TheDefaultReExecIsARelaunch(t *testing.T) {
	prev := os.Args
	t.Cleanup(func() { os.Args = prev })
	os.Args = []string{"evolve", "loop", "--goal-text", "work bravo-console"}

	if got := strings.Join(chainReExecArgvFn(), " "); got != "evolve loop --relaunch --goal-text work bravo-console" {
		t.Fatalf("the chain re-exec argv must carry --relaunch, got %q", got)
	}
}
