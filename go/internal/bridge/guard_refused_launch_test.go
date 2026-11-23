package bridge

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func launchWithDenial(t *testing.T, exitCode int, stderrFixture string, guard, cwdOverride string) (core.BridgeResponse, string, error) {
	t.Helper()
	wt := t.TempDir()
	cwd := wt
	if cwdOverride != "" {
		cwd = cwdOverride
	}
	script := launchFixtureScript(exitCode, stderrFixture, "deny="+fixtureDenial(t, wt, guard, cwd, "git push origin main"))
	return launchThroughFixture(t, context.Background(), Deps{}, script, core.BridgeRequest{Worktree: wt})
}

func TestEngineLaunch_AStallAfterAShipDenialIsGuardRefusedInTheResponseAndTheLedger(t *testing.T) {
	resp, ws, err := launchWithDenial(t, ExitArtifactTimeout, "last-line-only", "ship", "")
	if resp.CauseCode != core.CauseGuardRefused {
		t.Fatalf("response cause %q, want guard_refused", resp.CauseCode)
	}
	if rec := lastLedgerRecord(t, ws); rec.CauseCode != resp.CauseCode {
		t.Fatalf("the llmcalls row says %q and the response says %q: one cause, two homes", rec.CauseCode, resp.CauseCode)
	}
	if !errors.Is(err, core.ErrArtifactTimeout) {
		t.Fatalf("the error keeps the artifact-timeout sentinel: %v", err)
	}
}

func TestEngineLaunch_ADenialThatDoesNotCountKeepsTheStallCause(t *testing.T) {
	other := t.TempDir()
	for name, tc := range map[string]struct {
		code   int
		stderr string
		guard  string
		cwd    string
		want   string
	}{
		"a family-bound guard":         {ExitArtifactTimeout, "last-line-only", "role", "", "artifact_timeout"},
		"a denial of another cell":     {ExitArtifactTimeout, "last-line-only", "ship", other, "artifact_timeout"},
		"a typed infrastructure stall": {ExitArtifactTimeout, "marker-submit-wedged", "ship", "", "submit_wedged"},
		"a boot timeout":               {ExitREPLBootTimeout, "empty", "ship", "", "repl_boot_timeout"},
	} {
		resp, _, _ := launchWithDenial(t, tc.code, tc.stderr, tc.guard, tc.cwd)
		if resp.CauseCode == core.CauseGuardRefused || (tc.want != "" && resp.CauseCode != tc.want) {
			t.Errorf("%s: cause %q, want %q", name, resp.CauseCode, tc.want)
		}
	}
}

func TestEngineLaunch_ADenialBeforeTheLaunchDoesNotEndTheChain(t *testing.T) {
	wt := t.TempDir()
	writeFixtureDenial(fixtureDenial(t, wt, "ship", wt, "git push origin main"))
	resp, _, _ := launchThroughFixture(t, context.Background(), Deps{}, launchFixtureScript(ExitArtifactTimeout, "last-line-only"), core.BridgeRequest{Worktree: wt})
	if resp.CauseCode == core.CauseGuardRefused {
		t.Fatalf("a denial of an earlier attempt must not match: %q", resp.CauseCode)
	}
}

func TestTieringExit_TheStallExitOfLlmrouteIsTheStallExitOfTheClassifier(t *testing.T) {
	t.Parallel()
	if got := llmroute.TieringExit(launchoutcome.ExitArtifactTimeout, core.CauseGuardRefused); got != llmroute.ExitGuardRefused {
		t.Fatalf("exit %d with a guard refusal maps to %d, want %d: llmroute must read the same stall exit as launchoutcome", launchoutcome.ExitArtifactTimeout, got, llmroute.ExitGuardRefused)
	}
}
