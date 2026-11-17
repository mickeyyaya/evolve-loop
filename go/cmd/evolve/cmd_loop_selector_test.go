package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

func selectorFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(filepath.Join(evolveDir, "inbox", "consumed"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, it := range []struct {
		id     string
		weight string
	}{{"alpha-ready", "0.9"}, {"bravo-ready", "0.8"}, {"charlie-ready", "0.7"}, {"delta-ready", "0.6"}} {
		writeInboxFile(t, evolveDir, it.id+".json", `{"id":"`+it.id+`","weight":`+it.weight+`,"files":["pkg/`+it.id+`/x.go"]}`)
	}
	writeInboxFile(t, evolveDir, "echo.json", `{"id":"echo-console","weight":0.95,"priority_class":"feature","route":"console","files":["pkg/echo-console/x.go"]}`)
	writeInboxFile(t, evolveDir, "consumed/f.json", `{"id":"foxtrot-done","consumed":{"at":"2026-10-01T00:00:00Z","via":"pr-1"}}`)
	return root
}

func selectorLaunch(root string, extra ...string) []string {
	return append([]string{"--project-root", root, "--goal-text", "work the backlog"}, extra...)
}

func selectorDryRun(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	return runLoop(append(args, "--dry-run"), nil, &stdout, &stderr), clipSelectorStderr(stderr.String())
}

func clipSelectorStderr(s string) string {
	const limit = 2000
	if len(s) > limit {
		return s[:limit] + "…"
	}
	return s
}

func selectorPlan(t *testing.T, args []string) []string {
	t.Helper()
	var stderr bytes.Buffer
	cfg, rc := parseLoopArgs(args, &stderr)
	if rc != 0 {
		t.Fatalf("parseLoopArgs(%q) rc=%d, want 0\n%s", args, rc, clipSelectorStderr(stderr.String()))
	}
	data, _, err := productionWavePlanFn(cfg, &erroringReadStorage{}, 2, io.Discard)(context.Background(), 0)
	if err != nil {
		t.Fatalf("plan for %q: %v", args, err)
	}
	return plannedTopNIDs(t, data)
}

func plannedTopNIDs(t *testing.T, data []byte) []string {
	t.Helper()
	var d struct {
		TopN []struct {
			ID string `json:"id"`
		} `json:"top_n"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("plan is not a triage decision: %v\n%s", err, data)
	}
	var ids []string
	for _, c := range d.TopN {
		ids = append(ids, c.ID)
	}
	sort.Strings(ids)
	return ids
}

func argvFlagValue(argv []string, name string) (string, bool) {
	for i, a := range argv {
		if n, hasValue := flagTokenName(a); n == name {
			if hasValue {
				_, v, _ := strings.Cut(a, "=")
				return v, true
			}
			if i+1 < len(argv) {
				return argv[i+1], true
			}
		}
	}
	return "", false
}

func selectorFingerprint(t *testing.T, dir string) map[string][sha256.Size]byte {
	t.Helper()
	out := map[string][sha256.Size]byte{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if d.IsDir() {
			out[rel+"/"] = [sha256.Size]byte{}
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		out[rel+"@"+info.ModTime().String()] = sha256.Sum256(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSelectorFlags_ExcludeNeverAssignsAnExcludedIDToALane(t *testing.T) {
	root := selectorFixture(t)
	for name, tc := range map[string]struct {
		flags []string
		want  []string
	}{
		"one id":            {[]string{"--exclude", "alpha-ready"}, []string{"bravo-ready", "charlie-ready"}},
		"equals form":       {[]string{"--exclude=alpha-ready"}, []string{"bravo-ready", "charlie-ready"}},
		"comma separated":   {[]string{"--exclude", "alpha-ready,bravo-ready"}, []string{"charlie-ready", "delta-ready"}},
		"a lower-ranked id": {[]string{"--exclude", "bravo-ready"}, []string{"alpha-ready", "charlie-ready"}},
	} {
		if got := selectorPlan(t, selectorLaunch(root, tc.flags...)); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: wave plan = %v, want %v", name, got, tc.want)
		}
	}
}

func TestSelectorFlags_OnlyAssignsOnlyTheListedIDs(t *testing.T) {
	root := selectorFixture(t)
	for name, tc := range map[string]struct {
		flags []string
		want  []string
	}{
		"two low-ranked ids": {[]string{"--only", "charlie-ready,delta-ready"}, []string{"charlie-ready", "delta-ready"}},
		"equals form":        {[]string{"--only=charlie-ready,delta-ready"}, []string{"charlie-ready", "delta-ready"}},
		"a list of one":      {[]string{"--only", "delta-ready"}, []string{"delta-ready"}},
	} {
		if got := selectorPlan(t, selectorLaunch(root, tc.flags...)); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: wave plan = %v, want %v", name, got, tc.want)
		}
	}
}

func TestSelectorFlags_OnlyAndExcludeTogetherNarrowTheListedIDs(t *testing.T) {
	root := selectorFixture(t)

	got := selectorPlan(t, selectorLaunch(root, "--only", "alpha-ready,bravo-ready,charlie-ready", "--exclude", "alpha-ready"))

	if want := []string{"bravo-ready", "charlie-ready"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("wave plan = %v, want %v", got, want)
	}
}

func TestSelectorFlags_AnOnlyIDNoLaneCanTakeExits10AtLaunch(t *testing.T) {
	root := selectorFixture(t)
	for name, tc := range map[string]struct {
		only   string
		reason string
	}{
		"console-routed":                {"echo-console", "echo-console: route:console"},
		"already consumed":              {"foxtrot-done", "foxtrot-done"},
		"not in the inbox":              {"zulu-missing", "zulu-missing"},
		"one inert id among ready ones": {"alpha-ready,echo-console", "echo-console: route:console"},
	} {
		code, stderr := selectorDryRun(t, selectorLaunch(root, "--only", tc.only)...)
		if code != exitGoalNamesInertItems {
			t.Errorf("%s: code=%d, want %d\n%s", name, code, exitGoalNamesInertItems, stderr)
		}
		for _, want := range []string{"GOAL_NAMES_INERT_ITEMS", tc.reason} {
			if !strings.Contains(stderr, want) {
				t.Errorf("%s: stderr lacks %q:\n%s", name, want, stderr)
			}
		}
	}
}

func TestSelectorFlags_TheOnlyCheckRunsWhateverTheGoalText(t *testing.T) {
	root := selectorFixture(t)

	code, stderr := selectorDryRun(t, "--project-root", root, "--only", "echo-console")

	if code != exitGoalNamesInertItems || !strings.Contains(stderr, "GOAL_NAMES_INERT_ITEMS") || !strings.Contains(stderr, "echo-console: route:console") {
		t.Fatalf("code=%d stderr=%s — a launch with no goal text still validates --only", code, stderr)
	}
}

func TestSelectorFlags_TheOnlyCheckRunsAtTheFrontDoor(t *testing.T) {
	root := selectorFixture(t)
	var stdout, stderr bytes.Buffer

	code := runLoop(selectorLaunch(root, "--only", "echo-console", "--preflight-only"), nil, &stdout, &stderr)

	if code != exitGoalNamesInertItems || !strings.Contains(stderr.String(), "GOAL_NAMES_INERT_ITEMS") {
		t.Fatalf("code=%d stderr=%s — --preflight-only and --detach validate --only before they run", code, clipSelectorStderr(stderr.String()))
	}
}

func TestSelectorFlags_OnlyIDsTheLanesCanTakeLaunch(t *testing.T) {
	root := selectorFixture(t)

	code, stderr := selectorDryRun(t, selectorLaunch(root, "--only", "alpha-ready,bravo-ready", "--exclude", "charlie-ready")...)

	if code != 0 || strings.Contains(stderr, "GOAL_NAMES_INERT_ITEMS") {
		t.Fatalf("code=%d stderr=%s — every --only id is takeable, so the launch proceeds", code, stderr)
	}
}

func TestSelectorFlags_AnIDInBothListsExits10NamingTheID(t *testing.T) {
	root := selectorFixture(t)

	code, stderr := selectorDryRun(t, selectorLaunch(root, "--only", "alpha-ready,bravo-ready", "--exclude", "charlie-ready,bravo-ready")...)

	if code != exitGoalNamesInertItems || !strings.Contains(stderr, "bravo-ready") {
		t.Fatalf("code=%d stderr=%s — an id in --only and --exclude exits 10 and names the id", code, stderr)
	}
	for _, innocent := range []string{"alpha-ready", "charlie-ready"} {
		if strings.Contains(stderr, innocent) {
			t.Errorf("stderr names %q, which is in only one list:\n%s", innocent, stderr)
		}
	}
}

func TestSelectorFlags_ARelaunchedLaunchIsNotCheckedAgainstItemsItAlreadyShipped(t *testing.T) {
	root := selectorFixture(t)

	code, stderr := selectorDryRun(t, selectorLaunch(root, "--relaunch", "--only", "foxtrot-done,alpha-ready")...)
	if code != 0 || strings.Contains(stderr, "GOAL_NAMES_INERT_ITEMS") {
		t.Fatalf("relaunch: code=%d stderr=%s — the parent checked the selector; the child must not refuse items the earlier waves shipped", code, stderr)
	}

	writeHandoff(t, filepath.Join(root, ".evolve"), os.Getpid(), 0)
	code, stderr = selectorDryRun(t, selectorLaunch(root, "--only", "foxtrot-done,alpha-ready")...)
	if code != 0 || strings.Contains(stderr, "GOAL_NAMES_INERT_ITEMS") {
		t.Fatalf("chain re-exec handoff: code=%d stderr=%s — the re-exec continues its run; it must not refuse a shipped --only id", code, stderr)
	}
}

func TestSelectorFlags_NoInboxFileChanges(t *testing.T) {
	root := selectorFixture(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	before := selectorFingerprint(t, inbox)

	valid := selectorLaunch(root, "--only", "bravo-ready,charlie-ready", "--exclude", "alpha-ready")
	if got := selectorPlan(t, selectorLaunch(root, "--only", "charlie-ready,delta-ready", "--exclude", "alpha-ready")); !reflect.DeepEqual(got, []string{"charlie-ready", "delta-ready"}) {
		t.Fatalf("wave plan = %v, want [charlie-ready delta-ready]", got)
	}
	if code, stderr := selectorDryRun(t, valid...); code != 0 {
		t.Fatalf("valid selector launch: code=%d\n%s", code, stderr)
	}
	selectorDryRun(t, selectorLaunch(root, "--only", "echo-console")...)
	selectorDryRun(t, selectorLaunch(root, "--only", "bravo-ready", "--exclude", "bravo-ready")...)

	if after := selectorFingerprint(t, inbox); !reflect.DeepEqual(before, after) {
		t.Fatalf("the inbox changed: %d entries before, %d after — a selector scopes one launch and edits no item", len(before), len(after))
	}
}

func TestSelectorFlags_TheDetachedChildCarriesTheSelector(t *testing.T) {
	root := selectorFixture(t)
	var stderr bytes.Buffer

	parent, rc := parseLoopArgs(selectorLaunch(root, "--detach", "--log", filepath.Join(root, "loop.log"), "--only", "charlie-ready,delta-ready", "--exclude", "alpha-ready"), &stderr)
	if rc != 0 {
		t.Fatalf("parseLoopArgs rc=%d\n%s", rc, clipSelectorStderr(stderr.String()))
	}

	if only, ok := argvFlagValue(parent.DetachArgv, "only"); !ok || only != "charlie-ready,delta-ready" {
		t.Errorf("detached child argv %q lacks --only charlie-ready,delta-ready", parent.DetachArgv)
	}
	if exclude, ok := argvFlagValue(parent.DetachArgv, "exclude"); !ok || exclude != "alpha-ready" {
		t.Errorf("detached child argv %q lacks --exclude alpha-ready", parent.DetachArgv)
	}
	if got := selectorPlan(t, parent.DetachArgv); !reflect.DeepEqual(got, []string{"charlie-ready", "delta-ready"}) {
		t.Errorf("the detached child plans %v, want [charlie-ready delta-ready]", got)
	}
}

func TestSelectorFlags_TheChainReExecCarriesTheSelector(t *testing.T) {
	root := selectorFixture(t)
	prev := os.Args
	t.Cleanup(func() { os.Args = prev })
	os.Args = append([]string{"evolve", "loop"}, selectorLaunch(root, "--only", "charlie-ready,delta-ready", "--exclude", "alpha-ready")...)

	argv := chainReExecArgvFn()

	if only, ok := argvFlagValue(argv, "only"); !ok || only != "charlie-ready,delta-ready" {
		t.Errorf("chain re-exec argv %q lacks --only charlie-ready,delta-ready", argv)
	}
	if exclude, ok := argvFlagValue(argv, "exclude"); !ok || exclude != "alpha-ready" {
		t.Errorf("chain re-exec argv %q lacks --exclude alpha-ready", argv)
	}
	if !slices.Contains(argv, "--relaunch") {
		t.Errorf("chain re-exec argv %q lacks --relaunch", argv)
	}
	if got := selectorPlan(t, argv[2:]); !reflect.DeepEqual(got, []string{"charlie-ready", "delta-ready"}) {
		t.Errorf("the re-executed loop plans %v, want [charlie-ready delta-ready]", got)
	}
}

func boundaryLoopStepPlan(t *testing.T, steps []boundaryStep) []string {
	t.Helper()
	last := steps[len(steps)-1]
	if last.verb != "loop" {
		t.Fatalf("the last boundary step is %q, want the loop launch", last.verb)
	}
	if only, ok := argvFlagValue(last.args, "only"); !ok || only != "charlie-ready,delta-ready" {
		t.Errorf("boundary loop step %q lacks --only charlie-ready,delta-ready", last.args)
	}
	if exclude, ok := argvFlagValue(last.args, "exclude"); !ok || exclude != "alpha-ready" {
		t.Errorf("boundary loop step %q lacks --exclude alpha-ready", last.args)
	}
	return selectorPlan(t, last.args)
}

func TestSelectorFlags_TheBoundaryLoopStepCarriesTheSelector(t *testing.T) {
	root := selectorFixture(t)
	goalFile := filepath.Join(root, "goal.txt")
	if err := os.WriteFile(goalFile, []byte("work the backlog\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a, err := parseBoundaryArgs([]string{"--goal-text-file", goalFile, "--project-root", root, "--only", "charlie-ready,delta-ready", "--exclude", "alpha-ready"})
	if err != nil {
		t.Fatalf("parseBoundaryArgs: %v", err)
	}
	a.runID = "selector-run"

	if got := boundaryLoopStepPlan(t, boundarySteps(a, root)); !reflect.DeepEqual(got, []string{"charlie-ready", "delta-ready"}) {
		t.Errorf("the relaunched loop plans %v, want [charlie-ready delta-ready]", got)
	}
}

func TestSelectorFlags_TheWaveNextLoopStepCarriesTheSelector(t *testing.T) {
	root := selectorFixture(t)

	next, err := parseWaveNextArgs([]string{"--project-root", root, "--only", "charlie-ready,delta-ready", "--exclude", "alpha-ready"})
	if err != nil {
		t.Fatalf("parseWaveNextArgs: %v", err)
	}
	a := next.boundary
	a.goalText, a.runID = "work the backlog", "selector-run"

	if got := boundaryLoopStepPlan(t, waveSteps(a, root, nil)); !reflect.DeepEqual(got, []string{"charlie-ready", "delta-ready"}) {
		t.Errorf("the relaunched loop plans %v, want [charlie-ready delta-ready]", got)
	}
}
