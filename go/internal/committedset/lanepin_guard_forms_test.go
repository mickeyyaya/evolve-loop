package committedset

import (
	"go/parser"
	"go/token"
	"testing"
)

func refsOf(t *testing.T, src string) []pinRef {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	return pinFileRefs(fset, file)
}

func TestPinFileRefs_FlagsEachFormOfAReadOfThePinFile(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ src, fn string }{
		"a path built in a variable":  {`package p; func f(w string) { p := filepath.Join(w, LaneScopeFile); os.ReadFile(p) }`, "f"},
		"a json helper":               {`package p; func f(w string) { readJSON(w, LaneScopeFile, &d) }`, "f"},
		"a helper that builds a path": {`package p; func pinPath(w string) string { return filepath.Join(w, LaneScopeFile) }; func f(w string) { os.ReadFile(pinPath(w)) }`, "pinPath"},
		"a package constant":          {`package p; const pin = "lane-scope.json"; func f(w string) { os.ReadFile(filepath.Join(w, pin)) }`, ""},
		"a literal in a sum":          {`package p; func f(w string) { os.ReadFile(w + "/lane-scope.json") }`, "f"},
		"a literal split across a +":  {`package p; func f(w string) { os.ReadFile(w + "/lane-" + "scope.json") }`, "f"},
		"the committedset constant":   {`package p; func f(w string) { os.Open(filepath.Join(w, committedset.LanePinFile)) }`, "f"},
		"a raw string":                {"package p; func f(w string) { os.Stat(w + `/lane-scope.json`) }", "f"},
		"a write":                     {`package p; func f(w string) { os.WriteFile(filepath.Join(w, LaneScopeFile), nil, 0) }`, "f"},
	} {
		got := refsOf(t, tc.src)
		if len(got) != 1 || got[0].Func != tc.fn {
			t.Errorf("%s: pinFileRefs = %+v, want one ref in %q", name, got, tc.fn)
		}
	}
}

func TestPinFileRefs_IgnoresCodeThatNeverNamesThePinFile(t *testing.T) {
	t.Parallel()
	for name, src := range map[string]string{
		"another file":            `package p; func f(w string) { os.ReadFile(filepath.Join(w, "scout-report.md")) }`,
		"a sum of other literals": `package p; func f(w string) { os.ReadFile(w + "/lane-" + "other.json") }`,
		"a call with no file":     `package p; func f(w string) { fmt.Fprintf(w, "%d", 1+2) }`,
	} {
		if got := refsOf(t, src); len(got) != 0 {
			t.Errorf("%s: pinFileRefs = %+v, want none", name, got)
		}
	}
}
