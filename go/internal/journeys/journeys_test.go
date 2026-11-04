package journeys

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodRow = `{"id":"family-lane","journey":"Two items of one family run one after the other.","test":"internal/orchestrator.TestPool_TwoTodosOfOneFamilyRunOneAfterTheOther","issue":"PR #1"}`

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParse_AValidRegistryKeepsEachRow(t *testing.T) {
	t.Parallel()
	retired := `{"id":"old-path","journey":"The old path works.","test":"cmd/evolve.TestOld","issue":"PR #2","retired":{"reason":"the path is gone"}}`

	reg, err := Parse([]byte(`{"rows":[` + goodRow + `,` + retired + `]}`))

	if err != nil || len(reg.Rows) != 2 || reg.Rows[1].Retired.Reason != "the path is gone" {
		t.Fatalf("Parse = %+v, %v; want two rows, the second retired with its reason", reg, err)
	}
	if active := reg.Active(); len(active) != 1 || active[0].ID != "family-lane" {
		t.Fatalf("Active = %+v, want only family-lane", active)
	}
}

func TestParse_RefusesEachBadRow(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"not json":          `{`,
		"unknown key":       `{"rows":[{"id":"a","journey":"j","test":"p.TestA","issue":"i","owner":"x"}]}`,
		"bad id":            `{"rows":[{"id":"Bad Id","journey":"j","test":"p.TestA","issue":"i"}]}`,
		"duplicate id":      `{"rows":[` + goodRow + `,` + goodRow + `]}`,
		"no journey":        `{"rows":[{"id":"a","journey":" ","test":"p.TestA","issue":"i"}]}`,
		"no issue":          `{"rows":[{"id":"a","journey":"j","test":"p.TestA","issue":""}]}`,
		"test not a Test":   `{"rows":[{"id":"a","journey":"j","test":"p.HelperA","issue":"i"}]}`,
		"test no package":   `{"rows":[{"id":"a","journey":"j","test":"TestA","issue":"i"}]}`,
		"retired no reason": `{"rows":[{"id":"a","journey":"j","test":"p.TestA","issue":"i","retired":{"reason":" "}}]}`,
	}
	for name, body := range cases {
		if _, err := Parse([]byte(body)); err == nil {
			t.Errorf("%s: Parse(%s) = nil error, want a refusal", name, body)
		}
	}
}

func TestValidID_AcceptsOnlyASlug(t *testing.T) {
	t.Parallel()
	for id, want := range map[string]bool{"family-lane": true, "a1": true, "": false, "A": false, "a--b": false, "-a": false, "a b": false} {
		if got := ValidID(id); got != want {
			t.Errorf("ValidID(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestLoad_ReadsTheRegistryUnderTheRepoRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, RegistryPath), `{"rows":[`+goodRow+`]}`)

	reg, err := Load(root)

	if err != nil || !reg.Known("family-lane") || reg.Known("absent") {
		t.Fatalf("Load = %+v, %v; want family-lane known and absent unknown", reg, err)
	}
	if _, err := Load(t.TempDir()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load of a root with no registry = %v, want os.ErrNotExist", err)
	}
}

func TestKnown_ARetiredRowIsNotKnown(t *testing.T) {
	t.Parallel()
	reg := Registry{Rows: []Row{{ID: "old", Retired: &Retired{Reason: "gone"}}}}

	if reg.Known("old") {
		t.Fatal("Known(old) = true, want false for a retired row")
	}
}

func TestRemoved_NamesEachRowThatLeftTheRegistry(t *testing.T) {
	t.Parallel()
	base := Registry{Rows: []Row{{ID: "kept"}, {ID: "dropped"}, {ID: "retired"}}}
	head := Registry{Rows: []Row{{ID: "kept"}, {ID: "retired", Retired: &Retired{Reason: "gone"}}, {ID: "new"}}}

	if got := head.Removed(base); len(got) != 1 || got[0] != "dropped" {
		t.Fatalf("Removed = %v, want [dropped]", got)
	}
}

func TestMissingTests_NamesEachActiveRowWhoseTestIsNotInTheTree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "pkg", "a_test.go"), "package pkg\n\nimport \"testing\"\n\nfunc TestHere(t *testing.T) {}\n\ntype s struct{}\n\nfunc (s) TestMethod(t *testing.T) {}\n")
	writeFile(t, filepath.Join(root, "pkg", "a.go"), "package pkg\n\nfunc TestInProduction() {}\n")
	rows := []Row{
		{ID: "here", Test: "pkg.TestHere"},
		{ID: "method", Test: "pkg.TestMethod"},
		{ID: "production", Test: "pkg.TestInProduction"},
		{ID: "absent", Test: "pkg.TestAbsent"},
		{ID: "nodir", Test: "nodir.TestHere"},
		{ID: "retired", Test: "pkg.TestAbsent", Retired: &Retired{Reason: "gone"}},
	}

	got, err := MissingTests(root, rows)

	if err != nil || strings.Join(got, ",") != "method,production,absent,nodir" {
		t.Fatalf("MissingTests = %v, %v; want [method production absent nodir]", got, err)
	}
}

func TestMissingTests_RefusesAFileThatDoesNotParse(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "pkg", "bad_test.go"), "package pkg\n\nfunc {\n")

	if _, err := MissingTests(root, []Row{{ID: "a", Test: "pkg.TestA"}}); err == nil {
		t.Fatal("MissingTests over a broken file = nil error, want the parse error")
	}
}
