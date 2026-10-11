package covchanged

import (
	"errors"
	"go/build"
	"io/fs"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type fixture struct {
	diff    string
	files   map[string]string
	profile string
}

type measured struct {
	Key        string
	Line       int
	Statements int
	Covered    int
	Profiled   bool
}

func darwinContext() build.Context {
	ctx := build.Default
	ctx.GOOS = "darwin"
	ctx.GOARCH = "arm64"
	ctx.BuildTags = []string{"integration"}
	return ctx
}

func (fx fixture) sources(t *testing.T) Sources {
	t.Helper()
	changed, err := ParseDiff([]byte(fx.diff))
	if err != nil {
		t.Fatalf("ParseDiff: %v", err)
	}
	profile, err := ParseProfile([]byte("mode: set\n"+fx.profile), testModule)
	if err != nil {
		t.Fatalf("ParseProfile: %v", err)
	}
	return Sources{
		Changed: changed,
		Profile: profile,
		ReadFile: func(rel string) ([]byte, error) {
			text, ok := fx.files[rel]
			if !ok {
				return nil, fs.ErrNotExist
			}
			return []byte(text), nil
		},
		Build: darwinContext(),
	}
}

func summarize(funcs []Func) []measured {
	out := []measured{}
	for _, f := range funcs {
		out = append(out, measured{f.Key, f.Line, f.Statements, f.Covered, f.Profiled})
	}
	return out
}

func added(file string, start int, lines ...string) string {
	header := "diff --git a/" + file + " b/" + file + "\n--- a/" + file + "\n+++ b/" + file + "\n"
	hunk := "@@ -1,0 +" + strconv.Itoa(start) + "," + strconv.Itoa(len(lines)) + " @@\n"
	return header + hunk + "+" + strings.Join(lines, "\n+") + "\n"
}

const fileA = "package p\n" +
	"\n" +
	"func F(x int) int {\n" +
	"\tif x > 0 {\n" +
	"\t\treturn 1\n" +
	"\t}\n" +
	"\t/* a block\n" +
	"\tcomment that\n" +
	"\tspans lines */\n" +
	"\treturn 0\n" +
	"}\n" +
	"\n" +
	"func G() int {\n" +
	"\t// a note\n" +
	"\treturn 2\n" +
	"}\n"

const profileA = "example.com/m/p/a.go:3.19,4.11 1 1\n" +
	"example.com/m/p/a.go:4.11,6.3 1 0\n" +
	"example.com/m/p/a.go:10.2,10.10 1 1\n" +
	"example.com/m/p/a.go:13.14,15.10 1 1\n"

func TestMeasure_CountsOnlyTheFunctionsAChangedCodeLineTouches(t *testing.T) {
	files := map[string]string{"p/a.go": fileA}
	for name, tc := range map[string]struct {
		fx   fixture
		want []measured
	}{
		"an empty diff measures nothing": {
			fixture{"", files, profileA},
			[]measured{},
		},
		"a comment-only line is ignored": {
			fixture{added("p/a.go", 14, "\t// a note"), files, profileA},
			[]measured{},
		},
		"a line inside a block comment opened above the hunk is ignored": {
			fixture{added("p/a.go", 8, "\tcomment that"), files, profileA},
			[]measured{},
		},
		"a blank line is ignored": {
			fixture{added("p/a.go", 12, ""), files, profileA},
			[]measured{},
		},
		"a changed body line measures its function by block": {
			fixture{added("p/a.go", 5, "\t\treturn 1"), files, profileA},
			[]measured{{"p.F", 3, 3, 2, true}},
		},
		"a rename of the signature counts as a change": {
			fixture{added("p/a.go", 13, "func G() int {"), files, profileA},
			[]measured{{"p.G", 13, 1, 1, true}},
		},
		"a non-Go path is ignored": {
			fixture{added("p/README.md", 1, "text"), files, profileA},
			[]measured{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := Measure(tc.fx.sources(t))
			if err != nil {
				t.Fatalf("Measure: %v", err)
			}
			if !reflect.DeepEqual(summarize(got.Funcs), tc.want) {
				t.Errorf("Measure = %+v, want %+v", summarize(got.Funcs), tc.want)
			}
		})
	}
}

func TestMeasure_AFunctionMovedToAnotherFileIsMeasuredWhereItLandsNow(t *testing.T) {
	moved := "package p\n\nfunc F() int {\n\treturn 1\n}\n"
	fx := fixture{
		diff:    "diff --git a/p/a.go b/p/a.go\n--- a/p/a.go\n+++ b/p/a.go\n@@ -3,3 +2,0 @@\n-func F() int {\n-\treturn 1\n-}\n" + added("p/b.go", 1, strings.Split(strings.TrimSuffix(moved, "\n"), "\n")...),
		files:   map[string]string{"p/a.go": "package p\n", "p/b.go": moved},
		profile: "example.com/m/p/b.go:3.14,4.10 1 0\n",
	}

	got, err := Measure(fx.sources(t))

	if err != nil {
		t.Fatalf("Measure: %v", err)
	}
	want := []measured{{"p.F", 3, 1, 0, true}}
	if !reflect.DeepEqual(summarize(got.Funcs), want) || got.Funcs[0].File != "p/b.go" {
		t.Errorf("Measure = %+v, want %+v in p/b.go", got.Funcs, want)
	}
}

func TestMeasure_ExemptsWhatTheProfileCannotOrNeedNotMeasure(t *testing.T) {
	src := "package p\n\nfunc F() {}\n"
	fx := fixture{
		diff: added("p/x_linux.go", 3, "func F() {}") +
			added("p/y.go", 3, "func F() {}") +
			added("p/testdata/t.go", 3, "func F() {}") +
			added("p/vendor/v.go", 3, "func F() {}") +
			added("p/z_test.go", 3, "func F() {}"),
		files: map[string]string{
			"p/x_linux.go":    src,
			"p/y.go":          "//go:build linux && !integration\n\n" + src,
			"p/testdata/t.go": src,
			"p/vendor/v.go":   src,
			"p/z_test.go":     src,
		},
	}

	got, err := Measure(fx.sources(t))

	if err != nil {
		t.Fatalf("Measure: %v", err)
	}
	want := []Exemption{
		{"p/testdata/t.go", ReasonTestdata},
		{"p/vendor/v.go", ReasonVendor},
		{"p/x_linux.go", ReasonBuildConstraints},
		{"p/y.go", ReasonBuildConstraints},
		{"p/z_test.go", ReasonTestFile},
	}
	if len(got.Funcs) != 0 || !reflect.DeepEqual(got.Exempt, want) {
		t.Errorf("Measure = funcs %+v exempt %+v, want no funcs and %+v", got.Funcs, got.Exempt, want)
	}
}

func TestMeasure_AFileTheBuildTagsSelectIsMeasured(t *testing.T) {
	src := "//go:build darwin && integration\n\npackage p\n\nfunc F() int {\n\treturn 1\n}\n"
	fx := fixture{
		diff:    added("p/x_darwin.go", 6, "\treturn 1"),
		files:   map[string]string{"p/x_darwin.go": src},
		profile: "example.com/m/p/x_darwin.go:5.14,6.10 1 1\n",
	}

	got, err := Measure(fx.sources(t))

	if err != nil || !reflect.DeepEqual(summarize(got.Funcs), []measured{{"p.F", 5, 1, 1, true}}) {
		t.Errorf("Measure = %+v, %v; want p.F measured at 1/1", got, err)
	}
}

func TestMeasure_NamesEveryReceiverFormByItsBaseType(t *testing.T) {
	src := "package p\n\n" +
		"func (s *Set[K]) Add(k K) { s.m[k] = true }\n" +
		"func (m Map[K, V]) Get(k K) V { return m.v[k] }\n" +
		"func (t T) M() int { return 1 }\n" +
		"func (t *T) N() int { return 2 }\n" +
		"func (T) O() int { return 3 }\n" +
		"func (p *(T)) P() int { return 4 }\n"
	fx := fixture{
		diff:  added("p/r.go", 3, strings.Split(strings.TrimSuffix(strings.SplitN(src, "\n\n", 2)[1], "\n"), "\n")...),
		files: map[string]string{"p/r.go": src},
		profile: "example.com/m/p/r.go:3.27,3.43 1 1\n" +
			"example.com/m/p/r.go:4.31,4.48 1 1\n" +
			"example.com/m/p/r.go:5.20,5.31 1 1\n" +
			"example.com/m/p/r.go:6.21,6.32 1 1\n" +
			"example.com/m/p/r.go:7.18,7.29 1 1\n" +
			"example.com/m/p/r.go:8.23,8.34 1 1\n",
	}

	got, err := Measure(fx.sources(t))

	if err != nil {
		t.Fatalf("Measure: %v", err)
	}
	var keys []string
	for _, f := range got.Funcs {
		keys = append(keys, f.Key+"|"+f.Name)
	}
	want := []string{"p.Set.Add|Set.Add", "p.Map.Get|Map.Get", "p.T.M|T.M", "p.T.N|T.N", "p.T.O|T.O", "p.T.P|T.P"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("keys = %v, want %v", keys, want)
	}
}

func TestMeasure_ABlockCountsInTheFunctionThatHoldsItsStart(t *testing.T) {
	src := "package p\n\nfunc F() { a() }\nfunc G() { b() }\n"
	fx := fixture{
		diff:  added("p/s.go", 3, "func F() { a() }", "func G() { b() }"),
		files: map[string]string{"p/s.go": src},
		profile: "example.com/m/p/s.go:3.10,4.16 2 0\n" +
			"example.com/m/p/s.go:4.10,4.16 1 1\n",
	}

	got, err := Measure(fx.sources(t))

	want := []measured{{"p.F", 3, 2, 0, true}, {"p.G", 4, 1, 1, true}}
	if err != nil || !reflect.DeepEqual(summarize(got.Funcs), want) {
		t.Errorf("Measure = %+v, %v; want %+v", summarize(got.Funcs), err, want)
	}
}

func TestMeasure_ZeroStatementsAndNoProfileAreTwoDifferentStates(t *testing.T) {
	src := "package p\n\nfunc Z() {}\n\nfunc N() int {\n\treturn 1\n}\n"
	fx := fixture{
		diff:    added("p/z.go", 3, "func Z() {}", "", "func N() int {", "\treturn 1", "}"),
		files:   map[string]string{"p/z.go": src},
		profile: "example.com/m/p/z.go:3.11,3.12 0 0\n",
	}

	got, err := Measure(fx.sources(t))

	if err != nil || len(got.Funcs) != 2 {
		t.Fatalf("Measure = %+v, %v; want two funcs", got.Funcs, err)
	}
	z, n := got.Funcs[0], got.Funcs[1]
	if !z.Full() || z.Percent() != 100 || !z.Profiled {
		t.Errorf("a profiled zero-statement function %+v must be full at 100%%", z)
	}
	if n.Full() || n.Profiled || n.Percent() != 0 {
		t.Errorf("a function the profile never saw %+v must not be full", n)
	}
	half := Func{Statements: 4, Covered: 3, Profiled: true}
	if half.Full() || half.Percent() != 75 {
		t.Errorf("3/4 = full %v %.1f%%, want not full at 75%%", half.Full(), half.Percent())
	}
}

func TestMeasure_FailsLoudlyOnAFileItCannotReadParseOrConstrain(t *testing.T) {
	for name, fx := range map[string]fixture{
		"a missing file":           {added("p/a.go", 1, "x"), map[string]string{}, ""},
		"a file that cannot parse": {added("p/a.go", 1, "x"), map[string]string{"p/a.go": "package p\nfunc {"}, ""},
		"a malformed build line":   {added("p/a.go", 3, "x"), map[string]string{"p/a.go": "//go:build (\n\npackage p\n"}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Measure(fx.sources(t)); err == nil {
				t.Error("Measure = nil error, want a failure")
			}
		})
	}
	src := fixture{added("p/a.go", 1, "x"), map[string]string{}, ""}.sources(t)
	if _, err := Measure(src); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Measure on a missing file = %v, want it to wrap fs.ErrNotExist", err)
	}
}

func TestMeasure_SkipsADeclarationWithoutABodyAndNamesAnEmptyReceiverByItsName(t *testing.T) {
	src := "package p\n\nvar V = 1\n\nfunc Ext()\n\nfunc () Anon() { V++ }\n"
	fx := fixture{
		diff:    added("p/d.go", 3, "var V = 1", "", "func Ext()", "", "func () Anon() { V++ }"),
		files:   map[string]string{"p/d.go": src},
		profile: "example.com/m/p/d.go:7.16,7.23 1 1\n",
	}

	got, err := Measure(fx.sources(t))

	if err != nil || !reflect.DeepEqual(summarize(got.Funcs), []measured{{"p.Anon", 7, 1, 1, true}}) {
		t.Errorf("Measure = %+v, %v; want only p.Anon at 1/1", summarize(got.Funcs), err)
	}
}

func TestMeasure_AGeneratedHeaderDoesNotExemptAFile(t *testing.T) {
	src := "// Code generated by hand. DO NOT EDIT.\n\npackage p\n\nfunc F() int {\n\treturn 1\n}\n"
	fx := fixture{
		diff:    added("p/gen.go", 6, "\treturn 1"),
		files:   map[string]string{"p/gen.go": src},
		profile: "example.com/m/p/gen.go:5.14,6.10 1 0\n",
	}

	got, err := Measure(fx.sources(t))

	if err != nil || len(got.Exempt) != 0 || !reflect.DeepEqual(summarize(got.Funcs), []measured{{"p.F", 5, 1, 0, true}}) {
		t.Errorf("Measure = %+v, %v; want p.F measured at 0/1 and no exemption", got, err)
	}
}

func TestMeasure_RemovedCodeTouchesTheFunctionThatHeldIt(t *testing.T) {
	files := map[string]string{"p/a.go": fileA}
	removal := func(header string, lines ...string) string {
		return "diff --git a/p/a.go b/p/a.go\n--- a/p/a.go\n+++ b/p/a.go\n" + header + "\n-" + strings.Join(lines, "\n-") + "\n"
	}
	for name, tc := range map[string]struct {
		diff string
		want []measured
	}{
		"a removed statement inside F": {
			removal("@@ -6 +5,0 @@", "\t\tx++"),
			[]measured{{"p.F", 3, 3, 2, true}},
		},
		"a removed comment inside G": {
			removal("@@ -15 +14,0 @@", "\t// an old note"),
			[]measured{},
		},
		"a removed function right after F": {
			removal("@@ -12,4 +11,0 @@", "", "func H() int {", "\treturn 3", "}"),
			[]measured{},
		},
		"code replaced by a comment inside G": {
			removal("@@ -14 +14 @@", "\tlog()") + "+\t// a note\n",
			[]measured{{"p.G", 13, 1, 1, true}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := Measure(fixture{tc.diff, files, profileA}.sources(t))
			if err != nil || !reflect.DeepEqual(summarize(got.Funcs), tc.want) {
				t.Errorf("Measure = %+v, %v; want %+v", summarize(got.Funcs), err, tc.want)
			}
		})
	}
}
