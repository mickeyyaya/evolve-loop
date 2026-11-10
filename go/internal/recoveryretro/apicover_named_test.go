package recoveryretro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

func TestMinFailures_IsTheFewestFailuresThatMakeARecovery(t *testing.T) {
	history, pass := utf8History()
	failures := history[:len(history)-1]

	if got := Recoveries(append(failures[len(failures)-MinFailures+1:], pass), pass); len(got) != 0 {
		t.Fatalf("Recoveries with %d failures = %+v, want none", MinFailures-1, got)
	}
	if got := Recoveries(append(failures[len(failures)-MinFailures:], pass), pass); len(got) != 1 {
		t.Fatalf("Recoveries with %d failures = %+v, want one", MinFailures, got)
	}
}

func TestHistoryWindow_BoundsTheDossiersThatWriteShadowReads(t *testing.T) {
	root, workspace := t.TempDir(), t.TempDir()
	pass := &dossier.Dossier{Cycle: 1000, FinalVerdict: dossier.VerdictPass, Tasks: tasks(utf8Item), CommitSHA: "c"}
	old := failDossier(1000-HistoryWindow-1, "build|a|1", "gate-block", "old")
	recent := failDossier(1000-HistoryWindow, "build|b|2", "gate-block", "recent")
	seedDossiers(t, dossier.CyclesDir(root), old, recent, pass)

	if written, err := WriteShadow(root, workspace, 1000, nil); err != nil || len(written) != 0 {
		t.Fatalf("WriteShadow = %v, %v; want no recovery, because the older failure is outside the window", written, err)
	}
	seedDossiers(t, dossier.CyclesDir(root), failDossier(999, "build|c|3", "gate-block", "last"))
	if written, err := WriteShadow(root, workspace, 1000, nil); err != nil || len(written) != 2 {
		t.Fatalf("WriteShadow = %v, %v; want one pair with two failures in the window", written, err)
	}
}

func TestDocsHome_IsTheSourceOfEachLessonYAML(t *testing.T) {
	workspace := t.TempDir()

	written, err := WriteShadow(t.TempDir(), workspace, 7, []cyclestate.CarryoverTodo{judgmentTodo()})
	if err != nil || len(written) != 2 {
		t.Fatalf("WriteShadow = %v, %v", written, err)
	}
	raw, _ := os.ReadFile(filepath.Join(workspace, DirName, "cycle-2003-judgment-plan-review.yaml"))
	if !strings.Contains(string(raw), `source: "`+DocsHome+`/cycle-2003-judgment-plan-review.md"`) {
		t.Fatalf("the lesson YAML does not name its home under %s:\n%s", DocsHome, raw)
	}
}

func TestPhaseModel_ComesFromEachPhaseRecordInOrder(t *testing.T) {
	got := modelsOf([]dossier.PhaseRecord{phase("scout", "flash"), phase("build", "opus")})

	if want := []PhaseModel{{Phase: "scout", Model: "flash"}, {Phase: "build", Model: "opus"}}; len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("modelsOf = %+v, want %+v", got, want)
	}
}

func TestJudgmentLesson_HoldsTheCycleAndThePhaseOfItsID(t *testing.T) {
	got := JudgmentLessons([]cyclestate.CarryoverTodo{{ID: "cycle-41-judgment-adversarial-review", Action: "a"}})

	if want := (JudgmentLesson{CarryoverTodo: cyclestate.CarryoverTodo{ID: "cycle-41-judgment-adversarial-review", Action: "a"}, Cycle: 41, Phase: "adversarial-review"}); len(got) != 1 || got[0] != want {
		t.Fatalf("JudgmentLessons = %+v, want %+v", got, want)
	}
}

func TestPendingDir_IsUnderTheEvolveDirectory(t *testing.T) {
	if got, want := PendingDir("/p"), filepath.Join("/p", ".evolve", "retrospectives-pending"); got != want {
		t.Fatalf("PendingDir = %q, want %q", got, want)
	}
}

func TestWriteInto_WritesThePairIntoTheNamedDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "named")

	written, err := WriteInto(dir, t.TempDir(), t.TempDir(), 7, []cyclestate.CarryoverTodo{judgmentTodo()})

	if err != nil || len(written) != 2 || filepath.Dir(written[0]) != dir {
		t.Fatalf("WriteInto = %v, %v; want one pair in %s", written, err, dir)
	}
}

func TestPublishPending_WithNoPendingDirectoryPublishesNothing(t *testing.T) {
	var log strings.Builder

	published, err := PublishPending(t.TempDir(), &log)

	if err != nil || len(published) != 0 || log.Len() != 0 {
		t.Fatalf("PublishPending = %v, %v, %q; want nothing", published, err, log.String())
	}
}

func TestPublishPending_AGitAddFailureKeepsThePairPendingAndRemovesTheCopy(t *testing.T) {
	root := t.TempDir()
	dir := PendingDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".md", ".yaml"} {
		if err := os.WriteFile(filepath.Join(dir, "cycle-1-x"+ext), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	published, err := PublishPending(root, &strings.Builder{})

	if err == nil || len(published) != 0 {
		t.Fatalf("PublishPending = %v, %v; want an error outside a git tree", published, err)
	}
	if _, serr := os.Stat(filepath.Join(root, DocsHome, "cycle-1-x.md")); serr == nil {
		t.Error("the failed publish left a document in docs")
	}
	if _, serr := os.Stat(filepath.Join(dir, "cycle-1-x.md")); serr != nil {
		t.Errorf("the failed publish dropped the pending pair: %v", serr)
	}
}

func TestPublishPending_AnUnreadablePendingPathIsAnError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(PendingDir(root), []byte("a file"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := PublishPending(root, &strings.Builder{}); err == nil {
		t.Fatal("PublishPending of a file at the pending path returned no error")
	}
}
