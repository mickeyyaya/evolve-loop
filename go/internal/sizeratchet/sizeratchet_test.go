package sizeratchet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRatchet_ModuleFunctionsFitTheirAllowances is the repo-wide gate: every
// non-test function in the module is within MaxLines or its listed allowance;
// an allowance is a ceiling, so a shrunk or stale entry is slack.
func TestRatchet_ModuleFunctionsFitTheirAllowances(t *testing.T) {
	if err := Scan(moduleRoot(t)); err != nil {
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

func TestScan_RunsWalkOffendersAndCheckFromOneRoot(t *testing.T) {
	root := t.TempDir()
	write := func(rel, text string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("p/p.go", "package p\n\nfunc F() {\n"+body(MaxLines+10)+"}\n")
	write(OffendersRelPath, "{}")
	if err := Scan(root); err == nil || !strings.Contains(err.Error(), "p.F") {
		t.Fatalf("Scan over an unlisted oversize function = %v, want an error naming p.F", err)
	}
	write(OffendersRelPath, `{"p.F": 100}`)
	if err := Scan(root); err != nil {
		t.Errorf("Scan within the allowance = %v, want nil", err)
	}
	if err := os.Remove(filepath.Join(root, OffendersRelPath)); err != nil {
		t.Fatal(err)
	}
	if err := Scan(root); err == nil {
		t.Error("Scan without an offender list must fail loudly")
	}
}
