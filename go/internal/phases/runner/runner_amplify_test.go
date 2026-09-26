package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// divergentBridge writes one string to the artifact and returns another as stdout, so a test sees which one Classify gets.
type divergentBridge struct {
	fileContent   string // written to req.ArtifactPath when non-empty; left unwritten when empty
	stdoutContent string
	err           error
	gotReq        core.BridgeRequest
}

func (f *divergentBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	f.gotReq = req
	if f.fileContent != "" && req.ArtifactPath != "" {
		if mkErr := os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755); mkErr != nil {
			return core.BridgeResponse{}, mkErr
		}
		if wErr := os.WriteFile(req.ArtifactPath, []byte(f.fileContent), 0o644); wErr != nil {
			return core.BridgeResponse{}, wErr
		}
	}
	return core.BridgeResponse{ExitCode: 0, Stdout: f.stdoutContent}, f.err
}

func (f *divergentBridge) Probe(ctx context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

// alwaysOKVerify reports every deliverable well-formed, isolating the runner's file-versus-stdout choice from parsing.
func alwaysOKVerify(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
	return verifiedFrom(deliverable.Result{OK: true, Phase: phase}, phase, roots), nil
}

func TestRun_NonTimeout_BuildPhase_PrefersWellFormedFileOverDivergentStdout(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-builder", "claude-tmux", nil)
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "sonnet", prompt: "x", verdict: core.VerdictPASS}
	const genuine = "# build\n<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"PASS\"} -->\n"
	const noisy = "Deliverable Contract example (PASS):\n" +
		"<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"PASS\"} -->\n" +
		"Deliverable Contract example (FAIL):\n" +
		"<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"FAIL\"} -->\n" +
		"(these are prompt-echoed examples, not the agent's real report)\n"
	bridge := &divergentBridge{fileContent: genuine, stdoutContent: noisy}
	r := New(Options{
		Hooks: hooks, Bridge: bridge, Prompts: fakePromptsFS("evolve-builder", "x"),
		VerifyFn: alwaysOKVerify,
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hooks.gotArtifact != genuine {
		t.Errorf("Classify received %q, want the genuine on-disk deliverable %q — the fix must be phase-agnostic (build, not just audit)", hooks.gotArtifact, genuine)
	}
	if resp.Reconciled {
		t.Errorf("resp.Reconciled=true for an ordinary non-timeout success that merely preferred the on-disk file; Reconciled is reserved for the ErrArtifactTimeout self-healing path (see PhaseResponse.Reconciled doc) — marking this completion reconciled would corrupt the audit ledger's reconciled_timeout trail with entries for phases that never timed out")
	}
}

func TestRun_NonTimeout_ContractedDeliverableFailsVerification_ShipVerdictDowngraded(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "claude-tmux", nil)
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	const passSentinelFile = "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n(challenge token omitted)\n"
	const panePass = "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"
	bridge := &divergentBridge{fileContent: passSentinelFile, stdoutContent: panePass}
	r := New(Options{
		Hooks: hooks, Bridge: bridge, Prompts: fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(deliverable.Result{
			OK:         false,
			Violations: []deliverable.Violation{{Code: "MISSING_CHALLENGE_TOKEN", Message: "deliverable did not echo the challenge token"}},
		}, nil),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("verdict=%q, want FAIL — a ship-eligible verdict from a verification-FAILED deliverable must be downgraded; it must not launder a PASS past the challenge-token gate (via the file OR the pane)", resp.Verdict)
	}
	if !diagsContain(resp.Diagnostics, "MISSING_CHALLENGE_TOKEN") {
		t.Errorf("contract Codes must surface as diagnostics behind the coherent FAIL; got %+v", resp.Diagnostics)
	}
}

func TestRun_NonTimeout_DeliverableFileNeverWritten_FallsBackToStdout(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-scout", "claude-tmux", nil)
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto", prompt: "x", verdict: core.VerdictPASS}
	const stdout = "# scout\n<!-- evolve-verdict: {\"phase\":\"scout\",\"verdict\":\"PASS\"} -->\n"
	bridge := &divergentBridge{stdoutContent: stdout} // fileContent left empty: nothing written to ArtifactPath
	r := New(Options{
		Hooks: hooks, Bridge: bridge, Prompts: fakePromptsFS("evolve-scout", "x"),
		VerifyFn: alwaysOKVerify,
	})

	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hooks.gotArtifact != stdout {
		t.Errorf("Classify received %q, want fallback to bridge stdout %q — a missing deliverable file must never crash the phase or silently blank the artifact; it must fall back exactly as before", hooks.gotArtifact, stdout)
	}
}
