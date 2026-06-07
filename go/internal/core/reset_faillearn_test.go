package core

// reset_faillearn_test.go — failure-floor Phase 2 (inbox
// retro-always-invariant, gap 2 / cycle-244 reproduction): `evolve cycle
// reset` must LEARN, not just archive. The seal writes a deterministic
// retrospective into the sealed archive dir, a failure-lesson YAML into
// instincts/lessons/, and appends an operator-reset failedApproaches
// entry — with the failedApproaches append ordered BEFORE the seal's own
// final state.json write (which stays the canonical last writer of
// lastCycleNumber / currentBatch / lastUpdated).

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSealCycle_WritesRetrospectiveIntoSealedDir(t *testing.T) {
	ev := t.TempDir()
	sealFixture(t, ev, 108)

	res, err := SealCycle(context.Background(), &recordingLedger{}, sealOpts(ev))
	if err != nil {
		t.Fatalf("SealCycle: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(res.ArchiveDir, "retrospective-report.md"))
	if err != nil {
		t.Fatalf("sealed archive must contain a deterministic retrospective: %v", err)
	}
	for _, want := range []string{"deterministic-fallback", "operator-reset", "scout", "operator reset (test)"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("retrospective missing %q:\n%s", want, data)
		}
	}
}

func TestSealCycle_WritesLessonIntoLessonsDir(t *testing.T) {
	ev := t.TempDir()
	sealFixture(t, ev, 108)

	if _, err := SealCycle(context.Background(), &recordingLedger{}, sealOpts(ev)); err != nil {
		t.Fatalf("SealCycle: %v", err)
	}

	lesson := filepath.Join(ev, "instincts", "lessons", "cycle-108-reset-scout.yaml")
	if _, err := os.Stat(lesson); err != nil {
		t.Fatalf("lesson %s must exist after seal: %v", lesson, err)
	}
}

func TestSealCycle_AppendsOperatorResetFailedApproach(t *testing.T) {
	ev := t.TempDir()
	sealFixture(t, ev, 108)

	if _, err := SealCycle(context.Background(), &recordingLedger{}, sealOpts(ev)); err != nil {
		t.Fatalf("SealCycle: %v", err)
	}

	// Ordering pin (load-bearing): the failedApproaches entry must be
	// visible in the FINAL state.json — i.e. failurelog.Record ran before
	// the seal's own read-modify-write, which stays the last writer.
	sm := readJSONMap(t, filepath.Join(ev, "state.json"))
	fa, _ := sm["failedApproaches"].([]any)
	if len(fa) != 1 {
		t.Fatalf("failedApproaches = %v, want exactly one operator-reset entry", fa)
	}
	entry, _ := fa[0].(map[string]any)
	if got := strFromAny(entry["classification"]); got != "operator-reset" {
		t.Errorf("classification = %q, want operator-reset", got)
	}
	if got := strFromAny(entry["summary"]); !strings.Contains(got, "scout") {
		t.Errorf("summary = %q, want the sealed phase mentioned", got)
	}
	// The seal's own writes must still win on its keys.
	if got := intFromAny(sm["lastCycleNumber"]); got != 108 {
		t.Errorf("lastCycleNumber = %d, want 108", got)
	}
	if got := strFromAny(sm["expected_ship_sha"]); got != "deadbeef-must-survive" {
		t.Errorf("expected_ship_sha must survive the seal; got %q", got)
	}
}

func TestSealCycle_DryRunWritesNoLearning(t *testing.T) {
	ev := t.TempDir()
	sealFixture(t, ev, 108)

	opts := sealOpts(ev)
	opts.DryRun = true
	if _, err := SealCycle(context.Background(), &recordingLedger{}, opts); err != nil {
		t.Fatalf("SealCycle dry-run: %v", err)
	}

	if entries, _ := os.ReadDir(filepath.Join(ev, "instincts", "lessons")); len(entries) != 0 {
		t.Errorf("dry-run wrote lessons: %v", entries)
	}
	sm := readJSONMap(t, filepath.Join(ev, "state.json"))
	if fa, _ := sm["failedApproaches"].([]any); len(fa) != 0 {
		t.Errorf("dry-run appended failedApproaches: %v", fa)
	}
}
