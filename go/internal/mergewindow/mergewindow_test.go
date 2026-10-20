package mergewindow

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func requiredWorkflow(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "..", ".github", "workflows", "required.yml"))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func liveRule(t *testing.T) Rule {
	t.Helper()
	r, err := Parse(requiredWorkflow(t))
	if err != nil {
		t.Fatalf("Parse(required.yml): %v", err)
	}
	return r
}

func TestClassify_TheLiveCIRoutingTableDecidesEachPath(t *testing.T) {
	r := liveRule(t)
	cases := map[string]Class{
		"docs/plans/pr-merge-latency-2026-10.md":    Window,
		"docs/research-index.md":                    Window,
		"docs/research/wave-latency-2026-10.md":     Window,
		"docs/reports/x.md":                         Window,
		"docs/architecture/adr/0148-x.md":           Window,
		"README.md":                                 Window,
		".evolve/inbox/2026-10-20T00-00-00Z-x.json": Window,
		".evolve/inbox/consumed/x.json":             Window,
		".github/workflows/go.yml":                  Window,
		".github/CODEOWNERS":                        Window,
		"docs/explain/builds/x.md":                  Boundary,
		"docs/conventions/ste100-writing.md":        Boundary,
		"docs/architecture/diagram.png":             Boundary,
		"go/internal/mergewindow/mergewindow.go":    Boundary,
		"go/.cover-strict":                          Boundary,
		".evolve/policy.json":                       Boundary,
		"skills/docs-write/SKILL.md":                Boundary,
		"agents/evolve-builder.md":                  Boundary,
		"landing/index.html":                        Boundary,
		"Makefile":                                  Boundary,
		"docs":                                      Boundary,
		".github":                                   Boundary,
		"docs/a.md .github/x":                       Boundary,
		"docs/a\nb.md":                              Window,
	}
	for path, want := range cases {
		if got := r.Classify([]string{path}); got.Class != want {
			t.Errorf("Classify(%q) = %+v, want %s", path, got, want)
		}
	}
}

func TestClassify_OneBoundaryPathMakesThePRBoundaryAndIsNamed(t *testing.T) {
	r := liveRule(t)
	got := r.Classify([]string{"docs/plans/a.md", "go/x.go", ".evolve/inbox/a.json", "Makefile", "go/x.go"})
	want := Verdict{Class: Boundary, BoundaryPaths: []string{"docs/plans/a.md", "go/x.go", "Makefile"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify = %+v, want %+v", got, want)
	}
	if got := r.Classify([]string{"docs/plans/a.md", "README.md", ".github/workflows/required.yml"}); got.Class != Window || len(got.BoundaryPaths) != 0 {
		t.Fatalf("a docs and CI PR = %+v, want window", got)
	}
	if got := r.Classify([]string{".evolve/inbox/a.json", "docs/research/x.md", ".github/CODEOWNERS"}); got.Class != Window {
		t.Fatalf("an inbox, research and CI PR = %+v, want window", got)
	}
}

func TestClassify_ADocsAndInboxPRIsBoundaryBecauseCIRunsTheGoSuiteForIt(t *testing.T) {
	got := liveRule(t).Classify([]string{"README.md", ".evolve/inbox/x.json"})
	want := Verdict{Class: Boundary, BoundaryPaths: []string{"README.md"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify = %+v, want %+v", got, want)
	}
}

func TestClassify_AnEmptyChangeIsBoundaryAsCIRoutesItToEverySuite(t *testing.T) {
	if got := liveRule(t).Classify(nil); got.Class != Boundary || len(got.BoundaryPaths) != 0 {
		t.Fatalf("Classify(nil) = %+v, want boundary with no named path", got)
	}
}

func liveText(t *testing.T) string {
	t.Helper()
	return string(requiredWorkflow(t))
}

func mutate(t *testing.T, old, replacement string) []byte {
	t.Helper()
	text := liveText(t)
	if !strings.Contains(text, old) {
		t.Fatalf("required.yml has no %q", old)
	}
	return []byte(strings.Replace(text, old, replacement, 1))
}

const loopStart = "while IFS= read -r -d '' path; do\n"
const loopEnd = "          done < \"$RUNNER_TEMP/ci-paths\""

func table(t *testing.T, classes, suites string) []byte {
	t.Helper()
	text := liveText(t)
	head := text[:strings.Index(text, loopStart)+len(loopStart)]
	tail := text[strings.Index(text, loopEnd):]
	return []byte(head + "case \"$path\" in\n" + classes + "esac\ncase \"$path\" in\n" + suites + "esac\n" + tail)
}

const goodClasses = "    docs/*.md) docs_found=true ;;\n    *) other_found=true ;;\n"
const goodSuites = "    go/*) go_required=true ;;\n    .evolve/inbox/*) inbox_found=true ;;\n    *) go_required=true; landing_required=true ;;\n"

func TestParse_ReadsASmallTableAndMatchesStarAcrossSlashes(t *testing.T) {
	r, err := Parse(table(t, goodClasses, goodSuites))
	if err != nil {
		t.Fatal(err)
	}
	for _, paths := range [][]string{{"docs/a/b/c.md"}, {".evolve/inbox/a.json"}} {
		if got := r.Classify(paths); got.Class != Window {
			t.Fatalf("%q = %+v, want window", paths, got)
		}
	}
	if got := r.Classify([]string{"docs/a.txt"}); got.Class != Boundary {
		t.Fatalf("docs/a.txt = %+v, want boundary", got)
	}
}

func TestParse_FailsClosedOnATableItCannotRead(t *testing.T) {
	cases := map[string]struct {
		body []byte
		want string
	}{
		"no changes job":     {mutate(t, "\n  changes:\n", "\n  paths:\n"), "no job \"changes\""},
		"no loop":            {mutate(t, loopStart, "while read path; do\n"), "want one path loop"},
		"two loops":          {mutate(t, loopStart, loopStart+loopStart), "found 2"},
		"no end of loop":     {mutate(t, loopEnd, "          finished"), "no path loop end"},
		"open third block":   {mutate(t, loopEnd, "case \"$path\" in\n"+loopEnd), "2 case blocks"},
		"if inside loop":     {mutate(t, loopEnd, "if [[ \"$path\" == .evolve/inbox/* ]]; then go_required=true; fi\n"+loopEnd), "unparsed line"},
		"unquoted block":     {mutate(t, loopEnd, "case $path in\n.evolve/inbox/*) go_required=true ;;\nesac\n"+loopEnd), "unparsed line"},
		"flag after loop":    {mutate(t, "route false false true true", "if [[ \"$inbox_found\" == true ]]; then go_required=true; fi\nroute false false true true"), "routing flag go_required"},
		"route after loop":   {mutate(t, "route false false true true", "route true true false false true\nroute false false true true"), "after the path loop changed"},
		"flag before loop":   {mutate(t, "go_required=false", "go_required=true"), "routing flag go_required"},
		"exit before loop":   {mutate(t, "go_required=false", "exit 0\ngo_required=false"), "before the path loop changed"},
		"no run block":       {mutate(t, "run: |", "run: >"), "before the path loop changed"},
		"output forced":      {mutate(t, "go: ${{ steps.paths.outputs.go }}", "go: 'true'"), "before the path loop changed"},
		"env override":       {mutate(t, "HEAD_SHA: ${{ github.event.pull_request.head.sha || github.sha }}", "HEAD_SHA: ${{ github.sha }}"), "before the path loop changed"},
		"step condition":     {mutate(t, "name: Select required suites", "name: Select required suites\n        if: false"), "before the path loop changed"},
		"step before script": {mutate(t, "- id: paths", "- run: echo go=true >> $GITHUB_OUTPUT\n      - id: paths"), "before the path loop changed"},
		"go job always":      {mutate(t, "if: needs.changes.outputs.go == 'true'", "if: always()"), "gate of job go"},
		"landing job reads":  {mutate(t, "if: needs.changes.outputs.landing == 'true'", "if: needs.changes.outputs.docs == 'true'"), "gate of job landing"},
		"no go job":          {mutate(t, "\n  go:\n", "\n  go-suite:\n"), "no job \"go\""},
		"digit flag":         {mutate(t, "route false false true true", "go2_required=true\nroute false false true true"), "routing flag go2_required"},
		"route before loop":  {mutate(t, "go_required=false", "if grep -qz inbox \"$RUNNER_TEMP/ci-paths\"; then route true true; fi\ngo_required=false"), "before the path loop changed"},
		"empty block":        {table(t, "", goodSuites), "no catch-all"},
		"no catch-all":       {table(t, "    docs/*.md) docs_found=true ;;\n", goodSuites), "no catch-all"},
		"bad arm":            {table(t, "    docs/*.md) docs_found=true\n"+goodClasses, goodSuites), "unreadable arm"},
		"class glob":         {table(t, "    docs/?.md) docs_found=true ;;\n"+goodClasses, goodSuites), "unsupported glob"},
		"quoted glob":        {table(t, "    \"docs\"/*.md) docs_found=true ;;\n"+goodClasses, goodSuites), "unsupported glob"},
		"empty alternative":  {table(t, "    docs/*.md|) docs_found=true ;;\n"+goodClasses, goodSuites), "unsupported glob"},
		"false action":       {table(t, "    docs/*.md) docs_found=false ;;\n"+goodClasses, goodSuites), "unreadable action"},
		"renamed class flag": {table(t, "    docs/*.md) docs_seen=true ;;\n"+goodClasses, goodSuites), "unknown flag docs_seen"},
		"renamed suite flag": {table(t, goodClasses, "    go/*) go_needed=true ;;\n"+goodSuites), "unknown flag go_needed"},
		"no docs flag":       {table(t, "    *) other_found=true ;;\n", goodSuites), "never sets docs_found"},
		"no go flag":         {table(t, goodClasses, "    *) landing_required=true ;;\n"), "must set go_required"},
		"no landing flag":    {table(t, goodClasses, "    *) go_required=true ;;\n"), "must set landing_required"},
		"no other flag":      {table(t, "    docs/*.md) docs_found=true ;;\n    *) ;;\n", goodSuites), "must set other_found"},
		"early catch-all":    {table(t, "    *) other_found=true ;;\n"+goodClasses, goodSuites), "hides arm 2"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(c.body)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("Parse = %v, want an error with %q", err, c.want)
			}
		})
	}
}
