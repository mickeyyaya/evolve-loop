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

// divergentBridge writes one string to the contracted artifact file and
// returns a DIFFERENT string as bridge stdout — simulating a driver that
// captured noisy tmux scrollback (e.g. a Deliverable Contract's own
// prompt-echoed example verdict sentinels) instead of, or alongside, the
// agent's genuine on-disk deliverable. Unlike fakeBridge (runner_test.go),
// which forces Stdout to equal the written file content, this type lets a
// test assert exactly which of the two sources Classify actually receives.
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

// alwaysOKVerify is an Options.VerifyFn stub that unconditionally reports the
// deliverable as well-formed, regardless of what (if anything) is on disk —
// used to isolate the runner's own file-vs-stdout selection logic from
// deliverable.Verify's real parsing rules (covered separately in
// internal/deliverable and internal/phasecontract tests).
func alwaysOKVerify(phase string, _ phasecontract.Roots) (deliverable.Result, error) {
	return deliverable.Result{OK: true, Phase: phase}, nil
}

// alwaysFailVerify is the negative counterpart: the deliverable is always
// reported as failing well-formedness checks, exercising the "never
// downgrades" fail-open guarantee.
func alwaysFailVerify(phase string, _ phasecontract.Roots) (deliverable.Result, error) {
	return deliverable.Result{OK: false, Phase: phase}, nil
}

// TestRun_NonTimeout_BuildPhase_PrefersWellFormedFileOverDivergentStdout
// amplifies runner-classify-prefer-deliverable-file beyond the audit-only RED
// test: the build-report explicitly claims the fix is "phase-agnostic ...
// not just audit". A regression that re-hardcodes the audit phase name, or
// only wires the fallback into the audit Hooks, would pass the original test
// but fail this one.
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

// TestRun_NonTimeout_DeliverableFailsVerification_FallsBackToStdout guards the
// "never downgrades" half of the contract: a plausible buggy implementation
// might prefer the file whenever it merely EXISTS, skipping the verifyFn OK
// check. This adversarial case makes that regression concrete: the file is
// present but reported not-well-formed, so stdout (which — in this scenario —
// carries the agent's real failure detail) must still win.
func TestRun_NonTimeout_DeliverableFailsVerification_FallsBackToStdout(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "claude-tmux", nil)
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictFAIL}
	const malformed = "not a well-formed deliverable at all\n"
	const stdout = "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\"} -->\nreal failure detail\n"
	bridge := &divergentBridge{fileContent: malformed, stdoutContent: stdout}
	r := New(Options{
		Hooks: hooks, Bridge: bridge, Prompts: fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: alwaysFailVerify,
	})

	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hooks.gotArtifact != stdout {
		t.Errorf("Classify received %q, want fallback to bridge stdout %q — a deliverable that FAILS verification must never be preferred; the fix may only ever upgrade toward a well-formed file, never trust a malformed one", hooks.gotArtifact, stdout)
	}
}

// TestRun_NonTimeout_DeliverableFileNeverWritten_FallsBackToStdout covers the
// "absent" branch of the fail-open guarantee: verifyFn (lying, or checking a
// stale cache) reports OK=true, but the agent's contracted file was never
// actually written to disk (a clean exit that skipped the write, not a
// timeout). The runner must fall back to stdout rather than erroring out or
// silently classifying against an empty/missing artifact.
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
