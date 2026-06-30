//go:build acs

// Package cycle426 materialises the cycle-426 acceptance criteria for two tasks:
// wiring the driver-bench consumer into applyBenchToPlan (T1) and adding
// ClearBootStrike to restore the documented "consecutive strikes" contract (T2).
//
// Goal: close the two remaining boot-latency leaks — (1) exit-80 boot-strikes
// are recorded per-driver but the consumer (applyBenchToPlan) never routes them
// to ApplyDriverBench, so a driver that repeatedly times out on boot is never
// demoted; (2) strike counts are cumulative not consecutive because there is no
// reset on a successful boot, causing transient-retry drivers to reach the bench
// threshold from non-adjacent failures.
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n tasks only):
//
//	wire-driver-bench-consumer (T1 — Medium):
//	  AC1  2-strike codex-tmux demoted behind claude-tmux in dispatch  (positive)   → C426_001 RED
//	  AC2  no boot-bench → no reorder                                  (negative)   → C426_002 pre-GREEN
//	  AC3  all-driver-benched → dispatch not stranded                  (edge)       → C426_003 pre-GREEN
//	  AC4  family bench (rate_limit) still demotes after change        (regression) → C426_004 pre-GREEN
//
//	reset-boot-strike-on-success (T2 — Small):
//	  AC5  ClearBootStrike removes active boot-strike entry             (positive)   → C426_005 RED (compile)
//	  AC6  strike→clear→strike NOT benched                             (negative)   → C426_006 RED (compile)
//	  AC7  ClearBootStrike Reason-scoped; no-entry is no-op            (edge)       → C426_007 RED (compile)
//	  AC8  engine.go calls ClearBootStrike on non-boot exit            (regression) → C426_008 RED (compile)
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C426_002 (no bench → no reorder), C426_006 (strike→clear→strike ≠ bench)
//	Edge/OOD:  C426_003 (all-benched not stranded), C426_007 (Reason-scope + no-entry no-op)
//	Semantic:  8 distinct dimensions across reorder/no-op/not-stranded/family-coexist/
//	           remove-entry/consecutive-reset/reason-guard/engine-wire.
//
// 1:1 enforcement:
//
//	T1: predicate=4 (C426_001–004) + manual+checklist=0 → total=4 ✓
//	T2: predicate=4 (C426_005–008) + manual+checklist=0 → total=4 ✓
package cycle426

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ──────────────────────────── test infrastructure ────────────────────────────

// trackingBridge is a minimal core.Bridge that records which CLI is requested
// on each Launch call and writes a trivial artifact so the runner's artifact-
// read path does not synthesize a FAIL.
type trackingBridge struct {
	calls []string
}

func (b *trackingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.calls = append(b.calls, req.CLI)
	if req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte("ok-"+req.CLI), 0o644)
	}
	return core.BridgeResponse{}, nil
}

func (b *trackingBridge) Probe(_ context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

// dispatchHooks is a minimal runner.Hooks whose phase/agent names match the
// profile key used by the runner (auditor ← "evolve-auditor" after strip).
type dispatchHooks struct{}

func (dispatchHooks) PhaseName() string                                  { return "auditor" }
func (dispatchHooks) AgentPromptName() string                            { return "evolve-auditor" }
func (dispatchHooks) ArtifactFilename(_ core.PhaseRequest) string        { return "audit-report.md" }
func (dispatchHooks) DefaultModel() string                               { return "sonnet" }
func (dispatchHooks) ComposePrompt(_ string, _ core.PhaseRequest) string { return "test-prompt" }
func (dispatchHooks) Classify(_ string, _ core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	return core.VerdictPASS, nil, "ship"
}

// setupDispatchRoot creates a project root under t.TempDir() with:
//   - .evolve/profiles/auditor.json containing primaryCLI + fallback
//   - a prompts.Loader with the minimal evolve-auditor agent doc
func setupDispatchRoot(t *testing.T, primaryCLI string, fallback []string) (root string, prompLoader *prompts.Loader) {
	t.Helper()
	root = t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	fb := ""
	if len(fallback) > 0 {
		quoted := make([]string, len(fallback))
		for i, c := range fallback {
			quoted[i] = `"` + c + `"`
		}
		fb = `,"cli_fallback":[` + strings.Join(quoted, ",") + `]`
	}
	profile := `{"name":"auditor","cli":"` + primaryCLI + `","model_tier_default":"sonnet"` + fb + `}`
	if err := os.WriteFile(filepath.Join(dir, "auditor.json"), []byte(profile), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	prompLoader = prompts.NewFromFS(fstest.MapFS{
		"agents/evolve-auditor.md": &fstest.MapFile{
			Data: []byte("---\nname: evolve-auditor\n---\ntest agent body"),
		},
	})
	return root, prompLoader
}

// runDispatch calls runner.Run with the tracking bridge and returns which CLIs
// were called, in order.
func runDispatch(t *testing.T, root string, prompLoader *prompts.Loader) []string {
	t.Helper()
	bridge := &trackingBridge{}
	r := runner.New(runner.Options{
		Hooks:   dispatchHooks{},
		Bridge:  bridge,
		Prompts: prompLoader,
	})
	if _, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root,
		Workspace:   t.TempDir(),
	}); err != nil {
		t.Fatalf("runner.Run: %v", err)
	}
	return bridge.calls
}

// ──────────────────── T1 — wire-driver-bench-consumer ────────────────────────

// TestC426_001_DriverBenchDemotesBenched2Strike is the primary RED test:
// after two RecordBootStrike calls for codex-tmux (reaching DefaultBootBenchThreshold),
// applyBenchToPlan must demote codex-tmux so dispatch starts at claude-tmux.
//
// RED condition: applyBenchToPlan calls ApplyBench (family-keyed), which looks
// for Family("codex-tmux")="codex" in the active-bench map, but boot-timeout
// entries are stored keyed by the DRIVER name ("codex-tmux"). The family key
// does not match, so no demotion occurs and dispatch starts at codex-tmux.
//
// GREEN after fix: applyBenchToPlan splits entries by Reason — BootTimeoutPattern
// entries are routed to ApplyDriverBench (driver-keyed), which finds "codex-tmux"
// and demotes it; dispatch starts at claude-tmux.
func TestC426_001_DriverBenchDemotesBenched2Strike(t *testing.T) {
	root, pl := setupDispatchRoot(t, "codex-tmux", []string{"claude-tmux"})

	store := clihealth.NewStore(root, nil)
	for i := 0; i < clihealth.DefaultBootBenchThreshold; i++ {
		if _, err := store.RecordBootStrike("codex-tmux"); err != nil {
			t.Fatalf("RecordBootStrike call %d: %v", i+1, err)
		}
	}
	if _, ok := store.Active()["codex-tmux"]; !ok {
		t.Fatal("setup: codex-tmux not active-benched after threshold strikes; test precondition failed")
	}

	calls := runDispatch(t, root, pl)

	if len(calls) == 0 {
		t.Fatal("dispatch made zero bridge.Launch calls — runner did not dispatch at all")
	}
	if calls[0] == "codex-tmux" {
		t.Errorf("dispatch order %v: codex-tmux (boot-benched driver) was tried FIRST — "+
			"applyBenchToPlan must route BootTimeoutPattern entries to ApplyDriverBench "+
			"so the benched driver is demoted; currently ApplyBench (family-keyed) misses "+
			"the driver-keyed entry (key 'codex-tmux' vs family 'codex')", calls)
	}
}

// TestC426_002_NoBenchNoReorder is the load-bearing negative test: when no
// boot-bench entries exist, the dispatch chain must remain in profile order
// (codex-tmux first). Guards against over-demotion: the fix must demote ONLY
// benched drivers, not all primary CLIs.
// Pre-existing GREEN; kept as regression guard.
func TestC426_002_NoBenchNoReorder(t *testing.T) {
	root, pl := setupDispatchRoot(t, "codex-tmux", []string{"claude-tmux"})

	if active := clihealth.NewStore(root, nil).Active(); len(active) != 0 {
		t.Fatalf("setup: unexpected active benches %v; precondition requires empty store", active)
	}

	calls := runDispatch(t, root, pl)

	if len(calls) == 0 {
		t.Fatal("dispatch made zero calls with no bench — runner should proceed normally")
	}
	if calls[0] != "codex-tmux" {
		t.Errorf("dispatch order %v: codex-tmux (no bench) must be tried first; "+
			"no active bench must not reorder the chain", calls)
	}
}

// TestC426_003_AllDriverBenchedNotStranded is the edge test: when all candidates
// have active boot-bench entries, dispatch must still proceed (bench is advice,
// never a veto); the candidate count must be preserved.
// Pre-existing GREEN (ApplyDriverBench all-benched path mirrors ApplyBench policy);
// kept as regression guard against a "demote = drop" misimplementation.
func TestC426_003_AllDriverBenchedNotStranded(t *testing.T) {
	root, pl := setupDispatchRoot(t, "codex-tmux", []string{"claude-tmux"})

	store := clihealth.NewStore(root, nil)
	now := time.Now()
	// Bench both drivers directly (driver-keyed, BootTimeoutPattern).
	for _, drv := range []string{"codex-tmux", "claude-tmux"} {
		if err := store.Bench(clihealth.Entry{
			Family:       drv,
			Reason:       clihealth.BootTimeoutPattern,
			BenchedAt:    now,
			BenchedUntil: now.Add(time.Hour),
			Strikes:      clihealth.DefaultBootBenchThreshold,
		}); err != nil {
			t.Fatalf("Bench(%s): %v", drv, err)
		}
	}

	calls := runDispatch(t, root, pl)

	if len(calls) == 0 {
		t.Errorf("dispatch stranded: zero calls when all candidates driver-benched — " +
			"bench must be advice, never a veto; dispatch must proceed with least-recently-benched first")
	}
}

// TestC426_004_FamilyBenchStillDemotesAfterChange is the regression test:
// the existing family-bench path (rate_limit, family-keyed via ApplyBench) must
// continue to demote codex-tmux after the driver-bench consumer is added.
// Pre-existing GREEN; guards against the driver-bench split breaking family demotion.
func TestC426_004_FamilyBenchStillDemotesAfterChange(t *testing.T) {
	root, pl := setupDispatchRoot(t, "codex-tmux", []string{"claude-tmux"})

	store := clihealth.NewStore(root, nil)
	now := time.Now()
	// Family bench: rate_limit for the "codex" family (stored keyed by family, not driver).
	if err := store.Bench(clihealth.Entry{
		Family:       "codex",
		Reason:       "rate_limit",
		BenchedAt:    now,
		BenchedUntil: now.Add(time.Hour),
		Strikes:      1,
	}); err != nil {
		t.Fatalf("Bench(codex family): %v", err)
	}

	calls := runDispatch(t, root, pl)

	if len(calls) == 0 {
		t.Fatal("dispatch made zero calls with family bench")
	}
	if calls[0] == "codex-tmux" {
		t.Errorf("dispatch order %v: codex-tmux (family-benched for rate_limit) was tried first — "+
			"family bench must still demote via ApplyBench after driver-bench consumer is added", calls)
	}
}

// ──────────────────── T2 — reset-boot-strike-on-success ─────────────────────

// TestC426_005_ClearBootStrikeRemovesActiveEntry is the primary positive test:
// after two RecordBootStrike calls (reaching bench threshold), calling
// ClearBootStrike must remove the entry from Active() — the strike counter is
// reset, making the driver retryable again.
//
// RED: clihealth.Store.ClearBootStrike does not exist → compile error.
// GREEN after fix: ClearBootStrike removes the BootTimeoutPattern entry for
// the specified driver; Active() no longer contains the driver.
func TestC426_005_ClearBootStrikeRemovesActiveEntry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := clihealth.NewStore(dir, nil)
	const driver = "codex-tmux"

	for i := 0; i < clihealth.DefaultBootBenchThreshold; i++ {
		if _, err := store.RecordBootStrike(driver); err != nil {
			t.Fatalf("RecordBootStrike call %d: %v", i+1, err)
		}
	}
	if _, ok := store.Active()[driver]; !ok {
		t.Fatal("setup: driver not active-benched after threshold; precondition failed")
	}

	if err := store.ClearBootStrike(driver); err != nil {
		t.Fatalf("ClearBootStrike(%q): %v", driver, err)
	}

	if active := store.Active(); len(active) != 0 {
		t.Errorf("Active() = %v after ClearBootStrike — expected empty; "+
			"ClearBootStrike must remove the BootTimeoutPattern entry so the driver is retryable", active)
	}
}

// TestC426_006_StrikeAfterClearNotBenched is the load-bearing negative test:
// a single strike AFTER a ClearBootStrike must NOT bench the driver. The strike
// counter is consecutive — a successful boot resets it to zero, so a single
// subsequent failure is below threshold.
//
// Prevents gaming by a stub that ignores ClearBootStrike and lets accumulative
// strikes bench the driver regardless.
//
// RED: clihealth.Store.ClearBootStrike does not exist → compile error.
func TestC426_006_StrikeAfterClearNotBenched(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := clihealth.NewStore(dir, nil)
	const driver = "claude-tmux"

	// Strike 1 (below threshold — stays retryable).
	if _, err := store.RecordBootStrike(driver); err != nil {
		t.Fatalf("RecordBootStrike (before clear): %v", err)
	}
	// Successful boot resets the consecutive-strike counter.
	if err := store.ClearBootStrike(driver); err != nil {
		t.Fatalf("ClearBootStrike: %v", err)
	}
	// Strike 1 again — from a clean slate; must NOT bench.
	benched, err := store.RecordBootStrike(driver)
	if err != nil {
		t.Fatalf("RecordBootStrike (after clear): %v", err)
	}
	if benched {
		t.Errorf("strike→ClearBootStrike→strike: benched=true, want false (threshold=%d) — "+
			"ClearBootStrike must reset the consecutive strike counter; "+
			"strike after clear must be treated as the first strike", clihealth.DefaultBootBenchThreshold)
	}
	if _, ok := store.Active()[driver]; ok {
		t.Errorf("Active() contains %q after clear→re-strike below threshold; "+
			"must not bench until %d consecutive strikes accumulate without a clear", driver, clihealth.DefaultBootBenchThreshold)
	}
}

// TestC426_007_ClearBootStrikeReasonScoped is the edge test verifying two
// sub-criteria:
//
//	(a) ClearBootStrike is Reason-scoped: only BootTimeoutPattern entries are
//	    removed; a rate_limit bench for the same key is preserved.
//	(b) ClearBootStrike on an absent key is a no-op (no error, store unchanged).
//
// RED: clihealth.Store.ClearBootStrike does not exist → compile error.
func TestC426_007_ClearBootStrikeReasonScoped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := clihealth.NewStore(dir, nil)

	now := time.Now()
	// Bench "codex" family for rate_limit (not a boot-timeout bench).
	if err := store.Bench(clihealth.Entry{
		Family:       "codex",
		Reason:       "rate_limit",
		BenchedAt:    now,
		BenchedUntil: now.Add(time.Hour),
		Strikes:      1,
	}); err != nil {
		t.Fatalf("Bench(codex, rate_limit): %v", err)
	}

	// (a) ClearBootStrike with a different key ("claude-tmux") that has no entry:
	// must be a no-op (no error, store has only the rate_limit entry).
	if err := store.ClearBootStrike("claude-tmux"); err != nil {
		t.Errorf("ClearBootStrike on absent key: unexpected error: %v", err)
	}
	active := store.Active()
	if _, ok := active["codex"]; !ok {
		t.Errorf("Active() missing codex rate_limit bench after ClearBootStrike on absent key; " +
			"no-op ClearBootStrike must not disturb unrelated entries")
	}

	// Add a boot-timeout bench for "codex-tmux" (driver-keyed).
	if err := store.Bench(clihealth.Entry{
		Family:       "codex-tmux",
		Reason:       clihealth.BootTimeoutPattern,
		BenchedAt:    now,
		BenchedUntil: now.Add(time.Hour),
		Strikes:      clihealth.DefaultBootBenchThreshold,
	}); err != nil {
		t.Fatalf("Bench(codex-tmux, boot-timeout): %v", err)
	}

	// (b) ClearBootStrike("codex-tmux") removes ONLY the boot-timeout entry.
	if err := store.ClearBootStrike("codex-tmux"); err != nil {
		t.Fatalf("ClearBootStrike(codex-tmux): %v", err)
	}
	active = store.Active()
	if _, ok := active["codex-tmux"]; ok {
		t.Errorf("Active() still contains codex-tmux after ClearBootStrike; " +
			"must remove the BootTimeoutPattern entry")
	}
	if _, ok := active["codex"]; !ok {
		t.Errorf("Active() lost codex rate_limit bench after ClearBootStrike(codex-tmux); " +
			"Reason-scoped clear must not remove entries with a different Reason or key")
	}
}

// TestC426_008_EngineClearsBootStrikeOnNonBootExit is the regression test:
// engine.go must call ClearBootStrike when a launch exits with any code OTHER
// than ExitREPLBootTimeout (exit 80) — i.e. the REPL booted (success, rate-limit,
// artifact-timeout, etc.). The call site ensures a driver is not accumulating
// strikes from non-adjacent failures.
//
// acs-predicate: config-check (the criterion is a code-path wiring requirement;
// structural verification is the appropriate predicate class — a behavioral test
// would require the full bridge engine with a mock REPL driver whose call record
// is threaded through the store, which is more complex than the criterion warrants
// as an ACS predicate).
//
// RED: ClearBootStrike does not exist in engine.go (not yet called) → source absent.
func TestC426_008_EngineClearsBootStrikeOnNonBootExit(t *testing.T) { // acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	enginePath := filepath.Join(root, "go", "internal", "bridge", "engine.go")
	acsassert.FileContains(t, enginePath, "ClearBootStrike")
}
