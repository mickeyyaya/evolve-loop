package bridge

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// growingFallback writes body at path with a monotonically advancing mtime,
// modelling an agent appending to a deliverable it wrote to the wrong place.
func growingFallback(t *testing.T, path string, i int) {
	t.Helper()
	writeArtifact(t, path,
		"# report\n"+strings.Repeat("section\n", i),
		fixedMTime.Add(time.Duration(i)*time.Second))
}

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

func TestArtifactDetector_RelocationHappensOnceFallbackSettles(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	fallback := filepath.Join(ws, "workspace", "report.md")
	d := newArtifactDetectorAt(ws, canonical)

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
