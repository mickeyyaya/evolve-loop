package stelint_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

const repoRoot = "../../.."

func TestParseWordTable_ReadsTheRealStandard(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(stelint.StandardPath)))
	if err != nil {
		t.Fatalf("read the standard: %v", err)
	}

	got, err := stelint.ParseWordTable(raw)

	if err != nil {
		t.Fatalf("ParseWordTable(the standard): %v", err)
	}
	want := map[string]string{"utilize": "use", "in order to": "to", "via": "through, with, by", "e.g.": "for example", "etc.": "(list all the items)"}
	found := map[string]string{}
	for _, s := range got {
		found[s.Phrase] = s.Replacement
	}
	for phrase, replacement := range want {
		if found[phrase] != replacement {
			t.Errorf("phrase %q -> %q, want %q", phrase, found[phrase], replacement)
		}
	}
	if len(got) < 30 {
		t.Errorf("parsed %d rows, want every row of the house list (at least 30)", len(got))
	}
}

func TestParseWordTable_ReadsOnlyTheTableUnderItsHeading(t *testing.T) {
	doc := "# Std\n\n| Do not write | Write |\n|---|---|\n| other | x |\n\n## " + stelint.WordTableHeading +
		"\n\nThe lint reads this table.\n\n| Do not write | Write |\n|---|---|\n| utilize | use |\n| `in order to` | to |\n\n## Next\n\n| a | b |\n|---|---|\n| later | y |\n"

	got, err := stelint.ParseWordTable([]byte(doc))

	if err != nil {
		t.Fatal(err)
	}
	want := []stelint.Substitution{{Phrase: "utilize", Replacement: "use"}, {Phrase: "in order to", Replacement: "to"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("ParseWordTable = %+v, want %+v", got, want)
	}
}

func TestParseWordTable_RefusesAStandardWithoutTheTable(t *testing.T) {
	for name, doc := range map[string]string{
		"no section":           "# Std\n\n| Do not write | Write |\n|---|---|\n| utilize | use |\n",
		"a section, no table":  "## " + stelint.WordTableHeading + "\n\nNo table here.\n\n## Next\n\n| a | b |\n|---|---|\n| x | y |\n",
		"a table with no rows": "## " + stelint.WordTableHeading + "\n\n| Do not write | Write |\n|---|---|\n",
		"a row with one cell":  "## " + stelint.WordTableHeading + "\n\n| Do not write | Write |\n|---|---|\n| utilize |\n",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := stelint.ParseWordTable([]byte(doc))
			if err == nil {
				t.Fatalf("ParseWordTable = %+v, want an error", got)
			}
			if !strings.Contains(err.Error(), stelint.WordTableHeading) {
				t.Errorf("error %q must name the section", err)
			}
		})
	}
}

func TestInDocsScope_IsTheDocsTreeAndTheFourRootFiles(t *testing.T) {
	for path, want := range map[string]bool{
		"docs/a.md":              true,
		"docs/x/y/z.md":          true,
		"./docs/a.md":            true,
		"README.md":              true,
		"CLAUDE.md":              true,
		"AGENTS.md":              true,
		"CHANGELOG.md":           true,
		"docs/a.txt":             false,
		"docs.md":                false,
		"go/README.md":           false,
		"skills/x/SKILL.md":      false,
		"agents/evolve-build.md": false,
		"other/README.md":        false,
		"skills/docs/a.md":       false,
	} {
		if got := stelint.InDocsScope(path); got != want {
			t.Errorf("InDocsScope(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestInGoScope_IsProductionGoOutsideTheACSTree(t *testing.T) {
	for path, want := range map[string]bool{
		"go/internal/x/a.go":            true,
		"go/cmd/evolve/main.go":         true,
		"./go/pkg/v/v.go":               true,
		"go/internal/x/a_test.go":       false,
		"go/acs/regression/x/a.go":      false,
		"go/internal/x/testdata/a.go":   false,
		"docs/a.go":                     false,
		"go/internal/x/README.md":       false,
		"go/internal/acs/notacsroot.go": true,
	} {
		if got := stelint.InGoScope(path); got != want {
			t.Errorf("InGoScope(%q) = %v, want %v", path, got, want)
		}
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadStandard_ReadsTheTableUnderTheRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, filepath.FromSlash(stelint.StandardPath)), "## "+stelint.WordTableHeading+"\n\n| a | b |\n|---|---|\n| leverage | use |\n")

	got, err := stelint.LoadStandard(root)

	if err != nil || len(got) != 1 || got[0] != (stelint.Substitution{Phrase: "leverage", Replacement: "use"}) {
		t.Fatalf("LoadStandard = %+v, %v", got, err)
	}
}

func TestLoadStandard_AMissingStandardIsNotExist(t *testing.T) {
	_, err := stelint.LoadStandard(t.TempDir())
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want fs.ErrNotExist", err)
	}
}

func TestLoadStandard_ABrokenStandardIsAnError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, filepath.FromSlash(stelint.StandardPath)), "# no table\n")
	if _, err := stelint.LoadStandard(root); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want a parse error", err)
	}
}

func TestLintFile_ChecksByExtension(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "a.md")
	src := filepath.Join(dir, "a.go")
	writeFile(t, md, "We utilize it.\n")
	writeFile(t, src, "package a\n\nimport \"errors\"\n\nvar e = errors.New(\"we utilize it\")\n")
	opt := stelint.Options{Words: houseWords}

	mdFindings, mdErr := stelint.LintFile(md, opt)
	goFindings, goErr := stelint.LintFile(src, opt)

	if mdErr != nil || len(mdFindings) != 1 || mdFindings[0].Line != 1 || mdFindings[0].Rule != stelint.RuleWord {
		t.Errorf("LintFile(md) = %+v, %v", mdFindings, mdErr)
	}
	if goErr != nil || len(goFindings) != 1 || goFindings[0].Line != 5 || goFindings[0].Rule != stelint.RuleWord {
		t.Errorf("LintFile(go) = %+v, %v", goFindings, goErr)
	}
}

func TestLintFile_ExemptsTheWordTableOnlyInTheStandard(t *testing.T) {
	root := t.TempDir()
	doc := "## " + stelint.WordTableHeading + "\n\n| a | b |\n|---|---|\n| utilize | use |\n"
	standard := filepath.Join(root, filepath.FromSlash(stelint.StandardPath))
	other := filepath.Join(root, "docs", "other.md")
	writeFile(t, standard, doc)
	writeFile(t, other, doc)
	opt := stelint.Options{Words: houseWords}

	inStandard, errStandard := stelint.LintFile(standard, opt)
	inOther, errOther := stelint.LintFile(other, opt)

	if errStandard != nil || len(inStandard) != 0 {
		t.Errorf("the standard = %+v, %v; want its word table exempt", inStandard, errStandard)
	}
	if errOther != nil || len(inOther) != 1 || inOther[0].Line != 5 {
		t.Errorf("another document = %+v, %v; want the row on line 5 checked", inOther, errOther)
	}
}

func TestLintFile_AMissingFileIsNotExist(t *testing.T) {
	_, err := stelint.LintFile(filepath.Join(t.TempDir(), "gone.md"), stelint.Options{})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want fs.ErrNotExist", err)
	}
}
