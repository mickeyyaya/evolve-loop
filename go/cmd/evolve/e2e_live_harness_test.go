// Shared robustness harness for the LIVE e2e tiers (T0–T3) that exercise real
// LLM CLIs. See docs/testing/live-e2e-plan.md.
//
// The whole point of this file is to make live testing ROBUST rather than
// flaky: auth-preflight that skips (never fails), a transient-vs-contract
// failure classifier with bounded retry+backoff, a cumulative budget ceiling
// with a per-CLI cost report, and failure-artifact capture for triage. Live
// assertions are STRUCTURAL (artifact parses, verdict ∈ valid set, ledger
// roles) — never model wording.
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

// liveGate skips the test unless the named env var is "1". The single switch
// that keeps live (spending) tests dormant in default/CI runs.
func liveGate(t *testing.T, env string) {
	t.Helper()
	if testing.Short() {
		t.Skipf("%s: live e2e skipped in -short mode", env)
	}
	if os.Getenv(env) != "1" {
		t.Skipf("%s not set; set %s=1 to run live (spends real quota)", env, env)
	}
}

// liveCLI describes one real CLI target: its driver name, the binary to probe,
// its cheapest model tier, and its model-vendor family (for cross-family soak).
type liveCLI struct {
	Driver    string // bridge --cli identifier (claude-p, codex, agy, ollama-tmux, *-tmux)
	Binary    string // executable to probe on PATH
	CheapTier string // cheapest model tier for this CLI (fast/haiku/…)
	Family    string // anthropic | openai | google | local
}

// liveHeadlessCLIs / liveTmuxCLIs are the matrix domains. ollama is tmux-only
// (no headless driver), and is FREE — the canary that proves the live path at
// zero spend.
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

// liveCLIAvailable reports whether a live run of cli can even be attempted.
// Binary-on-PATH is necessary; auth is assumed when the operator opted in via
// the tier gate (a real auth check would itself cost a call). The claude
// keychain false-negative (`evolve setup detect` reports MISCONFIGURED while
// the launching Claude session's OAuth works) is handled by trusting a present
// `claude` binary — NEVER hard-failing on it. Returns (ok, reasonToSkip).
func liveCLIAvailable(cli liveCLI) (bool, string) {
	if _, err := exec.LookPath(cli.Binary); err != nil {
		return false, fmt.Sprintf("%s binary not on PATH", cli.Binary)
	}
	return true, ""
}

// transientMarkers are substrings that mark a provider/infrastructure hiccup
// (retryable) rather than a broken contract (hard fail). Conservative on
// purpose: when in doubt the failure is treated as a real contract break.
var transientMarkers = []string{
	"429", "rate limit", "rate-limit", "ratelimit", "too many requests",
	"overloaded", "RESOURCE_EXHAUSTED", "quota", "service unavailable", "503", "502",
	"connection reset", "connection refused", "i/o timeout", "timed out", "timeout",
	"EOF", "temporarily unavailable", "network",
	"exit=81", "exit=124", "ExitArtifactTimeout", "ExitREPLBootTimeout", "bridge artifact timeout",
	// Subscription usage/quota exhaustion. The CLI booted and authenticated;
	// the account is simply capped until a reset date — a provider limit, not a
	// broken contract. Observed live 2026-05-30: codex "You've hit your usage
	// limit. Upgrade to Plus … try again at Jun 4th". claude/agy use similar
	// phrasings ("usage limit reached", "limit will reset"). These are
	// quota-SPECIFIC substrings unlikely in model-generated output. We
	// deliberately exclude the codex message's generic "try again at" tail:
	// phaseStderrTail folds the model's own stdout/stderr into the classified
	// string, and that ordinary English phrase could mask a real contract break
	// (the quota-specific markers below already catch the codex/claude/agy caps).
	"usage limit", "upgrade to plus", "limit will reset", "plan limit",
}

// isTransientFailure classifies combined cycle output. Transient → retry then
// quarantine-skip; otherwise the failure is a contract break → hard fail.
func isTransientFailure(out string) bool {
	low := strings.ToLower(out)
	for _, m := range transientMarkers {
		if strings.Contains(low, strings.ToLower(m)) {
			return true
		}
	}
	return false
}

// isTransient folds the error text into the classified output. A timeout from
// runWithTimeout carries its signal in err (not stdout), so classifying stdout
// alone would mis-grade a nested-CLI hang as a contract break. Callers that have
// both out and err should use this; classify the union.
func isTransient(out string, err error) bool {
	s := out
	if err != nil {
		s += " " + err.Error()
	}
	return isTransientFailure(s)
}

// --- cumulative budget ceiling -------------------------------------------------

var (
	liveBudgetMu     sync.Mutex
	liveSpentUSD     float64
	liveBudgetCapUSD = -1.0 // <0 = uncapped (parsed lazily)
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

// liveBudgetRemaining reports whether more spend is allowed and how much room
// is left. Uncapped ⇒ always (true, +inf-ish).
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

// ledgerTotalCost sums cost_usd across all ledger entries.
func ledgerTotalCost(entries []ledgerEntry) float64 {
	var sum float64
	for _, e := range entries {
		sum += e.CostUSD
	}
	return sum
}

// --- failure-artifact capture --------------------------------------------------

// captureLiveFailure copies a failed cycle's workspace logs/artifacts into a
// retained dir under the repo's testdata so a flaky/real live failure is
// triageable after the temp project is gone. Returns the retained path.
func captureLiveFailure(t *testing.T, repoRoot, projRoot, label string) string {
	t.Helper()
	dst := filepath.Join(repoRoot, "go", "testdata", "live-failures",
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

// --- live cycle runner with retry ---------------------------------------------

// liveCycleCfg configures one live `evolve cycle run` attempt.
type liveCycleCfg struct {
	EvolveBin string
	RepoRoot  string
	Driver    string // EVOLVE_CLI for every phase
	Tier      string // model_tier_default written into profiles
	GoalHash  string
	ExtraEnv  []string // e.g. EVOLVE_<AGENT>_CLI for cross-family
	Timeout   time.Duration
	BudgetUSD float64 // per-cycle --budget-usd cap
}

// liveResult is the observable outcome of a live cycle.
type liveResult struct {
	Entries            []ledgerEntry
	Shipped            bool
	Cost               float64
	Out                string
	Err                error
	ProjRoot           string // the temp project (alive until the test ends) for artifact capture
	TransientExhausted bool   // transient retries exhausted → caller should Skip
}

// requireTmuxForLive skips when tmux is absent — the *-tmux live drivers need it.
func requireTmuxForLive(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not on PATH; skipping live tmux drivers")
	}
}

// runLiveCycle runs one live cycle with bounded retry on transient failures.
// Contract failures return immediately (caller hard-fails); transient failures
// retry with exponential backoff up to EVOLVE_E2E_LIVE_RETRIES (default 2),
// then return with TransientExhausted=true so the caller quarantine-skips.
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
			return res // contract failure — surface to caller immediately
		}
		t.Logf("[live-retry] %s attempt %d/%d hit a transient failure; backing off", cfg.GoalHash, attempt, maxAttempts)
		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt*attempt*5) * time.Second) // 5s, 20s, …
		}
	}
	res.TransientExhausted = true
	return res
}

// runLiveCycleOnce performs a single live cycle attempt (no retry).
func runLiveCycleOnce(t *testing.T, cfg liveCycleCfg) liveResult {
	t.Helper()
	projRoot := setupTempProject(t, cfg.RepoRoot)
	writeLiveProfiles(t, projRoot, cfg.Driver, cfg.Tier)
	fakeHome := t.TempDir()

	env := append(os.Environ(),
		"EVOLVE_CLI="+cfg.Driver,
		"EVOLVE_PROMPTS_DIR="+cfg.RepoRoot,
		// Native-only ship: the legacy EVOLVE_NATIVE_SHIP=0 + EVOLVE_SHIP_SCRIPT
		// fake-ship hatch was removed in the Go-only consolidation, so a live
		// cycle runs the native shipper. Shipped (below) is OBSERVED, not asserted.
		"EVOLVE_STRICT_AUDIT=0",
		"EVOLVE_RESEARCH_HOOK_DISABLED=1",
		// Redirect codex/agy preflight writes away from the operator's real home.
		"EVOLVE_CODEX_CONFIG_PATH="+filepath.Join(fakeHome, ".codex", "config.toml"),
		// NOTE: do NOT override HOME — the real CLIs need their auth/credentials,
		// which live in the operator's real ~/. (Offline tests redirect HOME; live
		// must not.)
	)
	env = append(env, cfg.ExtraEnv...)

	args := []string{"cycle", "run",
		"--project-root", projRoot,
		"--goal-hash", cfg.GoalHash,
		"--evolve-dir", filepath.Join(projRoot, ".evolve"),
		"--budget-usd", strconv.FormatFloat(cfg.BudgetUSD, 'f', 2, 64),
	}
	cmd := exec.Command(cfg.EvolveBin, args...)
	cmd.Env = env
	cmd.Dir = projRoot

	out, err := runWithTimeout(cmd, cfg.Timeout)
	// The classifier matches against Out, but the `evolve cycle run` subprocess
	// only surfaces a structured "bridge: launch exit=1" line — the real provider
	// message (e.g. codex "You've hit your usage limit") lands in the per-phase
	// *-stderr.log artifact, not the subprocess stdout. Fold those artifacts into
	// Out so a quota/provider failure is classified transient instead of being
	// mis-graded a contract break. (Confirmed live 2026-05-30: without this the
	// codex quota cap red-failed the suite.)
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

// phaseStderrTail concatenates every per-phase *-stderr.log under .evolve/runs/
// so the flake classifier sees the real CLI/provider error text (which the
// orchestrator captures to artifacts, not to its own stdout). Best-effort: a
// missing runs dir or unreadable file contributes nothing.
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

// readLedgerSafe is readLedger without t.Fatal — a live attempt may legitimately
// leave no ledger (early provider failure), which the caller classifies.
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

// liveBridgeLaunch runs ONE `evolve bridge launch` with a trivial "write PONG"
// prompt at the given concrete model — the cheapest possible real-CLI call.
// Returns the artifact byte size (-1 if not written), combined output, and err.
// Shared by the T0 smoke and T2 model-tier matrix.
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

// writeLiveProfiles rewrites the phase profiles with model_tier_default=tier so
// the live cycle uses each CLI's cheapest model. cli is left at "claude" in the
// file (EVOLVE_CLI overrides it per phase), but allowed_clis=["all"] keeps the
// floor from rejecting whatever driver the env selects.
func writeLiveProfiles(t *testing.T, projRoot, driver, tier string) {
	t.Helper()
	profilesDir := filepath.Join(projRoot, ".evolve", "profiles")
	for _, name := range []string{"intent", "scout", "triage", "tdd-engineer", "builder", "auditor", "retrospective"} {
		body := fmt.Sprintf(
			`{"name":%q,"role":%q,"cli":%q,"allowed_clis":["all"],"model_tier_default":%q,"model_tier_envelope":{"min":"fast","default":%q,"max":"deep"},"allowed_tools":["Read","Write","Bash"]}`,
			name, name, driver, tier, tier)
		if err := os.WriteFile(filepath.Join(profilesDir, name+".json"), []byte(body), 0o644); err != nil {
			t.Fatalf("write live profile %s: %v", name, err)
		}
	}
}
