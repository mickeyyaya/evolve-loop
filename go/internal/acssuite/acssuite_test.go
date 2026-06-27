package acssuite

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRun_Validation covers the required-field guards.
func TestRun_Validation(t *testing.T) {
	if _, err := Run(Options{Cycle: 1}); err == nil {
		t.Error("want error for empty Root")
	}
	if _, err := Run(Options{Root: t.TempDir(), Cycle: 0}); err == nil {
		t.Error("want error for non-positive Cycle")
	}
}

// TestWriteVerdict_RoundTrip — a verdict built via record() writes atomically and
// re-parses to the schema the audit + ship gates read (red_count/green_count/
// verdict/predicate_suite.total).
func TestWriteVerdict_RoundTrip(t *testing.T) {
	v := Verdict{SchemaVersion: "1.0", Cycle: 2}
	v.record(Result{ACID: "cycle2/TestC2_001_Ok", Predicate: "go/acs/cycle2/...:TestC2_001_Ok", ResultStr: "green"})
	v.PredicateSuite.SkippedCount = v.SkipCount
	v.PredicateSuite.Total = len(v.Results)
	v.Verdict = "PASS"
	v.ShipEligible = true

	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	dst, err := WriteVerdict(evolveDir, v)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		RedCount       int    `json:"red_count"`
		GreenCount     int    `json:"green_count"`
		Verdict        string `json:"verdict"`
		PredicateSuite struct {
			Total int `json:"total"`
		} `json:"predicate_suite"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("re-parse acs-verdict.json: %v", err)
	}
	if got.Verdict != "PASS" || got.GreenCount != 1 || got.RedCount != 0 || got.PredicateSuite.Total != 1 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

// TestWriteVerdict_MkdirError — a non-directory evolveDir surfaces an error
// rather than silently dropping the verdict.
func TestWriteVerdict_MkdirError(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// evolveDir under a regular file → MkdirAll fails.
	if _, err := WriteVerdict(filepath.Join(blocker, "evolve"), Verdict{Cycle: 1}); err == nil {
		t.Error("want error when evolveDir cannot be created")
	}
}

// TestWriteVerdict_RenameError — when the destination acs-verdict.json already
// exists as a DIRECTORY, the final os.Rename(tmp, dst) cannot complete, so
// WriteVerdict surfaces a "rename" error rather than reporting success.
func TestWriteVerdict_RenameError(t *testing.T) {
	evolveDir := t.TempDir()
	collide := filepath.Join(evolveDir, "runs", "cycle-1", "acs-verdict.json")
	if err := os.MkdirAll(collide, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteVerdict(evolveDir, Verdict{Cycle: 1}); err == nil {
		t.Fatal("WriteVerdict must error when the destination cannot be renamed onto")
	} else if !strings.Contains(err.Error(), "rename") {
		t.Errorf("error must carry the 'rename' context; got %q", err.Error())
	}
}

// TestWriteVerdict_CreateTempError — the cycle dir exists but is read-only:
// MkdirAll is a no-op (dir present), then os.CreateTemp cannot create the temp
// file, so WriteVerdict surfaces a "create tmp" error rather than silently
// dropping the verdict.
func TestWriteVerdict_CreateTempError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("read-only-dir permission denial does not hold for root")
	}
	evolveDir := t.TempDir()
	cycleDir := filepath.Join(evolveDir, "runs", "cycle-1")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cycleDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cycleDir, 0o755) })

	if _, err := WriteVerdict(evolveDir, Verdict{Cycle: 1}); err == nil {
		t.Fatal("WriteVerdict must error when the temp file cannot be created")
	} else if !strings.Contains(err.Error(), "create tmp") {
		t.Errorf("error must carry the 'create tmp' context; got %q", err.Error())
	}
}

func TestWriteVerdict_CloseError(t *testing.T) {
	old := writeVerdictClose
	t.Cleanup(func() { writeVerdictClose = old })
	writeVerdictClose = func(f *os.File) error {
		_ = old(f)
		return errors.New("close failed")
	}

	if _, err := WriteVerdict(t.TempDir(), Verdict{Cycle: 1}); err == nil {
		t.Fatal("WriteVerdict must surface temp close errors")
	} else if !strings.Contains(err.Error(), "close tmp") {
		t.Fatalf("error = %v, want close tmp context", err)
	}
}

func TestWriteVerdict_WriteFileError(t *testing.T) {
	old := writeVerdictWriteFile
	t.Cleanup(func() { writeVerdictWriteFile = old })
	writeVerdictWriteFile = func(string, []byte, os.FileMode) error { return errors.New("write failed") }

	if _, err := WriteVerdict(t.TempDir(), Verdict{Cycle: 1}); err == nil {
		t.Fatal("WriteVerdict must surface temp write errors")
	} else if !strings.Contains(err.Error(), "write") {
		t.Fatalf("error = %v, want write context", err)
	}
}
