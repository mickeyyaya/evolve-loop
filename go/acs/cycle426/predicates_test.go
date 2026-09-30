//go:build acs

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

type dispatchHooks struct{}

func (dispatchHooks) PhaseName() string                                  { return "auditor" }
func (dispatchHooks) AgentPromptName() string                            { return "evolve-auditor" }
func (dispatchHooks) ArtifactFilename(_ core.PhaseRequest) string        { return "audit-report.md" }
func (dispatchHooks) DefaultModel() string                               { return "sonnet" }
func (dispatchHooks) ComposePrompt(_ string, _ core.PhaseRequest) string { return "test-prompt" }
func (dispatchHooks) Classify(_ string, _ core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	return core.VerdictPASS, nil, "ship"
}

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

func TestC426_003_AllDriverBenchedNotStranded(t *testing.T) {
	root, pl := setupDispatchRoot(t, "codex-tmux", []string{"claude-tmux"})

	store := clihealth.NewStore(root, nil)
	now := time.Now()
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

func TestC426_004_FamilyBenchStillDemotesAfterChange(t *testing.T) {
	root, pl := setupDispatchRoot(t, "codex-tmux", []string{"claude-tmux"})

	store := clihealth.NewStore(root, nil)
	now := time.Now()
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

func TestC426_006_StrikeAfterClearNotBenched(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := clihealth.NewStore(dir, nil)
	const driver = "claude-tmux"

	if _, err := store.RecordBootStrike(driver); err != nil {
		t.Fatalf("RecordBootStrike (before clear): %v", err)
	}
	if err := store.ClearBootStrike(driver); err != nil {
		t.Fatalf("ClearBootStrike: %v", err)
	}
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

func TestC426_007_ClearBootStrikeReasonScoped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := clihealth.NewStore(dir, nil)

	now := time.Now()
	if err := store.Bench(clihealth.Entry{
		Family:       "codex",
		Reason:       "rate_limit",
		BenchedAt:    now,
		BenchedUntil: now.Add(time.Hour),
		Strikes:      1,
	}); err != nil {
		t.Fatalf("Bench(codex, rate_limit): %v", err)
	}

	if err := store.ClearBootStrike("claude-tmux"); err != nil {
		t.Errorf("ClearBootStrike on absent key: unexpected error: %v", err)
	}
	active := store.Active()
	if _, ok := active["codex"]; !ok {
		t.Errorf("Active() missing codex rate_limit bench after ClearBootStrike on absent key; " +
			"no-op ClearBootStrike must not disturb unrelated entries")
	}

	if err := store.Bench(clihealth.Entry{
		Family:       "codex-tmux",
		Reason:       clihealth.BootTimeoutPattern,
		BenchedAt:    now,
		BenchedUntil: now.Add(time.Hour),
		Strikes:      clihealth.DefaultBootBenchThreshold,
	}); err != nil {
		t.Fatalf("Bench(codex-tmux, boot-timeout): %v", err)
	}

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

// acs-predicate: config-check (the criterion is a code-path wiring requirement;
func TestC426_008_EngineClearsBootStrikeOnNonBootExit(t *testing.T) { // acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	enginePath := filepath.Join(root, "go", "internal", "bridge", "engine.go")
	acsassert.FileContains(t, enginePath, "ClearBootStrike")
}
