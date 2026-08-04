package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// repro_cycle1285_core_test.go — executable reproduction of the cycle-1285
// adversarial review's F1 (HIGH), the finding that lives on the failure-floor
// side of the diff.
//
// It drives writeDeterministicLearning — the production seam the failure path
// calls at failure_learning.go:366/372 — not faillearn.WriteArtifacts directly,
// because the collision is MINTED by this package: retroRemediationItems derives
// the inbox id from remediationSlug(title), and remediationSlug stops at
// remediationSlugMaxRunes = 60 (failure_learning.go:515, :522-541).
//
// Chain: two defect lines sharing a 60-rune slug prefix → one id, two different
// titles → inbox.go:114-116 (the cycle-1282 DEF-4 fix) raises a hard error →
// writer.go:30-32 (the WithInbox ordering) returns BEFORE the retrospective and
// the lesson are written → failure_learning.go:460-462 downgrades the whole
// thing to one stderr WARN.
//
// Net effect: a failing cycle produces NO retrospective and NO lesson. That is
// the cycle-1255 state — a defect with no durable record — reached through the
// mechanism built to make it unreachable, and reached more completely, because
// 1255 at least had a report. No adversary is required; two real defects from
// one subsystem routinely share a 60-character prefix.

// collidingDefects returns two distinct, entirely ordinary defect lines whose
// remediationSlug is identical because they diverge only after rune 60.
//
// The shared prefix is written out rather than computed so the fixture states
// its own premise: if remediationSlugMaxRunes changes, the guard below fails
// loudly instead of the test quietly ceasing to reproduce anything.
func collidingDefects() []string {
	const prefix = "evidenceResolves accepts an unrelated in-repo file as closure evidence"
	return []string{
		prefix + " for the ledger row",
		prefix + " for the manifest row",
	}
}

// TestRepro1285_F1_CollidingRemediationSlugSuppressesRetrospectiveAndLesson —
// F1. The floor's own guarantee ("the retrospective survives") is voided by
// agent-authored defect text.
func TestRepro1285_F1_CollidingRemediationSlugSuppressesRetrospectiveAndLesson(t *testing.T) {
	o, fl, root := remediationFixture(t)
	defects := collidingDefects()

	items := retroRemediationItems(root, fl.Cycle, defects)
	if len(items) != 2 {
		t.Fatalf("fixture: retroRemediationItems returned %d items, want 2", len(items))
	}
	// cycle-1287: the cycle-1285 original SKIPPED here when the ids stopped
	// colliding, on the reasoning that a non-colliding fixture no longer
	// reproduces anything. Adopted into the tree as a permanent lock, that guard
	// would make the fix itself green this test by skipping it — a vacuous pass,
	// and the exact green-by-skip pattern batch-integrity-review-2026-08-04.md
	// files as a finding. Distinct ids are not the fixture failing to reproduce;
	// they ARE the fix (remediationFingerprint), so they are asserted. The three
	// damage checks below then run for real rather than being stepped over.
	if items[0].ID == items[1].ID {
		t.Errorf("two defect lines diverging only after rune %d minted ONE inbox id %q — the id must be injective over the FULL defect text, or the second real defect is dropped and (via the DEF-4 collision refusal) the retrospective and lesson are suppressed with it",
			remediationSlugMaxRunes, items[0].ID)
	}

	o.writeDeterministicLearning(fl,
		"audit phase exited 1 after 3 attempts",
		&phasecontract.FailureBlock{Class: "deliverable-rejected", Defects: defects},
	)

	// 1. The retrospective — the artifact the floor exists to guarantee.
	if _, err := os.Stat(filepath.Join(fl.CycleState.WorkspacePath, "retrospective-report.md")); err != nil {
		t.Errorf("no retrospective-report.md after a failure carrying two defects with a shared 60-rune slug prefix (%v).\n"+
			"WithInbox writes the inbox FIRST and aborts the call on failure (writer.go:30-32), so agent-authored defect TEXT can suppress the durable record entirely — the 1255 state, reached through the mechanism built to prevent it.", err)
	}

	// 2. The lesson — the corpus entry a later cycle's research phase reads.
	lessons, err := os.ReadDir(filepath.Join(root, ".evolve", "instincts", "lessons"))
	if err != nil || len(lessons) == 0 {
		t.Errorf("no failure lesson was written (err=%v, entries=%d) — the lesson is written after the retrospective and is lost to the same abort", err, len(lessons))
	}

	// 3. Both remediation items must reach the queue. Colliding ids are the
	//    trigger; dropping one of two real defects is the second-order damage,
	//    and it is what DEF-4 was filed to stop.
	if files := inboxFiles(t, root); len(files) != 2 {
		t.Errorf("inbox holds %v; want one addressable item per defect. Two distinct defects share id %q because remediationSlug truncates at %d runes — the id needs a disambiguating suffix derived from the FULL text",
			files, items[0].ID, remediationSlugMaxRunes)
	}
}
