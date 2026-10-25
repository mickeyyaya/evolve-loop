package core

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

func TestDefaultBuildFloorChecks_AtEnforceAFailingDocGoesBackToTheBuild(t *testing.T) {
	t.Parallel()
	in := steFloorFixture(t, map[string]string{"docs/new.md": "# New\n\nIntro.\n\n" + longSentence(30) + "\n"})
	writeFile(t, filepath.Join(in.ProjectRoot, ".evolve", "policy.json"), `{"docs_floor":{"ste_stage":"enforce"}}`)
	var sink injectedStderr
	in.Stderr = &sink

	failures := DefaultBuildFloorChecks(context.Background(), in)

	var ste []string
	for _, f := range failures {
		if strings.Contains(f, "[ste-lint]") {
			ste = append(ste, f)
		}
	}
	if len(ste) != 1 || !strings.Contains(ste[0], "1 finding(s) in 1 file(s) (first: docs/new.md:5 STE-SENTENCE)") || !strings.Contains(ste[0], "evolve docs ste-lint --strict") {
		t.Fatalf("floor failures = %q, want one ste-lint failure that names the first finding and the command", failures)
	}
	if strings.Contains(sink.String(), "[ste-lint] WARN: 1 finding") {
		t.Errorf("at enforce the finding is a failure, not a WARN: %q", sink.String())
	}
}

func TestDefaultBuildFloorChecks_AtEnforceACleanDocPasses(t *testing.T) {
	t.Parallel()
	in := steFloorFixture(t, map[string]string{"docs/new.md": "Short and clean.\n"})
	writeFile(t, filepath.Join(in.ProjectRoot, ".evolve", "policy.json"), `{"docs_floor":{"ste_stage":"enforce"}}`)
	var sink injectedStderr
	in.Stderr = &sink

	for _, f := range DefaultBuildFloorChecks(context.Background(), in) {
		if strings.Contains(f, "[ste-lint]") {
			t.Fatalf("a clean doc must pass the enforced floor: %q", f)
		}
	}
}

func TestDefaultBuildFloorChecks_AtEnforceABrokenStandardSendsTheDocBack(t *testing.T) {
	t.Parallel()
	in := steFloorFixture(t, map[string]string{stelint.StandardPath: "# The standard\n\nNo word table.\n", "docs/new.md": longSentence(30) + "\n"})
	writeFile(t, filepath.Join(in.ProjectRoot, ".evolve", "policy.json"), `{"docs_floor":{"ste_stage":"enforce"}}`)
	var sink injectedStderr
	in.Stderr = &sink

	var ste []string
	for _, f := range DefaultBuildFloorChecks(context.Background(), in) {
		if strings.Contains(f, "[ste-lint]") {
			ste = append(ste, f)
		}
	}
	if len(ste) != 1 || !strings.Contains(ste[0], "does not load") {
		t.Fatalf("floor failures = %q: at enforce a lane that breaks the standard cannot pass its docs unjudged", ste)
	}
}

func TestDocsFloorChecks_APolicyThatDoesNotLoadWarnsAndFailsClosed(t *testing.T) {
	t.Parallel()
	in := steFloorFixture(t, map[string]string{"docs/new.md": longSentence(30) + "\n"})
	writeFile(t, filepath.Join(in.ProjectRoot, ".evolve", "policy.json"), `{"docs_floor":{"ste_stage":"shadow","bogus":1}}`)
	var sink injectedStderr
	in.Stderr = &sink

	failures := docsFloorChecks(context.Background(), in, changedFloorPaths(context.Background(), in))

	if !strings.Contains(sink.String(), `[policy] WARN`) || !strings.Contains(sink.String(), `"docs_floor.bogus"`) {
		t.Errorf("stderr = %q, want a WARN that names the refused key", sink.String())
	}
	if len(failures) != 1 || !strings.Contains(failures[0], "[ste-lint]") {
		t.Fatalf("failures = %q: a refused policy runs the STE floor at enforce, never at the shadow stage it asked for", failures)
	}
}

func TestDefaultBuildFloorChecks_AtEnforceAnUnreadableDocWarnsBesideTheFailure(t *testing.T) {
	t.Parallel()
	in := steFloorFixture(t, map[string]string{"docs/new.md": longSentence(30) + "\n"})
	if err := os.MkdirAll(filepath.Join(in.Worktree, "docs", "dir.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(in.ProjectRoot, ".evolve", "policy.json"), `{"docs_floor":{"ste_stage":"enforce"}}`)
	var sink injectedStderr
	in.Stderr = &sink

	failures := docsFloorChecks(context.Background(), in, append(changedFloorPaths(context.Background(), in), "docs/dir.md"))

	if len(failures) != 1 || !strings.Contains(failures[0], "docs/new.md:1 STE-SENTENCE") {
		t.Fatalf("failures = %q, want the one finding of docs/new.md", failures)
	}
	if !strings.Contains(sink.String(), "[ste-lint] WARN: docs/dir.md was not checked") {
		t.Errorf("stderr = %q, want a WARN that names the unread doc", sink.String())
	}
}

func steExistingDocFixture(t *testing.T, addition string) ReviewInput {
	t.Helper()
	in := steFloorFixture(t, map[string]string{"go/x.txt": "x\n"})
	writeFile(t, filepath.Join(in.Worktree, "docs", "old.md"), "We utilize the old text.\n\n"+addition+"\n")
	writeFile(t, filepath.Join(in.ProjectRoot, ".evolve", "policy.json"), `{"docs_floor":{"ste_stage":"enforce"}}`)
	return in
}

func TestDefaultBuildFloorChecks_AtEnforceOnlyTheAddedLinesOfADocCount(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		addition string
		fails    bool
	}{
		"a clean addition to a doc with old findings": {"Short and clean.", false},
		"a failing addition":                          {longSentence(30), true},
	} {
		t.Run(name, func(t *testing.T) {
			in := steExistingDocFixture(t, tc.addition)
			var sink injectedStderr
			in.Stderr = &sink

			var ste []string
			for _, f := range DefaultBuildFloorChecks(context.Background(), in) {
				if strings.Contains(f, "[ste-lint]") {
					ste = append(ste, f)
				}
			}
			if (len(ste) == 1) != tc.fails {
				t.Fatalf("ste failures = %q, want a failure: %v; CI lints only the added lines, and so does the floor", ste, tc.fails)
			}
		})
	}
}

func TestSteAddedKeep_EachGitOrReadFailureIsAnError(t *testing.T) {
	t.Parallel()
	findings := []stelint.Finding{{Line: 1, Rule: "STE-WORD"}}
	for name, tc := range map[string]struct {
		run      func(args []string, stdout io.Writer) (int, error)
		worktree string
		want     string
		kept     int
	}{
		"git cannot start":              {func([]string, io.Writer) (int, error) { return -1, errors.New("no git") }, "", "no git", 0},
		"a new doc keeps every finding": {func([]string, io.Writer) (int, error) { return 128, nil }, "", "", 1},
		"the diff fails": {func(args []string, _ io.Writer) (int, error) {
			if args[0] == "cat-file" {
				return 0, nil
			}
			return 1, nil
		}, "", "exit 1", 0},
		"the diff does not parse": {func(args []string, out io.Writer) (int, error) {
			if args[0] != "cat-file" {
				_, _ = io.WriteString(out, "@@ garbage\n")
			}
			return 0, nil
		}, "", "covchanged", 0},
		"the doc cannot be read": {func([]string, io.Writer) (int, error) { return 0, nil }, "/no/such/worktree", "no such file", 0},
	} {
		t.Run(name, func(t *testing.T) {
			git := gitexec.Git{Dir: "/w", Exec: func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
				return tc.run(args, stdout)
			}}
			got, err := steAddedKeep(context.Background(), git, tc.worktree, "base")("docs/a.md", findings)
			if (tc.want == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), tc.want)) || len(got) != tc.kept {
				t.Fatalf("kept %v, err %v; want %d kept and an error holding %q", got, err, tc.kept, tc.want)
			}
		})
	}
}

func TestSteFindingLines_AScopeThatCannotBeReadJudgesTheWholeFile(t *testing.T) {
	t.Parallel()
	in := steFloorFixture(t, map[string]string{"docs/new.md": longSentence(30) + "\n"})
	words, err := stelint.LoadStandard(in.Worktree)
	if err != nil {
		t.Fatal(err)
	}
	failing := func(string, []stelint.Finding) ([]stelint.Finding, error) { return nil, errors.New("git gone") }

	lines, finding := steFindingLines(in.Worktree, []string{"docs/new.md"}, stelint.Options{Words: words}, failing)

	if !strings.Contains(finding, "docs/new.md:1 STE-SENTENCE") || len(lines) != 1 || !strings.Contains(lines[0], "judges the whole file") {
		t.Fatalf("finding = %q, lines = %q: an unknown scope must not pass a doc CI can still refuse", finding, lines)
	}
}
