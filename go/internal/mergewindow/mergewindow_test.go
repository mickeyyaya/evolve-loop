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
	want := Verdict{Class: Boundary, BoundaryPaths: []string{"go/x.go", "Makefile"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify = %+v, want %+v", got, want)
	}
	if got := r.Classify([]string{"docs/plans/a.md", ".evolve/inbox/a.json", ".github/workflows/required.yml"}); got.Class != Window || len(got.BoundaryPaths) != 0 {
		t.Fatalf("a docs, inbox and CI PR = %+v, want window", got)
	}
}

func TestClassify_AnEmptyChangeIsBoundaryAsCIRoutesItToEverySuite(t *testing.T) {
	if got := liveRule(t).Classify(nil); got.Class != Boundary || len(got.BoundaryPaths) != 0 {
		t.Fatalf("Classify(nil) = %+v, want boundary with no named path", got)
	}
}

const loop = "while IFS= read -r -d '' path; do\n"
const done = "done < \"$RUNNER_TEMP/ci-paths\"\n"

func table(classes, suites string) []byte {
	return []byte("steps:\n" + loop +
		"  case \"$path\" in\n" + classes + "  esac\n" +
		"  case \"$path\" in\n" + suites + "  esac\n" + done +
		"  case \"$x\" in\n    *) other_found=true ;;\n  esac\n")
}

const goodClasses = "    docs/*.md) docs_found=true ;;\n    *) other_found=true ;;\n"
const goodSuites = "    go/*) go_required=true ;;\n    .evolve/inbox/*) inbox_found=true ;;\n    *) go_required=true; landing_required=true ;;\n"

func TestParse_ReadsASmallTableAndMatchesStarAcrossSlashes(t *testing.T) {
	r, err := Parse(table(goodClasses, goodSuites))
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Classify([]string{"docs/a/b/c.md", ".evolve/inbox/a.json"}); got.Class != Window {
		t.Fatalf("nested docs and inbox = %+v, want window", got)
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
		"no loop":            {[]byte("jobs: {}\n"), "want one path loop"},
		"two loops":          {append([]byte(loop+done), table(goodClasses, goodSuites)...), "found 2"},
		"no end of loop":     {[]byte(loop + "  case \"$path\" in\n    *) other_found=true ;;\n  esac\n"), "no path loop"},
		"one block":          {[]byte(loop + "  case \"$path\" in\n" + goodClasses + "  esac\n" + done), "2 case blocks"},
		"open block":         {[]byte(loop + "  case \"$path\" in\n" + goodClasses + done), "2 case blocks"},
		"empty block":        {table("", goodSuites), "no catch-all"},
		"no catch-all":       {table("    docs/*.md) docs_found=true ;;\n", goodSuites), "no catch-all"},
		"bad arm":            {table("    docs/*.md) docs_found=true\n"+goodClasses, goodSuites), "unreadable arm"},
		"class glob":         {table("    docs/?.md) docs_found=true ;;\n"+goodClasses, goodSuites), "unsupported glob"},
		"quoted glob":        {table("    \"docs\"/*.md) docs_found=true ;;\n"+goodClasses, goodSuites), "unsupported glob"},
		"empty alternative":  {table("    docs/*.md|) docs_found=true ;;\n"+goodClasses, goodSuites), "unsupported glob"},
		"false action":       {table("    docs/*.md) docs_found=false ;;\n"+goodClasses, goodSuites), "unreadable action"},
		"renamed class flag": {table("    docs/*.md) docs_seen=true ;;\n"+goodClasses, goodSuites), "unknown flag docs_seen"},
		"renamed suite flag": {table(goodClasses, "    go/*) go_needed=true ;;\n"+goodSuites), "unknown flag go_needed"},
		"no docs flag":       {table("    *) other_found=true ;;\n", goodSuites), "never sets docs_found"},
		"no go flag":         {table(goodClasses, "    *) landing_required=true ;;\n"), "must set go_required"},
		"no landing flag":    {table(goodClasses, "    *) go_required=true ;;\n"), "must set landing_required"},
		"no other flag":      {table("    docs/*.md) docs_found=true ;;\n    *) ;;\n", goodSuites), "must set other_found"},
		"early catch-all":    {table("    *) other_found=true ;;\n"+goodClasses, goodSuites), "hides arm 2"},
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
