package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

func TestWriteCycleDossier_CommitFalse_WritesFilesOnly(t *testing.T) {
	root := t.TempDir() // deliberately not a git repository
	ws := t.TempDir()
	params := cycleDossierParams{ProjectRoot: root, WorkspacePath: ws, Cycle: 3, Goal: "simulate", RunID: "run-sim", Outcome: CycleOutcomeShippedViaBuild}
	params.Destination = DossierFilesOnly
	if err := writeCycleDossier(nil, params); err != nil {
		t.Fatalf("files-only dossier on a non-git root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "knowledge-base", "cycles", "cycle-3.json")); err != nil {
		t.Errorf("the dossier file is still written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		t.Error("a files-only dossier must not create a repository")
	}
	params.Cycle, params.Destination = 4, DossierCommitted
	if err := writeCycleDossier(nil, params); err == nil {
		t.Error("the committing default on a non-git root cannot succeed — the two modes are distinguishable")
	}
}

func gitCommitCount(t *testing.T, root string) int {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "rev-list", "--all", "--count").Output()
	if err != nil {
		t.Fatalf("rev-list: %v", err)
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}

func TestWithDossierDestination_IsTheRootsKnob(t *testing.T) {
	for _, tc := range []struct {
		name     string
		opts     []Option
		env      map[string]string
		commits  int
		inCorpus bool
	}{
		{"default commits", nil, nil, 1, true},
		{"simulate root writes only", []Option{WithDossierDestination(DossierFilesOnly)}, nil, 0, true},
		{"a fleet lane leaves it pending", nil, map[string]string{"EVOLVE_FLEET": "1"}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := initDossierRepo(t)
			before := gitCommitCount(t, root)
			opts := append([]Option{WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()})}, tc.opts...)
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), opts...)
			res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "dossier-knob", Env: tc.env})
			if err != nil {
				t.Fatalf("RunCycle: %v", err)
			}
			name := "cycle-" + strconv.Itoa(res.Cycle) + ".json"
			_, corpusErr := os.Stat(filepath.Join(root, "knowledge-base", "cycles", name))
			_, pendingErr := os.Stat(filepath.Join(root, ".evolve", "dossiers-pending", name))
			if (corpusErr == nil) != tc.inCorpus || (pendingErr == nil) == tc.inCorpus {
				t.Errorf("corpus present=%v pending present=%v, want the dossier in the corpus=%v, else pending", corpusErr == nil, pendingErr == nil, tc.inCorpus)
			}
			if got := gitCommitCount(t, root) - before; got != tc.commits {
				t.Errorf("commits added = %d, want %d", got, tc.commits)
			}
		})
	}
}

func TestDossierDestination_AFleetLaneDefersOnlyACommit(t *testing.T) {
	lane := map[string]string{"EVOLVE_FLEET": "1"}
	for _, tc := range []struct {
		configured DossierDestination
		env        map[string]string
		want       DossierDestination
	}{
		{DossierCommitted, nil, DossierCommitted},
		{DossierCommitted, lane, DossierPending},
		{DossierFilesOnly, lane, DossierFilesOnly},
		{DossierFilesOnly, nil, DossierFilesOnly},
	} {
		cr := &cycleRun{o: &Orchestrator{dossierDestination: tc.configured}, req: CycleRequest{Env: tc.env}}
		if got := cr.dossierDestination(); got != tc.want {
			t.Errorf("configured %d, lane env %v: destination = %d, want %d", tc.configured, tc.env, got, tc.want)
		}
	}
}

func TestWriteCycleDossier_APendingDossierTakesNoLockAndCommitsNothing(t *testing.T) {
	root := initDossierRepo(t)
	before := gitCommitCount(t, root)
	spy := &mutexSpyLocker{}
	p := cycleDossierParams{ProjectRoot: root, WorkspacePath: t.TempDir(), Cycle: 12, Goal: "lane closeout", RunID: "run", Outcome: VerdictFAIL, Destination: DossierPending}

	err := writeCycleDossier(spy.acquire, p)

	if err != nil {
		t.Fatalf("writeCycleDossier: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dossier.PendingDir(root), "cycle-12.json")); err != nil {
		t.Errorf("the pending pair was not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "knowledge-base", "cycles", "cycle-12.json")); !os.IsNotExist(err) {
		t.Errorf("a pending dossier reached the corpus: %v", err)
	}
	if got := atomic.LoadInt32(&spy.acquired); got != 0 {
		t.Errorf("git-mutation lock acquired %d times, want 0: a pending write touches no git state", got)
	}
	if got := gitCommitCount(t, root) - before; got != 0 {
		t.Errorf("commits added = %d, want 0", got)
	}
}
