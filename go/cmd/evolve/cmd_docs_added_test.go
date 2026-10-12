package main

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

func addedFixture(t *testing.T, base map[string]string) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	files := map[string]string{stelint.StandardPath: steFixtureStandard}
	for k, v := range base {
		files[k] = v
	}
	writeSteFiles(t, repo.Dir, files)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	return repo, repo.Git("rev-parse", "HEAD")
}

func TestDocsSteLint_AddedReportsOnlyTheNewText(t *testing.T) {
	oldDoc := "We utilize it.\n\nClean line.\n"
	for name, tc := range map[string]struct {
		base    map[string]string
		change  map[string]string
		commit  bool
		remove  []string
		wantOut string
	}{
		"a finding on an old line of a changed file is not reported": {
			base:    map[string]string{"docs/a.md": oldDoc},
			change:  map[string]string{"docs/a.md": oldDoc + "\nAnother clean line.\n"},
			wantOut: "ste-lint: 0 finding(s) in 0 file(s); 1 file(s) checked\n",
		},
		"a finding on an added line is reported with its line": {
			base:    map[string]string{"docs/a.md": oldDoc},
			change:  map[string]string{"docs/a.md": oldDoc + "\nGo via it.\n"},
			wantOut: "docs/a.md:5 STE-WORD replace \"via\" with \"through\"\nste-lint: 1 finding(s) in 1 file(s); 1 file(s) checked\n",
		},
		"a new committed file reports every finding": {
			base:    map[string]string{"README.md": "Clean.\n"},
			change:  map[string]string{"docs/n.md": "We utilize it.\n\nGo via it.\n"},
			commit:  true,
			wantOut: "docs/n.md:1 STE-WORD replace \"utilize\" with \"use\"\ndocs/n.md:3 STE-WORD replace \"via\" with \"through\"\nste-lint: 2 finding(s) in 1 file(s); 1 file(s) checked\n",
		},
		"an untracked file reports every finding": {
			base:    map[string]string{"README.md": "Clean.\n"},
			change:  map[string]string{"docs/u.md": "We utilize it.\n"},
			wantOut: "docs/u.md:1 STE-WORD replace \"utilize\" with \"use\"\nste-lint: 1 finding(s) in 1 file(s); 1 file(s) checked\n",
		},
		"a paragraph that holds an added line is reported": {
			base:    map[string]string{"docs/p.md": "We utilize it\nand more\nhere.\n\nGo via it.\n"},
			change:  map[string]string{"docs/p.md": "We utilize it\nand more\nthere.\n\nGo via it.\n"},
			wantOut: "docs/p.md:1 STE-WORD replace \"utilize\" with \"use\"\nste-lint: 1 finding(s) in 1 file(s); 1 file(s) checked\n",
		},
		"a list item beside an added item is not reported": {
			base:    map[string]string{"docs/l.md": "- We utilize it.\n- Clean.\n"},
			change:  map[string]string{"docs/l.md": "- We utilize it.\n- Clean.\n- Fine.\n"},
			wantOut: "ste-lint: 0 finding(s) in 0 file(s); 1 file(s) checked\n",
		},
		"a deleted-only change reports nothing": {
			base:    map[string]string{"docs/d.md": "We utilize it.\n\nGo via it.\n\nClean.\n"},
			change:  map[string]string{"docs/d.md": "We utilize it.\n\nGo via it.\n"},
			wantOut: "ste-lint: 0 finding(s) in 0 file(s); 1 file(s) checked\n",
		},
		"an added table row line is reported on its own": {
			base:    map[string]string{"docs/t.md": "| a | b |\n|---|---|\n| We utilize it | x |\n"},
			change:  map[string]string{"docs/t.md": "| a | b |\n|---|---|\n| We utilize it | x |\n| Go via it | y |\n"},
			wantOut: "docs/t.md:4 STE-WORD replace \"via\" with \"through\"\nste-lint: 1 finding(s) in 1 file(s); 1 file(s) checked\n",
		},
		"a path with a space keeps its added lines": {
			base:    map[string]string{"docs/my doc.md": "Clean.\n"},
			change:  map[string]string{"docs/my doc.md": "Clean.\n\nGo via it.\n"},
			wantOut: "docs/my doc.md:3 STE-WORD replace \"via\" with \"through\"\nste-lint: 1 finding(s) in 1 file(s); 1 file(s) checked\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			repo, base := addedFixture(t, tc.base)
			writeSteFiles(t, repo.Dir, tc.change)
			if tc.commit {
				repo.Git("add", "-A")
				repo.Git("commit", "-q", "-m", "change")
			}

			code, stdout, stderr := runDocsCLI("ste-lint", "--strict", "--changed", base, "--added", "--project-root", repo.Dir)

			wantCode := 0
			if !strings.HasPrefix(tc.wantOut, "ste-lint: 0 ") {
				wantCode = 1
			}
			if code != wantCode || stdout != tc.wantOut {
				t.Fatalf("code %d, stdout:\n%s\nwant code %d, stdout:\n%s\nstderr: %s", code, stdout, wantCode, tc.wantOut, stderr)
			}
		})
	}
}

func TestDocsSteLint_ChangedWithoutAddedStillJudgesTheWholeFile(t *testing.T) {
	repo, base := addedFixture(t, map[string]string{"docs/a.md": "We utilize it.\n"})
	writeSteFiles(t, repo.Dir, map[string]string{"docs/a.md": "We utilize it.\n\nClean.\n"})

	code, stdout, _ := runDocsCLI("ste-lint", "--strict", "--changed", base, "--project-root", repo.Dir)

	want := "docs/a.md:1 STE-WORD replace \"utilize\" with \"use\"\nste-lint: 1 finding(s) in 1 file(s); 1 file(s) checked\n"
	if code != 1 || stdout != want {
		t.Fatalf("code %d, stdout:\n%s\nwant code 1, stdout:\n%s", code, stdout, want)
	}
}

func TestDocsSteLint_AddedWithoutChangedIsAUsageError(t *testing.T) {
	code, _, stderr := runDocsCLI("ste-lint", "--added")

	if code != exitUsage || !strings.Contains(stderr, "--added needs --changed") {
		t.Fatalf("code %d, stderr %q; want %d naming the rule", code, stderr, exitUsage)
	}
}

func TestDocsSteLint_AddedGoModeJudgesOnlyTheAddedLine(t *testing.T) {
	old := "package p\n\nimport \"errors\"\n\nvar e = errors.New(\"we utilize it\")\n"
	repo, base := addedFixture(t, map[string]string{"go/internal/p/a.go": old})
	writeSteFiles(t, repo.Dir, map[string]string{"go/internal/p/a.go": old + "\nvar f = errors.New(\"go via it\")\n"})

	code, stdout, stderr := runDocsCLI("ste-lint", "--go", "--strict", "--changed", base, "--added", "--project-root", repo.Dir)

	want := "go/internal/p/a.go:7 STE-WORD replace \"via\" with \"through\"\nste-lint: 1 finding(s) in 1 file(s); 1 file(s) checked\n"
	if code != 1 || stdout != want {
		t.Fatalf("code %d, stdout:\n%s\nwant:\n%s\nstderr: %s", code, stdout, want, stderr)
	}
}

func TestDocsSteLint_AddedKeepsTheSkipNoticeOfAGeneratedFile(t *testing.T) {
	gen := "<!-- GENERATED by a tool. DO NOT EDIT. -->\n\nWe utilize it.\n"
	repo, base := addedFixture(t, map[string]string{"docs/g.md": gen})
	writeSteFiles(t, repo.Dir, map[string]string{"docs/g.md": gen + "\nMore.\n"})

	code, stdout, _ := runDocsCLI("ste-lint", "--strict", "--json", "--changed", base, "--added", "--project-root", repo.Dir)

	if code != 0 || !strings.Contains(stdout, `"skipped": [`) || strings.Contains(stdout, `"skipped": []`) {
		t.Fatalf("code %d, stdout:\n%s\nwant the skip notice kept", code, stdout)
	}
}

func TestDocsSteLint_AddedWithAnUnknownRefExitsTwo(t *testing.T) {
	repo, _ := addedFixture(t, nil)

	code, _, stderr := runDocsCLI("ste-lint", "--changed", "no-such-ref", "--added", "--project-root", repo.Dir)

	if code != exitIO || !strings.Contains(stderr, "no-such-ref") {
		t.Fatalf("code %d, stderr %q; want %d naming the ref", code, stderr, exitIO)
	}
}

func TestScopeToAddedLines_ADiffTheParserRefusesIsAnError(t *testing.T) {
	repo, base := addedFixture(t, nil)
	original := steParseDiff
	steParseDiff = func([]byte) (covchanged.Diff, error) { return covchanged.Diff{}, errors.New("refused") }
	t.Cleanup(func() { steParseDiff = original })

	_, err := scopeToAddedLines(repo.Dir, base, nil, nil)

	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("err = %v; want the parser's refusal", err)
	}
}

func TestScopeToAddedLines_AnUnknownBaseIsAnError(t *testing.T) {
	repo, _ := addedFixture(t, nil)

	_, err := scopeToAddedLines(repo.Dir, "no-such-ref", nil, nil)

	if err == nil || !strings.Contains(err.Error(), "no-such-ref") {
		t.Fatalf("err = %v; want the git failure naming the base", err)
	}
}

func TestKeepAddedFindings_AnUnreadableFileIsAnError(t *testing.T) {
	target := steTarget{abs: t.TempDir() + "/gone.md", added: map[int]bool{1: true}}

	_, err := keepAddedFindings(target, []stelint.Finding{{Line: 1}})

	if err == nil {
		t.Fatal("keepAddedFindings read a missing file")
	}
}

func TestDocsSteLint_AddedExitsTwoWhenTheSecondReadFails(t *testing.T) {
	repo, base := addedFixture(t, map[string]string{"docs/a.md": "Clean.\n"})
	writeSteFiles(t, repo.Dir, map[string]string{"docs/a.md": "Clean.\n\nGo via it.\n"})
	original := steReadFile
	steReadFile = func(string) ([]byte, error) { return nil, errors.New("read refused") }
	t.Cleanup(func() { steReadFile = original })

	code, _, stderr := runDocsCLI("ste-lint", "--changed", base, "--added", "--project-root", repo.Dir)

	if code != exitIO || !strings.Contains(stderr, "docs/a.md: read refused") {
		t.Fatalf("code %d, stderr %q; want %d naming the file and the failure", code, stderr, exitIO)
	}
}

func TestDocsSteLint_AddedExitsTwoWhenTheDiffIsRefused(t *testing.T) {
	repo, base := addedFixture(t, map[string]string{"docs/a.md": "Clean.\n"})
	writeSteFiles(t, repo.Dir, map[string]string{"docs/a.md": "Clean.\n\nMore.\n"})
	original := steParseDiff
	steParseDiff = func([]byte) (covchanged.Diff, error) { return covchanged.Diff{}, errors.New("refused") }
	t.Cleanup(func() { steParseDiff = original })

	code, _, stderr := runDocsCLI("ste-lint", "--changed", base, "--added", "--project-root", repo.Dir)

	if code != exitIO || !strings.Contains(stderr, "refused") {
		t.Fatalf("code %d, stderr %q; want %d naming the refusal", code, stderr, exitIO)
	}
}

func TestSteBlockSpan(t *testing.T) {
	lines := strings.Split("# Head\n\nfirst\nsecond\n- item one\n  more\n- item two\n\n| a | b |\nlast\n", "\n")
	for name, tc := range map[string]struct {
		line, from, to int
	}{
		"a heading is alone":                     {1, 1, 1},
		"a blank line is alone":                  {2, 2, 2},
		"a paragraph runs to the next list item": {3, 3, 4},
		"a list item runs to its continuation":   {5, 5, 6},
		"a continuation joins its item":          {6, 5, 6},
		"the next item stands alone":             {7, 7, 7},
		"a table row is alone":                   {9, 9, 9},
		"a line after a table row is its own":    {10, 10, 10},
		"a line before the file is itself":       {0, 0, 0},
		"a line past the file is itself":         {99, 99, 99},
	} {
		t.Run(name, func(t *testing.T) {
			from, to := steBlockSpan(lines, tc.line)
			if got, want := []int{from, to}, []int{tc.from, tc.to}; !reflect.DeepEqual(got, want) {
				t.Fatalf("steBlockSpan(%d) = %v, want %v", tc.line, got, want)
			}
		})
	}
}

func FuzzSteBlockSpan(f *testing.F) {
	f.Add("a\n\nb\n- c\n| d |\n# e\n", 3)
	f.Add("", 1)
	f.Add("x", -4)
	f.Fuzz(func(t *testing.T, text string, line int) {
		lines := strings.Split(text, "\n")
		from, to := steBlockSpan(lines, line)
		if from > to || from > line || to < line {
			t.Fatalf("steBlockSpan(%q, %d) = %d, %d; want a span that holds the line", text, line, from, to)
		}
		if line >= 1 && line <= len(lines) && (from < 1 || to > len(lines)) {
			t.Fatalf("steBlockSpan(%q, %d) = %d, %d; want a span inside the file", text, line, from, to)
		}
	})
}

func TestKeepAddedFindings_AFileLevelFindingIsKeptWhateverTheAddedLines(t *testing.T) {
	path := t.TempDir() + "/a.md"
	if err := os.WriteFile(path, []byte("Old.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := steTarget{abs: path, added: map[int]bool{}}

	kept, err := keepAddedFindings(target, []stelint.Finding{{Line: 0, Rule: "file-level"}})

	if err != nil || len(kept) != 1 {
		t.Fatalf("kept %v, err %v; want the file-level finding kept", kept, err)
	}
}
