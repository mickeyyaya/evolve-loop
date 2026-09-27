package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// collidingDefects returns two distinct, entirely ordinary defect lines whose
// remediationSlug is identical because they diverge only after rune 60.
//
// The shared prefix is written out rather than computed so the fixture states
// its own premise: the engine unit's test pins remediationSlugMaxRunes == 60 and
// the two distinct ids, so a bound change fails loudly there instead of this
// test quietly ceasing to reproduce anything.
func collidingDefects() []string {
	const prefix = "evidenceResolves accepts an unrelated in-repo file as closure evidence"
	return []string{
		prefix + " for the ledger row",
		prefix + " for the manifest row",
	}
}

// TestRepro1285_F1_CollidingRemediationSlugSuppressesRetrospectiveAndLesson
// drives writeDeterministicLearning — the production seam the failure path
// calls (the three fallback tails of recordFailureLearning) — not
// faillearn.WriteArtifacts directly, because the collision is minted by the
// failure-learning engine (internal/core/failurelearning): remediationItems
// derives the inbox id from remediationSlug(title), and remediationSlug stops
// at remediationSlugMaxRunes.
func TestRepro1285_F1_CollidingRemediationSlugSuppressesRetrospectiveAndLesson(t *testing.T) {
	o, fl, root := remediationFixture(t)
	defects := collidingDefects()
	// The fixture premise — two defect lines diverging only after the 60-rune
	// slug bound mint TWO ids — is pinned in the engine unit
	// (failurelearning.TestRemediationItems_IDsAreInjectiveOverTheFullTitle);
	// the three damage checks below run for real through the facade.

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
	//    trigger; dropping one of two real defects is the second-order damage.
	if files := inboxFiles(t, root); len(files) != 2 {
		t.Errorf("inbox holds %v; want one addressable item per defect — two distinct defects sharing a 60-rune slug prefix need the disambiguating suffix derived from the FULL text", files)
	}
}
