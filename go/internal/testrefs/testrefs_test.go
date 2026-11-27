package testrefs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, text := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestDefined_ReadsDeclarationsOnlyAndSkipsFixtureDirs(t *testing.T) {
	t.Parallel()
	root := writeTree(t, map[string]string{
		"a/a_test.go":          "package a\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {}\nfunc TestA_One(t *testing.T) {}\nfunc Testable() {}\nfunc helper() {}\nfunc (x T) TestM() {}\n",
		"a/str_test.go":        "package a\n\nvar s = \"func TestInString(t *testing.T) {}\"\n",
		"a/tagged_test.go":     "//go:build integration\n\npackage a\n\nfunc Test9Digit(t *testing.T) {}\n",
		"a/a.go":               "package a\n\nfunc TestNotInTestFile() {}\n",
		"a/testdata/x_test.go": "not go",
		"vendor/v_test.go":     "not go",
		".hidden/h_test.go":    "not go",
		"_skip/s_test.go":      "not go",
	})
	got, err := Defined(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Test9Digit", "TestA_One", "TestB"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Defined = %q, want %q", got, want)
	}
}

func TestDefined_AnUnparsableFileOrAbsentRootIsAnError(t *testing.T) {
	t.Parallel()
	bad := writeTree(t, map[string]string{"a/a_test.go": "not go"})
	if _, err := Defined(bad); err == nil {
		t.Error("an unparsable test file must fail")
	}
	_, err := Defined(filepath.Join(t.TempDir(), "gone"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("an absent root must wrap ErrNotExist, got %v", err)
	}
}

func TestCites_FindsBackticksInTheDocsOfRecordOnly(t *testing.T) {
	t.Parallel()
	root := writeTree(t, map[string]string{
		"docs/architecture/a.md":     "x `TestA` and `TestB/sub` y\n`Testable` `TestFoo_` `TestBar*` `go test -run TestC` `testD`\n",
		"docs/architecture/adr/b.md": "`TestDeep`\n",
		"docs/incidents/i.md":        "`TestInc`\n",
		"docs/conventions/c.md":      "`TestConv`\n",
		"docs/operations/o.md":       "`TestOps`\n",
		"skills/s/SKILL.md":          "`TestSkill`\n",
		"agents/x.md":                "`TestAgent`\n",
		"agents/sub/y.md":            "`TestNestedAgent`\n",
		"agents/z.txt":               "`TestText`\n",
		"docs/plans/p.md":            "`TestPlan`\n",
		"docs/research/r.md":         "`TestResearch`\n",
		"docs/private/q.md":          "`TestPrivate`\n",
		"docs/architecture/gen.md":   "`TestKept`\n<!-- GENERATED:x BEGIN `TestOnBegin` -->\n`TestGen`\n<!-- GENERATED:x END `TestOnEnd` -->\n`TestAfter`\n<!-- GENERATED:odd -->\n`TestOdd`\n",
	})
	got, err := Cites(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range got {
		names = append(names, c.Doc+":"+c.Name)
	}
	want := []string{
		"docs/architecture/a.md:TestA", "docs/architecture/a.md:TestB",
		"docs/architecture/a.md:TestFoo_", "docs/architecture/a.md:TestBar*",
		"docs/architecture/adr/b.md:TestDeep",
		"docs/architecture/gen.md:TestKept", "docs/architecture/gen.md:TestAfter", "docs/architecture/gen.md:TestOdd",
		"docs/incidents/i.md:TestInc", "docs/conventions/c.md:TestConv", "docs/operations/o.md:TestOps",
		"skills/s/SKILL.md:TestSkill", "agents/x.md:TestAgent",
	}
	if !sameSet(names, want) {
		t.Errorf("Cites = %q, want %q", names, want)
	}
	if got[0].Line != 1 || got[2].Line != 2 {
		t.Errorf("lines = %d, %d, want 1, 2", got[0].Line, got[2].Line)
	}
}

func sameSet(a, b []string) bool {
	m := map[string]int{}
	for _, s := range a {
		m[s]++
	}
	for _, s := range b {
		m[s]--
	}
	for _, n := range m {
		if n != 0 {
			return false
		}
	}
	return true
}

func TestCites_AnAbsentDocTreeIsEmptyAndAnUnreadableOneFails(t *testing.T) {
	t.Parallel()
	empty, err := Cites(t.TempDir())
	if err != nil || len(empty) != 0 {
		t.Errorf("an absent tree is empty, got %v, %v", empty, err)
	}
	root := writeTree(t, map[string]string{"docs/architecture/a.md": "`TestA`\n"})
	if err := os.Chmod(filepath.Join(root, "docs/architecture/a.md"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := Cites(root); err == nil {
		t.Error("an unreadable doc must fail")
	}
	root = writeTree(t, map[string]string{"agents/a.md": "x\n"})
	if err := os.Chmod(filepath.Join(root, "agents"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "agents"), 0o755) })
	if _, err := Cites(root); err == nil {
		t.Error("an unreadable doc dir must fail")
	}
}

func TestCites_AFailureUnderTheRecursiveDirsIsReturned(t *testing.T) {
	t.Parallel()
	root := writeTree(t, map[string]string{"docs/architecture/a.md": "x\n"})
	if err := os.Chmod(filepath.Join(root, "docs/architecture"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "docs/architecture"), 0o755) })
	if _, err := Cites(root); err == nil {
		t.Error("an unreadable recursive doc dir must fail")
	}
}

func TestMissing_ResolvesExactNamesAndPrefixPatternsAndSortsTheRest(t *testing.T) {
	t.Parallel()
	defined := []string{"TestAlpha", "TestFoo_One", "TestZed"}
	cites := []Cite{
		{Doc: "b.md", Name: "TestZed"},
		{Doc: "b.md", Name: "TestNope"},
		{Doc: "b.md", Name: "TestAbsent"},
		{Doc: "b.md", Name: "TestAbsent"},
		{Doc: "a.md", Name: "TestFoo_"},
		{Doc: "a.md", Name: "TestFoo*"},
		{Doc: "a.md", Name: "TestAl*"},
		{Doc: "a.md", Name: "TestBar_"},
		{Doc: "a.md", Name: "TestBaz*"},
		{Doc: "a.md", Name: "TestZ"},
	}
	got := Missing(defined, cites)
	want := map[string][]string{"a.md": {"TestBar_", "TestBaz*", "TestZ"}, "b.md": {"TestAbsent", "TestNope"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Missing = %v, want %v", got, want)
	}
}

func TestCheck_ANewMissFailsAndAStaleRowFailsAndSharedRowsPass(t *testing.T) {
	t.Parallel()
	missing := map[string][]string{"a.md": {"TestOld", "TestNew"}, "c.md": {"TestC"}}
	baseline := map[string][]string{"a.md": {"TestOld", "TestGone"}, "b.md": {"TestB"}}
	for _, doc := range []string{"d", "e", "f", "g", "h", "i"} {
		missing[doc+".md"] = []string{"Test" + strings.ToUpper(doc)}
	}
	got := Check(missing, baseline)
	if len(got) != 10 {
		t.Fatalf("problems = %q, want 10", got)
	}
	joined := strings.Join(got, "\n")
	for _, frag := range []string{"a.md cites TestNew", "c.md cites TestC", "a.md: baseline row TestGone is stale", "b.md: baseline row TestB is stale"} {
		if !strings.Contains(joined, frag) {
			t.Errorf("problems lack %q: %q", frag, got)
		}
	}
	if strings.Contains(joined, "TestOld") {
		t.Errorf("a listed miss must pass: %q", got)
	}
	if !sort.StringsAreSorted(got) {
		t.Errorf("problems are not sorted: %q", got)
	}
	if again := Check(missing, baseline); !reflect.DeepEqual(got, again) {
		t.Errorf("output is not deterministic: %q vs %q", got, again)
	}
	if got := Check(nil, nil); len(got) != 0 {
		t.Errorf("empty inputs pass, got %q", got)
	}
}

func fixtureRepo(t *testing.T, baseline string) string {
	t.Helper()
	files := map[string]string{
		"go/p/p_test.go":         "package p\n\nfunc TestReal(t *testing.T) {}\n",
		"docs/architecture/a.md": "`TestReal` `TestGhost`\n",
	}
	if baseline != "" {
		files["go/"+BaselineRelPath] = baseline
	}
	return writeTree(t, files)
}

func TestScan_ComparesTheTreeToTheBaseline(t *testing.T) {
	t.Parallel()
	got, err := Scan(fixtureRepo(t, `{"docs/architecture/a.md":["TestGhost"]}`))
	if err != nil || len(got) != 0 {
		t.Errorf("a listed miss passes, got %q, %v", got, err)
	}
	got, err = Scan(fixtureRepo(t, `{}`))
	if err != nil || len(got) != 1 || !strings.Contains(got[0], "TestGhost") {
		t.Errorf("an unlisted miss fails, got %q, %v", got, err)
	}
	got, err = Scan(fixtureRepo(t, `{"docs/architecture/a.md":["TestGhost","TestReal"]}`))
	if err != nil || len(got) != 1 || !strings.Contains(got[0], "stale") {
		t.Errorf("a stale row fails, got %q, %v", got, err)
	}
}

func TestScan_ABadOrAbsentBaselineOrTreeIsAnError(t *testing.T) {
	t.Parallel()
	if _, err := Scan(fixtureRepo(t, "")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("an absent baseline must wrap ErrNotExist, got %v", err)
	}
	if _, err := Scan(fixtureRepo(t, "not json")); err == nil {
		t.Error("a bad baseline must fail")
	}
	if _, err := Scan(t.TempDir()); err == nil {
		t.Error("an absent go tree must fail")
	}
}

func baselineReader(files map[string]string) func(string) ([]byte, error) {
	return func(p string) ([]byte, error) {
		if text, ok := files[p]; ok {
			return []byte(text), nil
		}
		return nil, fs.ErrNotExist
	}
}

func TestCheckAgainstBase_RefusesARowThatTheBaseDidNotHold(t *testing.T) {
	t.Parallel()
	path := "go/" + BaselineRelPath
	base := map[string]string{path: `{"a.md":["TestOld"],"b.md":["TestB"]}`}
	cases := map[string]struct {
		head string
		want []string
	}{
		"a shrunk baseline passes":         {`{"a.md":["TestOld"]}`, nil},
		"an equal baseline passes":         {`{"a.md":["TestOld"],"b.md":["TestB"]}`, nil},
		"a grown row in a known doc fails": {`{"a.md":["TestGhost","TestOld"],"b.md":["TestB"]}`, []string{"a.md: baseline row TestGhost"}},
		"a row in a new doc fails":         {`{"c.md":["TestNew"]}`, []string{"c.md: baseline row TestNew"}},
		"many grown rows are sorted":       {`{"h.md":["TestH"],"c.md":["TestC"],"f.md":["TestF"],"a.md":["TestA"],"g.md":["TestG"],"i.md":["TestI"],"e.md":["TestE"],"d.md":["TestD"]}`, []string{"a.md: baseline row TestA", "c.md: baseline row TestC", "d.md: baseline row TestD", "e.md: baseline row TestE", "f.md: baseline row TestF", "g.md: baseline row TestG", "h.md: baseline row TestH", "i.md: baseline row TestI"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := CheckAgainstBase("go/", baselineReader(base), baselineReader(map[string]string{path: tc.head}))
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(got, "\n")
			if len(tc.want) == 0 {
				if got != "" {
					t.Errorf("refusal = %q, want none", got)
				}
				return
			}
			if len(lines) != len(tc.want) {
				t.Fatalf("refusal = %q, want %d line(s)", got, len(tc.want))
			}
			for i, frag := range tc.want {
				if !strings.Contains(lines[i], frag) || !strings.Contains(lines[i], "only shrinks") {
					t.Errorf("line %d = %q, want %q and the rule", i, lines[i], frag)
				}
			}
		})
	}
}

func TestCheckAgainstBase_IsSilentWithoutABaseListAndCarriesEachReadFailure(t *testing.T) {
	t.Parallel()
	path := BaselineRelPath
	if got, err := CheckAgainstBase("", baselineReader(nil), baselineReader(map[string]string{path: `{"a.md":["TestX"]}`})); got != "" || err != nil {
		t.Errorf("no baseline at the base is a new list: %q, %v", got, err)
	}
	down := errors.New("disk down")
	failing := func(string) ([]byte, error) { return nil, down }
	good := baselineReader(map[string]string{path: `{}`})
	bad := baselineReader(map[string]string{path: "null{"})
	cases := map[string][2]func(string) ([]byte, error){
		"an unreadable base": {failing, good},
		"an unparsable base": {bad, good},
		"an unreadable head": {good, failing},
		"an unparsable head": {good, bad},
		"an absent head":     {good, baselineReader(nil)},
	}
	for name, readers := range cases {
		if got, err := CheckAgainstBase("", readers[0], readers[1]); err == nil || got != "" {
			t.Errorf("%s: %q, %v, want an error and no refusal", name, got, err)
		}
	}
}

func TestBaselineRelPath_IsInsideThePackage(t *testing.T) {
	t.Parallel()
	if BaselineRelPath != "internal/testrefs/baseline.json" {
		t.Errorf("BaselineRelPath = %q", BaselineRelPath)
	}
}

func TestScan_AnUnreadableDocTreeIsAnError(t *testing.T) {
	t.Parallel()
	root := fixtureRepo(t, `{}`)
	dir := filepath.Join(root, "docs/architecture")
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if _, err := Scan(root); err == nil {
		t.Error("an unreadable doc tree must fail")
	}
}
