package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestRun_ARunnerWithoutABridgeFailsBeforeAnyDispatchWithAnEmptyResponse(t *testing.T) {
	r := New(Options{Hooks: &fakeHooks{phase: "scout", agent: "evolve-scout"}, Prompts: fakePromptsFS("evolve-scout", "x")})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir(), ProjectRoot: t.TempDir()})

	if err == nil || !strings.Contains(err.Error(), "scout: bridge required") {
		t.Fatalf("err = %v", err)
	}
	if resp.Verdict != "" || resp.Phase != "" {
		t.Errorf("an unprepared phase has no verdict: %+v", resp)
	}
}

func TestRun_AnUnlaunchedDispatchIsAFailedPhaseNotAJudgedOne(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-scout", "claude-tmux", nil)
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: fmt.Errorf("walk refused every attempt: %w", bridgechain.ErrUnlaunched)}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x")})

	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})

	if err == nil || resp.Verdict != core.VerdictFAIL || resp.Phase != "scout" {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
	if hooks.classifyCalls != 0 {
		t.Errorf("an unlaunched agent produced nothing to classify; Classify ran %d time(s)", hooks.classifyCalls)
	}
	if len(resp.Diagnostics) == 0 || !strings.Contains(resp.Diagnostics[len(resp.Diagnostics)-1].Message, "walk refused every attempt") {
		t.Errorf("the cause must ride the response: %+v", resp.Diagnostics)
	}
}

func captureStderrF4(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	func() {
		defer func() {
			os.Stderr = orig
			w.Close()
		}()
		fn()
	}()
	return <-done
}

func TestLogDecision_EveryRoutingTraceLineIsLoggedUnderThePhase(t *testing.T) {
	var diag strings.Builder
	b := New(Options{Hooks: &fakeHooks{phase: "scout", agent: "evolve-scout"}, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-scout", "x"), Diag: log.Console{Out: &diag, Err: &diag}})
	d := cliroute.Decision{
		Plan:  llmroute.Plan{Candidates: []string{"claude-tmux"}, PrimarySource: "profile"},
		Trace: []string{"rule: default", "benched: none"},
	}

	got := captureStderrF4(t, func() {
		b.logDecision(core.PhaseRequest{}, phasePreparation{phase: "scout", profileName: "scout", profilePath: "/p.json"}, d, policy.Policy{})
	})

	for _, want := range []string{
		"[runner] phase=scout routing: rule: default",
		"[runner] phase=scout routing: benched: none",
		"[runner] phase=scout agent=scout cli=claude-tmux (source=profile) profile=/p.json",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("log lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "fallback=") {
		t.Errorf("a single-candidate plan has no fallback list:\n%s", got)
	}
	if !strings.Contains(diag.String(), "no advisor overlay (profile default)") {
		t.Errorf("the overlay line goes to the injected diag sink: %q", diag.String())
	}
}

func TestFailedPhase_CarriesTheEarlyResponseWhenThereIsOneAndAnEmptyOneWhenNot(t *testing.T) {
	boom := errors.New("preparation failed")
	early := &core.PhaseResponse{Phase: "scout", Verdict: core.VerdictFAIL, ArtifactsDir: "/ws"}

	got, err := failedPhase(early, boom)
	if !errors.Is(err, boom) || got.Phase != "scout" || got.Verdict != core.VerdictFAIL || got.ArtifactsDir != "/ws" {
		t.Fatalf("with an early response: got=%+v err=%v", got, err)
	}
	got, err = failedPhase(nil, boom)
	if !errors.Is(err, boom) || got.Phase != "" || got.Verdict != "" {
		t.Fatalf("without one the response is empty: got=%+v err=%v", got, err)
	}
}

func TestLogDecision_ThePolicyPinAdvisorOverlayAndDefaultEachGetTheirOwnLine(t *testing.T) {
	pinned := policy.Policy{Pins: map[string]policy.Pin{"scout": {CLI: "claude-tmux", Model: "opus"}}}
	cases := []struct {
		name        string
		req         core.PhaseRequest
		rule        string
		pol         policy.Policy
		wantDiag    string
		wantStderr  string
		notInDiag   string
		notInStderr string
	}{
		{"a legacy pin that the policy actually holds", core.PhaseRequest{}, cliroute.RuleLegacyPin, pinned, "", `policy pin: cli="claude-tmux" model="opus"`, "no advisor overlay", ""},
		{"a legacy-pin decision without a policy pin falls to the default line", core.PhaseRequest{}, cliroute.RuleLegacyPin, policy.Policy{}, "no advisor overlay (profile default)", "", "policy pin", "policy pin"},
		{"a policy pin without a legacy-pin decision is not announced", core.PhaseRequest{}, "", pinned, "no advisor overlay (profile default)", "", "", "policy pin"},
		{"an advisor overlay", core.PhaseRequest{ModelRoutingCLI: "codex-tmux", ModelRoutingTier: "deep"}, "", policy.Policy{}, "advisor overlay cli=codex-tmux tier=deep", "", "no advisor overlay", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var diag strings.Builder
			b := New(Options{Hooks: &fakeHooks{phase: "scout", agent: "evolve-scout"}, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-scout", "x"), Diag: log.Console{Out: &diag, Err: &diag}})
			d := cliroute.Decision{Rule: tc.rule, Plan: llmroute.Plan{Candidates: []string{"claude-tmux", "agy-tmux"}, PrimarySource: "profile", Triggers: []int{85}}}

			stderr := captureStderrF4(t, func() {
				b.logDecision(tc.req, phasePreparation{phase: "scout", profileName: "scout", profilePath: "/p.json"}, d, tc.pol)
			})

			if tc.wantDiag != "" && !strings.Contains(diag.String(), tc.wantDiag) {
				t.Errorf("diag lacks %q: %q", tc.wantDiag, diag.String())
			}
			if tc.wantStderr != "" && !strings.Contains(stderr, tc.wantStderr) {
				t.Errorf("stderr lacks %q: %q", tc.wantStderr, stderr)
			}
			if tc.notInDiag != "" && strings.Contains(diag.String(), tc.notInDiag) {
				t.Errorf("diag must not contain %q: %q", tc.notInDiag, diag.String())
			}
			if tc.notInStderr != "" && strings.Contains(stderr, tc.notInStderr) {
				t.Errorf("stderr must not contain %q: %q", tc.notInStderr, stderr)
			}
			if !strings.Contains(stderr, "fallback=[agy-tmux] triggers=[85]") {
				t.Errorf("a multi-candidate plan lists its fallbacks and triggers: %q", stderr)
			}
		})
	}
}
