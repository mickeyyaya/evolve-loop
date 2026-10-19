package runner

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const cycle283WallTail = "■ You've hit your usage limit. Upgrade to Pro (https://chatgpt.com/explore/pro), " +
	"visit https://chatgpt.com/codex/settings/usage to purchase more credits or try again at 6:11 AM."

type escalatingBridge struct {
	scriptedBridge
	escalateCLI string
	reportCLI   string
	capturedAt  time.Time
	pattern     string
}

func (e *escalatingBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if req.CLI == e.escalateCLI {
		capturedAt := e.capturedAt
		if capturedAt.IsZero() {
			capturedAt = time.Now()
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

func TestRun_Exit85RateLimitBenchesFamily(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	eb := &escalatingBridge{
		scriptedBridge: scriptedBridge{responses: map[string]scriptedResp{
			"codex-tmux":  exit85(),
			"claude-tmux": {},
		}},
		escalateCLI: "codex-tmux", reportCLI: "codex-tmux",
		pattern: "rate_limit",
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

func TestRun_CLIMismatchReportDoesNotBench(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	eb := &escalatingBridge{
		scriptedBridge: scriptedBridge{responses: map[string]scriptedResp{
			"codex-tmux":  exit85(),
			"claude-tmux": {},
		}},
		escalateCLI: "codex-tmux", reportCLI: "claude-tmux",
		pattern: "rate_limit",
	}
	runPhase(t, root, eb)
	if benches, _ := clihealth.NewStore(root, nil).Load(); len(benches) != 0 {
		t.Errorf("cli-mismatch report benched anyway: %v", benches)
	}
}

func TestRun_StaleReportDoesNotBench(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	eb := &escalatingBridge{
		scriptedBridge: scriptedBridge{responses: map[string]scriptedResp{
			"codex-tmux":  exit85(),
			"claude-tmux": {},
		}},
		escalateCLI: "codex-tmux", reportCLI: "codex-tmux",
		capturedAt: time.Now().Add(-time.Hour), pattern: "rate_limit",
	}
	runPhase(t, root, eb)
	if benches, _ := clihealth.NewStore(root, nil).Load(); len(benches) != 0 {
		t.Errorf("stale report benched anyway: %v", benches)
	}
}

func TestRun_NonBenchablePatternDoesNotBench(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	eb := &escalatingBridge{
		scriptedBridge: scriptedBridge{responses: map[string]scriptedResp{
			"codex-tmux":  exit85(),
			"claude-tmux": {},
		}},
		escalateCLI: "codex-tmux", reportCLI: "codex-tmux",
		pattern: "trust_prompt",
	}
	runPhase(t, root, eb)
	if benches, _ := clihealth.NewStore(root, nil).Load(); len(benches) != 0 {
		t.Errorf("non-benchable pattern benched anyway: %v", benches)
	}
}

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

func TestRun_AllFamiliesBenchedNeverStrands(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	now := time.Now()
	st := clihealth.NewStore(root, nil)
	_ = st.Bench(clihealth.Entry{Family: "codex", Reason: "rate_limit",
		BenchedAt: now, BenchedUntil: now.Add(time.Hour)})
	_ = st.Bench(clihealth.Entry{Family: "claude", Reason: "rate_limit",
		BenchedAt: now.Add(-time.Minute), BenchedUntil: now.Add(time.Hour)})
	sb := &scriptedBridge{responses: map[string]scriptedResp{"claude-tmux": {}}}
	runPhase(t, root, sb)
	if len(sb.calls) == 0 {
		t.Fatal("all-benched phase was stranded — bench must be advice, never a veto")
	}
	if sb.calls[0] != "claude-tmux" {
		t.Errorf("dispatch order %v, want least-recently-benched (claude-tmux) first", sb.calls)
	}
}

func TestRun_Cycle1951AnAgyExit85AtDeepBenchesOnlyTheArgonModelGroup(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	profile := `{"name":"evolve-builder","cli":"agy-tmux","model_tier_default":"deep","cli_fallback":["claude-tmux"]}`
	if err := os.WriteFile(filepath.Join(dir, "builder.json"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	eb := &escalatingBridge{
		scriptedBridge: scriptedBridge{responses: map[string]scriptedResp{"agy-tmux": exit85(), "claude-tmux": {}}},
		escalateCLI:    "agy-tmux", reportCLI: "agy-tmux", pattern: clihealth.ExhaustedPattern,
	}
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "deep", prompt: "x", verdict: core.VerdictPASS, nextPhase: "audit"}
	r := New(Options{Hooks: hooks, Bridge: eb, Prompts: fakePromptsFS("evolve-builder", "x")})
	if _, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	benches, err := clihealth.NewStore(root, nil).Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, family := benches["agy"]; family {
		t.Fatalf("benches %v; an exit 85 on agy at deep benched all of agy, not the Argon model group (cycle 1951)", benches)
	}
	if _, scoped := benches["agy@Gemini 4 Argon"]; !scoped {
		t.Fatalf("benches %v; want the wall benched on the Argon model group", benches)
	}
}
