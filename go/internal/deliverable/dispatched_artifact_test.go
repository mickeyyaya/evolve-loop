package deliverable

// dispatched_artifact_test.go — pin for the intent-delta contract-path skew
// fix (inbox intent-delta-contract-path-skew 0.88): the runner threads the
// EXACT dispatched artifact path through Roots.DispatchedArtifact, so Verify
// judges the file the phase was ASKED to write — intent in DELTA mode
// dispatches intent-delta.md while its registry contract names intent.md,
// and Verify previously judged the wrong file (found by PR #389's
// single-read work, deliberately not papered over there).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func TestVerify_DispatchedArtifactOverridesContractPath(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	// The registry contract for intent names intent.md — leave it ABSENT and
	// write the DISPATCHED delta artifact instead (the delta-mode shape).
	delta := filepath.Join(ws, "intent-delta.md")
	if err := os.WriteFile(delta, []byte("[intent-unchanged]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Verify("intent", phasecontract.Roots{Workspace: ws, DispatchedArtifact: delta})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if res.ArtifactPath != delta {
		t.Fatalf("Verify judged %q, want the DISPATCHED %q — the contract-path skew is back", res.ArtifactPath, delta)
	}
	if !strings.Contains(res.Content, "[intent-unchanged]") {
		t.Fatalf("verified bytes are not the dispatched file's: %q", res.Content)
	}

	// Without the override, the contract path stays authoritative (CLI/gate
	// callers): intent.md is absent, so the verify reports the miss AT the
	// contract path.
	res2, err := Verify("intent", phasecontract.Roots{Workspace: ws})
	if err != nil {
		t.Fatalf("Verify (no override): %v", err)
	}
	if !strings.HasSuffix(res2.ArtifactPath, "intent.md") || res2.OK {
		t.Fatalf("contract-path callers must keep the registry path: path=%q ok=%v", res2.ArtifactPath, res2.OK)
	}
}
