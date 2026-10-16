package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func collidingDefects() []string {
	const prefix = "evidenceResolves accepts an unrelated in-repo file as closure evidence"
	return []string{
		prefix + " for the ledger row",
		prefix + " for the manifest row",
	}
}

func TestRepro1285_F1_CollidingRemediationSlugSuppressesRetrospectiveAndLesson(t *testing.T) {
	t.Parallel()

	o, fl, root := remediationFixture(t)
	defects := collidingDefects()

	o.writeDeterministicLearning(fl,
		"audit phase exited 1 after 3 attempts",
		&phasecontract.FailureBlock{Class: "deliverable-rejected", Defects: defects},
	)

	if _, err := os.Stat(filepath.Join(fl.CycleState.WorkspacePath, "retrospective-report.md")); err != nil {
		t.Errorf("no retrospective-report.md after a failure carrying two defects with a shared 60-rune slug prefix (%v).\n"+
			"WithInbox writes the inbox FIRST and aborts the call on failure (writer.go:30-32), so agent-authored defect TEXT can suppress the durable record entirely — the 1255 state, reached through the mechanism built to prevent it.", err)
	}

	lessons, err := os.ReadDir(filepath.Join(root, ".evolve", "instincts", "lessons"))
	if err != nil || len(lessons) == 0 {
		t.Errorf("no failure lesson was written (err=%v, entries=%d) — the lesson is written after the retrospective and is lost to the same abort", err, len(lessons))
	}

	if files := inboxFiles(t, root); len(files) != 2 {
		t.Errorf("inbox holds %v; want one addressable item per defect — two distinct defects sharing a 60-rune slug prefix need the disambiguating suffix derived from the FULL text", files)
	}
}
