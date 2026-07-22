package main

// cmd_loop_blockerbreaker_test.go — wiring pins for the mid-batch pipeline-
// blocker breaker (unit-green != live-green): the helper must read REAL digest
// artifacts, honor batch scoping, and on a trip leave the ADR-0072 breadcrumbs
// (escalation dossier + P0 pipeline-repair inbox item) exactly like the forged-
// verdict halt.

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeDigestFixture(t *testing.T, evolveDir string, cycle int, fingerprint, preClass string) {
	t.Helper()
	d := filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"cycle":` + strconv.Itoa(cycle) + `,"fingerprint":"` + fingerprint + `","pre_class":"` + preClass + `"}`
	if err := os.WriteFile(filepath.Join(d, "failure-digest.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBlockerBreakerHalt_TripsAndEscalates(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeDigestFixture(t, evolveDir, 11, "build|guard-abort|aaa", "guard-abort")
	writeDigestFixture(t, evolveDir, 12, "audit|guard-abort|bbb", "guard-abort")

	rc, halted := blockerBreakerHalt(evolveDir, root, 10, io.Discard)
	if !halted {
		t.Fatal("two guard-abort digests in-batch must halt (compiled default ceiling 2)")
	}
	if rc == 0 {
		t.Fatal("halt must return a non-zero loop exit code")
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "pipeline-escalation.json")); err != nil {
		t.Fatal("halt must write the ADR-0072 escalation dossier")
	}
	entries, _ := os.ReadDir(filepath.Join(evolveDir, "inbox"))
	found := false
	for _, e := range entries {
		if strings.Contains(e.Name(), "pipeline-defect-pipeline-blocker") {
			found = true
		}
	}
	if !found {
		t.Fatal("halt must auto-file the P0 pipeline-defect inbox item (never_stop_queue: the queue is injected even as the loop halts)")
	}
}

func TestBlockerBreakerHalt_HistoricDigestsExcluded(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	// Same two guard-aborts, but the batch starts AFTER them — a fixed
	// historic blocker must never halt a fresh healthy run.
	writeDigestFixture(t, evolveDir, 11, "build|guard-abort|aaa", "guard-abort")
	writeDigestFixture(t, evolveDir, 12, "audit|guard-abort|bbb", "guard-abort")

	if _, halted := blockerBreakerHalt(evolveDir, root, 12, io.Discard); halted {
		t.Fatal("digests at or before batchStartCycle must be out of scope")
	}
}

func TestBlockerBreakerHalt_QuietOnHealthyBatch(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeDigestFixture(t, evolveDir, 11, "audit|gate-block|aaa", "gate-block")
	writeDigestFixture(t, evolveDir, 12, "audit|gate-block|bbb", "gate-block")

	if _, halted := blockerBreakerHalt(evolveDir, root, 10, io.Discard); halted {
		t.Fatal("distinct honest task failures must not trip the breaker")
	}
}
