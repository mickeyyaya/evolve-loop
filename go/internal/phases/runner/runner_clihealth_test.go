package runner

// RED contract for the CLI-health bench hooks (cycle-283 forensics): when a
// candidate exits 85 and the bridge's escalation-report.json classifies a
// benchable pattern (rate_limit), the runner must REMEMBER it — bench the CLI
// FAMILY in .evolve/cli-health.json — and the NEXT dispatch must start at a
// healthy CLI instead of re-burning the benched primary's boot window.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/clihealth"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// cycle283WallTail is the verbatim wall line from the real cycle-283
// escalation report — the fixture the whole feature was built from.
const cycle283WallTail = "■ You've hit your usage limit. Upgrade to Pro (https://chatgpt.com/explore/pro), " +
	"visit https://chatgpt.com/codex/settings/usage to purchase more credits or try again at 6:11 AM."

// escalatingBridge wraps scriptedBridge and, when the named CLI launches,
// writes a workspace escalation-report.json the way bridge/autorespond.go
// does (unprefixed, before rc 85 propagates).
type escalatingBridge struct {
	scriptedBridge
	escalateCLI string // CLI whose launch drops the report
	reportCLI   string // "cli" field in the report (mismatch tests)
	capturedAt  time.Time
	pattern     string
}

func (e *escalatingBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if req.CLI == e.escalateCLI {
		capturedAt := e.capturedAt
		if capturedAt.IsZero() {
			capturedAt = time.Now() // autorespond stamps DURING the launch
		}
		rep := map[string]any{
			"schema_version": 1,
			"captured_at":    capturedAt.UTC().Format(time.RFC3339Nano),
			"cli":            e.reportCLI,
			"pattern_name":   e.pattern,
			"reason":         "escalate",
			"pane_tail":      cycle283WallTail,
		}
		b, _ := json.Marshal(rep)
		_ = os.WriteFile(filepath.Join(req.Workspace, "escalation-report.json"), b, 0o644)
	}
	return e.scriptedBridge.Launch(ctx, req)
}

func exit85() scriptedResp {
	return scriptedResp{
		resp: core.BridgeResponse{ExitCode: 85, Stderr: "unknown interactive prompt"},
		err:  errors.New("bridge: launch exit=85"),
	}
}

func runPhase(t *testing.T, root string, bridge core.Bridge) {
	t.Helper()
	hooks := &fakeHooks{
		phase: "auditor", agent: "evolve-auditor", model: "sonnet",
		prompt: "x", verdict: core.VerdictPASS, nextPhase: "ship",
	}
	r := New(Options{Hooks: hooks, Bridge: bridge, Prompts: fakePromptsFS("evolve-auditor", "x")})
	_, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root,
		Workspace:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// TestRun_Exit85RateLimitBenchesFamily: the cycle-283 replay. codex exits 85
// with a fresh rate_limit escalation report → the runner benches family
// "codex" with reason rate_limit; the fallback completes the phase.
func TestRun_Exit85RateLimitBenchesFamily(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	eb := &escalatingBridge{
		scriptedBridge: scriptedBridge{responses: map[string]scriptedResp{
			"codex-tmux":  exit85(),
			"claude-tmux": {},
		}},
		escalateCLI: "codex-tmux", reportCLI: "codex-tmux",
		pattern: "rate_limit", // capturedAt zero → stamped at launch (fresh)
	}
	runPhase(t, root, eb)

	benches, err := clihealth.NewStore(root, nil).Load()
	if err != nil {
		t.Fatalf("load benches: %v", err)
	}
	e, ok := benches["codex"]
	if !ok {
		t.Fatalf("RED: codex exit-85 rate_limit did NOT bench the family — the wall is forgotten and the "+
			"next phase re-burns the codex boot (cycle-283 class); benches=%v", benches)
	}
	if e.Reason != "rate_limit" {
		t.Errorf("bench reason=%q, want rate_limit", e.Reason)
	}
	if !e.BenchedUntil.After(e.BenchedAt) {
		t.Errorf("benched_until %v not after benched_at %v", e.BenchedUntil, e.BenchedAt)
	}
}

// TestRun_ActiveBenchDemotesFamilyOnDispatch: with codex actively benched,
// the dispatch chain must START at the fallback — zero benched-CLI launches.
func TestRun_ActiveBenchDemotesFamilyOnDispatch(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	now := time.Now()
	if err := (clihealth.NewStore(root, nil)).Bench(clihealth.Entry{
		Family: "codex", Reason: "rate_limit", BenchedAt: now, BenchedUntil: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	sb := &scriptedBridge{responses: map[string]scriptedResp{"claude-tmux": {}}}
	runPhase(t, root, sb)
	if len(sb.calls) == 0 || sb.calls[0] != "claude-tmux" {
		t.Fatalf("RED: dispatch order %v — an actively benched codex must be demoted so the chain starts at claude-tmux", sb.calls)
	}
}

// TestRun_ExpiredBenchDoesNotDemote: lazy expiry — a bench past benched_until
// no longer reorders the chain (the primary gets its canary shot).
func TestRun_ExpiredBenchDoesNotDemote(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	now := time.Now()
	_ = (clihealth.NewStore(root, nil)).Bench(clihealth.Entry{
		Family: "codex", Reason: "rate_limit", BenchedAt: now.Add(-2 * time.Hour), BenchedUntil: now.Add(-time.Hour),
	})
	sb := &scriptedBridge{responses: map[string]scriptedResp{"codex-tmux": {}}}
	runPhase(t, root, sb)
	if len(sb.calls) == 0 || sb.calls[0] != "codex-tmux" {
		t.Fatalf("dispatch order %v — an EXPIRED bench must not demote (canary-by-default)", sb.calls)
	}
}

// TestRun_CLIMismatchReportDoesNotBench: a stale report naming a different
// CLI must not bench the family that just exited 85.
func TestRun_CLIMismatchReportDoesNotBench(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	eb := &escalatingBridge{
		scriptedBridge: scriptedBridge{responses: map[string]scriptedResp{
			"codex-tmux":  exit85(),
			"claude-tmux": {},
		}},
		escalateCLI: "codex-tmux", reportCLI: "claude-tmux", // mismatch
		pattern: "rate_limit", // fresh stamp; only the cli mismatch rejects
	}
	runPhase(t, root, eb)
	if benches, _ := clihealth.NewStore(root, nil).Load(); len(benches) != 0 {
		t.Errorf("cli-mismatch report benched anyway: %v", benches)
	}
}

// TestRun_StaleReportDoesNotBench: a report captured BEFORE this dispatch
// started is a leftover from an earlier phase — must not bench.
func TestRun_StaleReportDoesNotBench(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	eb := &escalatingBridge{
		scriptedBridge: scriptedBridge{responses: map[string]scriptedResp{
			"codex-tmux":  exit85(),
			"claude-tmux": {},
		}},
		escalateCLI: "codex-tmux", reportCLI: "codex-tmux",
		capturedAt: time.Now().Add(-time.Hour), pattern: "rate_limit", // stale
	}
	runPhase(t, root, eb)
	if benches, _ := clihealth.NewStore(root, nil).Load(); len(benches) != 0 {
		t.Errorf("stale report benched anyway: %v", benches)
	}
}

// TestRun_NonBenchablePatternDoesNotBench: only the benchable set
// (rate_limit) writes a bench — an arbitrary escalation must not.
func TestRun_NonBenchablePatternDoesNotBench(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	eb := &escalatingBridge{
		scriptedBridge: scriptedBridge{responses: map[string]scriptedResp{
			"codex-tmux":  exit85(),
			"claude-tmux": {},
		}},
		escalateCLI: "codex-tmux", reportCLI: "codex-tmux",
		pattern: "trust_prompt", // fresh stamp; only the pattern rejects
	}
	runPhase(t, root, eb)
	if benches, _ := clihealth.NewStore(root, nil).Load(); len(benches) != 0 {
		t.Errorf("non-benchable pattern benched anyway: %v", benches)
	}
}

// TestRun_EnvDisableSkipsBenchAndDemotion: EVOLVE_CLI_HEALTH=0 disables both
// the write and the consult.
func TestRun_EnvDisableSkipsBenchAndDemotion(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	now := time.Now()
	_ = (clihealth.NewStore(root, nil)).Bench(clihealth.Entry{
		Family: "codex", Reason: "rate_limit", BenchedAt: now, BenchedUntil: now.Add(time.Hour),
	})
	sb := &scriptedBridge{responses: map[string]scriptedResp{"codex-tmux": {}}}
	hooks := &fakeHooks{phase: "auditor", agent: "evolve-auditor", model: "sonnet",
		prompt: "x", verdict: core.VerdictPASS, nextPhase: "ship"}
	r := New(Options{Hooks: hooks, Bridge: sb, Prompts: fakePromptsFS("evolve-auditor", "x")})
	if _, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root,
		Workspace:   t.TempDir(),
		Env:         map[string]string{"EVOLVE_CLI_HEALTH": "0"},
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(sb.calls) == 0 || sb.calls[0] != "codex-tmux" {
		t.Fatalf("dispatch order %v — EVOLVE_CLI_HEALTH=0 must disable demotion", sb.calls)
	}
}

// TestRun_AllFamiliesBenchedNeverStrands: bench is advice, not a veto — with
// every candidate's family benched, the phase still dispatches.
func TestRun_AllFamiliesBenchedNeverStrands(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	now := time.Now()
	st := clihealth.NewStore(root, nil)
	_ = st.Bench(clihealth.Entry{Family: "codex", Reason: "rate_limit",
		BenchedAt: now, BenchedUntil: now.Add(time.Hour)})
	_ = st.Bench(clihealth.Entry{Family: "claude", Reason: "rate_limit",
		BenchedAt: now.Add(-time.Minute), BenchedUntil: now.Add(time.Hour)})
	// claude benched EARLIER → least-recently-benched → tried first.
	sb := &scriptedBridge{responses: map[string]scriptedResp{"claude-tmux": {}}}
	runPhase(t, root, sb)
	if len(sb.calls) == 0 {
		t.Fatal("all-benched phase was stranded — bench must be advice, never a veto")
	}
	if sb.calls[0] != "claude-tmux" {
		t.Errorf("dispatch order %v, want least-recently-benched (claude-tmux) first", sb.calls)
	}
}
