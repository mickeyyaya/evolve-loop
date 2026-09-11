package acssuite

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHighestCyclePackageNumber(t *testing.T) {
	moduleDir := t.TempDir()
	acsDir := filepath.Join(moduleDir, "acs")
	for _, name := range []string{
		"cycle1",
		"cycle41",
		"cycle",
		"cycle0",
		"cycle-2",
		"cycle0042",
		"cycle58-backup",
		"cycle999999999999999999999999999999",
		"redteam",
		"regression",
	} {
		if err := os.MkdirAll(filepath.Join(acsDir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Any occupied canonical name is a collision, even when corruption left a
	// file where the cycle package directory should be.
	if err := os.WriteFile(filepath.Join(acsDir, "cycle57"), []byte("occupied\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := HighestCyclePackageNumber(moduleDir)
	if err != nil {
		t.Fatalf("HighestCyclePackageNumber: %v", err)
	}
	if got != 57 {
		t.Fatalf("HighestCyclePackageNumber = %d, want 57", got)
	}
}

func TestHighestCyclePackageNumber_MissingTree(t *testing.T) {
	got, err := HighestCyclePackageNumber(t.TempDir())
	if err != nil {
		t.Fatalf("HighestCyclePackageNumber: %v", err)
	}
	if got != 0 {
		t.Fatalf("HighestCyclePackageNumber = %d, want 0", got)
	}
}

func TestHighestCyclePackageNumber_ReportsUnreadableTree(t *testing.T) {
	moduleDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(moduleDir, "acs"), []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := HighestCyclePackageNumber(moduleDir); err == nil {
		t.Fatal("HighestCyclePackageNumber accepted an unreadable ACS tree")
	}
}

func TestCyclePackageOccupied(t *testing.T) {
	moduleDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(moduleDir, "acs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "acs", "cycle42"), []byte("occupied\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	occupied, err := CyclePackageOccupied(moduleDir, 42)
	if err != nil {
		t.Fatalf("CyclePackageOccupied: %v", err)
	}
	if !occupied {
		t.Fatal("CyclePackageOccupied returned false for an existing cycle42 path")
	}
	if err := os.Symlink("missing-target", filepath.Join(moduleDir, "acs", "cycle44")); err != nil {
		t.Fatal(err)
	}
	occupied, err = CyclePackageOccupied(moduleDir, 44)
	if err != nil {
		t.Fatalf("CyclePackageOccupied dangling symlink: %v", err)
	}
	if !occupied {
		t.Fatal("CyclePackageOccupied returned false for a dangling cycle44 symlink")
	}

	occupied, err = CyclePackageOccupied(moduleDir, 43)
	if err != nil {
		t.Fatalf("CyclePackageOccupied missing path: %v", err)
	}
	if occupied {
		t.Fatal("CyclePackageOccupied returned true for a missing cycle43 path")
	}
}
