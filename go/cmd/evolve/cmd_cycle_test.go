package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// cmd_cycle_test.go — `evolve cycle reset` seals an unfinished cycle
// (history preserved) and advances the cycle number. Mirrors the
// flag-parsing + temp-dir conventions of the other cmd_*_test.go files.

func TestParseGateStage(t *testing.T) {
	for input, want := range map[string]config.Stage{
		"off":     config.StageOff,
		"0":       config.StageOff,
		"shadow":  config.StageShadow,
		"enforce": config.StageEnforce,
		"unknown": config.StageOff,
	} {
		if got := parseGateStage(input); got != want {
			t.Errorf("parseGateStage(%q) = %v, want %v", input, got, want)
		}
	}
}

func seedResetDir(t *testing.T, cycleID, lastCycle int) (projectRoot, evolveDir string) {
	t.Helper()
	projectRoot = t.TempDir()
	evolveDir = filepath.Join(projectRoot, ".evolve")
	ws := filepath.Join(evolveDir, "runs", "cycle-"+itoaT(cycleID))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "scout-report.md"), []byte("partial\n"), 0o644); err != nil {
		t.Fatalf("seed ws file: %v", err)
	}
	writeJSONT(t, filepath.Join(evolveDir, "cycle-state.json"), map[string]any{
		"cycle_id": cycleID, "phase": "scout", "workspace_path": ws,
	})
	writeJSONT(t, filepath.Join(evolveDir, "state.json"), map[string]any{
		"lastCycleNumber": lastCycle, "version": 18,
		// Unmodelled-by-core.State field: must survive the seal's state.json
		// mutation at the CLI integration layer too, not just in core.
		"expected_ship_sha": "deadbeef-must-survive",
	})
	return projectRoot, evolveDir
}

// TestResolveRouterDispatch_Precedence pins the advisor's {cli,model} resolution
// order — policy.RouterPolicy (rc) > profile (router.json) > opus/claude-tmux
// fallback — the same precedence a phase uses, so the routing brain is configurable
// to any LLM CLI via policy.json rather than env vars.
func TestResolveRouterDispatch_Precedence(t *testing.T) {
	t.Parallel()
	writeRouterProfile := func(t *testing.T, dir, cli, tier string) {
		t.Helper()
		profDir := filepath.Join(dir, "profiles")
		if err := os.MkdirAll(profDir, 0o755); err != nil {
			t.Fatalf("mkdir profiles: %v", err)
		}
		body := `{"name":"router","role":"router"`
		if cli != "" {
			body += `,"cli":"` + cli + `"`
		}
		if tier != "" {
			body += `,"model_tier_default":"` + tier + `"`
		}
		body += "}"
		if err := os.WriteFile(filepath.Join(profDir, "router.json"), []byte(body), 0o644); err != nil {
			t.Fatalf("write router.json: %v", err)
		}
	}

	t.Run("fallback when no profile and no rc", func(t *testing.T) {
		t.Parallel()
		cli, model := resolveRouterDispatch(t.TempDir(), policy.RouterPolicy{}) // empty dir ⇒ no profile file
		if cli != "claude-tmux" || model != "opus" {
			t.Errorf("fallback = (%q,%q), want (claude-tmux,opus)", cli, model)
		}
	})

	t.Run("profile beats fallback", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeRouterProfile(t, dir, "codex-tmux", "deep")
		cli, model := resolveRouterDispatch(dir, policy.RouterPolicy{})
		if cli != "codex-tmux" || model != "deep" {
			t.Errorf("profile = (%q,%q), want (codex-tmux,deep)", cli, model)
		}
	})

	t.Run("rc beats profile", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeRouterProfile(t, dir, "codex-tmux", "deep")
		cli, model := resolveRouterDispatch(dir, policy.RouterPolicy{CLI: "agy", Model: "balanced"})
		if cli != "agy" || model != "balanced" {
			t.Errorf("rc = (%q,%q), want (agy,balanced) — rc must override profile", cli, model)
		}
	})

	t.Run("rc beats fallback when no profile", func(t *testing.T) {
		t.Parallel()
		cli, model := resolveRouterDispatch(t.TempDir(), policy.RouterPolicy{CLI: "gemini-tmux", Model: "fast"})
		if cli != "gemini-tmux" || model != "fast" {
			t.Errorf("rc-only = (%q,%q), want (gemini-tmux,fast)", cli, model)
		}
	})

	t.Run("partial profile: cli only keeps fallback model", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeRouterProfile(t, dir, "codex-tmux", "") // no model_tier_default
		cli, model := resolveRouterDispatch(dir, policy.RouterPolicy{})
		if cli != "codex-tmux" || model != "opus" {
			t.Errorf("partial = (%q,%q), want (codex-tmux,opus) — model falls back", cli, model)
		}
	})
}

// TestCycleContext_GoalOnlyWhenGiven pins that a supplied --goal becomes
// Context["goal"] (the convention Scout + the routing advisor read; NOT
// Context["strategy"], the strategy mode), while omitting it preserves the prior
// behavior (no goal key). commit_message is always present.
func TestCycleContext_GoalOnlyWhenGiven(t *testing.T) {
	t.Parallel()
	withGoal := cycleContext("abc12345", "redesign the auth subsystem")
	if withGoal["goal"] != "redesign the auth subsystem" {
		t.Errorf("goal=%q, want the goal text", withGoal["goal"])
	}
	if _, ok := withGoal["strategy"]; ok {
		t.Error("cycleContext must NOT set strategy (that is the mode key, set elsewhere)")
	}
	if withGoal["commit_message"] == "" {
		t.Error("commit_message must always be set")
	}
	if _, ok := cycleContext("abc12345", "")["goal"]; ok {
		t.Error("no goal key when goal text is empty (preserves prior behavior)")
	}
}

func TestRunCycleReset_DryRun(t *testing.T) {
	projectRoot, evolveDir := seedResetDir(t, 108, 107)
	var stdout, stderr bytes.Buffer
	rc := runCycleReset([]string{"--project-root", projectRoot, "--evolve-dir", evolveDir, "--dry-run"}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%q", rc, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "108") || !strings.Contains(out, "109") {
		t.Errorf("dry-run output should mention sealed 108 → next 109; got %q", out)
	}
	// Dry-run mutates nothing.
	if _, err := os.Stat(filepath.Join(evolveDir, "cycle-state.json")); err != nil {
		t.Errorf("dry-run must not remove cycle-state.json: %v", err)
	}
	sm := readJSONT(t, filepath.Join(evolveDir, "state.json"))
	if n, _ := sm["lastCycleNumber"].(float64); int(n) != 107 {
		t.Errorf("dry-run must not advance lastCycleNumber; got %v", sm["lastCycleNumber"])
	}
}

func TestRunCycleReset_Seals(t *testing.T) {
	projectRoot, evolveDir := seedResetDir(t, 108, 107)
	var stdout, stderr bytes.Buffer
	rc := runCycleReset([]string{"--project-root", projectRoot, "--evolve-dir", evolveDir}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%q", rc, stderr.String())
	}
	// cycle-state cleared.
	if _, err := os.Stat(filepath.Join(evolveDir, "cycle-state.json")); !os.IsNotExist(err) {
		t.Errorf("cycle-state.json should be removed after seal; err=%v", err)
	}
	// state advanced; the unmodelled field survives the CLI-layer round-trip.
	sm := readJSONT(t, filepath.Join(evolveDir, "state.json"))
	if n, _ := sm["lastCycleNumber"].(float64); int(n) != 108 {
		t.Errorf("lastCycleNumber=%v want 108", sm["lastCycleNumber"])
	}
	if got, _ := sm["expected_ship_sha"].(string); got != "deadbeef-must-survive" {
		t.Errorf("expected_ship_sha must survive the seal at the CLI layer; got %q", got)
	}
	// archive exists with the preserved workspace file.
	matches, _ := filepath.Glob(filepath.Join(evolveDir, "runs", "cycle-108.reset-*", "scout-report.md"))
	if len(matches) != 1 {
		t.Errorf("expected one sealed archive containing scout-report.md, got %v", matches)
	}
	// ledger entry written.
	if _, err := os.Stat(filepath.Join(evolveDir, "ledger.jsonl")); err != nil {
		t.Errorf("ledger.jsonl should exist after seal: %v", err)
	}
}

// TestRunCycleReset_HeldLockAloneDoesNotBlock pins the cycle-395 fix direction:
// holding the coarse .evolve/.lock WITHOUT a fresh run lease must NOT block a
// reset. The old lock pre-check was a false negative — the dispatcher's lock is
// per-CYCLE (released between cycles), so a sibling acquired it in the gap and
// sealed a running loop. Liveness is now the per-run lease (the FRESH-lease
// refusal is covered by TestRunCycleReset_LeaseFencing); a held lock with no
// proof-of-life owner does not gate the seal.
func TestRunCycleReset_HeldLockAloneDoesNotBlock(t *testing.T) {
	projectRoot, evolveDir := seedResetDir(t, 108, 107)
	// Hold the .evolve lock; write NO lease (no live owner is proven).
	st := storage.New(evolveDir)
	release, err := st.AcquireLock(context.Background())
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	defer func() { _ = release() }()

	var so, se bytes.Buffer
	if rc := runCycleReset([]string{"--project-root", projectRoot, "--evolve-dir", evolveDir}, &so, &se); rc != 0 {
		t.Fatalf("a held lock with no fresh lease must NOT block reset; rc=%d stderr=%q", rc, se.String())
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "cycle-state.json")); !os.IsNotExist(err) {
		t.Errorf("cycle-state.json should be cleared after seal; err=%v", err)
	}
}

func TestRunCycleReset_NothingToReset(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	rc := runCycleReset([]string{"--project-root", projectRoot, "--evolve-dir", evolveDir}, &stdout, &stderr)
	if rc == 0 {
		t.Fatalf("expected non-zero rc when nothing to reset; stdout=%q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "no in-progress cycle") {
		t.Errorf("expected 'no in-progress cycle' message; got %q", stderr.String())
	}
}

// --- helpers (test-local) ---

func itoaT(n int) string {
	return strconv.Itoa(n)
}

func writeJSONT(t *testing.T, path string, body any) {
	t.Helper()
	raw, _ := json.MarshalIndent(body, "", "  ")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readJSONT(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return m
}
