package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const (
	verifiedPASS = "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"
	swappedFAIL  = "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\"} -->\n(swapped in AFTER Verify read the file)\n"
)

// swapAfterVerify overwrites the deliverable right after verifying it: the racing writer, placed in the verify-classify window.
func swapAfterVerify(t *testing.T, res deliverable.Result) func(string, phasecontract.Roots) (deliverable.Result, error) {
	t.Helper()
	return func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
		verified := verifiedFrom(res, phase, roots)
		if err := os.WriteFile(filepath.Join(roots.Workspace, phase+"-report.md"), []byte(swappedFAIL), 0o644); err != nil {
			t.Fatalf("stage the racing write: %v", err)
		}
		return verified, nil
	}
}

func TestRun_CleanExit_FileSwappedAfterVerify_ClassifiesTheVerifiedBytes(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	nb := &noisyStdoutBridge{fileContent: verifiedPASS, stdout: "pane scrollback\n"}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   nb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: swapAfterVerify(t, deliverable.Result{OK: true}),
		SleepFn:  func(time.Duration) {},
	})

	if _, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hooks.gotArtifact != verifiedPASS {
		t.Errorf("Classify received %q, want the VERIFIED bytes %q — a re-read of the path classifies content Verify never saw (the TOCTOU)", hooks.gotArtifact, verifiedPASS)
	}
}

func TestRun_Reconciled_FileSwappedAfterVerify_ClassifiesTheVerifiedBytes(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr(), writeArtifact: verifiedPASS}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: swapAfterVerify(t, deliverable.Result{OK: true}),
		SleepFn:  func(time.Duration) {},
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("a well-formed deliverable on a teardown must reconcile: %v", err)
	}
	if !resp.Reconciled {
		t.Fatal("resp.Reconciled must be true on the reconciled teardown path")
	}
	if hooks.gotArtifact != verifiedPASS {
		t.Errorf("Classify received %q, want the VERIFIED bytes %q — the reconcile path must not re-read the artifact either", hooks.gotArtifact, verifiedPASS)
	}
}

func TestRun_DispatchedArtifactDiffersFromContractPath_ClassifiesTheDispatchedFile(t *testing.T) {
	ws := t.TempDir()
	const partial = "[intent-unchanged] goal_hash=abc12345\n"
	hooks := &fakeHooks{phase: "intent", artifact: "intent-delta.md", agent: "evolve-intent", model: "auto", prompt: "x", verdict: core.VerdictSKIPPED}
	nb := &noisyStdoutBridge{fileContent: partial, stdout: "pane scrollback\n"}
	// Verify resolved the contract path (intent.md), found nothing, and reports missing_artifact with no content.
	verifyOtherPath := func(_ string, roots phasecontract.Roots) (deliverable.Result, error) {
		return deliverable.Result{
			Phase:        "intent",
			ArtifactPath: filepath.Join(roots.Workspace, "intent.md"),
			Violations:   []deliverable.Violation{{Code: deliverable.CodeMissingArtifact, Message: "deliverable not found"}},
		}, nil
	}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   nb,
		Prompts:  fakePromptsFS("evolve-intent", "x"),
		VerifyFn: verifyOtherPath,
		SleepFn:  func(time.Duration) {},
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hooks.gotArtifact != partial {
		t.Errorf("Classify received %q, want the DISPATCHED artifact's bytes %q — Verify judged a different path, so its (empty) content must not be classified", hooks.gotArtifact, partial)
	}
	if resp.Verdict != core.VerdictSKIPPED {
		t.Errorf("verdict=%q, want SKIPPED — a legitimate non-ship verdict from partial content must survive", resp.Verdict)
	}
}

func TestRun_ContractedPhase_ClassifiesWithoutReadingTheArtifactAgain(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	nb := &noisyStdoutBridge{fileContent: verifiedPASS, stdout: "pane scrollback\n"}
	removeAfterVerify := func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
		verified := verifiedFrom(deliverable.Result{OK: true}, phase, roots)
		if err := os.Remove(filepath.Join(roots.Workspace, phase+"-report.md")); err != nil {
			t.Fatalf("stage the racing delete: %v", err)
		}
		return verified, nil
	}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   nb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: removeAfterVerify,
		SleepFn:  func(time.Duration) {},
	})

	if _, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hooks.gotArtifact != verifiedPASS {
		t.Errorf("Classify received %q, want the verified bytes %q — a deliverable deleted after Verify proves whether a second read happens", hooks.gotArtifact, verifiedPASS)
	}
}
