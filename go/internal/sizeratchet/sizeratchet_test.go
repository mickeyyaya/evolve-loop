package sizeratchet

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRatchet_ModuleFunctionsFitTheirAllowances is the repo-wide gate: every
// non-test function in the module is within MaxLines or its listed allowance;
// an allowance is a ceiling, so a shrunk or stale entry is slack.
func TestRatchet_ModuleFunctionsFitTheirAllowances(t *testing.T) {
	spans, err := Walk(moduleRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	offenders, err := LoadOffenders("offenders.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(spans, offenders); err != nil {
		t.Fatal(err)
	}
}

// moduleRoot walks up from the package directory to the nearest go.mod, so the
// ratchet reads whatever module it is tested in, never a path from outside it.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the package directory")
		}
		dir = parent
	}
}
