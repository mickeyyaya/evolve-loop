package orchestrator

import (
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestCensus_TwoItemsThatEditDifferentFilesOfALeafAreBothAdmissibleWhenTheLeafHasFileKeys(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		fileCells           []string
		admissible, waiting int
	}{
		"no file cells":       {nil, 1, 1},
		"cmd/evolve listed":   {[]string{"cmd/evolve"}, 2, 0},
		"another leaf listed": {[]string{"cmd/other"}, 1, 1},
	} {
		t.Run(name, func(t *testing.T) {
			inbox := filepath.Join(t.TempDir(), ".evolve", "inbox")
			writeItem(t, inbox, "e-one", map[string]any{"files": []string{"go/cmd/evolve/a.go"}})
			writeItem(t, inbox, "f-two", map[string]any{"files": []string{"go/cmd/evolve/b.go"}})
			c := Census{
				Inbox:     inboxmover.Options{InboxDir: inbox, Stderr: io.Discard},
				Rank:      inboxrank.Inputs{Config: policy.Policy{}.InboxPriorityConfig(), Now: epoch},
				Protected: guardsProtected,
				Graph:     importGraph(t),
				FileCells: tc.fileCells,
				Now:       func() time.Time { return epoch },
			}
			got, err := c.Count()
			if err != nil {
				t.Fatal(err)
			}
			if got.Counts[ClassAdmissible] != tc.admissible || got.Counts[ClassWaitingCell] != tc.waiting {
				t.Errorf("counts = %v, want %d admissible and %d waiting", got.Counts, tc.admissible, tc.waiting)
			}
		})
	}
}

func TestAdmitByCell_AFileCellLeafAdmitsTheNextFileAtOnce(t *testing.T) {
	t.Parallel()
	admit := AdmitByCell(admission.Admitter{Graph: importGraph(t), FileCells: []string{"cmd/evolve"}})
	running := fleet.Todo{ID: "lane", Files: []string{"go/cmd/evolve/a.go"}}
	pending := []fleet.Todo{
		{ID: "same", Files: []string{"go/cmd/evolve/a.go"}},
		{ID: "other", Files: []string{"go/cmd/evolve/b.go"}},
	}
	if got := admit([]fleet.Todo{running}, pending); got != 1 {
		t.Fatalf("admit = %d, want 1: the todo that edits b.go goes beside the lane on a.go, and the one that edits a.go waits", got)
	}
}
