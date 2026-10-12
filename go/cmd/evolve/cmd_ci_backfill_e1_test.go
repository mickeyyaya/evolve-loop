package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
)

func ciGoModFile(t testing.TB, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "go.mod")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGoModulePath_Cases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		body    string
		want    string
		wantErr string
	}{
		{"a quoted module path", "module \"example.com/q\"\n", "example.com/q", ""},
		{"the first module line wins", "module a.com/one\nmodule b.com/two\n", "a.com/one", ""},
		{"a module line with a trailing comment is not a module line", "module a.com/x // c\n", "", "has no module line"},
		{"an empty file", "", "", "has no module line"},
		{"only a go line", "go 1.23\n", "", "has no module line"},
		{"a line beyond the scanner limit", "module " + strings.Repeat("x", 70*1024) + "\n", "", "token too long"},
		{"a unicode module path", "module exämple.com/ü\n", "exämple.com/ü", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := ciGoModFile(t, tc.body)
			got, err := goModulePath(path)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || got != "" {
					t.Errorf("goModulePath = (%q, %v), want an error containing %q", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Errorf("goModulePath = (%q, %v), want %q", got, err, tc.want)
			}
		})
	}
}

func FuzzGoModulePath(f *testing.F) {
	for _, s := range []string{"module a.com/x\n", "", "go 1.23\nmodule \"q\"\n", "module\n", "module a b\n"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, body string) {
		got, err := goModulePath(ciGoModFile(t, body))
		if err == nil && (got == "" && !strings.Contains(body, `""`)) {
			t.Fatalf("goModulePath(%q) = empty path without an error", body)
		}
		if err == nil && strings.ContainsAny(got, " \t\n") {
			t.Fatalf("goModulePath(%q) = %q, a module path holds no whitespace", body, got)
		}
	})
}

func TestWriteClassifyReport_AMarshalFailureIsReturned(t *testing.T) {
	prev := ciClassifyMarshalIndent
	t.Cleanup(func() { ciClassifyMarshalIndent = prev })
	ciClassifyMarshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("boom") }
	var out bytes.Buffer

	err := writeClassifyReport(&out, ciwatch.ClassifyReport{RunID: 1}, true)

	if err == nil || err.Error() != "boom" || out.Len() != 0 {
		t.Errorf("err=%v stdout=%q, want the marshal error and nothing written", err, out.String())
	}
}

func TestOrDash(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"": "-", "pkg": "pkg", "-": "-", " ": " "} {
		if got := orDash(in); got != want {
			t.Errorf("orDash(%q) = %q, want %q", in, got, want)
		}
	}
}
