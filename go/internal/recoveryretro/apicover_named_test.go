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
