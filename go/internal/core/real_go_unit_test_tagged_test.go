package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func taggedModule(t *testing.T) string {
	t.Helper()
	mod := t.TempDir()
	files := map[string]string{
		"go.mod":                "module x\n\ngo 1.23\n",
		"gated/pass_test.go":    "//go:build acs\n\npackage gated\n\nimport \"testing\"\n\nfunc TestGatedOK(t *testing.T) {}\n",
		"gatedred/fail_test.go": "//go:build acs\n\npackage gatedred\n\nimport \"testing\"\n\nfunc TestGatedBad(t *testing.T) { t.Fatal(\"tagged boom\") }\n",
		"twotags/both_test.go":  "//go:build acs && integration\n\npackage twotags\n\nimport \"testing\"\n\nfunc TestBoth(t *testing.T) { t.Fatal(\"both tags reached\") }\n",
	}
	for rel, body := range files {
		p := filepath.Join(mod, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return mod
}

func TestRealGoUnitTestTagged_RunsTheTaggedTests(t *testing.T) {
	t.Parallel()

	mod := taggedModule(t)
	cases := []struct {
		name       string
		pkg        string
		tags       []string
		wantPassed bool
		wantOut    string
	}{
		{"tagged pass", "./gated", []string{"acs"}, true, "ok"},
		{"tagged fail", "./gatedred", []string{"acs"}, false, "tagged boom"},
		{"every tag is passed through", "./twotags", []string{"acs", "integration"}, false, "both tags reached"},
		{"missing tag excludes the files", "./gatedred", []string{"other"}, false, "build constraints exclude all Go files"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, passed := realGoUnitTestTagged(context.Background(), mod, tc.pkg, tc.tags)

			if passed != tc.wantPassed || !strings.Contains(out, tc.wantOut) {
				t.Errorf("= (passed %v), want %v with %q in output:\n%s", passed, tc.wantPassed, tc.wantOut, out)
			}
		})
	}
}

func TestRealGoUnitTestTagged_CancelledContextFails(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, passed := realGoUnitTestTagged(ctx, taggedModule(t), "./gated", []string{"acs"})

	if passed {
		t.Error("passed = true, want a cancelled run to fail")
	}
}
