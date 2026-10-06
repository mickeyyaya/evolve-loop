//go:build acs

package cycle1801

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func fakeTmuxEnv(t *testing.T) (env []string, callLog string) {
	t.Helper()
	bin := t.TempDir()
	callLog = filepath.Join(t.TempDir(), "tmux-calls.log")
	script := "#!/bin/sh\necho \"$@\" >> '" + callLog + "'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "tmux"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return append(hostEnvWithout("EVOLVE_"), pathWith(bin)), callLog
}

func tmuxCalls(t *testing.T, callLog string) string {
	t.Helper()
	raw, err := os.ReadFile(callLog)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(raw))
}

func TestC1801_001_BareGCRefusesBeforeAnyReaperRuns(t *testing.T) {
	env, callLog := fakeTmuxEnv(t)
	res := runEvolve(t, t.TempDir(), env, "gc")
	if res.code != 1 {
		t.Errorf("bare gc: want exit 1 (refused), got %s", res)
	}
	if !strings.Contains(res.stderr, "mutating run refused") {
		t.Errorf("bare gc stderr does not carry the refusal: %s", res)
	}
	if calls := tmuxCalls(t, callLog); calls != "" {
		t.Errorf("bare gc reached the tmux reapers before refusing: %q", calls)
	}
	if strings.Contains(res.stdout, "orphan session") || strings.Contains(res.stdout, "socket") {
		t.Errorf("bare gc printed reaper output before refusing: %s", res)
	}
}

func TestC1801_002_GCHelpStatesTheRefusalInsteadOfACwdDefault(t *testing.T) {
	res := runEvolve(t, t.TempDir(), hostEnvWithout("EVOLVE_"), "gc", "-h")
	help := res.combined()
	if !strings.Contains(help, "-project-root") {
		t.Fatalf("gc help lacks -project-root: %s", res)
	}
	if strings.Contains(help, "default = current directory") || strings.Contains(help, "defaults to the current directory") {
		t.Errorf("gc help still promises a cwd default for a mutating run: %s", res)
	}
	if !strings.Contains(strings.ToLower(help), "refused") {
		t.Errorf("gc help does not say a non-dry run without -project-root is refused: %s", res)
	}
}

func TestC1801_003_NonMutatingGCStillReachesTheReapers(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"dry-run without a root falls back to cwd", []string{"gc", "--dry-run"}},
		{"dry-run with an explicit root", []string{"gc", "--dry-run", "--project-root", t.TempDir()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env, callLog := fakeTmuxEnv(t)
			res := runEvolve(t, t.TempDir(), env, tc.args...)
			if strings.Contains(res.stderr, "mutating run refused") {
				t.Errorf("%v was refused, but only a mutating run without a root may be: %s", tc.args, res)
			}
			if tmuxCalls(t, callLog) == "" {
				t.Errorf("%v never reached tmux; validation-first must not skip the reapers: %s", tc.args, res)
			}
		})
	}
}

func parseCmdEvolveDir(t *testing.T) (dir string, files map[string]*ast.File) {
	t.Helper()
	dir = filepath.Join(acsassert.RepoRoot(t), "go", "cmd", "evolve")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	files = map[string]*ast.File{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files[e.Name()] = f
	}
	if len(files) == 0 {
		t.Fatalf("parsed no Go files under %s", dir)
	}
	return dir, files
}

func referencesHostReaper(n ast.Node) (string, bool) {
	se, ok := n.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	if se.Sel.Name != "ExecReapOrphans" && se.Sel.Name != "ExecReapOrphanSockets" {
		return "", false
	}
	return se.Sel.Name, true
}

// acs-predicate: config-check
func TestC1801_004_NoCmdEvolveTestReachesTheHostTmuxReapers(t *testing.T) {
	_, files := parseCmdEvolveDir(t)
	ctorDefaults := 0
	for name, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			fn, ok := referencesHostReaper(n)
			if !ok {
				return true
			}
			if strings.HasSuffix(name, "_test.go") {
				t.Errorf("%s references swarm.%s: a cmd/evolve test would reap host tmux sessions", name, fn)
			} else if name == "cmd_gc.go" {
				ctorDefaults++
			}
			return true
		})
	}
	if ctorDefaults == 0 {
		t.Errorf("cmd_gc.go no longer defaults to the real swarm reapers, so a mutating gc reaps nothing")
	}
	var seam *ast.StructType
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == "gcReapers" {
				seam, _ = ts.Type.(*ast.StructType)
			}
			return seam == nil
		})
	}
	if seam == nil {
		t.Fatalf("no gcReapers seam: tests cannot inject fake reapers")
	}
	fields := map[string]bool{}
	for _, fl := range seam.Fields.List {
		for _, n := range fl.Names {
			fields[n.Name] = true
		}
	}
	if !fields["sessions"] || !fields["sockets"] {
		t.Errorf("gcReapers must carry injectable sessions and sockets reapers, has %v", fields)
	}
}

func TestC1801_005_CIWatchShaGreenExitsZeroAndPrintsTheConclusion(t *testing.T) {
	fx := newCIFixture(t, 60, fakeGHState{Runs: []fakeRun{
		{Workflow: requiredWorkflow, ID: 501, Conclusion: "success", Pending: 1},
		{Workflow: releaseWorkflow, ID: 502, Conclusion: "failure"},
	}})
	res := fx.watch(t, "--sha", fx.head)
	if res.code != 0 {
		t.Fatalf("ci watch --sha on a green required run: want exit 0, got %s", res)
	}
	if !hasLineWithAll(res.stdout, requiredWorkflow, "success") {
		t.Errorf("stdout does not print the required workflow's conclusion on one line: %s", res)
	}
	if got := fx.observations(t, requiredWorkflow); got < 2 {
		t.Errorf("required run was in progress on the first poll; want it polled until complete (>=2 observations), got %d", got)
	}
	if items := fx.inboxItems(t); len(items) != 0 {
		t.Errorf("a green run filed inbox items: %v", items)
	}
}

func TestC1801_006_CIWatchShaRedExitsOneAndFilesOneFixForwardItem(t *testing.T) {
	fx := newCIFixture(t, 60, fakeGHState{Runs: []fakeRun{
		{Workflow: requiredWorkflow, ID: 601, Conclusion: "failure"},
	}})
	res := fx.watch(t, "--sha", fx.head, "--cycle", "4242")
	if res.code != 1 {
		t.Fatalf("ci watch --sha on a red required run: want exit 1, got %s", res)
	}
	if !hasLineWithAll(res.stdout, requiredWorkflow, "failure") {
		t.Errorf("stdout does not print the required workflow's red conclusion on one line: %s", res)
	}
	items := fx.inboxItems(t)
	if len(items) != 1 {
		t.Fatalf("a red main SHA must file exactly one fix-forward inbox item, found %d: %v", len(items), items)
	}
	raw, err := os.ReadFile(items[0])
	if err != nil {
		t.Fatal(err)
	}
	var item map[string]any
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatalf("inbox item %s is not a JSON object: %v\n%s", items[0], err, raw)
	}
	if item["source"] != "ciwatch" {
		t.Errorf("inbox item was not filed by ciwatch.Watch (source=%v): %s", item["source"], raw)
	}
	body := string(raw)
	if !strings.Contains(body, fx.head[:12]) {
		t.Errorf("inbox item does not name the watched SHA %s: %s", fx.head[:12], body)
	}
	if !strings.Contains(body, "4242") {
		t.Errorf("inbox item does not name --cycle 4242: %s", body)
	}
}

func TestC1801_007_CIWatchRedPrintsTheClassifyVerdict(t *testing.T) {
	fx := newCIFixture(t, 60, fakeGHState{Runs: []fakeRun{
		{Workflow: requiredWorkflow, ID: 701, Conclusion: "failure"},
	}})
	res := fx.watch(t, "--sha", fx.head)
	if res.code != 1 {
		t.Fatalf("ci watch --sha on a red required run: want exit 1, got %s", res)
	}
	if !strings.Contains(res.stdout, redTestName) {
		t.Errorf("red watch output does not name the failing test %s from the classify result: %s", redTestName, res)
	}
	if !regexp.MustCompile(`(?i)retry.?safe`).MatchString(res.stdout) {
		t.Errorf("red watch output carries no ci classify verdict (retry-safe): %s", res)
	}
}

func TestC1801_008_CIWatchPRRedFilesNoInboxItem(t *testing.T) {
	fx := newCIFixture(t, 60, fakeGHState{Branch: "feature", Event: "pull_request", Runs: []fakeRun{
		{Workflow: requiredWorkflow, ID: 801, Conclusion: "failure", Pending: 1},
	}})
	res := fx.watch(t, "--pr", "7", "--cycle", "4242")
	if res.code != 1 {
		t.Fatalf("ci watch --pr on a red run: want exit 1, got %s", res)
	}
	if got := fx.observations(t, requiredWorkflow); got < 2 {
		t.Errorf("PR run was in progress on the first poll; want it polled until complete (>=2 observations), got %d", got)
	}
	if items := fx.inboxItems(t); len(items) != 0 {
		t.Errorf("a red PR run must file no inbox item, found %v", items)
	}
}

func TestC1801_009_CIWatchTagWaitsForRequiredAndRelease(t *testing.T) {
	cases := []struct {
		name               string
		required, release  string
		releasePending     int
		wantCode, wantItem int
	}{
		{"both green after release finishes", "success", "success", 1, 0, 0},
		{"release red with required green", "success", "failure", 0, 1, 1},
		{"required red with release green", "failure", "success", 0, 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newCIFixture(t, 60, fakeGHState{Runs: []fakeRun{
				{Workflow: requiredWorkflow, ID: 901, Conclusion: tc.required},
				{Workflow: releaseWorkflow, ID: 902, Conclusion: tc.release, Pending: tc.releasePending},
			}})
			res := fx.watch(t, "--tag", fixtureTag)
			if res.code != tc.wantCode {
				t.Fatalf("ci watch --tag: want exit %d, got %s", tc.wantCode, res)
			}
			if fx.observations(t, requiredWorkflow) == 0 {
				t.Errorf("--tag never observed %s: %v", requiredWorkflow, fx.ghCalls(t))
			}
			if got := fx.observations(t, releaseWorkflow); got < 1+tc.releasePending {
				t.Errorf("--tag must wait for %s until it completes: want >=%d observations, got %d", releaseWorkflow, 1+tc.releasePending, got)
			}
			if !hasLineWithAll(res.stdout, requiredWorkflow, tc.required) || !hasLineWithAll(res.stdout, releaseWorkflow, tc.release) {
				t.Errorf("stdout must print each workflow's conclusion: %s", res)
			}
			if items := fx.inboxItems(t); len(items) != tc.wantItem {
				t.Errorf("--tag red must file %d fix-forward item(s), found %v", tc.wantItem, items)
			}
		})
	}
}

func TestC1801_010_CIWatchExitsTwoWhenRunsCannotBeObserved(t *testing.T) {
	cases := []struct {
		name     string
		timeoutS int
		gh       fakeGHState
		stderr   *regexp.Regexp
	}{
		{"gh fails", 60, fakeGHState{Broken: true, Runs: []fakeRun{{Workflow: requiredWorkflow, ID: 1001, Conclusion: "failure"}}}, regexp.MustCompile(`\bgh\b|502|unreachable`)},
		{"run never completes within the policy timeout", 1, fakeGHState{Runs: []fakeRun{{Workflow: requiredWorkflow, ID: 1002, Conclusion: "failure", Pending: 1 << 20}}}, regexp.MustCompile(`(?i)tim(ed|e)\s?out`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newCIFixture(t, tc.timeoutS, tc.gh)
			res := fx.watch(t, "--sha", fx.head, "--cycle", "4242")
			if res.code != 2 {
				t.Fatalf("want exit 2, got %s", res)
			}
			if !tc.stderr.MatchString(res.stderr) {
				t.Errorf("stderr does not say why the run could not be observed (want %s): %s", tc.stderr, res)
			}
			if len(fx.ghCalls(t)) == 0 {
				t.Errorf("exit 2 without ever asking gh: %s", res)
			}
			if items := fx.inboxItems(t); len(items) != 0 {
				t.Errorf("an unobserved run must file no inbox item, found %v", items)
			}
		})
	}
}

func TestC1801_011_CIWatchRejectsMalformedTargetsAsUsageErrors(t *testing.T) {
	cases := [][]string{
		{},
		{"--sha", "abc1234", "--pr", "7"},
		{"--sha", "not-a-sha"},
		{"--pr", "seven"},
		{"--tag"},
		{"--sha", "abc1234", "--cycle", "many"},
		{"--sha", "abc1234", "--frobnicate"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			fx := newCIFixture(t, 60, fakeGHState{Runs: []fakeRun{{Workflow: requiredWorkflow, ID: 1101, Conclusion: "success"}}})
			res := fx.watch(t, args...)
			if res.code != exitUsage {
				t.Errorf("ci watch %v: want usage exit %d, got %s", args, exitUsage, res)
			}
			if !strings.Contains(res.stderr, "ci watch") || !strings.Contains(res.stderr, "--sha") {
				t.Errorf("ci watch %v: stderr does not show the ci watch usage: %s", args, res)
			}
			if calls := fx.ghCalls(t); len(calls) != 0 {
				t.Errorf("ci watch %v called gh before rejecting its arguments: %v", args, calls)
			}
		})
	}
}

func TestC1801_012_CIWatchIsDiscoverableFromHelp(t *testing.T) {
	fx := newCIFixture(t, 60, fakeGHState{Runs: []fakeRun{{Workflow: requiredWorkflow, ID: 1201, Conclusion: "success"}}})
	res := fx.watch(t, "--help")
	if res.code != 0 && res.code != exitUsage {
		t.Errorf("ci watch --help: want exit 0 or %d, got %s", exitUsage, res)
	}
	for _, flag := range []string{"--sha", "--pr", "--tag", "--workflow", "--cycle"} {
		if !regexp.MustCompile(regexp.QuoteMeta(flag) + `\b`).MatchString(res.combined()) {
			t.Errorf("ci watch --help does not document %s: %s", flag, res)
		}
	}
	if calls := fx.ghCalls(t); len(calls) != 0 {
		t.Errorf("ci watch --help called gh: %v", calls)
	}
	top := runEvolve(t, fx.root, fx.env, "help")
	if !hasLineWithAll(top.combined(), "ci watch") {
		t.Errorf("evolve help does not list ci watch: %s", top)
	}
}

func TestC1801_013_CIWatchWatchesAnExplicitWorkflow(t *testing.T) {
	fx := newCIFixture(t, 60, fakeGHState{Runs: []fakeRun{
		{Workflow: requiredWorkflow, ID: 1301, Conclusion: "success"},
		{Workflow: "go.yml", ID: 1302, Conclusion: "failure"},
	}})
	res := fx.watch(t, "--sha", fx.head, "--workflow", "go.yml")
	if res.code != 1 {
		t.Fatalf("ci watch --workflow go.yml on a red go.yml run: want exit 1, got %s", res)
	}
	if fx.observations(t, "go.yml") == 0 {
		t.Errorf("--workflow go.yml never observed go.yml: %v", fx.ghCalls(t))
	}
	if !hasLineWithAll(res.stdout, "go.yml", "failure") {
		t.Errorf("stdout does not print go.yml's conclusion: %s", res)
	}
}
