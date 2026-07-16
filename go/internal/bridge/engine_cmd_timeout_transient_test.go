package bridge

// engine_cmd_timeout_transient_test.go — advisory-phase-contract-degrade
// residual: exit 124 (a driver killed by a command-level timeout, e.g. gnu
// `timeout` around a headless CLI) is infra weather — the sibling of the
// artifact-timeout 81 — but the engine classified it as a PLAIN error, so an
// OPTIONAL non-floor phase whose whole fallback chain died 124 was
// cycle-fatal instead of infra-skipped (optionalInfraSkip matches only the
// transient/timeout sentinels). These tests pin 124 → ErrTransientBridgeFailure
// and pin 127 (missing binary) staying PLAIN — an absent CLI is an environment
// defect that must fail loud, not retry/skip as weather.

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestEngineLaunch_CmdTimeout124_WrapsTransient: exit 124 carries the
// transient sentinel so retry backoff, optionalInfraSkip, and the reconcile
// IsInfraTeardownError predicate all treat it as the timeout-kill it is.
func TestEngineLaunch_CmdTimeout124_WrapsTransient(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-test", "")
	fr := &fakeRunner{exit: ExitCmdTimeout}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})

	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "a.md"), Agent: "build-planner",
	})
	if err == nil {
		t.Fatal("expected an error on exit 124")
	}
	if resp.ExitCode != ExitCmdTimeout {
		t.Errorf("resp.ExitCode = %d, want %d", resp.ExitCode, ExitCmdTimeout)
	}
	if !errors.Is(err, core.ErrTransientBridgeFailure) {
		t.Errorf("exit-124 error must wrap core.ErrTransientBridgeFailure (infra weather, sibling of 81); got %v", err)
	}
	if errors.Is(err, core.ErrArtifactTimeout) {
		t.Error("exit-124 must NOT wrap core.ErrArtifactTimeout (that sentinel is the 81 artifact-wait contract)")
	}
}

// TestEngineLaunch_MissingBinary127_StaysPlain: 127 is a launch-environment
// defect (CLI not installed), NOT weather — it must stay a plain error so
// nothing retries, reconciles, or silently skips over an absent binary. The
// CLI fallback chain still advances on the raw exit code (llmroute), which is
// the correct and only recovery for a missing family.
func TestEngineLaunch_MissingBinary127_StaysPlain(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-test", "")
	fr := &fakeRunner{exit: ExitMissingBinary}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})

	_, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "a.md"), Agent: "build-planner",
	})
	if err == nil {
		t.Fatal("expected an error on exit 127")
	}
	if errors.Is(err, core.ErrTransientBridgeFailure) || errors.Is(err, core.ErrArtifactTimeout) {
		t.Errorf("exit-127 must stay a plain error (missing binary is an env defect, not weather); got %v", err)
	}
}
