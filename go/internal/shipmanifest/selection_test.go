package shipmanifest

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func writeUnder(t *testing.T, root, rel, body string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

type fakeGit struct {
	status, ignored string
	statusErr       error
	probeErr        error
	calls           [][]string
}

func (f *fakeGit) read(args ...string) (string, error) {
	f.calls = append(f.calls, args)
	switch {
	case slices.Contains(args, "status"):
		return f.status, f.statusErr
	case slices.Contains(args, "check-ignore"):
		return f.ignored, f.probeErr
	}
	return "", errors.New("unexpected git " + strings.Join(args, " "))
}

func declaredWorkspace(t *testing.T, root string) string {
	t.Helper()
	for _, rel := range []string{"a.go", "docs/x.md", "gen/out.txt", "scratch.txt"} {
		writeUnder(t, root, rel, rel)
	}
	ws := t.TempDir()
	writeUnder(t, ws, "build-report.md", "Changed `a.go` and wrote `docs/x.md`.\n")
	writeUnder(t, ws, "test-report.md", "The fixture lives in `gen/out.txt`.\n")
	return ws
}

const fourChanges = " M a.go\n?? docs/x.md\n?? gen/out.txt\n?? scratch.txt\n"

func TestSelect_TakesTheDeclaredChangesAndDropsTheIgnoredOnes(t *testing.T) {
	root := t.TempDir()
	ws := declaredWorkspace(t, root)
	git := &fakeGit{status: fourChanges, ignored: "gen/out.txt\n"}
	var read GitRead = git.read
	sel, err := Select(read, root, ws)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.go", "docs/x.md"}; !slices.Equal(sel.Paths, want) {
		t.Fatalf("Paths = %v, want %v: the undeclared scratch file and the ignored declared file never stage", sel.Paths, want)
	}
	if !slices.Equal(sel.Ignored, []string{"gen/out.txt"}) || sel.ProbeErr != nil {
		t.Fatalf("Ignored = %v (probe %v), want the ignored declared path", sel.Ignored, sel.ProbeErr)
	}
	if !slices.Equal(sel.Manifest, []string{"a.go", "docs/x.md", "gen/out.txt"}) || sel.Changed != 4 {
		t.Fatalf("Manifest = %v, Changed = %d", sel.Manifest, sel.Changed)
	}
	for _, call := range git.calls {
		if !slices.Equal(call[:2], []string{"-c", "core.quotePath=false"}) {
			t.Errorf("every path read asks git for raw paths: %v", call)
		}
	}
	probe := git.calls[len(git.calls)-1]
	if !slices.Equal(probe[2:], []string{"check-ignore", "--", "a.go", "docs/x.md", "gen/out.txt"}) {
		t.Errorf("the ignore probe asks about exactly the selected paths: %v", probe)
	}
}

func TestSelect_AFailedIgnoreProbeKeepsEverySelectedPath(t *testing.T) {
	root := t.TempDir()
	ws := declaredWorkspace(t, root)
	sel, err := Select((&fakeGit{status: fourChanges, probeErr: errors.New("probe broke")}).read, root, ws)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.go", "docs/x.md", "gen/out.txt"}; !slices.Equal(sel.Paths, want) || sel.Ignored != nil {
		t.Fatalf("Paths = %v, Ignored = %v: a broken probe fails open with the full set", sel.Paths, sel.Ignored)
	}
	if sel.ProbeErr == nil || !strings.Contains(sel.ProbeErr.Error(), "probe broke") {
		t.Fatalf("the probe failure travels for the caller to report: %v", sel.ProbeErr)
	}
}

func TestSelect_AFailedStatusReadIsAnError(t *testing.T) {
	sel, err := Select((&fakeGit{statusErr: errors.New("no repo")}).read, t.TempDir(), "")
	if err == nil || sel.Paths != nil || sel.Manifest != nil || sel.Changed != 0 {
		t.Fatalf("without the changed set there is no selection: %+v %v", sel, err)
	}
	var none Selection
	if sel.ProbeErr != none.ProbeErr {
		t.Fatalf("a failed status read never reaches the ignore probe: %v", sel.ProbeErr)
	}
}

func TestRawPathRead_PutsTheQuotePathOverrideBeforeTheSubcommand(t *testing.T) {
	if got := RawPathRead("check-ignore", "--", "café.txt"); !slices.Equal(got, []string{"-c", "core.quotePath=false", "check-ignore", "--", "café.txt"}) {
		t.Fatalf("git takes -c only before the subcommand: %v", got)
	}
}

func TestSelect_AWorkspaceWithoutReportsSelectsNoUndeclaredPath(t *testing.T) {
	root := t.TempDir()
	declaredWorkspace(t, root)
	git := &fakeGit{status: fourChanges}
	sel, err := Select(git.read, root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(sel.Paths) != 0 || sel.Changed != 4 {
		t.Fatalf("Paths = %v (changed %d): a cycle without reports declares nothing, so no residue is adopted", sel.Paths, sel.Changed)
	}
	if len(git.calls) != 1 {
		t.Fatalf("nothing selected, so no ignore probe: %v", git.calls)
	}
}

func TestSelect_WithoutAWorkspaceEveryChangedPathIsSelected(t *testing.T) {
	root := t.TempDir()
	declaredWorkspace(t, root)
	sel, err := Select((&fakeGit{status: fourChanges + "D  gone.go\n"}).read, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.go", "docs/x.md", "gen/out.txt", "scratch.txt"}; !slices.Equal(sel.Paths, want) || sel.Manifest != nil {
		t.Fatalf("Paths = %v, Manifest = %v", sel.Paths, sel.Manifest)
	}
}

func TestSelect_NothingSelectedAsksNoIgnoreProbe(t *testing.T) {
	git := &fakeGit{}
	sel, err := Select(git.read, t.TempDir(), "")
	if err != nil || len(sel.Paths) != 0 {
		t.Fatalf("a clean tree selects nothing: %v %v", sel.Paths, err)
	}
	if len(git.calls) != 1 {
		t.Fatalf("an empty selection needs no ignore probe: %v", git.calls)
	}
}

func TestReportFiles_AreTheBuildAndTDDReports(t *testing.T) {
	if got := ReportFiles(); !slices.Equal(got, []string{"build-report.md", "test-report.md"}) {
		t.Fatalf("ReportFiles = %v", got)
	}
}

func TestGitIn_ReadsWithoutWritingTheIndexAndTakesExitOneAsAnAnswer(t *testing.T) {
	root := gittest.Fixture(t).Dir
	writeUnder(t, root, "a.go", "package a\n")
	if out, err := exec.Command("git", "-C", root, "add", "a.go").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	index := filepath.Join(root, ".git", "index")
	touched := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(root, "a.go"), touched, touched); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	read := GitIn(context.Background(), root)
	if out, err := read("status", "--porcelain"); err != nil || !strings.Contains(out, "a.go") {
		t.Fatalf("status reads the worktree: %q %v", out, err)
	}
	if later, err := os.ReadFile(index); err != nil || string(later) != string(before) {
		t.Fatalf("a read never refreshes the real index (%v)", err)
	}
	if out, err := read("check-ignore", "--", "a.go"); err != nil || out != "" {
		t.Fatalf("check-ignore's exit 1 means nothing is ignored, never a failure: %q %v", out, err)
	}
	if _, err := read("rev-parse", "--verify", "no-such-ref"); err == nil {
		t.Fatal("an exit above 1 is a failure")
	}
}

func probeAnswering(lines ...string) GitRead {
	return func(args ...string) (string, error) {
		if len(lines) == 0 {
			return "", nil
		}
		return strings.Join(lines, "\n") + "\n", nil
	}
}

func TestWithoutIgnored_DecodesQuotedProbeOutput(t *testing.T) {
	for _, tc := range []struct {
		name  string
		probe []string
		paths []string
		want  []string
	}{
		{"an octal-quoted ignored path is dropped", []string{`"caf\303\251.txt"`}, []string{"café.txt", "go/a.go"}, []string{"go/a.go"}},
		{"a quote-bearing ignored path is dropped", []string{`"we\"ird.txt"`}, []string{`we"ird.txt`, "go/a.go"}, []string{"go/a.go"}},
		{"an unquoted ignored path is dropped", []string{".evolve/evals/slug.md"}, []string{".evolve/evals/slug.md", "go/a.go"}, []string{"go/a.go"}},
		{"a different quoted path drops nothing", []string{`"oth\303\251r.txt"`}, []string{"café.txt", "go/a.go"}, []string{"café.txt", "go/a.go"}},
		{"the escaped spelling of an undeclared path drops nothing", []string{`caf\303\251.txt`}, []string{"café.txt"}, []string{"café.txt"}},
		{"nothing ignored drops nothing", nil, []string{"café.txt", `we"ird.txt`, "go/a.go"}, []string{"café.txt", `we"ird.txt`, "go/a.go"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kept, _, err := withoutIgnored(probeAnswering(tc.probe...), tc.paths)
			if err != nil || !slices.Equal(kept, tc.want) {
				t.Fatalf("kept %q (%v), want %q: an ignored path left in fails the add, an unignored one dropped under-stages the ship", kept, err, tc.want)
			}
		})
	}
}
