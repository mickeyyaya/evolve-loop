//go:build acs

package cycle1444

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const claudeWindow = 200_000

func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

func TestC1444_001_FillPctIsPercentOfEffectiveWindow(t *testing.T) {
	if got := tokenusage.EffectiveWindow("claude-tmux"); got != claudeWindow {
		t.Fatalf("EffectiveWindow(\"claude-tmux\") = %d, want %d", got, claudeWindow)
	}
	prompt := tokenusage.PromptTokens(cyclestate.TokenUsage{
		Input: 100_000, Output: 9_999_999, CacheRead: 20_000, CacheWrite: 0,
	})
	if prompt != 120_000 {
		t.Fatalf("PromptTokens = %d, want 120000 (Input+CacheRead+CacheWrite; Output must not fill the window)", prompt)
	}
	if got := tokenusage.FillPct(prompt, claudeWindow); math.Abs(got-60) > 0.001 {
		t.Errorf("FillPct(120000, %d) = %v, want 60 (percentage, not ratio)", claudeWindow, got)
	}
}

func TestC1444_002_UnmeasurableFillIsSentinelNeverZeroOrInf(t *testing.T) {
	for _, window := range []int{0, -1} {
		got := tokenusage.FillPct(120_000, window)
		if math.IsInf(got, 0) || math.IsNaN(got) {
			t.Fatalf("FillPct(120000, %d) = %v — divide-by-zero leaked", window, got)
		}
		if got != tokenusage.FillPctUnmeasured {
			t.Errorf("FillPct(120000, %d) = %v, want the unmeasured sentinel %v", window, got, tokenusage.FillPctUnmeasured)
		}
	}
	if tokenusage.FillPctUnmeasured >= 0 {
		t.Errorf("sentinel %v is non-negative — it can collide with a real 0–100%% reading", tokenusage.FillPctUnmeasured)
	}
	if got := tokenusage.EffectiveWindow("no-such-cli-family"); got != 0 {
		t.Errorf("EffectiveWindow(unknown family) = %d, want 0 (never guess a window)", got)
	}
}

func TestC1444_003_ResolverStampsFillFromRecoveredUsage(t *testing.T) {
	ws := t.TempDir()
	events := filepath.Join(ws, "build-events.ndjson")
	envelope := `{"kind":"result","data":{"cost_usd":0.4,"tokens":{"in":100000,"out":210,"cache_r":20000,"cache_c":0}}}` + "\n"
	if err := os.WriteFile(events, []byte(envelope), 0o644); err != nil {
		t.Fatalf("write events fixture: %v", err)
	}
	resolve := tokenusage.DefaultResolver(t.TempDir())

	covered, err := resolve(tokenusage.Window{Driver: "claude-tmux", EventsLogPath: events})
	if err != nil {
		t.Fatalf("resolver errored (telemetry must be best-effort): %v", err)
	}
	if covered.Source != tokenusage.SourceEventsResult {
		t.Fatalf("Source = %q, want %q — fixture never reached the events tier", covered.Source, tokenusage.SourceEventsResult)
	}
	if math.Abs(covered.FillPct-60) > 0.001 {
		t.Errorf("Result.FillPct = %v, want 60 — fill%% is not derived from the usage this resolve recovered", covered.FillPct)
	}

	uncovered, err := resolve(tokenusage.Window{Driver: "claude-tmux"})
	if err != nil {
		t.Fatalf("resolver errored: %v", err)
	}
	if uncovered.Source != tokenusage.SourceNone {
		t.Fatalf("uncovered Source = %q, want %q", uncovered.Source, tokenusage.SourceNone)
	}
	if uncovered.FillPct != tokenusage.FillPctUnmeasured {
		t.Errorf("uncovered FillPct = %v, want the sentinel — unmeasured must not read as 0%% full", uncovered.FillPct)
	}
}

func TestC1444_004_ThresholdResolutionNeverTrustsOperatorInput(t *testing.T) {
	if got := (policy.Policy{}).ContextFillConfig().WarnThresholdPct; got != 60 {
		t.Errorf("absent block: %d, want 60", got)
	}
	if got := (policy.Policy{ContextFill: &policy.ContextFillPolicy{}}).ContextFillConfig().WarnThresholdPct; got != 60 {
		t.Errorf("empty block: %d, want 60", got)
	}
	if got := (policy.Policy{ContextFill: &policy.ContextFillPolicy{WarnThresholdPct: 85}}).ContextFillConfig().WarnThresholdPct; got != 85 {
		t.Errorf("valid override: %d, want 85", got)
	}
	for _, bad := range []int{0, -1, 101, 900} {
		got := (policy.Policy{ContextFill: &policy.ContextFillPolicy{WarnThresholdPct: bad}}).ContextFillConfig().WarnThresholdPct
		if got != 60 {
			t.Errorf("out-of-range %d resolved to %d, want 60", bad, got)
		}
	}
}

func TestC1444_005_WarnReachableFromDispatchSeam(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir(t), "test", "-count=1", "-run", "TestContextFillWarn", "./internal/bridge")
	if err != nil || code != 0 {
		t.Errorf("fill WARN is not reachable from the production dispatch seam (exit=%d err=%v)\n%s\n%s", code, err, stdout, stderr)
	}
}

func TestC1444_006_PolicyThresholdReachesProductionDeps(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir(t), "test", "-count=1", "-run", "TestProductionDeps.*ContextFill", "./internal/adapters/bridge")
	if err != nil || code != 0 {
		t.Errorf("policy context_fill never reaches the production engine deps (exit=%d err=%v)\n%s\n%s", code, err, stdout, stderr)
	}
}
