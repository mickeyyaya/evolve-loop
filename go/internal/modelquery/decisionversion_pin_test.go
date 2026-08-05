package modelquery

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

// decisionSurfaceFiles are the sources whose semantics the reuse-gate
// fingerprint depends on. If any of them changes, a stale CandidatesHash
// could silently reuse a pre-fix classification — decisionVersion exists to
// invalidate those, but it is a hand-bumped constant (adversarial-review
// finding 4: nothing enforced the bump).
var decisionSurfaceFiles = []string{"classifier.go", "latest.go", "complete.go", "lineage.go"}

// decisionSurfacePin is the sha256 over the concatenated decision-surface
// sources, pinned at decisionVersion "v1". Brittle BY DESIGN — this is a
// ratchet, not a unit test.
const decisionSurfacePin = "814346a53c6dc25d4e21124322cab5471482cec5fa16a5b612c410229b01bff4"

// TestDecisionVersion_PinnedToAlgorithmSurface fails whenever a
// decision-surface file changes, forcing the editor to answer ONE question:
// did classification/promotion/completion SEMANTICS change?
//   - Yes → bump decisionVersion in fingerprint.go, then update the pin.
//   - No (comments, refactor) → update the pin only.
//
// Either way the reuse gate's correctness dependency is looked at instead of
// silently skipped.
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
