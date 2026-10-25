//go:build e2e

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gotooltest"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
)

var expectedPhasesHappyPath = []string{"scout", "triage", "tdd", "build", "audit", "ship"}

type matrixRow struct {
	id, family, driver string
}

var substitutionRows = []matrixRow{
	{id: "M1", family: "claude", driver: "claude-p"},
	{id: "M2", family: "agy", driver: "agy"},
	{id: "M3", family: "agy-claude", driver: "agy-claude-tmux"},
	{id: "M4", family: "codex", driver: "codex"},
	{id: "M5", family: "ollama", driver: "ollama-tmux"},
}

var stubBinaries = []string{"claude", "agy", "codex", "ollama", "gemini"}

func TestE2ECycleCLIMatrix(t *testing.T) {
	t.Parallel()
	evolveBin, fakeBin, repoRoot := mustBuildPipelineBins(t)
	for _, row := range substitutionRows {
		t.Run(row.id+"_"+row.family, func(t *testing.T) {
			t.Parallel()
			run := runMatrixCycle(t, row, evolveBin, fakeBin, repoRoot)
			failure := run.substitutionFailure(row)
			reportRow(t)(isolation.Judge(row.id, failure))
			if row.id != "M1" {
				return
			}
			if failure != nil || len(run.calls) == 0 {
				t.Fatalf("rows N1 and N4 read the stub records of a green M1 run; M1 failed or ran no seat: %v", failure)
			}
			reportRow(t)(isolation.Judge("N1", run.envLeak()))
			reportRow(t)(isolation.Judge("N4", run.agentRunNotRefused()))
			reportRow(t)(isolation.Judge("N6", run.indexMissing()))
		})
	}
}

func reportRow(t *testing.T) func(string, error) {
	t.Helper()
	return func(note string, err error) {
		t.Helper()
		if err != nil {
			t.Error(err)
			return
		}
		t.Log(note)
	}
}

type matrixRun struct {
	projRoot string
	err      error
	out      string
	entries  []ledgerEntry
	calls    []stubCall
}

type stubCall struct {
	binary     string
	claudeCode string
	depth      string
}

func runMatrixCycle(t *testing.T, row matrixRow, evolveBin, fakeBin, repoRoot string) matrixRun {
	t.Helper()
	projRoot := setupTempProject(t, repoRoot)
	writeMatrixProject(t, projRoot, row.driver)
	binDir := t.TempDir()
	callLog := filepath.Join(t.TempDir(), "calls.log")
	env := append(os.Environ(),
		"CLAUDECODE=1",
		"EVOLVE_PROMPTS_DIR="+repoRoot,
		"EVOLVE_RESEARCH_HOOK_DISABLED=1",
		"EVOLVE_SANDBOX=off",
		"BRIDGE_TESTING=1",
	)
	for _, bin := range stubBinaries {
		path := filepath.Join(binDir, bin)
		fakeclitest.Install(t, path, stubScript(bin, fakeBin, callLog))
		env = append(env, "BRIDGE_"+strings.ToUpper(bin)+"_BINARY="+path)
	}
	fakeclitest.Install(t, filepath.Join(binDir, "tmux"), tmuxRefusal)
	env = append(env, isolatedHome(t)...)
	env = append(withoutEnv(env, "PATH"), "PATH="+stubFirstPath(t, binDir))
	cmd := exec.Command(evolveBin, "cycle", "run",
		"--project-root", projRoot,
		"--goal-hash", "e2ematrix"+strings.ToLower(row.id),
		"--evolve-dir", filepath.Join(projRoot, ".evolve"),
	)
	cmd.Env = withoutEnv(env, "EVOLVE_CLI")
	cmd.Dir = projRoot
	out, err := runWithTimeout(cmd, 300*time.Second)
	if err != nil && strings.HasPrefix(err.Error(), "timed out") {
		t.Fatalf("inconclusive: the %s cycle timed out, which is not the routing gap of the row", row.family)
	}
	run := matrixRun{projRoot: projRoot, err: err, out: out, calls: readStubCalls(t, callLog)}
	if _, statErr := os.Stat(filepath.Join(projRoot, ".evolve", "ledger.jsonl")); statErr == nil {
		run.entries = readLedger(t, projRoot)
	}
	return run
}

func stubFirstPath(t *testing.T, binDir string) string {
	t.Helper()
	dirs := []string{binDir}
	for _, tool := range []string{"go", "git", "bash"} {
		path, err := exec.LookPath(tool)
		if err != nil {
			t.Fatalf("the matrix cycle needs %s: %v", tool, err)
		}
		dirs = append(dirs, filepath.Dir(path))
	}
	return strings.Join(append(dirs, "/usr/bin", "/bin", "/usr/sbin", "/sbin"), string(os.PathListSeparator))
}

const tmuxRefusal = "#!/bin/sh\necho 'the matrix harness has no tmux REPL fake' >&2\nexit 1\n"

func stubScript(bin, fakeBin, callLog string) string {
	return "#!/bin/sh\n" +
		`printf '%s CLAUDECODE=%s EVOLVE_DISPATCH_DEPTH=%s\n' ` + bin + ` "${CLAUDECODE-unset}" "${EVOLVE_DISPATCH_DEPTH-unset}" >> "` + callLog + `"` + "\n" +
		`exec "` + fakeBin + `" "$@"` + "\n"
}

func withoutEnv(env []string, key string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, key+"=") {
			out = append(out, kv)
		}
	}
	return out
}

func writeMatrixProject(t *testing.T, projRoot, driver string) {
	t.Helper()
	writeProjectPolicy(t, projRoot, map[string]any{
		"cli_routing": map[string]any{"clis": []string{driver}, "default": []string{driver}, "floors": map[string]any{}},
		"isolation":   map[string]any{"i2": map[string]any{"stage": "enforce"}},
	})
	profilesDir := filepath.Join(projRoot, ".evolve", "profiles")
	for _, name := range []string{"intent", "scout", "triage", "tdd-engineer", "builder", "auditor", "retrospective"} {
		body := fmt.Sprintf(`{"name":%q,"role":%q,"cli":%q,"model_tier_default":"sonnet","allowed_tools":["Read","Write","Bash"]}`, name, name, driver)
		if err := os.WriteFile(filepath.Join(profilesDir, name+".json"), []byte(body), 0o644); err != nil {
			t.Fatalf("write profile %s: %v", name, err)
		}
	}
}

func readStubCalls(t *testing.T, path string) []stubCall {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("read the stub call log: %v", err)
	}
	var calls []stubCall
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		f := strings.Fields(line)
		if len(f) != 3 {
			t.Fatalf("stub call log line %q", line)
		}
		calls = append(calls, stubCall{binary: f[0], claudeCode: strings.TrimPrefix(f[1], "CLAUDECODE="), depth: strings.TrimPrefix(f[2], "EVOLVE_DISPATCH_DEPTH=")})
	}
	return calls
}

func (r matrixRun) substitutionFailure(row matrixRow) error {
	if r.err != nil {
		return fmt.Errorf("evolve cycle run with %s as the only route: %v: %s\n%s", row.family, r.err, lastLineWith(r.out, "evolve cycle run:"), lastN(r.out, 1500))
	}
	var missing []string
	for _, role := range expectedPhasesHappyPath {
		if !ledgerHasRole(r.entries, role) {
			missing = append(missing, role)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("the ledger misses roles %v; roles %v", missing, ledgerRoles(r.entries))
	}
	want := strings.Split(row.driver, "-")[0]
	if len(r.calls) == 0 {
		return fmt.Errorf("no stub CLI ran, so no seat ran on %s", row.family)
	}
	for _, c := range r.calls {
		if c.binary != want {
			return fmt.Errorf("a seat ran the %s binary; %s is the only route", c.binary, row.family)
		}
	}
	return nil
}

func lastLineWith(out, marker string) string {
	lines := strings.Split(out, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], marker) {
			return strings.TrimSpace(lines[i])
		}
	}
	return "no line names the cause"
}

func (r matrixRun) indexMissing() error {
	workspaces, err := filepath.Glob(filepath.Join(r.projRoot, ".evolve", "runs", "cycle-*"))
	if err != nil || len(workspaces) == 0 {
		return fmt.Errorf("inconclusive: the M1 run left no cycle workspace (%v)", err)
	}
	indexes, err := filepath.Glob(filepath.Join(r.projRoot, ".evolve", "runs", "cycle-*", "inputs", "index.json"))
	if err != nil || len(indexes) == 0 {
		return fmt.Errorf("no inputs/index.json in the %d cycle workspace(s) of the M1 run, so a retry has no index to open its lineage through (%v)", len(workspaces), err)
	}
	return nil
}

func (r matrixRun) envLeak() error {
	for _, c := range r.calls {
		if c.claudeCode != "unset" {
			return fmt.Errorf("CLAUDECODE=%s of the parent reached the %s seat", c.claudeCode, c.binary)
		}
	}
	return nil
}

func (r matrixRun) agentRunNotRefused() error {
	for _, c := range r.calls {
		env := map[string]string{"EVOLVE_DISPATCH_DEPTH": c.depth}
		if c.depth == "unset" {
			env = map[string]string{}
		}
		if subagent.ReadDispatchDepth(func(k string) string { return env[k] }) == 0 {
			return fmt.Errorf("a %s phase pane has dispatch depth 0 (EVOLVE_DISPATCH_DEPTH=%s), so the depth refusal of evolve agent run (subagent.ReadDispatchDepth) does not fire inside it", c.binary, c.depth)
		}
	}
	return nil
}

func liveBinaryName(cli string) string {
	switch cli {
	case "claude-p", "claude-tmux":
		return "claude"
	case "codex", "codex-tmux":
		return "codex"
	case "agy", "agy-tmux":
		return "agy"
	default:
		return cli
	}
}

func buildBinary(t *testing.T, _ string, name, pkg, repoRoot string) string {
	t.Helper()
	out, err := builtBinaries.Build(gotooltest.Spec{Dir: filepath.Join(repoRoot, "go"), Pkg: pkg, Name: name})
	if err != nil {
		t.Fatalf("go build %s: %v", pkg, err)
	}
	return out
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go", "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("could not locate repo root from %s", thisFile)
	return ""
}

func setupTempProject(t *testing.T, repoRoot string) string {
	t.Helper()
	_ = repoRoot
	root := t.TempDir()

	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	writeProjectPolicy(t, root, nil)

	profilesDir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	for _, name := range []string{"intent", "scout", "triage", "tdd-engineer", "builder", "auditor", "retrospective"} {
		path := filepath.Join(profilesDir, name+".json")
		body := fmt.Sprintf(`{"name":%q,"role":%q,"cli":"claude","model_tier_default":"sonnet","allowed_tools":["Read","Write","Bash"]}`, name, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write profile %s: %v", name, err)
		}
	}

	statePath := filepath.Join(root, ".evolve", "state.json")
	seed := `{"lastUpdated":"2026-01-01T00:00:00Z","lastCycleNumber":0,"version":1,"currentBatch":{"cycleAccruedCostUSD":0}}`
	if err := os.WriteFile(statePath, []byte(seed), 0o644); err != nil {
		t.Fatalf("write state.json: %v", err)
	}

	for rel, body := range map[string]string{
		"go/go.mod": "module e2e.local/fixture\n\ngo 1.23\n",
		"go/acs/regression/auditfixture/predicate_test.go": `//go:build acs

package auditfixture

import (
	"os"
	"strings"
	"testing"
)

func TestAuditFixture(t *testing.T) {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("FAKE_CLI_AUDIT_VERDICT")), "FAIL") {
		t.Fatal("synthetic e2e failing predicate requested by the fixture")
	}
}
`,
	} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir predicate fixture: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write predicate fixture %s: %v", rel, err)
		}
	}

	gitInit(t, root)

	return root
}

func writeProjectPolicy(t *testing.T, root string, extra map[string]any) {
	t.Helper()
	policy := map[string]any{
		"catalog":    map[string]any{"auto_refresh": false},
		"cli_health": map[string]any{"usage_evidence_timeout_s": 1},
	}
	for key, block := range extra {
		policy[key] = block
	}
	body, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("marshal policy.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), append(body, '\n'), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}
}

func gitInit(t *testing.T, root string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "e2e@test.local")
	run("config", "user.name", "E2E Test")
	run("config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"),
		[]byte("tools/\n.evolve/\ngo/bin/\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# e2e\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "initial")
}

func writeFakeShipScript(t *testing.T, projRoot string) string {
	t.Helper()
	path := filepath.Join(projRoot, "fake-ship.sh")
	body := `#!/usr/bin/env bash
set -euo pipefail
class=""
msg=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --class) class="$2"; shift 2 ;;
    *) msg="$1"; shift ;;
  esac
done
[[ -n "$msg" ]] || { echo "fake-ship: empty message" >&2; exit 1; }
git add -A
git commit --allow-empty -m "$msg" >/dev/null
echo "fake-ship: committed class=$class msg=$msg"
`
	fakeclitest.Install(t, path, body)
	return path
}

func runWithTimeout(cmd *exec.Cmd, d time.Duration) (string, error) {
	type result struct {
		out []byte
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := cmd.CombinedOutput()
		done <- result{out: out, err: err}
	}()
	select {
	case r := <-done:
		return string(r.out), r.err
	case <-time.After(d):
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
		}
		return "", fmt.Errorf("timed out after %s", d)
	}
}

type ledgerEntry struct {
	Role    string  `json:"role"`
	Kind    string  `json:"kind"`
	CostUSD float64 `json:"cost_usd"`
}

func readLedger(t *testing.T, projRoot string) []ledgerEntry {
	t.Helper()
	path := filepath.Join(projRoot, ".evolve", "ledger.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	defer f.Close()
	var out []ledgerEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e ledgerEntry
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		if err := json.Unmarshal(line, &e); err != nil {
			t.Errorf("ledger line not JSON: %q (%v)", line, err)
			continue
		}
		out = append(out, e)
	}
	return out
}

func ledgerHasRole(entries []ledgerEntry, role string) bool {
	for _, e := range entries {
		if e.Role == role {
			return true
		}
	}
	return false
}

func ledgerRoles(entries []ledgerEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Role
	}
	return out
}

type stateFile struct {
	LastCycleNumber int `json:"lastCycleNumber"`
}

func readState(t *testing.T, projRoot string) stateFile {
	t.Helper()
	path := filepath.Join(projRoot, ".evolve", "state.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var s stateFile
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	return s
}

func dumpWorkspaceLogs(t *testing.T, projRoot string) {
	runs := filepath.Join(projRoot, ".evolve", "runs")
	_ = filepath.Walk(runs, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".log") {
			return nil
		}
		b, _ := os.ReadFile(path)
		t.Logf("--- %s ---\n%s", path, string(b))
		return nil
	})
}
