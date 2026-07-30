package runner

// runner_single_read_test.go — the verify→classify TOCTOU (deliverable-verified-
// bytes-single-read). The file-authoritative verdict path used to read the
// deliverable TWICE: deliverable.Verify read it to judge well-formedness, then
// Run re-read the same path to feed Classify. A process racing the just-finished
// bridge launch (a sibling lane's cleanup, an agent's stray re-write) could swap
// the file in between, so the CLASSIFIED bytes were not provably the bytes that
// PASSED Verify. The seam now returns the verified bytes (deliverable.Result.
// Content) and Run classifies THOSE — one read, no window.

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

// swapAfterVerify is a VerifyFn double that returns the bytes it verified and
// then OVERWRITES the deliverable — the racing writer, deterministically placed
// in the exact window between the verify read and the classify read.
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

// TestRun_CleanExit_FileSwappedAfterVerify_ClassifiesTheVerifiedBytes — the
// clean-exit contracted path. Verify judged verifiedPASS; a racing process then
// swapped the file to swappedFAIL. Classify must receive the VERIFIED bytes: the
// verdict and the content have to come from one read, or the recorded verdict
// belongs to content no gate ever checked.
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

// TestRun_Reconciled_FileSwappedAfterVerify_ClassifiesTheVerifiedBytes — the
// same invariant on the reconcile-on-teardown path, which had its own second
// read. A bridge infra teardown whose deliverable verified OK must classify the
// bytes that verified, not whatever the racing writer left behind.
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

// TestRun_DispatchedArtifactDiffersFromContractPath_ClassifiesTheDispatchedFile
// pins the one case the single-read fast path must NOT swallow: a phase whose
// dispatched artifact filename differs from its contract's. intent in DELTA mode
// writes intent-delta.md while the intent contract names intent.md, so Verify
// judged a DIFFERENT file and holds no bytes for this one — the runner must read
// the dispatched artifact so the phase's legitimate non-ship verdict
// ("[intent-unchanged]" → SKIPPED) still survives the ship-guard. Dropping this
// fallback silently FAILs every intent-delta cycle (caught by
// intent.TestRun_DeltaMode_IntentUnchanged_SKIPPED; pinned here at the seam).
func TestRun_DispatchedArtifactDiffersFromContractPath_ClassifiesTheDispatchedFile(t *testing.T) {
	ws := t.TempDir()
	const partial = "[intent-unchanged] goal_hash=abc12345\n"
	hooks := &fakeHooks{phase: "intent", artifact: "intent-delta.md", agent: "evolve-intent", model: "auto", prompt: "x", verdict: core.VerdictSKIPPED}
	nb := &noisyStdoutBridge{fileContent: partial, stdout: "pane scrollback\n"}
	// Production shape: Verify resolved the CONTRACT path (intent.md), found nothing
	// there, and so reports missing_artifact with no content.
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

// TestRun_ContractedPhase_ClassifiesWithoutReadingTheArtifactAgain is the
// mechanism proof, independent of any race: the deliverable file is REMOVED the
// instant after Verify returns its bytes. A path that still re-read would see
// ENOENT and fall back to the pane (or blank the artifact); the single-read path
// classifies the verified bytes because it never touches the file again.
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
