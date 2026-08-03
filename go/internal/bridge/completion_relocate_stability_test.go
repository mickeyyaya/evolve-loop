package bridge

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// completion_relocate_stability_test.go — the RED contract for cycle-1249's
// residual half of artifact-ready-crosspoll-debounce.
//
// What already landed (cycle-1233, folded in by 841e676f; verified-and-closed by
// cycle-1236 predicate 004): artifactDetector carries a (size, mtime) stability
// window ACROSS poll ticks, so a deliverable still being written at the CANONICAL
// path never completes the phase. completion_debounce_test.go pins that half.
//
// The hole that half does NOT close. artifactDetector.poll's very first
// statement is `artifactReady(d.cfg)` (completion.go:223), and artifactReady
// RELOCATES a non-canonical fallback the instant it observes it non-empty
// (driver_common.go, the cycle-108/141 tolerance) — before any stability
// observation has been made about that fallback. The debounce is therefore
// applied strictly downstream of an irreversible move:
//
//	tick 1: agent is mid-write at <worktree>/report.md (size>0, incomplete)
//	        → artifactReady relocates it to the canonical path, source removed
//	tick 2+: the window runs at the canonical path
//
// On the rename branch this is survivable — rename preserves the inode, so the
// agent's still-open fd keeps appending at the canonical path and the window
// then sees the growth. On the COPY+REMOVE branch (relocateFile's cross-device
// fallback) it is not: the copy snapshots a partial file, `.tmp.<pid>`-renames
// that truncated snapshot into the canonical path, and then REMOVES the source
// the agent is still writing to. The result is a permanently stable, permanently
// truncated deliverable — the debounce declares it finished, and the bytes the
// agent wrote after the copy are gone. That is the exact mid-write-truncation
// class deliverable.go:180 names this mechanism as the source-side closure of,
// reached through the path scout flagged (hypothesis 2) as the highest-risk one
// precisely because it carries an extra copy step.
//
// The fix contract: the stability window must gate the RELOCATION, not merely
// follow it. The detector must observe the artifact — wherever it currently
// lives, canonical or fallback — settle across artifactStableTicks consecutive
// ticks BEFORE the file is moved. A fallback that is still growing must be left
// exactly where it is.
//
// Test map (each AC gets a positive and a negative so no-op cannot pass):
//
//	AC-1249-1 defer-move    → RelocationDeferredWhileFallbackStillGrowing (negative:
//	                          the move must NOT happen) +
//	                          RelocationHappensOnceFallbackSettles (positive: it must
//	                          still happen, so "never relocate" is not a passing fix)
//	AC-1249-2 no-regression → RelocatedCompleteFallbackStillCompletes (the cycle-108/141
//	                          tolerance and its single-shot diagnostic survive)

// growingFallback writes body at path with a monotonically advancing mtime,
// modelling an agent appending to a deliverable it wrote to the wrong place.
func growingFallback(t *testing.T, path string, i int) {
	t.Helper()
	writeArtifact(t, path,
		"# report\n"+strings.Repeat("section\n", i),
		fixedMTime.Add(time.Duration(i)*time.Second))
}

// --- AC-1249-1 (negative): an in-flight fallback must not be moved ----------

// TestArtifactDetector_RelocationDeferredWhileFallbackStillGrowing is the
// anti-truncation assertion. It asserts on the SIDE EFFECT (did the file move?),
// not just on readiness, because readiness is already correct today: the landed
// canonical-path window returns ready=false here while the damage — the move of
// a half-written file, and on the copy branch the removal of the agent's source
// — has already been done on tick 1.
func TestArtifactDetector_RelocationDeferredWhileFallbackStillGrowing(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	fallback := filepath.Join(ws, "workspace", "report.md")
	d := newArtifactDetectorAt(ws, canonical)

	for i := 1; i <= 4; i++ {
		growingFallback(t, fallback, i)
		ready, _, _, err := d.poll(context.Background())
		if err != nil {
			t.Fatalf("poll %d: unexpected detector error: %v", i, err)
		}
		if ready {
			t.Fatalf("poll %d: a still-growing fallback artifact completed the phase", i)
		}
		if !fileNonEmpty(fallback) {
			t.Fatalf("poll %d: the fallback at %s was relocated while it was still being "+
				"written — the stability window must gate the MOVE, not merely follow it. "+
				"On relocateFile's copy+remove branch this snapshots a truncated file into "+
				"the canonical path and deletes the source the agent is still appending to.",
				i, fallback)
		}
	}
}

// --- AC-1249-1 (positive): a settled fallback must still be moved -----------

// TestArtifactDetector_RelocationHappensOnceFallbackSettles is the honest
// counterweight: "never relocate anything" would pass the negative above and
// break the cycle-108/141 tolerance outright. Once the fallback stops changing,
// the detector must move it to the canonical path, complete, and still carry the
// wrote-to-the-wrong-place diagnostic.
func TestArtifactDetector_RelocationHappensOnceFallbackSettles(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	fallback := filepath.Join(ws, "workspace", "report.md")
	d := newArtifactDetectorAt(ws, canonical)

	// Two growing ticks, then the agent stops writing.
	growingFallback(t, fallback, 1)
	if ready, _, _, err := d.poll(context.Background()); ready || err != nil {
		t.Fatalf("growing tick 1: got (ready=%v, err=%v), want (false, nil)", ready, err)
	}
	growingFallback(t, fallback, 2)
	if ready, _, _, err := d.poll(context.Background()); ready || err != nil {
		t.Fatalf("growing tick 2: got (ready=%v, err=%v), want (false, nil)", ready, err)
	}

	got, note := pollUntilReady(t, d, artifactStableTicks+3, nil)
	if got < 0 {
		t.Fatalf("a fallback that stopped changing was never accepted within %d polls — "+
			"deferring the move must SETTLE, not stall the cycle-108/141 tolerance",
			artifactStableTicks+3)
	}
	if !fileNonEmpty(canonical) {
		t.Fatalf("completed without the artifact at the canonical path %s — downstream phases "+
			"read only the canonical path", canonical)
	}
	if !strings.Contains(note, "relocated from non-canonical") || !strings.Contains(note, fallback) {
		t.Errorf("completion note = %q, want it to name the non-canonical source %s "+
			"(the diagnostic is single-shot and must survive the deferred move)", note, fallback)
	}
}

// --- AC-1249-2: the already-complete fallback case is unchanged -------------

// TestArtifactDetector_RelocatedCompleteFallbackStillCompletes is the regression
// axis. The overwhelmingly common non-canonical case is a single complete
// os.WriteFile at the wrong path; gating the move must not add latency beyond the
// window that already exists, nor lose the artifact.
func TestArtifactDetector_RelocatedCompleteFallbackStillCompletes(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	fallback := filepath.Join(ws, "workspace", "report.md")
	d := newArtifactDetectorAt(ws, canonical)

	writeArtifact(t, fallback, "# report\n\nDONE\n", fixedMTime)

	got, note := pollUntilReady(t, d, artifactStableTicks+3, nil)
	if got < 0 {
		t.Fatalf("a complete, never-changing fallback never completed within %d polls",
			artifactStableTicks+3)
	}
	if got > artifactStableTicks+1 {
		t.Errorf("a complete fallback took %d polls to complete; the window is %d ticks — "+
			"gating the move must not stack a SECOND window on top of the existing one",
			got, artifactStableTicks)
	}
	if !fileNonEmpty(canonical) {
		t.Fatalf("completed without the artifact at the canonical path %s", canonical)
	}
	if !strings.Contains(note, "relocated from non-canonical") {
		t.Errorf("completion note = %q, want the relocation diagnostic preserved", note)
	}
}
