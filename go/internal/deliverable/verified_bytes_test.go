package deliverable

// verified_bytes_test.go — the single-read seam (deliverable-verified-bytes-
// single-read). Verify READS the artifact to judge it; the host runner then had
// to re-read the same path to classify, so the classified bytes were only
// probably the bytes that passed Verify. Result.Content closes that window: the
// verdict AND the content come from ONE read, so "the file is the sole verdict
// source" is literal rather than "the file as of the Verify read".

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// TestVerify_OKResultCarriesTheVerifiedBytes — a well-formed deliverable's
// Result must carry the exact bytes Verify judged, alongside the path they came
// from. This is what BaseRunner.Run classifies (runner.go verdict-source block).
func TestVerify_OKResultCarriesTheVerifiedBytes(t *testing.T) {
	ws := t.TempDir()
	const body = "# Build Report\n\n## Changes\n- foo.go\n\nVerdict: PASS\n"
	writeFile(t, ws, "build-report.md", body)

	res, err := Verify("build", phasecontract.Roots{Workspace: ws})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK {
		t.Fatalf("want OK, got violations: %+v", res.Violations)
	}
	if res.Content != body {
		t.Errorf("Result.Content=%q, want the verified bytes %q — the caller must classify the bytes Verify read, not a second read of the path", res.Content, body)
	}
	if want := filepath.Join(ws, "build-report.md"); res.ArtifactPath != want {
		t.Errorf("ArtifactPath=%q, want %q", res.ArtifactPath, want)
	}
}

// TestVerify_MalformedResultStillCarriesTheBytes — a deliverable that FAILS
// verification must still surface its content: a phase can derive a legitimate
// non-ship verdict from partial content (intent delta's "[intent-unchanged]" →
// SKIPPED), so the runner hands those bytes to Classify and lets the ship-guard
// clamp any ship-eligible claim. Blanking Content here would break that path.
func TestVerify_MalformedResultStillCarriesTheBytes(t *testing.T) {
	ws := t.TempDir()
	const partial = "# Build Report\n\nno changes section here\nVerdict: PASS\n"
	writeFile(t, ws, "build-report.md", partial)

	res, err := Verify("build", phasecontract.Roots{Workspace: ws})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OK {
		t.Fatal("want !OK: the required Changes section is absent")
	}
	if res.Content != partial {
		t.Errorf("Result.Content=%q, want the verified bytes %q even on a !OK result", res.Content, partial)
	}
}

// TestVerify_MissingArtifact_ContentEmptyPathSet — the absent case: ArtifactPath
// still names the file-backed contract (so the caller knows the deliverable IS a
// file) while Content is empty, which is exactly the "Classify sees no sentinel →
// FAIL" input the coherent deliverable-production FAIL relies on.
func TestVerify_MissingArtifact_ContentEmptyPathSet(t *testing.T) {
	ws := t.TempDir()

	res, err := Verify("build", phasecontract.Roots{Workspace: ws})
	if err != nil {
		t.Fatalf("missing file is a confirmed violation, not ambiguity; got err=%v", err)
	}
	if res.Content != "" {
		t.Errorf("Content=%q, want empty for an absent deliverable", res.Content)
	}
	if res.ArtifactPath == "" {
		t.Error("ArtifactPath must be set for a file-backed contract even when the file is absent — it is the caller's file-backed discriminator")
	}
}

// TestVerify_NoArtifactContract_NoPathNoContent — ship's deliverable is the
// pushed commit, not a file. A NoArtifact contract verifies OK with NO path and
// NO content: an empty ArtifactPath is how a Result says "I describe no file", so a
// consumer keying on the path (the runner's classifiedArtifact) can never mistake
// the absence of content for an empty deliverable.
func TestVerify_NoArtifactContract_NoPathNoContent(t *testing.T) {
	res, err := Verify("ship", phasecontract.Roots{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK {
		t.Fatalf("a NoArtifact contract verifies OK; got %+v", res.Violations)
	}
	if res.ArtifactPath != "" || res.Content != "" {
		t.Errorf("NoArtifact must carry neither path nor content; got path=%q content=%q", res.ArtifactPath, res.Content)
	}
}
