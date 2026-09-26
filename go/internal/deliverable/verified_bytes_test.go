package deliverable

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

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

// Content survives a failed verify because a phase can derive a legitimate non-ship verdict from partial content.
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
