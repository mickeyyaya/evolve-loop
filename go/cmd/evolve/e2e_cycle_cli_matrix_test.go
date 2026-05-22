// End-to-end matrix test that drives a full evolve cycle (Scout →
// Triage → TDD → Build → Audit → Ship) for each of the three CLI
// backends — claude-p, codex, agy — through the real agent-bridge,
// with the bridge's BRIDGE_TESTING=1 + BRIDGE_*_BINARY seam pointed at
// our evolve-fake-cli stub.
//
// Hybrid spend mode:
//   - Default: offline. Zero spend, deterministic, runs in CI.
//   - EVOLVE_E2E_LIVE=1: also runs ONE live cycle per CLI. agy is
//     auto-skipped if the binary is not on PATH (per CLAUDE.md
//     EVOLVE_AGY_REQUIRE_FULL=0 default).
//
// What this test does NOT cover:
//   - Intent phase (default state-machine path skips it; gated by
//     EVOLVE_REQUIRE_INTENT=1 in production).
//   - Retro phase (only invoked on Audit FAIL/WARN).
//   - The real ship.sh (overridden via EVOLVE_SHIP_SCRIPT — see
//     writeFakeShipScript below for why).
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// expectedPhasesHappyPath is what the state machine drives on PASS-all.
var expectedPhasesHappyPath = []string{"scout", "triage", "tdd", "build", "audit", "ship"}

// allCLIs is the matrix domain. The order matches the existing bridge
// driver script set (no `claude-tmux` here — tmux is for interactive
// sessions, not the headless cycle path).
var allCLIs = []string{"claude-p", "codex", "agy"}

func TestE2ECycleCLIMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("E2E test; skipped in -short mode")
	}
	// Pre-flight: required tooling on PATH.
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

	// Live opt-in: EVOLVE_E2E_LIVE=1 runs ONE real-CLI cycle per backend.
	// Costs real money (~$0.50–2.00 per cycle, capped via --budget-usd).
	// agy is auto-skipped if its binary is not on PATH.
	if os.Getenv("EVOLVE_E2E_LIVE") != "1" {
		t.Logf("EVOLVE_E2E_LIVE not set; skipping live sub-tests. Set EVOLVE_E2E_LIVE=1 to run.")
		return
	}
	for _, cli := range allCLIs {
		cli := cli
		t.Run("live_"+cli, func(t *testing.T) {
			// Do NOT parallel: live cycles can rate-limit each other on
			// the same provider, and we want clearer cost attribution.
			runOneCycle(t, cycleRunConfig{
				CLI:       cli,
				EvolveBin: evolveBin,
				FakeBin:   fakeBin, // unused in live mode, but plumbed
				RepoRoot:  repoRoot,
				Live:      true,
			})
		})
	}
}

// cycleRunConfig bundles the per-cycle parameters so the same driver
// works for both offline and live modes.
type cycleRunConfig struct {
	CLI       string
	EvolveBin string
	FakeBin   string
	RepoRoot  string
	Live      bool
}

// runOneCycle is the heart of the matrix — it sets up a fresh isolated
// project root, runs `evolve cycle run`, and asserts the ledger +
// committed-state invariants.
func runOneCycle(t *testing.T, cfg cycleRunConfig) {
	t.Helper()
	projRoot := setupTempProject(t, cfg.RepoRoot)
	shipScript := writeFakeShipScript(t, projRoot)

	env := append(os.Environ(),
		"EVOLVE_CLI="+cfg.CLI,
		"EVOLVE_PROMPTS_DIR="+cfg.RepoRoot,
		"EVOLVE_SHIP_SCRIPT="+shipScript,
		// Disable the strict audit promotion so the WARN→FAIL bump doesn't
		// surprise us; the fake emits a clean PASS verdict anyway.
		"EVOLVE_STRICT_AUDIT=0",
		// Skip the deep-research quota for this test.
		"EVOLVE_RESEARCH_HOOK_DISABLED=1",
	)
	if !cfg.Live {
		env = append(env,
			"BRIDGE_TESTING=1",
			"BRIDGE_CLAUDE_BINARY="+cfg.FakeBin,
			"BRIDGE_CODEX_BINARY="+cfg.FakeBin,
			"BRIDGE_AGY_BINARY="+cfg.FakeBin,
		)
	} else {
		// Live mode: cap the per-cycle budget hard and require the CLI
		// binary actually exists on PATH.
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

	// 1. Ledger has every phase entry the happy path runs.
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

	// 2. state.json advances.
	state := readState(t, projRoot)
	if state.LastCycleNumber < 1 {
		t.Errorf("state.json:lastCycleNumber=%d, want >=1", state.LastCycleNumber)
	}

	// 3. Final commit landed in the temp repo (fake ship.sh did the
	// commit). git log should show our message.
	logOut, err := exec.Command("git", "-C", projRoot, "log", "--format=%s", "-1").Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(string(logOut), "e2etest") {
		t.Errorf("git log does not contain e2etest commit message; got %q", string(logOut))
	}
}

// liveBinaryName maps the bridge --cli identifier back to the executable
// name we look for on PATH. claude-p uses the `claude` binary, codex
// uses `codex`, agy uses `agy`.
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

// buildBinary builds a Go binary into outDir using the repo's go.mod.
// Caches across sub-tests within the same parent (idempotent because
// the destination path is deterministic).
func buildBinary(t *testing.T, outDir, name, pkg, repoRoot string) string {
	t.Helper()
	out := filepath.Join(outDir, name)
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Dir = filepath.Join(repoRoot, "go")
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", pkg, err, combined)
	}
	return out
}

// mustRepoRoot resolves the evolve-loop repo root from this test's
// file location (go/cmd/evolve/<this>.go). Walks up until it finds the
// tools/agent-bridge/ directory.
func mustRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "tools", "agent-bridge", "bin", "bridge")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("could not locate repo root from %s", thisFile)
	return ""
}

// setupTempProject builds an isolated project root in t.TempDir() with
// everything the cycle path needs:
//   - git init + initial commit (so ship's git commit has a parent)
//   - .evolve/profiles/{intent,scout,triage,tdd,build,audit,retro}.json
//     (stubs — bridge profile loader only requires `name`)
//   - tools/agent-bridge symlinked into the real repo's copy
//   - .evolve/state.json bootstrapped to cycle 0
func setupTempProject(t *testing.T, repoRoot string) string {
	t.Helper()
	root := t.TempDir()

	// Bridge needs a tools/agent-bridge tree at the project root because
	// the Go bridge adapter resolves `<projRoot>/tools/agent-bridge/bin/bridge`.
	if err := os.MkdirAll(filepath.Join(root, "tools"), 0o755); err != nil {
		t.Fatalf("mkdir tools/: %v", err)
	}
	if err := os.Symlink(filepath.Join(repoRoot, "tools", "agent-bridge"),
		filepath.Join(root, "tools", "agent-bridge")); err != nil {
		t.Fatalf("symlink agent-bridge: %v", err)
	}

	// Stub profiles. Names match what the Go phase code constructs in
	// each phase's runner — Go uses `audit.json` / `build.json` / `tdd.json`
	// rather than the bash-era `auditor.json` / `builder.json` / `tdd-engineer.json`.
	profilesDir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	for _, name := range []string{"intent", "scout", "triage", "tdd", "build", "audit", "retrospective"} {
		path := filepath.Join(profilesDir, name+".json")
		body := fmt.Sprintf(`{"name":%q,"role":%q,"cli":"claude","model_tier_default":"sonnet","allowed_tools":["Read","Write","Bash"]}`, name, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write profile %s: %v", name, err)
		}
	}

	// .evolve/state.json — minimal seed so storage adapter doesn't bail.
	statePath := filepath.Join(root, ".evolve", "state.json")
	seed := `{"lastUpdated":"2026-01-01T00:00:00Z","lastCycleNumber":0,"version":1,"currentBatch":{"cycleAccruedCostUSD":0}}`
	if err := os.WriteFile(statePath, []byte(seed), 0o644); err != nil {
		t.Fatalf("write state.json: %v", err)
	}

	// Git init + identity + initial commit (ship phase needs a parent).
	gitInit(t, root)

	return root
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
	// .gitignore so the symlinked agent-bridge doesn't pollute the index.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"),
		[]byte("tools/\n.evolve/runs/\n.evolve/ledger.jsonl\n.evolve/cycle-state.json\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	// Seed file so the initial commit isn't empty.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# e2e\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "initial")
}

// writeFakeShipScript drops a tiny ship.sh into the temp project. It
// substitutes for the real ship.sh, which would refuse to run in this
// synthetic environment because the audit-binding checks have no real
// cycle state to bind to. The fake just stages the workspace + makes a
// commit, which is enough to validate that the Ship phase ran end-to-end.
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
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake ship: %v", err)
	}
	return path
}

// runWithTimeout runs cmd with a hard timeout, returning combined
// stdout+stderr regardless of outcome.
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
		return "", fmt.Errorf("timed out after %s", d)
	}
}

// ledgerEntry is a partial view of the ledger.jsonl rows. We only need
// the role field for assertions.
type ledgerEntry struct {
	Role string `json:"role"`
	Kind string `json:"kind"`
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

// dumpWorkspaceLogs walks .evolve/runs/cycle-*/ in projRoot and prints
// every *.log file it finds. Bridge truncates stderr at 200 chars in
// its returned error; the full stderr lives on disk in <agent>-stderr.log.
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

// errUnused is a compile-time guard so go vet doesn't complain about the
// errors import when no error-typed value flows through the file.
var _ = errors.New
