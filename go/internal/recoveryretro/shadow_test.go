package recoveryretro

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

func seedDossiers(t *testing.T, dir string, ds ...*dossier.Dossier) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, d := range ds {
		raw, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, "cycle-"+strconv.Itoa(d.Cycle)+".json"), raw)
	}
}

func TestWriteShadow_APassAfterFailuresWritesTheRecoveryPairInTheWorkspaceOnly(t *testing.T) {
	root, workspace := t.TempDir(), t.TempDir()
	history, _ := utf8History()
	seedDossiers(t, dossier.CyclesDir(root), history[0], history[1])
	seedDossiers(t, dossier.PendingDir(root), history[2], history[3])
	if err := llmcalls.AppendWorkspace(workspace, llmcalls.Record{Phase: "build", Rung: "R1"}); err != nil {
		t.Fatal(err)
	}

	written, err := WriteShadow(root, workspace, 1809, nil)

	if err != nil {
		t.Fatalf("WriteShadow: %v", err)
	}
	base := filepath.Join(workspace, DirName, "cycle-1809-recovery-inboxbatch-utf8-and-resolution")
	if want := []string{base + ".md", base + ".yaml"}; !equalStrings(written, want) {
		t.Fatalf("written = %v, want %v", written, want)
	}
	doc, _ := os.ReadFile(base + ".md")
	if !strings.Contains(string(doc), "| 1806 | FAIL |") || !strings.Contains(string(doc), "`build R1`") {
		t.Fatalf("the shadow document lacks the history or the rung:\n%s", doc)
	}
	assertOnlyWorkspaceWrites(t, root)
}

func TestWriteShadow_AFailCloseoutWritesOnlyTheJudgmentPairs(t *testing.T) {
	root, workspace := t.TempDir(), t.TempDir()
	history, _ := utf8History()
	seedDossiers(t, dossier.CyclesDir(root), history[:3]...)
	todos := []cyclestate.CarryoverTodo{judgmentTodo(), {ID: "cycle-1808-failed-build", Action: "x"}}

	written, err := WriteShadow(root, workspace, 1808, todos)

	if err != nil {
		t.Fatalf("WriteShadow: %v", err)
	}
	base := filepath.Join(workspace, DirName, "cycle-2003-judgment-plan-review")
	if want := []string{base + ".md", base + ".yaml"}; !equalStrings(written, want) {
		t.Fatalf("written = %v, want %v", written, want)
	}
	assertOnlyWorkspaceWrites(t, root)
}

func TestWriteShadow_NothingToWriteCreatesNoDirectory(t *testing.T) {
	root, workspace := t.TempDir(), t.TempDir()

	written, err := WriteShadow(root, workspace, 5, nil)

	if err != nil || len(written) != 0 {
		t.Fatalf("WriteShadow = %v, %v; want nothing", written, err)
	}
	if _, serr := os.Stat(filepath.Join(workspace, DirName)); !errors.Is(serr, os.ErrNotExist) {
		t.Fatalf("WriteShadow created %s with nothing to write", DirName)
	}
}

func TestWriteShadow_AnEmptyWorkspaceIsAnError(t *testing.T) {
	if _, err := WriteShadow(t.TempDir(), " ", 5, nil); err == nil {
		t.Fatal("WriteShadow with no workspace returned no error")
	}
}

func TestWriteShadow_AWriteFailureIsReturnedAndTheOtherPairsStillWrite(t *testing.T) {
	root, workspace := t.TempDir(), t.TempDir()
	dir := filepath.Join(workspace, DirName)
	if err := os.MkdirAll(filepath.Join(dir, "cycle-2003-judgment-plan-review.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	todos := []cyclestate.CarryoverTodo{judgmentTodo(), {ID: "cycle-7-judgment-adversarial-review", Action: "y"}}

	written, err := WriteShadow(root, workspace, 7, todos)

	if err == nil || !strings.Contains(err.Error(), "cycle-2003-judgment-plan-review.md") {
		t.Fatalf("WriteShadow error = %v, want the failed path", err)
	}
	sort.Strings(written)
	want := []string{filepath.Join(dir, "cycle-2003-judgment-plan-review.yaml"), filepath.Join(dir, "cycle-7-judgment-adversarial-review.md"), filepath.Join(dir, "cycle-7-judgment-adversarial-review.yaml")}
	if !equalStrings(written, want) {
		t.Fatalf("written = %v, want %v", written, want)
	}
}

func TestWriteShadow_AnUnwritableDirectoryIsAnError(t *testing.T) {
	workspace := t.TempDir()
	writeFile(t, filepath.Join(workspace, DirName), []byte("a file"))

	if _, err := WriteShadow(t.TempDir(), workspace, 7, []cyclestate.CarryoverTodo{judgmentTodo()}); err == nil {
		t.Fatal("WriteShadow into a file returned no error")
	}
}

func TestWriteShadow_AnUnreadableCallLogIsAnError(t *testing.T) {
	root, workspace := t.TempDir(), t.TempDir()
	history, _ := utf8History()
	seedDossiers(t, dossier.CyclesDir(root), history...)
	if err := os.MkdirAll(llmcalls.Path(workspace), 0o755); err != nil {
		t.Fatal(err)
	}

	written, err := WriteShadow(root, workspace, 1809, nil)

	if err == nil || len(written) != 0 {
		t.Fatalf("WriteShadow = %v, %v; want the call-log error and no recovery pair, because its rung line can not be true", written, err)
	}
}

func assertOnlyWorkspaceWrites(t *testing.T, root string) {
	t.Helper()
	var extra []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(path) != ".json" {
			extra = append(extra, path)
		}
		return nil
	})
	if len(extra) != 0 {
		t.Fatalf("WriteShadow wrote outside the workspace: %v", extra)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestWriteShadow_ARecoveryWithNoCallLogHasNoRung(t *testing.T) {
	root, workspace := t.TempDir(), t.TempDir()
	history, _ := utf8History()
	seedDossiers(t, dossier.CyclesDir(root), history...)

	written, err := WriteShadow(root, workspace, 1809, nil)

	if err != nil || len(written) != 2 {
		t.Fatalf("WriteShadow = %v, %v; want one pair", written, err)
	}
	doc, _ := os.ReadFile(written[0])
	if !strings.Contains(string(doc), "The passed try used no ladder rung above `R0`.") {
		t.Fatalf("a recovery with no call log claims a rung:\n%s", doc)
	}
}

func TestWriteShadow_OnlyRungsAboveR0AreListedOnce(t *testing.T) {
	workspace := t.TempDir()
	for _, rec := range []llmcalls.Record{{Phase: "build", Rung: "R0"}, {Phase: "build"}, {Phase: "audit", Rung: "R2"}, {Phase: "audit", Rung: "R2"}} {
		if err := llmcalls.AppendWorkspace(workspace, rec); err != nil {
			t.Fatal(err)
		}
	}

	got, err := laddersAbove(workspace)

	if err != nil || !equalStrings(got, []string{"audit R2"}) {
		t.Fatalf("laddersAbove = %v, %v; want [audit R2]", got, err)
	}
}

func TestWriteShadow_ACycleInBothDossierDirsCountsOnce(t *testing.T) {
	root, workspace := t.TempDir(), t.TempDir()
	history, pass := utf8History()
	seedDossiers(t, dossier.CyclesDir(root), history[2], pass)
	seedDossiers(t, dossier.PendingDir(root), history[2])

	if written, err := WriteShadow(root, workspace, 1809, nil); err != nil || len(written) != 0 {
		t.Fatalf("WriteShadow = %v, %v; want no recovery from one failure that is in both dirs", written, err)
	}
}

func TestWriteShadow_TheCurrentDossierComesFromThePendingDirOrNowhere(t *testing.T) {
	root, workspace := t.TempDir(), t.TempDir()
	history, pass := utf8History()
	seedDossiers(t, dossier.CyclesDir(root), history[:3]...)
	if err := os.MkdirAll(dossier.PendingDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dossier.CyclesDir(root), "cycle-1809.json"), []byte("{not json"))

	if written, err := WriteShadow(root, workspace, 1809, nil); err != nil || len(written) != 0 {
		t.Fatalf("WriteShadow with a corrupt current dossier = %v, %v; want nothing", written, err)
	}
	seedDossiers(t, dossier.PendingDir(root), pass)
	if written, err := WriteShadow(root, workspace, 1809, nil); err != nil || len(written) != 2 {
		t.Fatalf("WriteShadow with a pending current dossier = %v, %v; want one pair", written, err)
	}
}
