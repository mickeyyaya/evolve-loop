package deliverable

// grace_test.go — the write-in-flight grace window (cycle-1212 salvage,
// review BLOCK: the grace path crosses every phase of every cycle and shipped
// untested in the first cut). All timing goes through the graceSleep seam —
// no real-time waits.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func withGraceSeam(t *testing.T, fn func(d time.Duration)) *int {
	t.Helper()
	calls := 0
	prev := graceSleep
	graceSleep = func(d time.Duration) {
		calls++
		if fn != nil {
			fn(d)
		}
	}
	t.Cleanup(func() { graceSleep = prev })
	return &calls
}

func TestReadDeliverableWithGrace_PresentContentIsOneReadNoSleep(t *testing.T) {
	calls := withGraceSeam(t, nil)
	p := filepath.Join(t.TempDir(), "report.md")
	if err := os.WriteFile(p, []byte("# Report\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	content, exists, err := readDeliverableWithGrace(p)
	if err != nil || !exists || content != "# Report\nbody\n" {
		t.Fatalf("got (%q, %v, %v), want the content, true, nil", content, exists, err)
	}
	if *calls != 0 {
		t.Fatalf("the common already-written case must pay ZERO sleeps, got %d", *calls)
	}
}

func TestReadDeliverableWithGrace_LateWriteInsideWindowIsRescued(t *testing.T) {
	p := filepath.Join(t.TempDir(), "report.md")
	wrote := false
	withGraceSeam(t, func(time.Duration) {
		if !wrote {
			wrote = true
			if err := os.WriteFile(p, []byte("late but complete\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	})
	content, exists, err := readDeliverableWithGrace(p)
	if err != nil || !exists || content != "late but complete\n" {
		t.Fatalf("a write landing inside the window must be rescued: got (%q, %v, %v)", content, exists, err)
	}
}

func TestReadDeliverableWithGrace_AbsentConfirmsMissingAfterWindow(t *testing.T) {
	calls := withGraceSeam(t, nil)
	_, exists, err := readDeliverableWithGrace(filepath.Join(t.TempDir(), "never-written.md"))
	if err != nil || exists {
		t.Fatalf("genuinely absent must confirm missing (exists=false, err=nil), got (%v, %v)", exists, err)
	}
	if *calls == 0 {
		t.Fatal("absence must have consumed the grace window (polled at least once) before confirming")
	}
}

func TestReadDeliverableWithGrace_BlankConfirmsEmptyAfterWindow(t *testing.T) {
	p := filepath.Join(t.TempDir(), "blank.md")
	if err := os.WriteFile(p, []byte("  \n\t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	content, exists, err := readDeliverableWithGrace(p)
	if err != nil || !exists {
		t.Fatalf("present-but-blank must confirm (exists=true, err=nil), got (%v, %v)", exists, err)
	}
	if len(content) == 0 {
		t.Fatal("the blank content is returned as-read (the caller's TrimSpace decides emptiness)")
	}
}

func TestReadDeliverableWithGrace_NonAbsenceIOFailsImmediatelyAsInfra(t *testing.T) {
	calls := withGraceSeam(t, nil)
	dir := t.TempDir() // reading a DIRECTORY: EISDIR-class, never clears on its own
	_, _, err := readDeliverableWithGrace(dir)
	if err == nil {
		t.Fatal("a non-absence read fault must surface as an error (infra), not a violation")
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Fatal("must not be classified as absence")
	}
	if *calls != 0 {
		t.Fatalf("infra faults never clear on their own — the grace budget must not be spent on them (got %d sleeps)", *calls)
	}
}
