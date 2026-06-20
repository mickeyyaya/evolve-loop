package subagent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file closes the apicover gap for symbols the existing tests exercise
// through their producer functions but never NAME by their exported type/const
// identifier. apicover flags such symbols UNCOVERED. Each test below binds the
// real producer's output to an explicitly-typed variable (naming the type) and
// asserts a load-bearing field, so the type identifier is both named AND its
// producing function is executed.

// TestTierHaiku_FlowsThroughModelTierResolver names the TierHaiku const and
// asserts it through its real consumer: ResolveModelTier returns the hint
// verbatim, and "haiku" is exactly TierHaiku. This pins the resolver's Rule-1
// contract (MODEL_TIER_HINT wins for every agent) using the named const rather
// than a bare string literal.
func TestTierHaiku_FlowsThroughModelTierResolver(t *testing.T) {
	tier, err := ResolveModelTier(
		ResolveModelTierRequest{
			ProfilePath:   "/dev/null",
			ModelTierHint: TierHaiku,
		},
		ResolveModelTierOptions{
			ReadProfile: stubReadProfile(`{"role":"builder","model_tier_default":"sonnet"}`),
			ReadState:   stubReadState("", os.ErrNotExist),
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != TierHaiku {
		t.Errorf("hint TierHaiku should win, got %q want %q", tier, TierHaiku)
	}
	// TierHaiku is the canonical "haiku" label adapters accept as -m.
	if TierHaiku != "haiku" {
		t.Errorf("TierHaiku=%q, want haiku", TierHaiku)
	}
}

// TestCheckCtxAdvisoryResult_BoundFromProducer names CheckCtxAdvisoryResult and
// binds it from its sole producer CheckCtxAdvisory, asserting the Emit/Threshold
// contract: a profile threshold below the current token count emits an advisory
// carrying both numbers.
func TestCheckCtxAdvisoryResult_BoundFromProducer(t *testing.T) {
	p := writeProfile(t, `{"role":"tester","context_clear_trigger_tokens":120000}`)
	var r CheckCtxAdvisoryResult
	r, err := CheckCtxAdvisory(p, 180000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Emit {
		t.Errorf("CheckCtxAdvisoryResult.Emit=false, want true for tokens>threshold")
	}
	if r.Threshold != 120000 {
		t.Errorf("CheckCtxAdvisoryResult.Threshold=%d, want 120000", r.Threshold)
	}
	if !strings.Contains(r.Message, "180000") || !strings.Contains(r.Message, "120000") {
		t.Errorf("CheckCtxAdvisoryResult.Message missing token/threshold: %q", r.Message)
	}
}

// TestCheckTokenResult_BoundFromProducer names CheckTokenResult and binds it
// from CheckToken, asserting the OK/Reason verdict on a token-bearing artifact.
func TestCheckTokenResult_BoundFromProducer(t *testing.T) {
	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "artifact.md")
	const token = "deadbeefcafe1234"
	if err := os.WriteFile(artifact, []byte("header\n<!-- challenge-token: "+token+" -->\nbody\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	var r CheckTokenResult = CheckToken(artifact, token)
	if !r.OK {
		t.Errorf("CheckTokenResult.OK=false, want true for present token; reason=%q", r.Reason)
	}
	if !strings.Contains(r.Reason, "OK:") {
		t.Errorf("CheckTokenResult.Reason=%q, want OK message", r.Reason)
	}

	// And the failing arm: an absent token flips OK and explains why.
	var miss CheckTokenResult = CheckToken(artifact, "not-the-token")
	if miss.OK {
		t.Errorf("CheckTokenResult.OK=true for absent token, want false")
	}
	if !strings.Contains(miss.Reason, "token absent") {
		t.Errorf("CheckTokenResult.Reason=%q, want 'token absent'", miss.Reason)
	}
}

// TestDispatchParallelResult_BoundFromProducer names DispatchParallelResult and
// binds it from DispatchParallel, asserting WorkerCount/QualityTier/exit codes
// on the happy fan-out path (reusing the package's dispatchHappyOpts fixture).
func TestDispatchParallelResult_BoundFromProducer(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}
	var r DispatchParallelResult
	r, err := DispatchParallel(context.Background(), DispatchParallelRequest{
		Agent:              "scout",
		Cycle:              3,
		WorkspacePath:      ws,
		ProfilesDir:        "/p",
		AdaptersDir:        "/a",
		ProjectRoot:        tmp,
		LedgerPath:         filepath.Join(tmp, "ledger.jsonl"),
		CachePrefixEnabled: false,
	}, dispatchHappyOpts(t, sampleScoutProfile))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.WorkerCount != 2 {
		t.Errorf("DispatchParallelResult.WorkerCount=%d, want 2", r.WorkerCount)
	}
	if r.FanoutExitCode != 0 || r.AggregatorExit != 0 {
		t.Errorf("DispatchParallelResult exit codes non-zero: fanout=%d agg=%d", r.FanoutExitCode, r.AggregatorExit)
	}
	if r.QualityTier != "full" {
		t.Errorf("DispatchParallelResult.QualityTier=%q, want full", r.QualityTier)
	}
}

// TestRunResult_BoundFromProducer names RunResult and binds it from Run,
// asserting the Verdict/CLI/Model fields on the happy single-agent path
// (reusing the package's runHappyOpts fixture).
func TestRunResult_BoundFromProducer(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "workspace")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}
	var r RunResult
	r, err := Run(context.Background(), RunRequest{
		Agent:            "scout",
		Cycle:            2,
		WorkspacePath:    ws,
		ProfilesDir:      "/p",
		AdaptersDir:      "/a",
		ProjectRoot:      tmp,
		PluginRoot:       tmp,
		PromptReader:     strings.NewReader("do it\n"),
		AdversarialAudit: true,
	}, runHappyOpts(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Verdict != VerdictPASS {
		t.Errorf("RunResult.Verdict=%q, want PASS", r.Verdict)
	}
	if r.CLI != "claude" || r.Model != "sonnet" {
		t.Errorf("RunResult CLI/Model = %q/%q, want claude/sonnet", r.CLI, r.Model)
	}
	if r.ChallengeToken == "" {
		t.Errorf("RunResult.ChallengeToken empty, want a minted token")
	}
}

// TestValidateProfileResult_BoundFromProducer names ValidateProfileResult and
// binds it from ValidateProfile, asserting the resolved CLI/Model/source fields
// on the happy validate path (reusing the package's happyOpts fixture).
func TestValidateProfileResult_BoundFromProducer(t *testing.T) {
	body := `{"role":"scout","cli":"claude","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/scout-report.md"}`
	var r ValidateProfileResult
	r, err := ValidateProfile(context.Background(), ValidateProfileRequest{
		Agent:       "scout",
		ProfilesDir: "/p",
		AdaptersDir: "/a",
		ProjectRoot: "/r",
	}, happyOpts(body, "claude"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.CLI != "claude" {
		t.Errorf("ValidateProfileResult.CLI=%q, want claude", r.CLI)
	}
	if r.Model != "sonnet" {
		t.Errorf("ValidateProfileResult.Model=%q, want sonnet", r.Model)
	}
	if r.CLIResolutionSrc != "profile" {
		t.Errorf("ValidateProfileResult.CLIResolutionSrc=%q, want profile", r.CLIResolutionSrc)
	}
	if r.AdapterExitCode != 0 {
		t.Errorf("ValidateProfileResult.AdapterExitCode=%d, want 0", r.AdapterExitCode)
	}
}
