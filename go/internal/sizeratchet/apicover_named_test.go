package sizeratchet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func body(lines int) string {
	return strings.Repeat("\t_ = 0\n", lines-2)
}

// TestWalk_KeysEveryNonTestFunctionByDirAndReceiver names Walk and FuncSpan:
// sizes exclude the doc comment, methods key by receiver base type, and test
// files and skipped directories are not read.
func TestWalk_KeysEveryNonTestFunctionByDirAndReceiver(t *testing.T) {
	root := t.TempDir()
	src := "package p\n\ntype T[K any] struct{}\n\n// Doc.\nfunc F() {\n" + body(MaxLines+1) + "}\n\nfunc (t *T[K]) M() {\n" + body(3) + "}\n"
	files := map[string]string{"p/p.go": src, "p/p_test.go": "not Go", "p/testdata/x.go": "not Go", "vendor/v.go": "not Go"}
	for rel, text := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []FuncSpan{{Key: "p.F", Lines: MaxLines + 1}, {Key: "p.T.M", Lines: 3}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Walk = %v, want %v", got, want)
	}
}

// TestLoadOffenders_RejectsAllowancesWithinTheLimit names LoadOffenders.
func TestLoadOffenders_RejectsAllowancesWithinTheLimit(t *testing.T) {
	dir := t.TempDir()
	for body, wantErr := range map[string]bool{`{"p.F": 51}`: false, `{"p.F": 50}`: true, `null`: true, `{"p.F": 5`: true} {
		path := filepath.Join(dir, "o.json")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadOffenders(path); (err != nil) != wantErr {
			t.Errorf("LoadOffenders(%s) err = %v, want error %v", body, err, wantErr)
		}
	}
}

// TestCheck_OnlyGrowthOrANewOffenderFails names Check: an allowance is a
// ceiling, so growth and a new offender fail while slack and stale entries pass.
func TestCheck_OnlyGrowthOrANewOffenderFails(t *testing.T) {
	slack := map[string]struct {
		spans     []FuncSpan
		offenders map[string]int
	}{
		"exact allowances":            {[]FuncSpan{{Key: "p.F", Lines: 60}, {Key: "p.G", Lines: MaxLines}}, map[string]int{"p.F": 60}},
		"a shrunk offender":           {[]FuncSpan{{Key: "p.F", Lines: 59}}, map[string]int{"p.F": 60}},
		"an offender back within cap": {[]FuncSpan{{Key: "p.F", Lines: MaxLines}}, map[string]int{"p.F": 60}},
		"a deleted offender's entry":  {nil, map[string]int{"p.Gone": 60}},
	}
	for name, tc := range slack {
		if err := Check(tc.spans, tc.offenders); err != nil {
			t.Errorf("%s is slack the boundary tighten removes, never a failure: %v", name, err)
		}
	}
	failures := map[string]struct {
		spans     []FuncSpan
		offenders map[string]int
	}{
		"shrink it to 50": {[]FuncSpan{{Key: "p.New", Lines: MaxLines + 1}}, nil},
		"shrink it back":  {[]FuncSpan{{Key: "p.F", Lines: 61}}, map[string]int{"p.F": 60}},
	}
	for want, tc := range failures {
		if err := Check(tc.spans, tc.offenders); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Check(%v, %v) = %v, want an error containing %q", tc.spans, tc.offenders, err, want)
		}
	}
}
