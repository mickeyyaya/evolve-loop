package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type captureSink struct{ entries []BookkeepingEntry }

func (s *captureSink) Add(e BookkeepingEntry) error {
	s.entries = append(s.entries, e)
	return nil
}

func seededRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.Fixture(t)
	r.Git("commit", "--allow-empty", "-m", "seed")
	return r
}

func commitCount(r *gittest.Repo) string {
	return strings.TrimSpace(r.Git("rev-list", "--count", "HEAD"))
}

func newBookkeepingRun(stage string, sink BookkeepingSink, env map[string]string) *cycleRun {
	o := &Orchestrator{}
	WithBookkeeping(stage, sink)(o)
	cr := &cycleRun{o: o}
	cr.req.Env = env
	return cr
}

func TestDossierDestination_BatchedOnlyAtEnforceWithASink(t *testing.T) {
	t.Parallel()
	sink := &captureSink{}
	cases := []struct {
		name  string
		stage string
		sink  BookkeepingSink
		env   map[string]string
		want  DossierDestination
	}{
		{"enforce with a sink", policy.FleetIntegrationEnforce, sink, nil, DossierBatched},
		{"shadow is unchanged", policy.FleetIntegrationShadow, sink, nil, DossierCommitted},
		{"enforce without a sink is unchanged", policy.FleetIntegrationEnforce, nil, nil, DossierCommitted},
		{"an explicit files-only destination survives enforce", policy.FleetIntegrationEnforce, sink, nil, DossierFilesOnly},
		{"a fleet lane still defers to pending", policy.FleetIntegrationEnforce, sink, map[string]string{ipcenv.FleetKey: "1"}, DossierPending},
	}
	for _, c := range cases {
		cr := newBookkeepingRun(c.stage, c.sink, c.env)
		if c.want == DossierFilesOnly {
			cr.o.dossierDestination = DossierFilesOnly
		}
		if got := cr.dossierDestination(); got != c.want {
			t.Errorf("%s: destination = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestWriteCycleDossier_AtEnforceGoesToTheBatchNotMain(t *testing.T) {
	t.Parallel()
	repo := seededRepo(t)
	root := repo.Dir
	before := commitCount(repo)
	sink := &captureSink{}
	p := cycleDossierParams{ProjectRoot: root, WorkspacePath: t.TempDir(), Cycle: 21, Goal: "g", RunID: "r21", Outcome: VerdictPASS, Destination: DossierBatched, Sink: sink}
	if err := writeCycleDossier(nil, p); err != nil {
		t.Fatalf("writeCycleDossier: %v", err)
	}
	if after := commitCount(repo); after != before {
		t.Fatalf("commits %s -> %s; a batched dossier must not commit to main", before, after)
	}
	if len(sink.entries) != 1 {
		t.Fatalf("sink entries = %d, want 1", len(sink.entries))
	}
	e := sink.entries[0]
	wantPaths := []string{"knowledge-base/cycles/cycle-21.json", "knowledge-base/cycles/cycle-21.md"}
	if e.Kind != BookkeepingDossier || strings.Join(e.Paths, ",") != strings.Join(wantPaths, ",") || e.Message != "dossier: cycle-21 closeout" {
		t.Fatalf("entry = %+v, want kind dossier_closeout, paths %v, message %q", e, wantPaths, "dossier: cycle-21 closeout")
	}
}

func TestWriteCycleDossier_AtShadowStillCommitsAndFeedsNoBatch(t *testing.T) {
	t.Parallel()
	repo := seededRepo(t)
	root := repo.Dir
	before := commitCount(repo)
	sink := &captureSink{}
	p := cycleDossierParams{ProjectRoot: root, WorkspacePath: t.TempDir(), Cycle: 22, Goal: "g", RunID: "r22", Outcome: VerdictPASS, Destination: DossierCommitted, Sink: sink}
	if err := writeCycleDossier(nil, p); err != nil {
		t.Fatalf("writeCycleDossier: %v", err)
	}
	if after := commitCount(repo); after == before {
		t.Fatalf("commits stayed %s; the shadow path must still commit", before)
	}
	if len(sink.entries) != 0 {
		t.Fatalf("sink entries = %d, want 0 at shadow", len(sink.entries))
	}
}

type refusingSink struct{}

func (refusingSink) Add(BookkeepingEntry) error { return errBookkeepingRefused }

func TestWriteCycleDossier_ASinkRefusalFailsLoudly(t *testing.T) {
	t.Parallel()
	root := initDossierRepo(t)
	p := cycleDossierParams{ProjectRoot: root, WorkspacePath: t.TempDir(), Cycle: 23, Goal: "g", RunID: "r23", Outcome: VerdictPASS, Destination: DossierBatched, Sink: refusingSink{}}
	if err := writeCycleDossier(nil, p); err == nil || !strings.Contains(err.Error(), "bookkeeping") {
		t.Fatalf("err = %v, want a bookkeeping error", err)
	}
}

var errBookkeepingRefused = errors.New("batch refused")

func TestDossierParams_CarriesTheDestinationAndTheSinkOfTheOrchestrator(t *testing.T) {
	t.Parallel()
	sink := &captureSink{}
	cr := newBookkeepingRun(policy.FleetIntegrationEnforce, sink, nil)
	cr.timingsFlushed = true
	p := cr.dossierParams(VerdictPASS)
	if p.Destination != DossierBatched || p.Sink != BookkeepingSink(sink) {
		t.Fatalf("params destination %v sink %v, want batched and the orchestrator sink", p.Destination, p.Sink)
	}
}

func TestWriteCycleDossier_AtEnforceStillWritesBothFilesIntoTheCorpus(t *testing.T) {
	t.Parallel()
	root := initDossierRepo(t)
	p := cycleDossierParams{ProjectRoot: root, WorkspacePath: t.TempDir(), Cycle: 24, Goal: "g", RunID: "r24", Outcome: VerdictPASS, Destination: DossierBatched, Sink: &captureSink{}}
	if err := writeCycleDossier(nil, p); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".json", ".md"} {
		if _, err := os.Stat(filepath.Join(root, "knowledge-base", "cycles", "cycle-24"+ext)); err != nil {
			t.Errorf("batched dossier left no %s file: %v", ext, err)
		}
	}
}

func TestWriteCycleDossier_ABatchedDossierWithNoSinkFailsLoudly(t *testing.T) {
	t.Parallel()
	root := initDossierRepo(t)
	p := cycleDossierParams{ProjectRoot: root, WorkspacePath: t.TempDir(), Cycle: 25, Goal: "g", RunID: "r25", Outcome: VerdictPASS, Destination: DossierBatched}
	if err := writeCycleDossier(nil, p); err == nil || !strings.Contains(err.Error(), "no sink") {
		t.Fatalf("err = %v, want an error naming the missing sink", err)
	}
}

func blockCorpusFile(t *testing.T, root string, cycle int) {
	t.Helper()
	blocker := filepath.Join(root, "knowledge-base", "cycles", fmt.Sprintf("cycle-%d.json", cycle))
	if err := os.MkdirAll(blocker, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestWriteCycleDossier_AFailedWriteAddsNoEntryToTheBatch(t *testing.T) {
	t.Parallel()
	root := initDossierRepo(t)
	blockCorpusFile(t, root, 26)
	sink := &captureSink{}
	p := cycleDossierParams{ProjectRoot: root, WorkspacePath: t.TempDir(), Cycle: 26, Goal: "g", RunID: "r26", Outcome: VerdictPASS, Destination: DossierBatched, Sink: sink}
	if err := writeCycleDossier(nil, p); err == nil {
		t.Fatal("a blocked corpus file did not fail the write")
	}
	if len(sink.entries) != 0 {
		t.Fatalf("entries = %d after a failed write, want 0", len(sink.entries))
	}
}

func TestCloseoutAtEnforce_AFailedWriteLeavesNoEntryNamingMissingFiles(t *testing.T) {
	t.Parallel()
	root := initDossierRepo(t)
	blockCorpusFile(t, root, 27)
	sink := &captureSink{}
	cr := newBookkeepingRun(policy.FleetIntegrationEnforce, sink, nil)
	cr.timingsFlushed = true
	cr.cycle = 27
	cr.req.GoalHash = "g27"
	cr.req.ProjectRoot = root
	cr.cs = CycleState{WorkspacePath: t.TempDir(), RunID: "r27"}
	if err := writeCycleDossier(nil, cr.dossierParams(VerdictPASS)); err == nil {
		t.Fatal("the closeout of a blocked corpus did not fail")
	}
	if len(sink.entries) != 0 {
		t.Fatalf("entries = %d, want 0: a flush would stage files never written", len(sink.entries))
	}
}
