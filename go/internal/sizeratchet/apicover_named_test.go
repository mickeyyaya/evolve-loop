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

// TestCheck_AllowancesOnlyShrink names Check: growth, slack and stale entries
// all fail, each named with the edit that fixes it.
func TestCheck_AllowancesOnlyShrink(t *testing.T) {
	if err := Check([]FuncSpan{{Key: "p.F", Lines: 60}, {Key: "p.G", Lines: MaxLines}}, map[string]int{"p.F": 60}); err != nil {
		t.Errorf("Check at exact allowances = %v, want nil", err)
	}
	cases := map[string]struct {
		spans     []FuncSpan
		offenders map[string]int
	}{
		"shrink it to 50": {[]FuncSpan{{Key: "p.New", Lines: MaxLines + 1}}, nil},
		"shrink it back":  {[]FuncSpan{{Key: "p.F", Lines: 61}}, map[string]int{"p.F": 60}},
		"allowance to 59": {[]FuncSpan{{Key: "p.F", Lines: 59}}, map[string]int{"p.F": 60}},
		"remove its":      {nil, map[string]int{"p.Gone": 60}},
	}
	for want, tc := range cases {
		if err := Check(tc.spans, tc.offenders); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Check(%v, %v) = %v, want an error containing %q", tc.spans, tc.offenders, err, want)
		}
	}
}
