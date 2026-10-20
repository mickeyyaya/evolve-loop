package gc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/guardslog"
)

func guardsLogFixture(t *testing.T, bytes int) (evolveDir, path string) {
	t.Helper()
	evolveDir = t.TempDir()
	path = filepath.Join(evolveDir, guardsLogName)
	if err := os.WriteFile(path, []byte(strings.Repeat("x", bytes)), 0o644); err != nil {
		t.Fatal(err)
	}
	return evolveDir, path
}

func TestPlan_GuardsLogOverSizeIsArchived(t *testing.T) {
	t.Parallel()
	evolveDir, path := guardsLogFixture(t, 2<<20)
	m, err := Plan(Options{EvolveDir: evolveDir, Policy: Policy{GuardsLogMaxMB: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Items) != 1 || m.Items[0] != (Item{Path: path, Action: ActionRotate, Rule: "guards_log_max_mb"}) {
		t.Fatalf("items = %+v, want one rotate of %s", m.Items, path)
	}
}

func TestPlan_GuardsLogUnderSizeUntouched(t *testing.T) {
	t.Parallel()
	evolveDir, _ := guardsLogFixture(t, 1<<20)
	if m, err := Plan(Options{EvolveDir: evolveDir, Policy: Policy{GuardsLogMaxMB: 1}}); err != nil || len(m.Items) != 0 {
		t.Fatalf("items = %+v, err %v; a log at the cap must stay", m.Items, err)
	}
	if m, err := Plan(Options{EvolveDir: t.TempDir()}); err != nil || len(m.Items) != 0 {
		t.Fatalf("no guards.log: items = %+v, err %v", m.Items, err)
	}
}

func TestApply_ARotatedGuardsLogLosesNoLineAndAgesOutWithTheDispatchLogs(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	path := filepath.Join(evolveDir, guardsLogName)
	ts := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if err := guardslog.Append(path, "guard", "before", ts); err != nil {
			t.Fatal(err)
		}
	}
	for round := 0; round < 2; round++ {
		if err := Apply(evolveDir, Manifest{Items: []Item{{Path: path, Action: ActionRotate, Rule: "guards_log_max_mb"}}}); err != nil {
			t.Fatal(err)
		}
		if err := guardslog.Append(path, "guard", "after", ts); err != nil {
			t.Fatal(err)
		}
	}
	rotated, _ := filepath.Glob(filepath.Join(evolveDir, "dispatch-logs", "guards-*.log"))
	if len(rotated) != 2 {
		t.Fatalf("rotated = %v, want two archives (the second takes a unique name)", rotated)
	}
	lines := 0
	for _, p := range append(rotated, path) {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		lines += strings.Count(string(raw), "\n")
	}
	if lines != 5 {
		t.Errorf("%d lines across the live log and its archives, want the 5 appended", lines)
	}
	cat := Policy{}.LogCatalog()
	if !matchesAnyHome(cat[0].Homes, "dispatch-logs", filepath.Base(rotated[0])) {
		t.Errorf("%s is not under a dispatch log home, so logs_ttl_days never ages it", rotated[0])
	}
}

func TestApply_ARotateThatCannotMoveTheLogIsAnError(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(evolveDir, "dispatch-logs"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(evolveDir, guardsLogName)
	if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Apply(evolveDir, Manifest{Items: []Item{{Path: path, Action: ActionRotate}}})
	if err == nil || !strings.Contains(err.Error(), "gc: rotate "+path) {
		t.Errorf("err = %v, want the rotate failure", err)
	}
}

func matchesAnyHome(homes []gcpolicy.LogHome, dir, name string) bool {
	for _, h := range homes {
		if ok, _ := filepath.Match(h.Glob, name); ok && h.Dir == dir {
			return true
		}
	}
	return false
}

func TestRotateItem_AMissingLogIsARenameError(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	path := filepath.Join(evolveDir, guardsLogName)
	err := rotateItem(evolveDir, path, time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "gc: rotate "+path+" → ") {
		t.Errorf("err = %v, want the rename failure that names both paths", err)
	}
}
