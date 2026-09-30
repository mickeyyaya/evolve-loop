package modelquery

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

var decisionSurfaceFiles = []string{"classifier.go", "latest.go", "complete.go", "lineage.go", "newestwins.go"}

const decisionSurfacePin = "0be88f6798bc08006fa74276b080b09b014c8620a5a10eacb0d3fe72e7dff04f"

func TestDecisionVersion_PinnedToAlgorithmSurface(t *testing.T) {
	t.Parallel()
	h := sha256.New()
	for _, f := range decisionSurfaceFiles {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		h.Write([]byte(f))
		h.Write([]byte{0})
		h.Write(raw)
		h.Write([]byte{0})
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != decisionSurfacePin {
		t.Fatalf("decision surface changed (sha256 %s, pinned %s at decisionVersion %q).\nIf semantics changed: bump decisionVersion in fingerprint.go AND update decisionSurfacePin.\nIf not (comments/refactor): update decisionSurfacePin only.", got, decisionSurfacePin, decisionVersion)
	}
}
