package evalgate

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCycleNumFromWorkspace_RejectsAlmostCycleBasenames(t *testing.T) {
	t.Parallel()

	cases := []string{
		filepath.Join(t.TempDir(), "cycle-"),
		filepath.Join(t.TempDir(), "cycle--1"),
		filepath.Join(t.TempDir(), "prefix-cycle-7"),
		filepath.Join(t.TempDir(), "cycle-7-extra"),
	}

	for _, workspace := range cases {
		workspace := workspace
		t.Run(filepath.Base(workspace), func(t *testing.T) {
			t.Parallel()
			if got := cycleNumFromWorkspace(workspace); got != 0 {
				t.Fatalf("cycleNumFromWorkspace(%q) = %d, want 0", workspace, got)
			}
		})
	}
}

func TestCycleNumFromWorkspace_RejectsVeryLargeDigitRuns(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join(t.TempDir(), "cycle-"+strings.Repeat("9", 256))
	if got := cycleNumFromWorkspace(workspace); got != 0 {
		t.Fatalf("cycleNumFromWorkspace(%q) = %d, want 0", filepath.Base(workspace), got)
	}
}
