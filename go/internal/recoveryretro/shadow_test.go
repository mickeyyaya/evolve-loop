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

func TestWriteShadow_AWriteFailureIsReturned(t *testing.T) {
	root, workspace := t.TempDir(), t.TempDir()
	dir := filepath.Join(workspace, DirName)
	if err := os.MkdirAll(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	written, err := WriteShadow(root, workspace, 7, []cyclestate.CarryoverTodo{judgmentTodo()})

	if err == nil || !strings.Contains(err.Error(), "cycle-2003-judgment-plan-review.md") || len(written) != 0 {
		t.Fatalf("WriteShadow = %v, %v; want the failed paths and nothing written", written, err)
	}
}

func TestWriteShadow_NeverFollowsALinkAtADocumentPath(t *testing.T) {
	workspace, outside := t.TempDir(), t.TempDir()
	dir := filepath.Join(workspace, DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(outside, "victim.txt")
	writeFile(t, target, []byte("keep"))
	if err := os.Symlink(target, filepath.Join(dir, "cycle-2003-judgment-plan-review.md")); err != nil {
		t.Fatal(err)
	}
	todos := []cyclestate.CarryoverTodo{judgmentTodo(), {ID: "cycle-7-judgment-adversarial-review", Action: "y"}}

	written, err := WriteShadow(t.TempDir(), workspace, 7, todos)

	sort.Strings(written)
	want := []string{filepath.Join(dir, "cycle-2003-judgment-plan-review.yaml"), filepath.Join(dir, "cycle-7-judgment-adversarial-review.md"), filepath.Join(dir, "cycle-7-judgment-adversarial-review.yaml")}
	if err != nil || !equalStrings(written, want) {
		t.Fatalf("WriteShadow = %v, %v; want %v", written, err, want)
	}
	if got, _ := os.ReadFile(target); string(got) != "keep" {
		t.Fatalf("WriteShadow wrote through a link: %q", got)
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

func TestWriteShadow_TheCurrentDossierCanComeFromThePendingDir(t *testing.T) {
	root, workspace := t.TempDir(), t.TempDir()
	history, pass := utf8History()
	seedDossiers(t, dossier.CyclesDir(root), history[:3]...)
	if err := os.MkdirAll(dossier.PendingDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	seedDossiers(t, dossier.PendingDir(root), pass)
	if written, err := WriteShadow(root, workspace, 1809, nil); err != nil || len(written) != 2 {
		t.Fatalf("WriteShadow with a pending current dossier = %v, %v; want one pair", written, err)
	}
}

func TestWriteShadow_NeverOverwritesADocumentThatExists(t *testing.T) {
	workspace := t.TempDir()
	dir := filepath.Join(workspace, DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	human := []byte("## Judgment\n\nA person wrote this.\n")
	writeFile(t, filepath.Join(dir, "cycle-2003-judgment-plan-review.md"), human)

	written, err := WriteShadow(t.TempDir(), workspace, 7, []cyclestate.CarryoverTodo{judgmentTodo()})

	if err != nil || !equalStrings(written, []string{filepath.Join(dir, "cycle-2003-judgment-plan-review.yaml")}) {
		t.Fatalf("WriteShadow = %v, %v; want only the absent YAML written", written, err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "cycle-2003-judgment-plan-review.md")); string(got) != string(human) {
		t.Fatalf("WriteShadow replaced a document that exists:\n%s", got)
	}
}

func TestWriteShadow_TwoItemsWithOneNameWriteOnePairAndReportTheCollision(t *testing.T) {
	root, workspace := t.TempDir(), t.TempDir()
	fail := func(cycle int) *dossier.Dossier {
		return &dossier.Dossier{Cycle: cycle, FinalVerdict: dossier.VerdictFail, Tasks: tasks("a_b", "a-b")}
	}
	pass := &dossier.Dossier{Cycle: 3, FinalVerdict: dossier.VerdictPass, Tasks: tasks("a_b", "a-b"), CommitSHA: "c"}
	seedDossiers(t, dossier.CyclesDir(root), fail(1), fail(2), pass)

	written, err := WriteShadow(root, workspace, 3, nil)

	if len(written) != 2 || err == nil || !strings.Contains(err.Error(), "cycle-3-recovery-a-b") {
		t.Fatalf("WriteShadow = %v, %v; want one pair and the collision error", written, err)
	}
}

func TestWriteShadow_ACorruptCurrentDossierIsAnError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(dossier.CyclesDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dossier.CyclesDir(root), "cycle-9.json"), []byte("{not json"))

	if _, err := WriteShadow(root, t.TempDir(), 9, nil); err == nil || !strings.Contains(err.Error(), "cycle-9.json") {
		t.Fatalf("WriteShadow with a corrupt current dossier = %v, want an error that names it", err)
	}
}

func TestWriteShadow_AnUnreadableCurrentDossierIsAnError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dossier.CyclesDir(root), "cycle-9.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := WriteShadow(root, t.TempDir(), 9, nil); err == nil || !strings.Contains(err.Error(), "read the dossier") {
		t.Fatalf("WriteShadow with a directory at the current dossier = %v, want the read error", err)
	}
}
