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
)

var expectedPhasesHappyPath = []string{"scout", "triage", "tdd", "build", "audit", "ship"}

var allCLIs = []string{"claude-p", "codex", "agy"}

func TestE2ECycleCLIMatrix(t *testing.T) {
	t.Parallel()
	t.Skip("legacy ship-script hatch removed (Stage 5.1); native CLI-matrix e2e pending port to go/test/e2e/ — see PORTING-LEDGER.md")
	if testing.Short() {
		t.Skip("E2E test; skipped in -short mode")
	}
	for _, bin := range []string{"git", "jq", "bash"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("required tool %q not on PATH; skipping E2E", bin)
		}
	}

	repoRoot := mustRepoRoot(t)
	binDir := t.TempDir()
	evolveBin := buildBinary(t, binDir, "evolve", "./cmd/evolve", repoRoot)
	fakeBin := buildBinary(t, binDir, "evolve-fake-cli", "./cmd/evolve-fake-cli", repoRoot)

	for _, cli := range allCLIs {
		cli := cli
		t.Run("offline_"+cli, func(t *testing.T) {
			t.Parallel()
			runOneCycle(t, cycleRunConfig{
				CLI:       cli,
				EvolveBin: evolveBin,
				FakeBin:   fakeBin,
				RepoRoot:  repoRoot,
				Live:      false,
			})
		})
	}

	if os.Getenv("EVOLVE_E2E_LIVE") != "1" {
		t.Logf("EVOLVE_E2E_LIVE not set; skipping live sub-tests. Set EVOLVE_E2E_LIVE=1 to run.")
		return
	}
	for _, cli := range allCLIs {
		cli := cli
		t.Run("live_"+cli, func(t *testing.T) {
			runOneCycle(t, cycleRunConfig{
				CLI:       cli,
				EvolveBin: evolveBin,
				FakeBin:   fakeBin,
				RepoRoot:  repoRoot,
				Live:      true,
			})
		})
	}
}

type cycleRunConfig struct {
	CLI       string
	EvolveBin string
	FakeBin   string
	RepoRoot  string
	Live      bool
}

func runOneCycle(t *testing.T, cfg cycleRunConfig) {
	t.Helper()
	projRoot := setupTempProject(t, cfg.RepoRoot)
	shipScript := writeFakeShipScript(t, projRoot)

	env := append(os.Environ(),
		"EVOLVE_SANDBOX=off",
		"EVOLVE_CLI="+cfg.CLI,
		"EVOLVE_PROMPTS_DIR="+cfg.RepoRoot,
		"EVOLVE_SHIP_SCRIPT="+shipScript,
		"EVOLVE_NATIVE_SHIP=0",
		"EVOLVE_RESEARCH_HOOK_DISABLED=1",
	)
	if !cfg.Live {
		env = append(env,
			"BRIDGE_TESTING=1",
			"BRIDGE_CLAUDE_BINARY="+cfg.FakeBin,
			"BRIDGE_CODEX_BINARY="+cfg.FakeBin,
			"BRIDGE_AGY_BINARY="+cfg.FakeBin,
		)
		env = append(env, isolatedHome(t)...)
	} else {
		if _, err := exec.LookPath(liveBinaryName(cfg.CLI)); err != nil {
			t.Skipf("live mode: %s binary not on PATH (%v); skipping", cfg.CLI, err)
		}
	}

	args := []string{"cycle", "run",
		"--project-root", projRoot,
		"--goal-hash", "e2etest" + cfg.CLI,
		"--evolve-dir", filepath.Join(projRoot, ".evolve"),
	}
	if cfg.Live {
		args = append(args, "--budget-usd", "0.50")
	}

	cmd := exec.Command(cfg.EvolveBin, args...)
	cmd.Env = env
	cmd.Dir = projRoot

	timeout := 60 * time.Second
	if cfg.Live {
		timeout = 5 * time.Minute
	}
	out, err := runWithTimeout(cmd, timeout)
	if err != nil {
		t.Logf("--- combined output ---\n%s", out)
		dumpWorkspaceLogs(t, projRoot)
		t.Fatalf("evolve cycle run failed: %v", err)
	}

	entries := readLedger(t, projRoot)
	if len(entries) == 0 {
		dumpWorkspaceLogs(t, projRoot)
		t.Fatalf("ledger is empty; cycle output:\n%s", out)
	}
	missingRole := false
	for _, want := range expectedPhasesHappyPath {
		if !ledgerHasRole(entries, want) {
			missingRole = true
			t.Errorf("ledger missing role=%q\nfull ledger roles: %v\ncycle output:\n%s",
				want, ledgerRoles(entries), out)
		}
	}
	if missingRole {
		dumpWorkspaceLogs(t, projRoot)
	}

	state := readState(t, projRoot)
	if state.LastCycleNumber < 1 {
		t.Errorf("state.json:lastCycleNumber=%d, want >=1", state.LastCycleNumber)
	}

	logOut, err := exec.Command("git", "-C", projRoot, "log", "--format=%s", "-1").Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(string(logOut), "e2etest") {
		t.Errorf("git log does not contain e2etest commit message; got %q", string(logOut))
	}
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
