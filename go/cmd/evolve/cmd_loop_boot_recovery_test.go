package main

// cmd_loop_boot_recovery_test.go — RED tests (cycle 507, task
// wire-boot-recovery-functions) for the WIRING of boot-time recovery into
// runLoop's boot path. THIS is the layer cycle 506 was missing: it built
// QuarantineDirtyTree / ShipSHAMismatch / AutosealStaleMarker (core, fully unit
// tested) but never called them from runLoop — audit F1, CRITICAL, "green unit
// test, absent integration" (the project's own warnship_apicover_ci_gap trap).
// The cycle was reset, so the functions no longer exist; this cycle reinstates
// them (core function contract in
// internal/core/boot_preflight_test.go + stale_marker_autoseal_test.go) AND
// wires them here.
//
// Contract the Builder implements (TDD-defined seam; mirrors the established
// runLoopPreflightFn / wireOrchestratorDepsFn package-var seam idiom):
//
//	type bootRecoveryResult struct { Quarantined, Sealed, SHAMismatch bool }
//	func defaultBootRecovery(ctx context.Context, cfg loopConfig, ledger core.Ledger, stderr io.Writer) bootRecoveryResult
//	var bootRecoverFn = defaultBootRecovery
//	// runLoop calls bootRecoverFn(ctx, cfg, deps.Ledger, stderr) BEFORE the
//	// readiness gate (loopPreflightHalts), so a dirty/stranded tree self-heals
//	// before the first cycle's tree-diff guard runs. Best-effort / fail-open:
//	// a recovery error WARNs but never halts the batch.
//
// RED now (undefined symbols → package main test build fails). Do NOT modify
// this file — implement the seam.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// --- git fixture helpers (br* prefix avoids collision with other main tests) ---

func brInitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	brGit(t, dir, "init", "-q")
	brGit(t, dir, "config", "user.email", "ci@example.com")
	brGit(t, dir, "config", "user.name", "ci")
	brGit(t, dir, "config", "commit.gpgsign", "false")
	// A HEAD commit so autoseal's git-head capture succeeds.
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	brGit(t, dir, "add", "-A")
	brGit(t, dir, "commit", "-m", "seed")
	return dir
}

func brGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func brPorcelain(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v\n%s", err, out)
	}
	return string(out)
}

// AC (integration, headline / live-repro): defaultBootRecovery must ACTUALLY
// quarantine leaked tracked-source dirt so the tree is clean — proving
// QuarantineDirtyTree is invoked by the orchestrator, not merely defined.
func TestDefaultBootRecovery_QuarantinesDirtyTree(t *testing.T) {
	repo := brInitRepo(t)
	evolveDir := filepath.Join(repo, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A committed source file, then a leaked uncommitted edit.
	src := filepath.Join(repo, "go", "internal", "leak.go")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("package leak\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	brGit(t, repo, "add", "-A")
	brGit(t, repo, "commit", "-m", "add leak.go")
	if err := os.WriteFile(src, []byte("package leak\n// leaked\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	res := bootRecoverFn(context.Background(),
		loopConfig{ProjectRoot: repo, EvolveDir: evolveDir}, newFakeLedger(), &stderr)

	if !res.Quarantined {
		t.Errorf("dirty tracked source must be quarantined; res=%+v stderr=%q", res, stderr.String())
	}
	if out := brPorcelain(t, repo); out != "" {
		t.Errorf("boot recovery must leave the tree clean; git status --porcelain = %q", out)
	}
}

// AC (edge, ship-SHA): with go/bin/evolve on disk and a MISMATCHING
// expected_ship_sha in state.json, defaultBootRecovery must flag the mismatch —
// proving ShipSHAMismatch is invoked from the orchestrator (the 498/500/502
// SELF_SHA_TAMPERED cascade, caught at boot).
func TestDefaultBootRecovery_DetectsShipSHAMismatch(t *testing.T) {
	repo := brInitRepo(t)
	evolveDir := filepath.Join(repo, ".evolve")
	binPath := filepath.Join(repo, "go", "bin", "evolve")
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binPath, []byte("\x7fELF-rebuilt-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// expected_ship_sha deliberately does NOT match the on-disk binary.
	brWriteJSON(t, filepath.Join(evolveDir, "state.json"),
		map[string]any{"expected_ship_sha": "0000not-the-real-sha0000"})

	var stderr bytes.Buffer
	res := bootRecoverFn(context.Background(),
		loopConfig{ProjectRoot: repo, EvolveDir: evolveDir}, newFakeLedger(), &stderr)

	if !res.SHAMismatch {
		t.Errorf("a ship binary whose SHA != expected_ship_sha must be flagged at boot; res=%+v stderr=%q", res, stderr.String())
	}
}

// AC (edge, autoseal): a stranded cycle-state marker whose owner PID is dead
// must be auto-sealed at boot — proving AutosealStaleMarker is invoked. Uses a
// real dead PID (999999) so the orchestrator's REAL liveness probe drives the
// decision (no injection needed).
func TestDefaultBootRecovery_AutosealsDeadOwnerMarker(t *testing.T) {
	repo := brInitRepo(t)
	evolveDir := filepath.Join(repo, ".evolve")
	workspace := filepath.Join(evolveDir, "runs", "cycle-491")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "scout-report.md"), []byte("partial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	brWriteJSON(t, filepath.Join(evolveDir, "cycle-state.json"), map[string]any{
		"cycle_id":       491,
		"phase":          "scout",
		"active_agent":   "scout",
		"workspace_path": workspace,
	})
	brWriteJSON(t, filepath.Join(evolveDir, "state.json"), map[string]any{
		"lastCycleNumber": 490,
	})
	// A run lease owned by a dead pid — the stranded marker's owner.
	if err := runlease.Write(workspace, runlease.Lease{RunID: "run-491", OwnerPID: 999999}, time.Now().UTC()); err != nil {
		t.Fatalf("seed lease: %v", err)
	}

	var stderr bytes.Buffer
	res := bootRecoverFn(context.Background(),
		loopConfig{ProjectRoot: repo, EvolveDir: evolveDir}, newFakeLedger(), &stderr)

	if !res.Sealed {
		t.Errorf("a dead-owner stranded marker must be auto-sealed at boot; res=%+v stderr=%q", res, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "cycle-state.json")); !os.IsNotExist(err) {
		t.Errorf("the stranded marker must be cleared after auto-seal (err=%v)", err)
	}
}

// AC (negative): a fully clean state — clean tree, no stranded marker, no ship
// binary / expected_ship_sha — triggers ZERO recovery actions and never panics.
func TestDefaultBootRecovery_CleanStateNoOp(t *testing.T) {
	repo := brInitRepo(t)
	evolveDir := filepath.Join(repo, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	res := bootRecoverFn(context.Background(),
		loopConfig{ProjectRoot: repo, EvolveDir: evolveDir}, newFakeLedger(), &stderr)

	if res.Quarantined || res.Sealed || res.SHAMismatch {
		t.Errorf("clean state must trigger no recovery action; got %+v", res)
	}
	if out := brPorcelain(t, repo); out != "" {
		t.Errorf("clean tree must stay clean; git status --porcelain = %q", out)
	}
}

// AC (the wiring cycle 506 lacked): runLoop must INVOKE bootRecoverFn during
// its boot path, BEFORE the readiness gate halts. A spy seam records the call;
// forcing the preflight gate to halt isolates the boot path (no cycle runs).
// This is the anti-dead-code assertion — a function defined but never called by
// runLoop fails HERE, exactly the trap 506 fell into.
func TestRunLoop_InvokesBootRecoveryBeforeGate(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	called := false
	capturedRoot := ""
	prevBR := bootRecoverFn
	defer func() { bootRecoverFn = prevBR }()
	bootRecoverFn = func(_ context.Context, cfg loopConfig, _ core.Ledger, _ io.Writer) bootRecoveryResult {
		called = true
		capturedRoot = cfg.ProjectRoot
		return bootRecoveryResult{}
	}

	prevDeps := wireOrchestratorDepsFn
	defer func() { wireOrchestratorDepsFn = prevDeps }()
	wireOrchestratorDepsFn = func(string, string) orchDeps {
		return orchDeps{Storage: &fixtures.FakeStorage{}, Ledger: newFakeLedger()}
	}
	prevPf := runLoopPreflightFn
	defer func() { runLoopPreflightFn = prevPf }()
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result { return forcedHalt() }

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "anything",
		"--cycles", "1",
		"--force-fresh",
	}, nil, &stdout, &stderr)

	if rc != 2 {
		t.Fatalf("rc=%d want 2 (preflight halt); stderr=%q", rc, stderr.String())
	}
	if !called {
		t.Fatal("runLoop must invoke bootRecoverFn during boot — a recovery function runLoop never calls is dead code (cycle 506 F1)")
	}
	if capturedRoot != projectRoot {
		t.Errorf("bootRecoverFn must receive the loop's project root; got %q want %q", capturedRoot, projectRoot)
	}
}

func brWriteJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
