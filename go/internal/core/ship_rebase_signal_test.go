package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type inboxEvents struct {
	mu     sync.Mutex
	events []signalcenter.Event
	center *signalcenter.Center
}

func newInboxEvents() *inboxEvents {
	ie := &inboxEvents{center: signalcenter.New()}
	ie.center.Subscribe(func(e signalcenter.Event) {
		ie.mu.Lock()
		defer ie.mu.Unlock()
		if e.Code == CodeFleetRebaseInboxResolved {
			ie.events = append(ie.events, e)
		}
	})
	return ie
}

func (ie *inboxEvents) seen() []signalcenter.Event {
	ie.center.Flush()
	ie.mu.Lock()
	defer ie.mu.Unlock()
	return append([]signalcenter.Event(nil), ie.events...)
}

func TestReauditCycle_AnInboxStampOnMainIsResolvedAndPublishesItsPaths(t *testing.T) {
	t.Parallel()

	l := newInboxLane(t,
		func(l *inboxLane) { writeInboxFile(t, filepath.Join(l.worktree, ownItemPath), laneItem) },
		stampOnMainFile(t))
	evolveDir := filepath.Join(l.repo.Dir, ".evolve")
	raw, err := json.Marshal(CycleState{CycleID: reauditCycle, Phase: string(PhaseShip), RunID: reauditRunID, WorkspacePath: l.workspace, ActiveWorktree: l.worktree})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ResolveCycleStatePath(evolveDir), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	ie := newInboxEvents()
	runner := &reauditRunner{verdict: VerdictPASS, fx: &reauditFixture{t: t, root: l.repo, worktree: l.worktree}}

	_, err = ReauditCycle(context.Background(), ReauditInput{
		ProjectRoot: l.repo.Dir, EvolveDir: evolveDir, Cycle: reauditCycle,
		Ledger: &fakeLedger{}, Audit: runner, Signals: ie.center,
	})

	got := ie.seen()
	if err != nil || len(got) != 1 || !strings.Contains(got[0].Fields["paths"], ownItemPath) {
		t.Fatalf("reaudit = %v, events = %+v, want one resolved event naming %s", err, got, ownItemPath)
	}
}
