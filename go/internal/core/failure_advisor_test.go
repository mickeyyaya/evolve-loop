package core

// failure_advisor_test.go — ADR-0044 Slice 5 RED tests: the LLM failure
// advisor (the AI escalation TAIL — reached only for CauseUnknown terminal
// states the deterministic registry cannot classify; Core Agent Rule 5).
// Modeled on phase_advisor_test.go: fakeBridge scripts the LLM, the advisor
// must parse a strict-JSON verdict, and EVERY failure mode (nil bridge,
// malformed JSON, invalid vocabulary) returns an error so the caller
// escalates instead of acting on garbage — fail-safe-to-deterministic.

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/recovery"
)

func baseFailureInput() FailureAdviseInput {
	return FailureAdviseInput{
		Phase: "build", CLI: "codex-tmux", ExitCode: 81,
		PaneTail:    "⚠ session credential vault locked; interactive unlock required",
		Workspace:   "/tmp/ws",
		ProjectRoot: "/proj",
		Cycle:       262,
		Env:         map[string]string{},
	}
}

func TestFailureAdvisor_ParsesCauseAndSignature(t *testing.T) {
	t.Parallel()
	fb := &fakeBridge{stdout: `{"cause":"dead_shell","pane_substr":"credential vault locked","justification":"the REPL exited to a locked vault prompt; no agent is running"}`}
	a := NewFailureAdvisor(fb)
	adv, err := a.Advise(context.Background(), baseFailureInput())
	if err != nil {
		t.Fatalf("Advise: %v", err)
	}
	if adv.Cause != string(recovery.CauseDeadShell) {
		t.Errorf("cause=%q, want dead_shell", adv.Cause)
	}
	if adv.PaneSubstr != "credential vault locked" {
		t.Errorf("pane_substr=%q", adv.PaneSubstr)
	}
	if adv.Justification == "" {
		t.Error("justification required — every recovery decision is justified")
	}
	// Dispatch contract: the failure-advisor profile + agent identity, the
	// workspace artifact, and the artifact completion (the uniform contract).
	if fb.gotReq.Agent != "failure-advisor" {
		t.Errorf("agent=%q, want failure-advisor", fb.gotReq.Agent)
	}
	if !strings.HasSuffix(fb.gotReq.Profile, "failure-advisor.json") {
		t.Errorf("profile=%q, want .evolve/profiles/failure-advisor.json", fb.gotReq.Profile)
	}
	if !strings.HasSuffix(fb.gotReq.ArtifactPath, "failure-advice.json") {
		t.Errorf("artifact=%q, want failure-advice.json in the workspace", fb.gotReq.ArtifactPath)
	}
	if !strings.Contains(fb.gotReq.Prompt, "credential vault locked") {
		t.Error("prompt must carry the pane evidence")
	}
}

// The persona's documented non-fatal signal: empty cause = "this pane is not
// fatal". Still an error (caller escalates — correct outcome), but the
// message must say NON-FATAL, not pretend the model hallucinated.
func TestFailureAdvisor_EmptyCause_NonFatalSignal(t *testing.T) {
	t.Parallel()
	a := NewFailureAdvisor(&fakeBridge{stdout: `{"cause":"","pane_substr":"","justification":"not fatal: agent recovering natively"}`})
	_, err := a.Advise(context.Background(), baseFailureInput())
	if err == nil {
		t.Fatal("empty cause must error so the caller escalates")
	}
	if !strings.Contains(err.Error(), "non-fatal") {
		t.Errorf("error must surface the non-fatal judgment distinctly; got %v", err)
	}
}

func TestFailureAdvisor_MalformedJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	a := NewFailureAdvisor(&fakeBridge{stdout: "I think the cause is probably a dead shell?"})
	if _, err := a.Advise(context.Background(), baseFailureInput()); err == nil {
		t.Fatal("prose instead of strict JSON must error (caller escalates; never act on unparsed judgment)")
	}
}

func TestFailureAdvisor_UnknownCause_ReturnsError(t *testing.T) {
	t.Parallel()
	a := NewFailureAdvisor(&fakeBridge{stdout: `{"cause":"gremlins","pane_substr":"something long enough","justification":"j"}`})
	if _, err := a.Advise(context.Background(), baseFailureInput()); err == nil {
		t.Fatal("a cause outside the typed vocabulary must error — hallucinated causes never enter the registry")
	}
}

func TestFailureAdvisor_NilBridge_FailSafe(t *testing.T) {
	t.Parallel()
	a := NewFailureAdvisor(nil)
	if _, err := a.Advise(context.Background(), baseFailureInput()); err == nil {
		t.Fatal("nil bridge must fail safe with an error, never panic or fabricate")
	}
}

func TestFailureAdvisor_BridgeError_Propagates(t *testing.T) {
	t.Parallel()
	a := NewFailureAdvisor(&fakeBridge{err: ErrArtifactTimeout})
	if _, err := a.Advise(context.Background(), baseFailureInput()); err == nil {
		t.Fatal("bridge failure must propagate (the advisor itself can stall — cycle-262's retro did)")
	}
}
