//go:build acs

package cycle942

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const auditedFooDiff = "diff --git a/foo.go b/foo.go\n" +
	"--- a/foo.go\n+++ b/foo.go\n" +
	"@@ -10,3 +10,4 @@ func A() {\n" +
	" \ta := 1\n \tb := 2\n+\tAUDITED_LINE := 3\n \treturn\n"

const composedFooDiff = "diff --git a/foo.go b/foo.go\n" +
	"--- a/foo.go\n+++ b/foo.go\n" +
	"@@ -10,3 +10,4 @@ func A() {\n" +
	" \ta := 1\n \tb := 2\n+\tCOMPOSED_HUNK_A := 3\n \treturn\n" +
	"@@ -50,2 +51,3 @@ func B() {\n" +
	" \tx := 1\n+\tCOMPOSED_HUNK_B := 2\n \ty := 2\n"

const composedDisjointDiff = "diff --git a/foo.go b/foo.go\n" +
	"--- a/foo.go\n+++ b/foo.go\n" +
	"@@ -50,2 +51,3 @@ func B() {\n" +
	" \tx := 1\n+\tCOMPOSED_HUNK_B := 2\n \ty := 2\n"

const resolvedInjectedDiff = "diff --git a/foo.go b/foo.go\n" +
	"--- a/foo.go\n+++ b/foo.go\n" +
	"@@ -10,3 +10,5 @@ func A() {\n" +
	" \ta := 1\n \tb := 2\n+\tAUDITED_LINE := 3\n+\tINJECTED := 99\n \treturn\n"

func TestC942_001_ScopedReviewSeesOnlyIntersectingHunks(t *testing.T) {
	scoped := string(core.IntersectingHunks([]byte(auditedFooDiff), []byte(composedFooDiff)))
	if !strings.Contains(scoped, "COMPOSED_HUNK_A") {
		t.Errorf("scoped payload dropped the intersecting hunk (COMPOSED_HUNK_A absent):\n%s", scoped)
	}
	if strings.Contains(scoped, "COMPOSED_HUNK_B") {
		t.Errorf("scoped payload leaked the DISJOINT hunk (COMPOSED_HUNK_B present) — not scoped to conflict regions:\n%s", scoped)
	}
}

func TestC942_002_DisjointHunksProduceEmptyScope(t *testing.T) {
	scoped := string(core.IntersectingHunks([]byte(auditedFooDiff), []byte(composedDisjointDiff)))
	if strings.TrimSpace(scoped) != "" {
		t.Errorf("disjoint footprints produced a non-empty scoped payload (want empty):\n%s", scoped)
	}
}

func TestC942_003_CompatibleComposesEntangledEscalates(t *testing.T) {
	if !core.ScopedReviewCompatible.Composes() {
		t.Errorf("ScopedReviewCompatible.Composes() = false, want true (compatible must skip re-audit)")
	}
	if core.ScopedReviewEntangled.Composes() {
		t.Errorf("ScopedReviewEntangled.Composes() = true, want false (entangled must escalate to full re-audit)")
	}
	if core.ScopedReviewMethod != "scoped-review" {
		t.Errorf("ScopedReviewMethod = %q, want %q", core.ScopedReviewMethod, "scoped-review")
	}
}

func TestC942_004_UnknownVerdictFailsClosed(t *testing.T) {
	if core.ScopedReviewVerdict("garbage").Composes() {
		t.Errorf("an unknown verdict composed — Composes() is not fail-closed")
	}
}

func TestC942_005_LLMResolutionReentersRung0Verification(t *testing.T) {
	ok, err := core.ReverifyResolution([]byte(auditedFooDiff), []byte(auditedFooDiff))
	if err != nil {
		t.Fatalf("ReverifyResolution(identical) errored: %v", err)
	}
	if !ok {
		t.Errorf("ReverifyResolution(identical) = false, want true (a semantics-preserving resolution re-verifies)")
	}

	rejected, err := core.ReverifyResolution([]byte(auditedFooDiff), []byte(resolvedInjectedDiff))
	if err != nil {
		t.Fatalf("ReverifyResolution(injected) errored: %v", err)
	}
	if rejected {
		t.Errorf("ReverifyResolution(injected) = true, want false — a resolution that changed semantics must be rejected, not trusted")
	}
}
