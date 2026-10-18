package sizeratchet

import (
	"errors"
	"go/token"
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

func readerOf(files map[string]string) func(string) ([]byte, error) {
	return func(p string) ([]byte, error) {
		if text, ok := files[p]; ok {
			return []byte(text), nil
		}
		return nil, fs.ErrNotExist
	}
}

func failingReader(err error) func(string) ([]byte, error) {
	return func(string) ([]byte, error) { return nil, err }
}

func TestMeasured_AcceptsOnlyNonTestGoFilesOutsideSkippedDirs(t *testing.T) {
	cases := map[string]bool{
		"a.go":                   true,
		"internal/x/a.go":        true,
		"a_test.go":              false,
		"internal/x/doc.md":      false,
		"vendor/v.go":            false,
		"internal/testdata/x.go": false,
		"internal/.hidden/x.go":  false,
		"internal/_tmp/x.go":     false,
	}
	for rel, want := range cases {
		if got := measured(rel); got != want {
			t.Errorf("measured(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestParseSpans_MeasuresSourceBytesUnderTheGivenDir(t *testing.T) {
	spans, err := parseSpans(token.NewFileSet(), "internal/x", "x.go", []byte(fileWith("F", 3)))
	if err != nil || !reflect.DeepEqual(spans, []FuncSpan{{Key: "internal/x.F", Lines: 3}}) {
		t.Errorf("parseSpans = %v, %v", spans, err)
	}
	if spans, err := parseSpans(token.NewFileSet(), "internal/x", "x.go", []byte("package p\nfunc (")); err == nil || spans != nil {
		t.Errorf("a source that does not parse: spans=%v err=%v", spans, err)
	}
}

func TestParseOffenders_NamesTheListInEachRefusal(t *testing.T) {
	for body, wantErr := range map[string]bool{`{"p.F": 51}`: false, `{"p.F": 50}`: true, `null`: true, `{"p.F": 5`: true} {
		offenders, err := parseOffenders([]byte(body), "the-list")
		if (err != nil) != wantErr {
			t.Errorf("parseOffenders(%s) = %v, %v, want error %v", body, offenders, err, wantErr)
		}
		if err != nil && !strings.Contains(err.Error(), "the-list") {
			t.Errorf("parseOffenders(%s) err = %v, want it to name the-list", body, err)
		}
	}
}

func TestReadSpans_ReadsEachMeasuredPathAndSkipsOneAbsentOnThatSide(t *testing.T) {
	files := map[string]string{"go/p/a.go": fileWith("F", 60), "go/p/a_test.go": "not Go", "go/vendor/v.go": "not Go", "docs/x.go": "not Go"}
	paths := []string{"go/p/a.go", "go/p/a_test.go", "go/vendor/v.go", "docs/x.go", "go/p/gone.go"}

	spans, err := readSpans(readerOf(files), "go/", paths)

	if err != nil || !reflect.DeepEqual(spans, []FuncSpan{{Key: "p.F", Lines: 60}}) {
		t.Errorf("readSpans = %v, %v, want only p.F at 60", spans, err)
	}
	if spans, err := readSpans(failingReader(errors.New("disk down")), "", []string{"a.go"}); err == nil || spans != nil || !strings.Contains(err.Error(), "disk down") {
		t.Errorf("a read failure: spans=%v err=%v", spans, err)
	}
	if spans, err := readSpans(readerOf(map[string]string{"a.go": "package p\nfunc ("}), "", []string{"a.go"}); err == nil || spans != nil {
		t.Errorf("a parse failure: spans=%v err=%v", spans, err)
	}
}

func TestCheckAgainstBase_RefusesRegrowthIntoSlackAndANewOffenderAgainstTheBaseList(t *testing.T) {
	base := map[string]string{"go/" + OffendersRelPath: `{"p.F": 60, "p.Healed": 60}`, "go/p/a.go": fileWith("F", 55), "go/p/c.go": fileWith("Healed", 40)}
	paths := []string{"go/p/a.go", "go/p/b.go", "go/p/c.go"}
	cases := map[string]struct {
		head map[string]string
		want []string
	}{
		"regrowth and a new offender": {map[string]string{"go/p/a.go": fileWith("F", 58), "go/p/b.go": fileWith("New", MaxLines+1), "go/p/c.go": fileWith("Healed", 40)}, []string{"p.F grew to 58 lines > allowance 55", "p.New is 51 lines > 50"}},
		"a healed entry regrown":      {map[string]string{"go/p/a.go": fileWith("F", 55), "go/p/c.go": fileWith("Healed", 52)}, []string{"p.Healed is 52 lines > 50"}},
		"a shrink and a deletion":     {map[string]string{"go/p/a.go": fileWith("F", 52)}, nil},
		"an unchanged tree":           {map[string]string{"go/p/a.go": fileWith("F", 55), "go/p/c.go": fileWith("Healed", 40)}, nil},
	}
	for name, tc := range cases {
		refusal, err := CheckAgainstBase("go/", paths, readerOf(base), readerOf(tc.head))
		if err != nil || (refusal == "") != (tc.want == nil) {
			t.Errorf("%s: refusal=%q err=%v, want problems %v", name, refusal, err, tc.want)
		}
		for _, want := range tc.want {
			if !strings.Contains(refusal, want) {
				t.Errorf("%s: refusal %q does not name %q", name, refusal, want)
			}
		}
	}
}

func TestCheckAgainstBase_IsSilentWithoutAListAndCarriesEachReadFailure(t *testing.T) {
	paths := []string{"p/a.go"}
	if refusal, err := CheckAgainstBase("", paths, readerOf(map[string]string{}), readerOf(map[string]string{"p/a.go": fileWith("F", 70)})); refusal != "" || err != nil {
		t.Errorf("a module without a list: refusal=%q err=%v, want silence", refusal, err)
	}
	failures := map[string]struct {
		atBase, onDisk func(string) ([]byte, error)
	}{
		"an unreadable base":                  {failingReader(errors.New("base gone")), readerOf(nil)},
		"a list that does not parse":          {readerOf(map[string]string{OffendersRelPath: "null"}), readerOf(nil)},
		"a base file that does not parse":     {readerOf(map[string]string{OffendersRelPath: "{}", "p/a.go": "package p\nfunc ("}), readerOf(nil)},
		"a worktree file that cannot be read": {readerOf(map[string]string{OffendersRelPath: "{}"}), failingReader(errors.New("disk down"))},
	}
	for name, tc := range failures {
		if refusal, err := CheckAgainstBase("", paths, tc.atBase, tc.onDisk); err == nil || refusal != "" {
			t.Errorf("%s: refusal=%q err=%v, want an error and no refusal", name, refusal, err)
		}
	}
}
