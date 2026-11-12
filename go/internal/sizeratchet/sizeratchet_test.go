package sizeratchet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestLoad_ReturnsTheSpansAndTheOffendersThatCheckJudges(t *testing.T) {
	root := t.TempDir()
	for rel, text := range map[string]string{"p/p.go": "package p\n\nfunc F() {\n" + body(MaxLines+10) + "}\n", OffendersRelPath: `{"p.F": 100}`} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	spans, offenders, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 1 || spans[0].Key != "p.F" || offenders["p.F"] != 100 {
		t.Errorf("Load = %v, %v, want the span p.F and its allowance 100", spans, offenders)
	}
	if spans, offenders, err := Load(filepath.Join(root, "absent")); err == nil || spans != nil || offenders != nil {
		t.Errorf("Load over a missing root = %v, %v, %v, want only an error", spans, offenders, err)
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
