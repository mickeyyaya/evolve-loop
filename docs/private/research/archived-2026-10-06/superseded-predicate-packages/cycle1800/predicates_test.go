//go:build acs

package cycle1800

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
)

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		root := acsassert.RepoRoot(t)
		dir, err := os.MkdirTemp("", "evolve-acs-1800-")
		if err != nil {
			buildErr = err
			return
		}
		builtBin = filepath.Join(dir, "evolve")
		cmd := exec.Command("go", "build", "-o", builtBin, "./cmd/evolve")
		cmd.Dir = filepath.Join(root, "go")
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = &buildFailure{out: string(out), err: err}
		}
	})
	if buildErr != nil {
		t.Fatalf("evolve binary build failed: %v", buildErr)
	}
	return builtBin
}

type buildFailure struct {
	out string
	err error
}

func (b *buildFailure) Error() string { return b.err.Error() + ": " + b.out }

type runResult struct {
	stdout, stderr string
	code           int
}

func runEvolve(t *testing.T, env []string, cwd string, args ...string) runResult {
	t.Helper()
	return runEvolveWithEnv(t, append(os.Environ(), env...), cwd, args...)
}

func runEvolveWithEnv(t *testing.T, fullEnv []string, cwd string, args ...string) runResult {
	t.Helper()
	cmd := exec.Command(evolveBinary(t), args...)
	cmd.Dir = cwd
	cmd.Env = fullEnv
	var so, se strings.Builder
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run evolve %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return runResult{so.String(), se.String(), code}
}

func fakeTmuxEnv(t *testing.T) (env []string, callLog string) {
	t.Helper()
	bin := t.TempDir()
	callLog = filepath.Join(t.TempDir(), "tmux-calls.log")
	script := "#!/bin/sh\necho \"$@\" >> '" + callLog + "'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "tmux"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH")}, callLog
}

func tmuxCalls(t *testing.T, callLog string) string {
	t.Helper()
	b, err := os.ReadFile(callLog)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

func TestC1800_001_BareGCRefusesBeforeAnyReaperRuns(t *testing.T) {
	env, callLog := fakeTmuxEnv(t)
	res := runEvolve(t, env, t.TempDir(), "gc")
	if res.code != 1 {
		t.Errorf("bare gc exit=%d, want 1", res.code)
	}
	if !strings.Contains(res.stderr, "mutating run refused") {
		t.Errorf("bare gc stderr lacks refusal: %q", res.stderr)
	}
	if calls := tmuxCalls(t, callLog); calls != "" {
		t.Errorf("bare gc reached tmux before refusing: %q", calls)
	}
	if strings.Contains(res.stdout, "orphan session") || strings.Contains(res.stdout, "socket") {
		t.Errorf("bare gc reported reaper output before refusing: %q", res.stdout)
	}
}

func TestC1800_002_GCHelpDoesNotPromiseACwdDefault(t *testing.T) {
	res := runEvolve(t, nil, t.TempDir(), "gc", "-h")
	help := res.stdout + res.stderr
	if !strings.Contains(help, "-project-root") {
		t.Fatalf("gc help lacks -project-root: %q", help)
	}
	if strings.Contains(help, "default = current directory") {
		t.Errorf("gc help still promises a cwd default for -project-root: %q", help)
	}
	if !strings.Contains(strings.ToLower(help), "refused") {
		t.Errorf("gc help does not say a non-dry run without -project-root is refused: %q", help)
	}
}

func TestC1800_003_GCWithExplicitRootStillReachesTheReapers(t *testing.T) {
	env, callLog := fakeTmuxEnv(t)
	runEvolve(t, env, t.TempDir(), "gc", "--dry-run", "--project-root", t.TempDir())
	if tmuxCalls(t, callLog) == "" {
		t.Errorf("gc --dry-run with an explicit root never reached tmux; validation-first must not skip the reapers")
	}
}

type fixtureEntry struct {
	Cycle          int    `json:"cycle"`
	Classification string `json:"classification"`
	Summary        string `json:"summary"`
	RecordedAt     string `json:"recordedAt"`
	ExpiresAt      string `json:"expiresAt"`
}

func writeState(t *testing.T, entries []fixtureEntry) (root, statePath string) {
	t.Helper()
	root = t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	statePath = filepath.Join(evolveDir, "state.json")
	raw, err := json.Marshal(map[string]any{"failedApproaches": entries, "lastCycleNumber": 9})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, statePath
}

func mixedEntries() []fixtureEntry {
	return []fixtureEntry{
		{1, "infrastructure-systemic", "tmux wedged", "2026-10-01T00:00:00Z", "2099-01-01T00:00:00Z"},
		{2, "code-build-fail", "build red", "2026-10-02T00:00:00Z", "2099-01-01T00:00:00Z"},
		{3, "ship-gate-config", "gate cfg", "2026-10-03T00:00:00Z", "2099-01-01T00:00:00Z"},
		{4, "code-audit-fail", "stale audit", "2020-01-01T00:00:00Z", "2020-02-01T00:00:00Z"},
	}
}

func approachClasses(t *testing.T, statePath string) []string {
	t.Helper()
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var st struct {
		FailedApproaches []fixtureEntry `json:"failedApproaches"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range st.FailedApproaches {
		out = append(out, e.Classification)
	}
	return out
}

func TestC1800_004_FailuresListJSONEmitsEveryEntryAndLeavesStateUntouched(t *testing.T) {
	root, statePath := writeState(t, mixedEntries())
	before, _ := os.ReadFile(statePath)
	res := runEvolve(t, nil, t.TempDir(), "failures", "list", "--project-root", root, "--json")
	if res.code != 0 {
		t.Fatalf("failures list exit=%d stderr=%q", res.code, res.stderr)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
		t.Fatalf("list --json is not a JSON array: %v: %q", err, res.stdout)
	}
	if len(got) != 4 {
		t.Fatalf("list --json returned %d entries, want 4", len(got))
	}
	for _, key := range []string{"recordedAt", "classification", "summary"} {
		if _, ok := got[0][key]; !ok {
			t.Errorf("entry lacks key %q: %v", key, got[0])
		}
	}
	after, _ := os.ReadFile(statePath)
	if string(before) != string(after) {
		t.Errorf("failures list mutated state.json")
	}
}

func TestC1800_005_FailuresListAbsentStateIsAnEmptyArray(t *testing.T) {
	res := runEvolve(t, nil, t.TempDir(), "failures", "list", "--project-root", t.TempDir(), "--json")
	if res.code != 0 {
		t.Fatalf("exit=%d stderr=%q", res.code, res.stderr)
	}
	if strings.TrimSpace(res.stdout) != "[]" {
		t.Errorf("absent state.json must list as [], got %q", res.stdout)
	}
}

func TestC1800_006_FailuresListClassFilterAndUnknownClass(t *testing.T) {
	root, _ := writeState(t, mixedEntries())
	res := runEvolve(t, nil, t.TempDir(), "failures", "list", "--project-root", root, "--json", "--class", "code-build-fail")
	var got []map[string]any
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil || len(got) != 1 {
		t.Errorf("--class code-build-fail: err=%v entries=%d stdout=%q", err, len(got), res.stdout)
	}
	bad := runEvolve(t, nil, t.TempDir(), "failures", "list", "--project-root", root, "--class", "no-such-class")
	if bad.code != 10 || !strings.Contains(bad.stderr, "unknown class") {
		t.Errorf("unknown class: exit=%d stderr=%q, want 10 + 'unknown class'", bad.code, bad.stderr)
	}
}

func TestC1800_007_FailuresUsageErrorsExitTen(t *testing.T) {
	for _, args := range [][]string{{"failures"}, {"failures", "frobnicate"}} {
		res := runEvolve(t, nil, t.TempDir(), args...)
		if res.code != 10 || !strings.Contains(res.stderr, "evolve failures: usage:") {
			t.Errorf("%v: exit=%d stderr=%q, want 10 + usage", args, res.code, res.stderr)
		}
	}
}

func TestC1800_008_FailuresMutatingVerbsRefuseWithoutProjectRoot(t *testing.T) {
	root, statePath := writeState(t, mixedEntries())
	before, _ := os.ReadFile(statePath)
	for _, sub := range []string{"reset", "prune"} {
		res := runEvolve(t, nil, root, "failures", sub)
		if res.code != 1 || !strings.Contains(res.stderr, "mutating run refused") {
			t.Errorf("%s without --project-root: exit=%d stderr=%q", sub, res.code, res.stderr)
		}
	}
	after, _ := os.ReadFile(statePath)
	if string(before) != string(after) {
		t.Errorf("a refused run mutated state.json")
	}
}

func TestC1800_009_FailuresResetDropsOnlyInfrastructureClassesAndIsIdempotent(t *testing.T) {
	root, statePath := writeState(t, mixedEntries())
	res := runEvolve(t, nil, t.TempDir(), "failures", "reset", "--project-root", root)
	if res.code != 0 {
		t.Fatalf("reset exit=%d stderr=%q", res.code, res.stderr)
	}
	got := strings.Join(approachClasses(t, statePath), ",")
	if got != "code-build-fail,code-audit-fail" {
		t.Errorf("after reset classes=%q, want code-build-fail,code-audit-fail", got)
	}
	again := runEvolve(t, nil, t.TempDir(), "failures", "reset", "--project-root", root)
	if again.code != 0 || strings.Join(approachClasses(t, statePath), ",") != got {
		t.Errorf("second reset not idempotent: exit=%d", again.code)
	}
}

func TestC1800_010_FailuresResetFingerprintIsAcknowledged(t *testing.T) {
	root, _ := writeState(t, mixedEntries())
	res := runEvolve(t, nil, t.TempDir(), "failures", "reset", "--project-root", root, "--fingerprint", "fp-acs-1800")
	if res.code != 0 {
		t.Fatalf("exit=%d stderr=%q", res.code, res.stderr)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "resolved-fingerprints.json"))
	if err != nil || !strings.Contains(string(raw), "fp-acs-1800") {
		t.Errorf("fingerprint not acked in resolved-fingerprints.json: err=%v body=%q", err, raw)
	}
}

func TestC1800_011_FailuresPruneRemovesOnlyExpiredEntries(t *testing.T) {
	root, statePath := writeState(t, mixedEntries())
	res := runEvolve(t, nil, t.TempDir(), "failures", "prune", "--project-root", root)
	if res.code != 0 {
		t.Fatalf("prune exit=%d stderr=%q", res.code, res.stderr)
	}
	got := strings.Join(approachClasses(t, statePath), ",")
	if got != "infrastructure-systemic,code-build-fail,ship-gate-config" {
		t.Errorf("after prune classes=%q, want only the expired code-audit-fail removed", got)
	}
}

func parseCmdEvolve(t *testing.T) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), "go", "cmd", "evolve")
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]*ast.File{}
	for _, p := range pkgs {
		for name, f := range p.Files {
			files[filepath.Base(name)] = f
		}
	}
	return fset, files
}

func funcDecl(files map[string]*ast.File, name string) *ast.FuncDecl {
	for _, f := range files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == name {
				return fd
			}
		}
	}
	return nil
}

func calls(fd *ast.FuncDecl, callee string) bool {
	found := false
	ast.Inspect(fd, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if id, ok := c.Fun.(*ast.Ident); ok && id.Name == callee {
				found = true
			}
		}
		return !found
	})
	return found
}

func selectorCalls(fd *ast.FuncDecl, pkg, fn string) bool {
	found := false
	ast.Inspect(fd, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if se, ok := c.Fun.(*ast.SelectorExpr); ok {
				if id, ok := se.X.(*ast.Ident); ok && id.Name == pkg && se.Sel.Name == fn {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

func TestC1800_012_GCReapersAreInjectableThroughTheGCRunStruct(t *testing.T) {
	_, files := parseCmdEvolve(t)
	var reapers, gcRun *ast.StructType
	for _, f := range files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, s := range gd.Specs {
				ts, ok := s.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				switch ts.Name.Name {
				case "gcReapers":
					reapers = st
				case "gcRun":
					gcRun = st
				}
			}
		}
	}
	if reapers == nil {
		t.Fatalf("no gcReapers struct: reapers are not an injectable seam")
	}
	have := map[string]bool{}
	for _, f := range reapers.Fields.List {
		for _, n := range f.Names {
			have[n.Name] = true
		}
	}
	if !have["sessions"] || !have["sockets"] {
		t.Errorf("gcReapers must carry sessions and sockets reapers, has %v", have)
	}
	typed := false
	for _, f := range gcRun.Fields.List {
		if id, ok := f.Type.(*ast.Ident); ok && id.Name == "gcReapers" {
			typed = true
		}
	}
	if !typed {
		t.Errorf("gcRun carries no gcReapers field")
	}
}

func TestC1800_013_OnlyTheGCRunConstructorReachesTheRealExecReapers(t *testing.T) {
	_, files := parseCmdEvolve(t)
	for name, f := range files {
		if name != "cmd_gc.go" {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			isCtor := fd.Recv == nil && fd.Name.Name == "newGCRun"
			for _, fn := range []string{"ExecReapOrphans", "ExecReapOrphanSockets"} {
				ref := false
				ast.Inspect(fd, func(n ast.Node) bool {
					if se, ok := n.(*ast.SelectorExpr); ok && se.Sel.Name == fn {
						if id, ok := se.X.(*ast.Ident); ok && id.Name == "swarm" {
							ref = true
						}
					}
					return true
				})
				if ref && !isCtor {
					t.Errorf("%s: func %s references swarm.%s outside newGCRun", name, fd.Name.Name, fn)
				}
				if !ref && isCtor {
					t.Errorf("newGCRun does not default to swarm.%s", fn)
				}
			}
		}
	}
}

func TestC1800_014_NoCmdEvolveTestReachesTheHostTmuxReapers(t *testing.T) {
	_, files := parseCmdEvolve(t)
	dir := filepath.Join(acsassert.RepoRoot(t), "go", "cmd", "evolve")
	entries, _ := os.ReadDir(dir)
	fset := token.NewFileSet()
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if se, ok := n.(*ast.SelectorExpr); ok && (se.Sel.Name == "ExecReapOrphans" || se.Sel.Name == "ExecReapOrphanSockets") {
				t.Errorf("%s references swarm.%s: a cmd/evolve test would kill host tmux sessions", e.Name(), se.Sel.Name)
			}
			return true
		})
	}
	if len(files) == 0 {
		t.Fatal("parsed no cmd/evolve files")
	}
}

func TestC1800_015_ResetIsOneSharedFunctionUsedByLoopResetAndFailuresReset(t *testing.T) {
	_, files := parseCmdEvolve(t)
	shared := funcDecl(files, "resetFailures")
	if shared == nil {
		t.Fatalf("no resetFailures: reset logic is not shared")
	}
	if !selectorCalls(shared, "failurelog", "PruneByClassification") || !selectorCalls(shared, "core", "AppendResolvedFingerprint") {
		t.Errorf("resetFailures must own both the class prune and the fingerprint ack")
	}
	for _, caller := range []string{"resetBatchState", "runFailuresReset"} {
		fd := funcDecl(files, caller)
		if fd == nil {
			t.Fatalf("missing %s", caller)
		}
		if !calls(fd, "resetFailures") {
			t.Errorf("%s does not call resetFailures", caller)
		}
		if selectorCalls(fd, "failurelog", "PruneByClassification") || selectorCalls(fd, "core", "AppendResolvedFingerprint") {
			t.Errorf("%s still re-implements the reset body", caller)
		}
	}
}

func TestC1800_016_FailuresPruneNeverBumpsCyclesUnpicked(t *testing.T) {
	root, statePath := writeState(t, mixedEntries())
	raw, _ := os.ReadFile(statePath)
	var st map[string]any
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	st["carryoverTodos"] = []any{map[string]any{"id": "todo-a", "cycles_unpicked": float64(2), "expiresAt": "2099-01-01T00:00:00Z"}}
	out, _ := json.Marshal(st)
	if err := os.WriteFile(statePath, out, 0o644); err != nil {
		t.Fatal(err)
	}
	if res := runEvolve(t, nil, t.TempDir(), "failures", "prune", "--project-root", root); res.code != 0 {
		t.Fatalf("prune exit=%d stderr=%q", res.code, res.stderr)
	}
	after, _ := os.ReadFile(statePath)
	var got struct {
		Todos []map[string]any `json:"carryoverTodos"`
	}
	if err := json.Unmarshal(after, &got); err != nil || len(got.Todos) != 1 {
		t.Fatalf("carryoverTodos lost: err=%v body=%s", err, after)
	}
	if got.Todos[0]["cycles_unpicked"] != float64(2) {
		t.Errorf("prune bumped cycles_unpicked to %v, want 2", got.Todos[0]["cycles_unpicked"])
	}
}

const (
	unparseableState           = "{not json"
	cycleStateThatHaltsTheLoop = "{unfinished cycle the loop must not overwrite"
	corruptResolvedFingerprint = "{corrupt resolved fingerprints"
)

func liveResetClassesState(t *testing.T) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"failedApproaches": []fixtureEntry{
		{1, "infrastructure-systemic", "tmux wedged", "2026-10-01T00:00:00Z", "2099-01-01T00:00:00Z"},
		{2, "code-build-fail", "build red", "2026-10-02T00:00:00Z", "2099-01-01T00:00:00Z"},
		{3, "ship-gate-config", "gate cfg", "2026-10-03T00:00:00Z", "2099-01-01T00:00:00Z"},
	}, "lastCycleNumber": 9})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func resetProject(t *testing.T, state, resolvedFingerprints string) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"state.json":                 state,
		"cycle-state.json":           cycleStateThatHaltsTheLoop,
		"resolved-fingerprints.json": resolvedFingerprints,
	}
	for name, body := range files {
		if body == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(evolveDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, evolveDir
}

func runLoopReset(t *testing.T, root, fingerprint string) runResult {
	t.Helper()
	tmuxEnv, _ := fakeTmuxEnv(t)
	env := []string{"GIT_CEILING_DIRECTORIES=" + filepath.Dir(root)}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "EVOLVE_") || strings.HasPrefix(kv, "PATH=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, tmuxEnv...)
	args := []string{"loop", "--reset", "--goal-text", "acs reset probe", "--project-root", root}
	if fingerprint != "" {
		args = append(args, "--fingerprint", fingerprint)
	}
	res := runEvolveWithEnv(t, env, root, args...)
	if res.code != 2 || !strings.Contains(res.stdout, `"unfinished_cycle"`) {
		t.Fatalf("loop did not halt at the unfinished-cycle guard before running a cycle: exit=%d stdout=%q stderr=%q", res.code, res.stdout, res.stderr)
	}
	return res
}

func recordedFingerprints(t *testing.T, evolveDir string) (body string, exists bool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(evolveDir, "resolved-fingerprints.json"))
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(raw), true
}

func hasLineWithPrefixContaining(out, prefix, substr string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, prefix) && strings.Contains(line, substr) {
			return true
		}
	}
	return false
}

func TestC1800_017_LoopResetAcknowledgesTheFingerprintEvenWhenStateIsUnparseable(t *testing.T) {
	root, evolveDir := resetProject(t, unparseableState, "")
	res := runLoopReset(t, root, "fp-acs-1800-loop")
	if !strings.Contains(res.stderr, "[loop] --reset: failurelog: parse state") {
		t.Errorf("the prune error is no longer reported under the base '[loop] --reset:' prefix: %q", res.stderr)
	}
	if !strings.Contains(res.stderr, `[loop] --reset --fingerprint: acknowledged "fp-acs-1800-loop"`) {
		t.Errorf("loop --reset skipped the fingerprint ack after a prune error: %q", res.stderr)
	}
	if body, _ := recordedFingerprints(t, evolveDir); !strings.Contains(body, "fp-acs-1800-loop") {
		t.Errorf("resolved-fingerprints.json does not acknowledge fp-acs-1800-loop after a prune error: %q", body)
	}
}

func TestC1800_018_LoopResetAckErrorStillReportsTheCommittedPruneUnderTheBasePrefixes(t *testing.T) {
	root, evolveDir := resetProject(t, liveResetClassesState(t), corruptResolvedFingerprint)
	res := runLoopReset(t, root, "fp-acs-1800-loop")
	if got := strings.Join(approachClasses(t, filepath.Join(evolveDir, "state.json")), ","); got != "code-build-fail" {
		t.Fatalf("the reset prune was not committed: classes=%q", got)
	}
	if !strings.Contains(res.stderr, "[loop] --reset: pruned 2 failedApproaches") {
		t.Errorf("a committed prune went unreported because the ack failed: %q", res.stderr)
	}
	if !hasLineWithPrefixContaining(res.stderr, "[loop] --reset --fingerprint: ", "resolved-fingerprints.json") {
		t.Errorf("the ack error lost its base '[loop] --reset --fingerprint:' prefix: %q", res.stderr)
	}
	if strings.Contains(res.stderr, "[loop] --reset: acknowledge fingerprint") {
		t.Errorf("the ack error is reported under the prune prefix: %q", res.stderr)
	}
	if strings.Contains(res.stderr, `acknowledged "fp-acs-1800-loop"`) {
		t.Errorf("a failed ack was reported as acknowledged: %q", res.stderr)
	}
	if body, _ := recordedFingerprints(t, evolveDir); body != corruptResolvedFingerprint {
		t.Errorf("a failed ack rewrote the operator's resolved-fingerprints.json: %q", body)
	}
}

func TestC1800_019_FailuresResetAcknowledgesTheFingerprintEvenWhenStateIsUnparseable(t *testing.T) {
	root, evolveDir := resetProject(t, unparseableState, "")
	res := runEvolve(t, nil, t.TempDir(), "failures", "reset", "--project-root", root, "--fingerprint", "fp-acs-1800-failures")
	if res.code != 1 || !strings.Contains(res.stderr, "parse state") {
		t.Errorf("the prune error must still fail the run: exit=%d stderr=%q", res.code, res.stderr)
	}
	if body, _ := recordedFingerprints(t, evolveDir); !strings.Contains(body, "fp-acs-1800-failures") {
		t.Errorf("failures reset diverges from loop --reset: fingerprint not acknowledged after a prune error: %q", body)
	}
}

func TestC1800_020_FailuresResetAckErrorStillReportsTheCommittedPrune(t *testing.T) {
	root, evolveDir := resetProject(t, liveResetClassesState(t), corruptResolvedFingerprint)
	res := runEvolve(t, nil, t.TempDir(), "failures", "reset", "--project-root", root, "--fingerprint", "fp-acs-1800-failures")
	if got := strings.Join(approachClasses(t, filepath.Join(evolveDir, "state.json")), ","); got != "code-build-fail" {
		t.Fatalf("the reset prune was not committed: classes=%q", got)
	}
	if res.code != 1 || !strings.Contains(res.stderr, "resolved-fingerprints.json") {
		t.Errorf("the ack error must fail the run and name the file: exit=%d stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout+res.stderr, "pruned 2") {
		t.Errorf("a committed prune went unreported because the ack failed: stdout=%q stderr=%q", res.stdout, res.stderr)
	}
	if strings.Contains(res.stdout+res.stderr, `acknowledged "fp-acs-1800-failures"`) {
		t.Errorf("a failed ack was reported as acknowledged: stdout=%q", res.stdout)
	}
	if body, _ := recordedFingerprints(t, evolveDir); body != corruptResolvedFingerprint {
		t.Errorf("a failed ack rewrote the operator's resolved-fingerprints.json: %q", body)
	}
}

func TestC1800_021_ResetWithoutAFingerprintNeverWritesAnAckAndStillReportsThePruneError(t *testing.T) {
	loopRoot, loopEvolveDir := resetProject(t, unparseableState, "")
	loopRes := runLoopReset(t, loopRoot, "")
	if !strings.Contains(loopRes.stderr, "[loop] --reset: failurelog: parse state") {
		t.Errorf("loop --reset swallowed the prune error: %q", loopRes.stderr)
	}
	if strings.Contains(loopRes.stderr, "[loop] --reset --fingerprint") {
		t.Errorf("loop --reset without --fingerprint printed a fingerprint line: %q", loopRes.stderr)
	}
	if _, exists := recordedFingerprints(t, loopEvolveDir); exists {
		t.Errorf("loop --reset without --fingerprint wrote resolved-fingerprints.json")
	}
	failRoot, failEvolveDir := resetProject(t, unparseableState, "")
	failRes := runEvolve(t, nil, t.TempDir(), "failures", "reset", "--project-root", failRoot)
	if failRes.code != 1 || !strings.Contains(failRes.stderr, "parse state") {
		t.Errorf("failures reset swallowed the prune error: exit=%d stderr=%q", failRes.code, failRes.stderr)
	}
	if _, exists := recordedFingerprints(t, failEvolveDir); exists {
		t.Errorf("failures reset without --fingerprint wrote resolved-fingerprints.json")
	}
}

func TestC1800_022_LoopResetAndFailuresResetLeaveIdenticalStateAndKeepTheBaseSuccessLines(t *testing.T) {
	loopRoot, loopEvolveDir := resetProject(t, liveResetClassesState(t), "")
	loopRes := runLoopReset(t, loopRoot, "fp-acs-1800-parity")
	failRoot, failEvolveDir := resetProject(t, liveResetClassesState(t), "")
	if res := runEvolve(t, nil, t.TempDir(), "failures", "reset", "--project-root", failRoot, "--fingerprint", "fp-acs-1800-parity"); res.code != 0 {
		t.Fatalf("failures reset exit=%d stderr=%q", res.code, res.stderr)
	}
	loopClasses := strings.Join(approachClasses(t, filepath.Join(loopEvolveDir, "state.json")), ",")
	failClasses := strings.Join(approachClasses(t, filepath.Join(failEvolveDir, "state.json")), ",")
	if loopClasses != "code-build-fail" || failClasses != loopClasses {
		t.Errorf("state diverges: loop --reset=%q failures reset=%q, want both code-build-fail", loopClasses, failClasses)
	}
	for _, dir := range []string{loopEvolveDir, failEvolveDir} {
		if body, _ := recordedFingerprints(t, dir); !strings.Contains(body, "fp-acs-1800-parity") {
			t.Errorf("%s: fingerprint not acknowledged: %q", dir, body)
		}
	}
	for _, line := range []string{
		"[loop] --reset: pruned 2 failedApproaches (infrastructure-{systemic,transient} + ship-gate-config) (3→1)",
		`[loop] --reset --fingerprint: acknowledged "fp-acs-1800-parity" in resolved-fingerprints.json — blocker-breaker will exclude it going forward`,
	} {
		if !strings.Contains(loopRes.stderr, line) {
			t.Errorf("loop --reset success line changed; want %q in %q", line, loopRes.stderr)
		}
	}
}
