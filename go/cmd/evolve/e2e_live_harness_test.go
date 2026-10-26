//go:build e2e

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func liveGate(t *testing.T, env string) {
	t.Helper()
	if testing.Short() {
		t.Skipf("%s: live e2e skipped in -short mode", env)
	}
	if os.Getenv(env) != "1" {
		t.Skipf("%s not set; set %s=1 to run live (spends real quota)", env, env)
	}
}

type liveCLI struct {
	Driver    string
	Binary    string
	CheapTier string
	Family    string
}

var liveHeadlessCLIs = []liveCLI{
	{Driver: "claude-p", Binary: "claude", CheapTier: "fast", Family: "anthropic"},
	{Driver: "codex", Binary: "codex", CheapTier: "fast", Family: "openai"},
	{Driver: "agy", Binary: "agy", CheapTier: "fast", Family: "google"},
}

var liveTmuxCLIs = []liveCLI{
	{Driver: "claude-tmux", Binary: "claude", CheapTier: "fast", Family: "anthropic"},
	{Driver: "codex-tmux", Binary: "codex", CheapTier: "fast", Family: "openai"},
	{Driver: "agy-tmux", Binary: "agy", CheapTier: "fast", Family: "google"},
	{Driver: "ollama-tmux", Binary: "ollama", CheapTier: "fast", Family: "local"},
}

func liveCLIAvailable(cli liveCLI) (bool, string) {
	if _, err := exec.LookPath(cli.Binary); err != nil {
		return false, fmt.Sprintf("%s binary not on PATH", cli.Binary)
	}
	return true, ""
}

var transientMarkers = []string{
	"429", "rate limit", "rate-limit", "ratelimit", "too many requests",
	"overloaded", "RESOURCE_EXHAUSTED", "quota", "service unavailable", "503", "502",
	"connection reset", "connection refused", "i/o timeout", "timed out", "timeout",
	"EOF", "temporarily unavailable", "network",
	"exit=81", "exit=124", "ExitArtifactTimeout", "ExitREPLBootTimeout", "bridge artifact timeout",
	"usage limit", "upgrade to plus", "limit will reset", "plan limit",
}

func isTransientFailure(out string) bool {
	low := strings.ToLower(out)
	for _, m := range transientMarkers {
		if strings.Contains(low, strings.ToLower(m)) {
			return true
		}
	}
	return false
}

func isTransient(out string, err error) bool {
	s := out
	if err != nil {
		s += " " + err.Error()
	}
	return isTransientFailure(s)
}

var (
	liveBudgetMu     sync.Mutex
	liveSpentUSD     float64
	liveBudgetCapUSD = -1.0
	liveBudgetOnce   sync.Once
)

func liveBudgetCap() float64 {
	liveBudgetOnce.Do(func() {
		if v := os.Getenv("EVOLVE_E2E_LIVE_BUDGET_USD"); v != "" {
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				liveBudgetCapUSD = f
			}
		}
	})
	return liveBudgetCapUSD
}

func liveBudgetRemaining() (bool, float64) {
	cap := liveBudgetCap()
	if cap < 0 {
		return true, 1e9
	}
	liveBudgetMu.Lock()
	defer liveBudgetMu.Unlock()
	return liveSpentUSD < cap, cap - liveSpentUSD
}

func recordLiveSpend(usd float64) {
	liveBudgetMu.Lock()
	liveSpentUSD += usd
	total := liveSpentUSD
	liveBudgetMu.Unlock()
	if cap := liveBudgetCap(); cap >= 0 {
		fmt.Printf("[live-budget] +$%.4f → $%.4f / $%.2f cap\n", usd, total, cap)
	} else {
		fmt.Printf("[live-budget] +$%.4f → $%.4f (uncapped)\n", usd, total)
	}
}

func ledgerTotalCost(entries []ledgerEntry) float64 {
	var sum float64
	for _, e := range entries {
		sum += e.CostUSD
	}
	return sum
}

func captureLiveFailure(t *testing.T, projRoot, label string) string {
	t.Helper()
	dst := filepath.Join(os.TempDir(), "evolve-live-failures",
		fmt.Sprintf("%s-%d", strings.ReplaceAll(label, "/", "_"), os.Getpid()))
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Logf("captureLiveFailure: mkdir %s: %v", dst, err)
		return ""
	}
	runs := filepath.Join(projRoot, ".evolve", "runs")
	_ = filepath.Walk(runs, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(projRoot, p)
		out := filepath.Join(dst, strings.ReplaceAll(rel, string(os.PathSeparator), "_"))
		if b, rerr := os.ReadFile(p); rerr == nil {
			_ = os.WriteFile(out, b, 0o644)
		}
		return nil
	})
	if b, err := os.ReadFile(filepath.Join(projRoot, ".evolve", "ledger.jsonl")); err == nil {
		_ = os.WriteFile(filepath.Join(dst, "ledger.jsonl"), b, 0o644)
	}
	t.Logf("[live-capture] failure artifacts for %s → %s", label, dst)
	return dst
}

type liveCycleCfg struct {
	EvolveBin string
	RepoRoot  string
	Driver    string
	Tier      string
	GoalHash  string
	Goal      string
	ExtraEnv  []string
	Timeout   time.Duration
}

type liveResult struct {
	Entries            []ledgerEntry
	Shipped            bool
	Cost               float64
	Out                string
	Err                error
	ProjRoot           string
	TransientExhausted bool
}

func requireTmuxForLive(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not on PATH; skipping live tmux drivers")
	}
}

func runLiveCycle(t *testing.T, cfg liveCycleCfg) liveResult {
	t.Helper()
	if ok, _ := liveBudgetRemaining(); !ok {
		t.Skipf("live budget exhausted ($%.2f cap); skipping %s", liveBudgetCap(), cfg.GoalHash)
	}
	maxAttempts := 2
	if v := os.Getenv("EVOLVE_E2E_LIVE_RETRIES"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 1 {
			maxAttempts = n
		}
	}
	var res liveResult
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		res = runLiveCycleOnce(t, cfg)
		recordLiveSpend(res.Cost)
		if res.Err == nil {
			return res
		}
		if !isTransientFailure(res.Out) {
			return res
		}
		t.Logf("[live-retry] %s attempt %d/%d hit a transient failure; backing off", cfg.GoalHash, attempt, maxAttempts)
		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt*attempt*5) * time.Second)
		}
	}
	res.TransientExhausted = true
	return res
}

func runLiveCycleOnce(t *testing.T, cfg liveCycleCfg) liveResult {
	t.Helper()
	projRoot := setupTempProject(t, cfg.RepoRoot)
	writeLiveProfiles(t, projRoot, cfg.Driver, cfg.Tier)
	fakeHome := t.TempDir()

	env := append(os.Environ(),
		"EVOLVE_CLI="+cfg.Driver,
		"EVOLVE_RESEARCH_HOOK_DISABLED=1",
		"EVOLVE_CODEX_CONFIG_PATH="+filepath.Join(fakeHome, ".codex", "config.toml"),
	)
	env = append(env, cfg.ExtraEnv...)

	args := []string{"cycle", "run",
		"--project-root", projRoot,
		"--goal-hash", cfg.GoalHash,
		"--evolve-dir", filepath.Join(projRoot, ".evolve"),
	}
	if cfg.Goal != "" {
		args = append(args, "--goal", cfg.Goal)
	}
	cmd := exec.Command(cfg.EvolveBin, args...)
	cmd.Env = env
	cmd.Dir = projRoot

	out, err := runWithTimeout(cmd, cfg.Timeout)
	out += "\n" + phaseStderrTail(projRoot)
	entries := readLedgerSafe(projRoot)
	return liveResult{
		Entries:  entries,
		Shipped:  gitLogContains(t, projRoot, cfg.GoalHash),
		Cost:     ledgerTotalCost(entries),
		Out:      out,
		Err:      err,
		ProjRoot: projRoot,
	}
}

func phaseStderrTail(projRoot string) string {
	runs := filepath.Join(projRoot, ".evolve", "runs")
	var b strings.Builder
	_ = filepath.Walk(runs, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, "-stderr.log") {
			return nil
		}
		if data, rerr := os.ReadFile(p); rerr == nil {
			b.Write(data)
			b.WriteByte('\n')
		}
		return nil
	})
	return b.String()
}

func readLedgerSafe(projRoot string) []ledgerEntry {
	path := filepath.Join(projRoot, ".evolve", "ledger.jsonl")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []ledgerEntry
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e ledgerEntry
		if json.Unmarshal([]byte(line), &e) == nil {
			out = append(out, e)
		}
	}
	return out
}

func liveBridgeLaunch(t *testing.T, evolveBin, driver, model string, timeout time.Duration) (int64, string, error) {
	t.Helper()
	dir := t.TempDir()
	ws := filepath.Join(dir, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(ws, "smoke-artifact.md")
	promptFile := filepath.Join(dir, "prompt.txt")
	prompt := fmt.Sprintf("# Live Smoke\n\nAutomated connectivity check. Write the single word PONG "+
		"to the file %s and then stop. No other actions.\n", artifact)
	if err := os.WriteFile(promptFile, []byte(prompt), 0o644); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(dir, "smoke-profile.json")
	if err := os.WriteFile(profile,
		[]byte(`{"name":"smoke","allowed_clis":["all"],"allowed_tools":["Read","Write","Bash"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeHome := t.TempDir()
	env := append(os.Environ(), "EVOLVE_CODEX_CONFIG_PATH="+filepath.Join(fakeHome, ".codex", "config.toml"))

	cmd := exec.Command(evolveBin, "bridge", "launch",
		"--cli="+driver, "--profile="+profile, "--model="+model,
		"--prompt-file="+promptFile, "--workspace="+ws, "--artifact="+artifact,
		"--worktree="+dir, "--cycle=0", "--allow-bypass",
	)
	cmd.Env = env
	cmd.Dir = dir
	out, err := runWithTimeout(cmd, timeout)
	size := int64(-1)
	if info, statErr := os.Stat(artifact); statErr == nil {
		size = info.Size()
	}
	return size, out, err
}

func writeLiveProfiles(t *testing.T, projRoot, driver, tier string) {
	t.Helper()
	profilesDir := filepath.Join(projRoot, ".evolve", "profiles")
	for _, name := range []string{"intent", "scout", "triage", "tdd-engineer", "builder", "auditor", "retrospective", "router"} {
		body := fmt.Sprintf(
			`{"name":%q,"role":%q,"cli":%q,"allowed_clis":["all"],"model_tier_default":%q,"model_tier_envelope":{"min":"fast","default":%q,"max":"deep"},"allowed_tools":["Read","Write","Bash"]}`,
			name, name, driver, tier, tier)
		if err := os.WriteFile(filepath.Join(profilesDir, name+".json"), []byte(body), 0o644); err != nil {
			t.Fatalf("write live profile %s: %v", name, err)
		}
	}
}
