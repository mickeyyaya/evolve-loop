//go:build integration

package acssuite

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRun_EmptyActiveScopeCannotHideBehindPassingScope(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"go/go.mod":                       "module fixture\n\ngo 1.23\n",
		"go/acs/cycle7/acs_test.go":       "package cycle7\nimport (\"testing\"; \"os\")\nfunc TestMain(*testing.M) { os.Exit(0) }\nfunc TestRequired(t *testing.T) {}\n",
		"go/acs/regression/a/acs_test.go": "package a\nimport \"testing\"\nfunc TestGreen(t *testing.T) {}\n",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	v, err := Run(Options{Root: root, Cycle: 7})
	if err == nil && v.ShipEligible {
		t.Fatal("required cycle predicate never ran, but another scope laundered its absence")
	}
}

func TestRun_FilteredDeclaredPredicateCannotDisappear(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go/go.mod":                 "module fixture\n\ngo 1.23\n",
		"go/acs/cycle7/acs_test.go": "package cycle7\nimport (\"testing\"; \"os\"; \"flag\")\nfunc TestMain(m *testing.M) { flag.Set(\"test.run\", \"^TestGreen$\"); os.Exit(m.Run()) }\nfunc TestGreen(t *testing.T) {}\nfunc TestRequired(t *testing.T) { t.Fatal(\"must execute\") }\n",
	}
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	v, err := Run(Options{Root: root, Cycle: 7})
	if err == nil && v.ShipEligible {
		t.Fatal("nonempty passing subset hid a declared required predicate")
	}
}
