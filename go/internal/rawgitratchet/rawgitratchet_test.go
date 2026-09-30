package rawgitratchet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// moduleRoot walks up from the working directory (the package directory under
// go test) to the directory holding go.mod, so a copy of the module scans itself.
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
			t.Fatal("no go.mod above the working directory")
		}
		dir = parent
	}
}

// TestRatchet_NoNewRawGitFixtures is the ratchet: every raw git init site in
// the module's bound test files must be listed in baseline.json, exactly.
func TestRatchet_NoNewRawGitFixtures(t *testing.T) {
	note, err := Scan(moduleRoot(t))
	if note != "" {
		t.Log(note)
	}
	if err != nil {
		t.Error(err)
	}
}

func TestScan_BindsFilesSitesAndBaselineFromOneRoot(t *testing.T) {
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
	write("p/raw_test.go", "package p\n\nimport \"os/exec\"\n\nfunc raw() { exec.Command(\"git\", \"init\").Run() }\n")
	write(BaselineRelPath, "{}")
	if _, err := Scan(root); err == nil || !strings.Contains(err.Error(), "p/raw_test.go") {
		t.Fatalf("Scan over an unlisted raw init = %v, want an error naming p/raw_test.go", err)
	}
	write(BaselineRelPath, `{"p/raw_test.go": 1}`)
	if _, err := Scan(root); err != nil {
		t.Errorf("Scan with the site baselined = %v, want nil", err)
	}
	if err := os.Remove(filepath.Join(root, BaselineRelPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(root); err == nil {
		t.Error("Scan without a baseline must fail loudly")
	}
}
