package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

const notesSelfcheckSource = `package core

func writeArtifact(v []string) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile("x", data, 0o644); err != nil {
		return
	}
}
`

func notesTree(t *testing.T, profile string) (worktree, moduleDir string) {
	t.Helper()
	worktree = t.TempDir()
	moduleDir = filepath.Join(worktree, "go")
	writeBlockerFile(t, worktree, "go/go.mod", "module example.com/m\n\ngo 1.23\n")
	writeBlockerFile(t, worktree, "go/internal/core/phase_bindings_selfcheck.go", notesSelfcheckSource)
	writeBlockerFile(t, worktree, "go/internal/core/plain.go", notesSelfcheckSource)
	writeBlockerFile(t, worktree, "go/cover.out", profile)
	return worktree, moduleDir
}

func TestBlockNoter_AProtectedUncoveredBlockSaysToDeclareABlocker(t *testing.T) {
	t.Parallel()
	wt, mod := notesTree(t, "mode: set\nexample.com/m/internal/core/phase_bindings_selfcheck.go:5.16,7.3 1 0\nexample.com/m/internal/core/phase_bindings_selfcheck.go:3.30,5.16 2 1\n")

	got := strings.Join(newBlockNoter(wt, mod, protectsSelfcheck).notes(filepath.Join(mod, "cover.out"), "internal/core"), "\n")

	for _, want := range []string{
		"uncovered go/internal/core/phase_bindings_selfcheck.go:5-7 (1 statement(s))",
		"protected control-plane path",
		"BLOCKER protected-path go/internal/core/phase_bindings_selfcheck.go:5",
		"json.MarshalIndent",
		"seam",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("notes lack %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, ":3-5") {
		t.Errorf("a covered block is not listed:\n%s", got)
	}
}

func TestBlockNoter_AnUnprotectedReachableBlockGetsNoBlockerAdvice(t *testing.T) {
	t.Parallel()
	wt, mod := notesTree(t, "mode: set\nexample.com/m/internal/core/zeta.go:1.1,2.2 1 0\nexample.com/m/internal/core/plain.go:8.71,10.3 1 0\nexample.com/m/internal/other/o.go:1.1,2.2 1 0\n")

	got := strings.Join(newBlockNoter(wt, mod, protectsSelfcheck).notes(filepath.Join(mod, "cover.out"), "internal/core"), "\n")

	if !strings.Contains(got, "uncovered go/internal/core/plain.go:8-10") || strings.Index(got, "plain.go") > strings.Index(got, "zeta.go") {
		t.Errorf("notes = %q", got)
	}
	if strings.Contains(got, "BLOCKER") || strings.Contains(got, "seam") || strings.Contains(got, "internal/other") {
		t.Errorf("a reachable, editable block in this package only gets its location:\n%s", got)
	}
}

func TestBlockNoter_ManyBlocksAreCutWithAMarkerTheProverRefuses(t *testing.T) {
	t.Parallel()
	var profile strings.Builder
	profile.WriteString("mode: set\n")
	for i := 1; i <= blockNoteCap+3; i++ {
		fmt.Fprintf(&profile, "example.com/m/internal/core/many.go:%d.1,%d.9 1 0\n", i, i)
	}
	wt, mod := notesTree(t, profile.String())

	got := newBlockNoter(wt, mod, nil).notes(filepath.Join(mod, "cover.out"), "internal/core")

	if len(got) != blockNoteCap+1 || !strings.Contains(got[len(got)-1], "… 3 more uncovered block(s)") {
		t.Errorf("notes = %q", got)
	}
}

func TestBlockNoter_AnUnreadableProfileOrModuleGivesNoNotes(t *testing.T) {
	t.Parallel()
	wt, mod := notesTree(t, "not a profile")
	for name, n := range map[string]struct {
		noter   blockNoter
		profile string
	}{
		"a bad profile":     {newBlockNoter(wt, mod, nil), filepath.Join(mod, "cover.out")},
		"a missing profile": {newBlockNoter(wt, mod, nil), filepath.Join(mod, "absent.out")},
		"no module":         {blockNoter{}, filepath.Join(mod, "cover.out")},
	} {
		if got := n.noter.notes(n.profile, "internal/core"); got != nil {
			t.Errorf("%s: notes = %q", name, got)
		}
	}
}

func TestNewBlockNoter_TheRepoPrefixIsTheModuleDirUnderTheWorktree(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ worktree, module, want string }{
		"a go/ module":       {"/w", "/w/go", "go/"},
		"a root module":      {"/w", "/w", ""},
		"no worktree":        {"", "/w/go", ""},
		"a module elsewhere": {"/w", "/x/go", ""},
	} {
		if got := newBlockNoter(tc.worktree, tc.module, nil).repoPrefix; got != tc.want {
			t.Errorf("%s: prefix = %q, want %q", name, got, tc.want)
		}
	}
}

func TestInfallibleCallBefore_IsAHintNotAProof(t *testing.T) {
	t.Parallel()
	src := strings.Split(notesSelfcheckSource, "\n")
	for name, tc := range map[string]struct {
		line int
		want string
	}{
		"the MarshalIndent error return": {5, "json.MarshalIndent"},
		"a WriteFile error return":       {8, ""},
		"a line past the end":            {99, ""},
		"line zero":                      {0, ""},
	} {
		if got := infallibleCallBefore(src, tc.line); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}

func TestWithProtectedFileNotes_APredicateFailureOnAProtectedFileSaysToDeclareABlocker(t *testing.T) {
	t.Parallel()
	in := []string{cycle1946PredicateFailure, "./internal/core: unit tests FAIL\n" + selfcheckPath, cycle1950Failure}

	got := withProtectedFileNotes(in, protectsSelfcheck)

	if !strings.Contains(got[0], "BLOCKER protected-path "+selfcheckPath+":138") {
		t.Errorf("the predicate failure gets the note: %q", got[0])
	}
	if got[1] != in[1] || got[2] != in[2] {
		t.Errorf("a plain unit-test failure and an unprotected file stay as they are: %q", got[1:])
	}
	if in[0] != cycle1946PredicateFailure {
		t.Error("the input slice is not changed")
	}
	if strings.Count(withProtectedFileNotes([]string{cycle1946PredicateFailure + selfcheckPath}, protectsSelfcheck)[0], "note:") != 1 {
		t.Error("one note for each protected file")
	}
	if withNil := withProtectedFileNotes(in, nil); withNil[0] != in[0] {
		t.Error("no manifest, no note")
	}
}

func TestRepoPathOf_ResolvesModuleQualifiedAndRepoRelativeTokens(t *testing.T) {
	t.Parallel()
	for token, want := range map[string]string{
		"github.com/mickeyyaya/evolve-loop/go/internal/core/x.go": "go/internal/core/x.go",
		"go/internal/core/x.go":                                   "go/internal/core/x.go",
		"x.go":                                                    "",
	} {
		if got := repoPathOf(token); got != want {
			t.Errorf("repoPathOf(%q) = %q, want %q", token, got, want)
		}
	}
}

func TestBuildHandoffFloorReview_CarriesTheFailuresAndTheProtectedNote(t *testing.T) {
	t.Parallel()
	floor := BuildHandoffFloor{{Name: "acs", Run: func(_ context.Context, _ ReviewInput) []string { return []string{cycle1946PredicateFailure} }}}

	rr := floor.Review(context.Background(), ReviewInput{Phase: string(PhaseBuild), Protected: protectsSelfcheck})

	if rr.Approve || len(rr.Failures) != 1 || !strings.Contains(rr.Failures[0], "BLOCKER protected-path") || !strings.Contains(rr.Reason, "BLOCKER protected-path") {
		t.Errorf("review = %+v", rr)
	}
}
