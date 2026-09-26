package deliverable

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
	// Leave the contract's intent.md absent and write the dispatched delta artifact.
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

	// Without the override the contract path stays authoritative, so the miss is reported there.
	res2, err := Verify("intent", phasecontract.Roots{Workspace: ws})
	if err != nil {
		t.Fatalf("Verify (no override): %v", err)
	}
	if !strings.HasSuffix(res2.ArtifactPath, "intent.md") || res2.OK {
		t.Fatalf("contract-path callers must keep the registry path: path=%q ok=%v", res2.ArtifactPath, res2.OK)
	}
}
