package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// WS3-S1 (ADR-0052): the advisor persists the RAW prompt and response —
// secret-redacted — BEFORE the caller parses them, so a routing decision is
// debuggable (WS3-S4 explain) and replayable (WS3-S5) even when the parsed
// plan is unremarkable. Capture is best-effort / fail-open: a write failure
// must never fail the advisor, or a forensic nicety would become a routing
// outage that degrades the whole cycle to the static path.

// readAdvisorArtifact reads a capture artifact, failing the test if it is
// absent — the capture is the behavior under test, so a missing file is a
// real failure, not a skip.
func readAdvisorArtifact(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read capture artifact %s: %v", path, err)
	}
	return string(b)
}

func TestAdvisorLaunch_PersistsRedactedPromptAndResponse(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	const stdout = `[{"phase":"scout","run":true,"justification":"fresh discovery work"}]`
	fb := &fakeBridge{stdout: stdout}
	in := baseRouteInput()
	in.Workspace = ws

	if _, err := NewPhaseAdvisor(fb).Plan(in); err != nil {
		t.Fatalf("Plan: %v", err)
	}

	// kind == "plan" for the whole-cycle plan (WS3-S5 replays
	// advisor-response-plan.txt by this exact name).
	gotPrompt := readAdvisorArtifact(t, filepath.Join(ws, "advisor-prompt-plan.txt"))
	gotResp := readAdvisorArtifact(t, filepath.Join(ws, "advisor-response-plan.txt"))

	// The response artifact is the raw bridge stdout (no secrets here, so
	// redaction is the identity) — byte-for-byte, so WS3-S5 can reparse it.
	if gotResp != stdout {
		t.Errorf("response artifact = %q, want the raw stdout %q", gotResp, stdout)
	}
	// The prompt artifact is exactly the prompt the bridge received (no secrets
	// ⇒ identity). Capturing the SENT prompt — before parse — is the point.
	if gotPrompt != fb.gotReq.Prompt {
		t.Errorf("prompt artifact must equal the prompt sent to the bridge\n got: %q\nwant: %q", gotPrompt, fb.gotReq.Prompt)
	}
}

func TestAdvisorLaunch_RedactsSecrets(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	// A secret-shaped token (sk- key) pasted into the goal — exactly the leak a
	// persisted, possibly cross-vendor-replayed artifact must never carry (S6).
	const secret = "sk-livesecret0123456789ABCDEF"
	fb := &fakeBridge{stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}
	in := baseRouteInput()
	in.Workspace = ws
	in.GoalText = "rotate the leaked key " + secret + " across the fleet"

	if _, err := NewPhaseAdvisor(fb).Plan(in); err != nil {
		t.Fatalf("Plan: %v", err)
	}

	gotPrompt := readAdvisorArtifact(t, filepath.Join(ws, "advisor-prompt-plan.txt"))
	if strings.Contains(gotPrompt, secret) {
		t.Errorf("persisted prompt must REDACT the secret-shaped token; it leaked:\n%s", gotPrompt)
	}
	if !strings.Contains(gotPrompt, "[REDACTED]") {
		t.Errorf("persisted prompt must carry the redaction marker:\n%s", gotPrompt)
	}
	// Invariant: we redact only the PERSISTED COPY. The live prompt the advisor
	// reasons over still carries the real text, or routing quality degrades.
	if !strings.Contains(fb.gotReq.Prompt, secret) {
		t.Error("the live prompt sent to the bridge must keep the real text; only the persisted artifact is redacted")
	}
}

func TestAdvisorLaunch_ArtifactWriteFailureDoesNotFailAdvisor(t *testing.T) {
	t.Parallel()
	fb := &fakeBridge{stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}
	adv := NewPhaseAdvisor(fb)
	// Inject a writer that always fails — the capture must swallow it.
	adv.writeArtifact = func(string, []byte) error { return errors.New("disk full") }
	in := baseRouteInput()
	in.Workspace = t.TempDir()

	plan, err := adv.Plan(in)
	if err != nil {
		t.Fatalf("a forensic-capture write failure must NOT fail the advisor (fail-open): %v", err)
	}
	if plan == nil || len(plan.Entries) != 1 {
		t.Fatalf("plan must still parse despite capture failure: %+v", plan)
	}
}

// TestAdvisorLaunch_CaptureKindPerDecision proves the <kind> token distinguishes
// the two decision types so a plan capture never clobbers a proposal capture in
// the same workspace.
func TestAdvisorLaunch_CaptureKindPerDecision(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	fb := &fakeBridge{stdout: `{"next_phase":"audit","justification":"build green"}`}
	in := baseRouteInput()
	in.Workspace = ws

	if _, err := NewPhaseAdvisor(fb).Propose(in); err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, "advisor-prompt-proposal.txt")); err != nil {
		t.Errorf("proposal capture must use kind=proposal: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, "advisor-response-proposal.txt")); err != nil {
		t.Errorf("proposal response capture must use kind=proposal: %v", err)
	}
}
