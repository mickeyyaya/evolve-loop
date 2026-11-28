package codeindex

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestApicover_IndependentTruthAndFixture(t *testing.T) {
	t.Parallel()
	symbols, err := IndependentTruth()
	if err != nil || len(symbols) == 0 {
		t.Fatalf("IndependentTruth() = %v, %v; want non-empty symbols", symbols, err)
	}
	dir := t.TempDir()
	if err := WriteIndependentFixture(dir); err != nil {
		t.Fatalf("WriteIndependentFixture: %v", err)
	}
	for _, rel := range []string{filepath.Join("go", "go.mod"), filepath.Join("go", "work", "work.go")} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("missing fixture file %s: %v", rel, err)
		}
	}
	if _, err := parseIndependentTruth([]byte("{")); err == nil {
		t.Error("parseIndependentTruth on invalid JSON must fail")
	}
	fileTarget := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(fileTarget, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteIndependentFixture(fileTarget); err == nil {
		t.Error("WriteIndependentFixture into a regular file must fail")
	}
	dirTarget := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dirTarget, "go", "go.mod"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteIndependentFixture(dirTarget); err == nil {
		t.Error("WriteIndependentFixture over an existing directory at go.mod must fail")
	}
	run := BenchRun{TruthSet: "independent", Now: time.Now, LoadAvg: func() float64 { return 0 }}
	report, err := RunBench(context.Background(), run)
	if err != nil || len(report.Rows) != 0 {
		t.Fatalf("RunBench = %+v, %v", report, err)
	}
	row := BenchRow{TruthSet: run.TruthSet, Backend: "b", Query: QueryCallers}
	if row.TruthSet != "independent" {
		t.Errorf("row.TruthSet = %q, want independent", row.TruthSet)
	}
}
