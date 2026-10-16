package modelquery

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
)

var decisionSurfaceFiles = []string{"classifier.go", "latest.go", "complete.go", "lineage.go", "newestwins.go"}

const decisionSurfacePin = "9adc4ff63b3a5532443929c34dd9c3e322d036e62731fea270da7980df9fc8d0"

func TestDecisionVersion_PinnedToAlgorithmSurface(t *testing.T) {
	t.Parallel()
	h := sha256.New()
	for _, f := range decisionSurfaceFiles {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		code, err := commentaudit.StripComments(f, raw)
		if err != nil {
			t.Fatalf("strip comments from %s: %v", f, err)
		}
		h.Write([]byte(f))
		h.Write([]byte{0})
		h.Write(code.Source)
		h.Write([]byte{0})
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != decisionSurfacePin {
		t.Fatalf("decision surface code changed (sha256 %s, pinned %s at decisionVersion %q).\nIf semantics changed: bump decisionVersion in fingerprint.go AND update decisionSurfacePin.\nIf not (a refactor): update decisionSurfacePin only. Comment edits never move this pin.", got, decisionSurfacePin, decisionVersion)
	}
}
