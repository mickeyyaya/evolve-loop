package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

func steFloorFixture(t *testing.T, changed map[string]string) ReviewInput {
	t.Helper()
	standard, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(stelint.StandardPath)))
	if err != nil {
		t.Fatalf("read the standard: %v", err)
	}
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, filepath.FromSlash(stelint.StandardPath)), string(standard))
	writeFile(t, filepath.Join(repo.Dir, "docs", "old.md"), "We utilize the old text.\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	for path, body := range changed {
		writeFile(t, filepath.Join(repo.Dir, filepath.FromSlash(path)), body)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "change")
	return ReviewInput{Phase: string(PhaseBuild), ProjectRoot: t.TempDir(), Worktree: repo.Dir, WorktreeBaseSHA: base}
}

func longSentence(words int) string {
	return strings.TrimSpace(strings.Repeat("word ", words)) + "."
}

func TestDefaultBuildFloorChecks_WarnsOnSteFindingsInTheChangedDocs(t *testing.T) {
	in := steFloorFixture(t, map[string]string{
		"docs/new.md":   "# New\n\nIntro.\n\n" + longSentence(30) + "\n",
		"README.md":     "Go via the bridge.\n",
		"skills/x.md":   "We utilize it. " + longSentence(40) + "\n",
		"docs/notes.md": "Short and clean.\n",
	})

	var failures []string
	got := captureStderr(t, func() { failures = DefaultBuildFloorChecks(context.Background(), in) })

	want := "[ste-lint] WARN: 2 finding(s) in 2 file(s) (first: README.md:1 STE-WORD)\n"
	if !strings.Contains(got, want) {
		t.Fatalf("stderr does not carry %q:\n%s", want, got)
	}
	if strings.Count(got, "[ste-lint]") != 1 {
		t.Errorf("the floor must print one ste-lint line:\n%s", got)
	}
	for _, f := range failures {
		if strings.Contains(f, "ste") || strings.Contains(f, "STE") {
			t.Errorf("the STE lint must never fail the handoff: %q", f)
		}
	}
}

func TestDocsFloorWarn_NamesTheFirstFindingOfTheChangedDocs(t *testing.T) {
	in := steFloorFixture(t, map[string]string{"docs/new.md": "# New\n\nIntro.\n\n" + longSentence(30) + "\n"})

	got := captureStderr(t, func() { docsFloorWarn(in, changedFloorPaths(context.Background(), in)) })

	if want := "[ste-lint] WARN: 1 finding(s) in 1 file(s) (first: docs/new.md:5 STE-SENTENCE)\n"; !strings.Contains(got, want) {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestDocsFloorWarn_SteStageOffPrintsNothing(t *testing.T) {
	in := steFloorFixture(t, map[string]string{"docs/new.md": longSentence(30) + "\n"})
	writeFile(t, filepath.Join(in.ProjectRoot, ".evolve", "policy.json"), `{"docs_floor":{"ste_stage":"off"}}`)

	got := captureStderr(t, func() { docsFloorWarn(in, changedFloorPaths(context.Background(), in)) })

	if strings.Contains(got, "[ste-lint]") {
		t.Fatalf("ste_stage off must print nothing, got %q", got)
	}
}

func TestDocsFloorWarn_EveryOtherStageOnlyWarns(t *testing.T) {
	for _, stage := range []string{"shadow", "enforce"} {
		t.Run(stage, func(t *testing.T) {
			in := steFloorFixture(t, map[string]string{"docs/new.md": longSentence(30) + "\n"})
			writeFile(t, filepath.Join(in.ProjectRoot, ".evolve", "policy.json"), `{"docs_floor":{"ste_stage":"`+stage+`"}}`)

			got := captureStderr(t, func() { docsFloorWarn(in, changedFloorPaths(context.Background(), in)) })

			if !strings.Contains(got, "[ste-lint] WARN: 1 finding(s) in 1 file(s) (first: docs/new.md:1 STE-SENTENCE)") {
				t.Fatalf("stage %s: stderr = %q, want the WARN line", stage, got)
			}
		})
	}
}

func TestDocsFloorWarn_CleanOrOutOfScopeDocsPrintNothing(t *testing.T) {
	for name, changed := range map[string]map[string]string{
		"a clean doc":          {"docs/new.md": "Short and clean.\n"},
		"an out-of-scope doc":  {"skills/x/SKILL.md": "We utilize it.\n", "agents/a.md": longSentence(40) + "\n"},
		"an unchanged bad doc": {"go/x.txt": "x\n"},
		"a deleted bad doc":    {"go/y.txt": "y\n"},
	} {
		t.Run(name, func(t *testing.T) {
			in := steFloorFixture(t, changed)
			paths := changedFloorPaths(context.Background(), in)
			if name == "a deleted bad doc" {
				paths = append(paths, "docs/gone.md")
			}

			got := captureStderr(t, func() { docsFloorWarn(in, paths) })

			if strings.Contains(got, "[ste-lint]") {
				t.Fatalf("stderr = %q, want no ste-lint line", got)
			}
		})
	}
}

func TestDocsFloorWarn_AWorktreeWithoutTheStandardIsSilent(t *testing.T) {
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "docs", "a.md"), "x\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "docs", "new.md"), longSentence(30)+"\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "change")
	in := ReviewInput{Phase: string(PhaseBuild), ProjectRoot: t.TempDir(), Worktree: repo.Dir, WorktreeBaseSHA: base}

	got := captureStderr(t, func() { docsFloorWarn(in, changedFloorPaths(context.Background(), in)) })

	if strings.Contains(got, "[ste-lint]") {
		t.Fatalf("a project without the standard has no STE rule; stderr = %q", got)
	}
}

func TestDocsFloorWarn_ABrokenStandardWarnsThatTheLintDidNotRun(t *testing.T) {
	in := steFloorFixture(t, map[string]string{
		stelint.StandardPath: "# The standard\n\nNo word table.\n",
		"docs/new.md":        longSentence(30) + "\n",
	})

	got := captureStderr(t, func() { docsFloorWarn(in, changedFloorPaths(context.Background(), in)) })

	if !strings.Contains(got, "[ste-lint] WARN:") || !strings.Contains(got, "did not run") {
		t.Fatalf("a broken standard must WARN that the lint did not run; stderr = %q", got)
	}
}

func TestDocsFloorWarn_AChangedGeneratedDocIsNotAFinding(t *testing.T) {
	in := steFloorFixture(t, map[string]string{"docs/gen.md": "<!-- Code generated by x. DO NOT EDIT. -->\n\n" + longSentence(30) + "\n"})

	got := captureStderr(t, func() { docsFloorWarn(in, changedFloorPaths(context.Background(), in)) })

	if strings.Contains(got, "[ste-lint]") {
		t.Fatalf("a generated doc is not a finding; stderr = %q", got)
	}
}

func TestSteLintFloorLines_ABrokenStandardIsSilentWhenNoDocChanged(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, filepath.FromSlash(stelint.StandardPath)), "# Broken\n")

	if got := steLintFloorLines(dir, []string{"go/x.go"}); len(got) != 0 {
		t.Fatalf("no doc changed, so the floor must not read the standard; got %q", got)
	}
}

func TestDocsFloorWarn_NamesADocThatItCannotRead(t *testing.T) {
	in := steFloorFixture(t, map[string]string{"docs/new.md": longSentence(30) + "\n"})
	if err := os.MkdirAll(filepath.Join(in.Worktree, "docs", "dir.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := captureStderr(t, func() {
		docsFloorWarn(in, append(changedFloorPaths(context.Background(), in), "docs/dir.md"))
	})

	if !strings.Contains(got, "[ste-lint] WARN: docs/dir.md was not checked:") || !strings.Contains(got, "[ste-lint] WARN: 1 finding(s) in 1 file(s) (first: docs/new.md:1 STE-SENTENCE)") {
		t.Fatalf("stderr = %q, want the unread doc named beside the findings line", got)
	}
}
