package gc

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestLaneHoldsPausedAndBound(t *testing.T) {
	root := t.TempDir()
	if err := continuation.WriteRegistryEntry(root, "item-1", continuation.Continuation{Branch: "cycle-x-7", Cycle: 7}); err != nil {
		t.Fatal(err)
	}
	o := WorktreeOptions{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve")}
	held, err := o.laneHolds()
	if err != nil {
		t.Fatalf("laneHolds: %v", err)
	}
	if !held.boundBranches["cycle-x-7"] {
		t.Errorf("boundBranches = %v, want cycle-x-7", held.boundBranches)
	}
}

func TestLaneHoldsFailsOnUnreadableSources(t *testing.T) {
	cases := []struct {
		name  string
		setup func(root string)
		want  string
	}{
		{"paused", func(root string) {
			if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".evolve", "runs"), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "gc: read paused lanes"},
		{"registry", func(root string) {
			p := continuation.RegistryPath(root)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "gc: read continuation registry"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			c.setup(root)
			o := WorktreeOptions{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve")}
			if _, err := o.laneHolds(); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %q", err, c.want)
			}
		})
	}
}

func TestLaneHoldsHolds(t *testing.T) {
	h := laneHolds{pausedCycles: map[int]bool{7: true}, boundBranches: map[string]bool{"cycle-x-9": true}}
	cases := []struct {
		path, branch string
		want         bool
	}{
		{"/w/cycle-x-7", "cycle-x-7", true},
		{"/w/cycle-x-8", "cycle-x-9", true},
		{"/w/cycle-x-8", "cycle-x-8", false},
		{"/w/cycle-nonum", "cycle-nonum", false},
	}
	for _, c := range cases {
		if got := h.holds(c.path, c.branch); got != c.want {
			t.Errorf("holds(%s, %s) = %v, want %v", c.path, c.branch, got, c.want)
		}
	}
}

func TestSalvageReasonByLaneAge(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	base := t.TempDir()
	lane := func(name string, age time.Duration) string {
		p := filepath.Join(base, name)
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
		return p
	}
	o := WorktreeOptions{ProjectRoot: t.TempDir(), Policy: WorktreesPolicy{SalvageAfterHours: 24}, Now: func() time.Time { return now }}
	held := laneHolds{pausedCycles: map[int]bool{}, boundBranches: map[string]bool{"kept": true}}
	cases := []struct {
		name   string
		path   string
		branch string
		want   bool
	}{
		{"old", lane("cycle-x-1", 25*time.Hour), "b", true},
		{"young", lane("cycle-x-2", 23*time.Hour), "b", false},
		{"bound", lane("cycle-x-3", 48*time.Hour), "kept", false},
		{"missing", filepath.Join(base, "cycle-x-4"), "b", false},
	}
	for _, c := range cases {
		reason, ok := o.salvageReason(c.path, c.branch, held)
		if ok != c.want {
			t.Errorf("%s: ok = %v, want %v", c.name, ok, c.want)
		}
		if ok && !strings.Contains(reason, "lane ended") {
			t.Errorf("%s: reason %q does not say the lane ended", c.name, reason)
		}
	}
}

func TestLaneHoldsReadsAQuotaPausedCycle(t *testing.T) {
	repo := gittest.Fixture(t)
	root := repo.Dir
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1700")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scope, err := json.Marshal(core.LaneScope{TodoIDs: []string{"item-1700"}, GoalHash: "wave-goal"})
	if err != nil {
		t.Fatal(err)
	}
	state, err := json.Marshal(map[string]any{
		"cycle_id": 1700,
		"phase":    "build",
		"checkpoint": map[string]any{
			"enabled":               true,
			"reason":                "quota-likely",
			"resumeFromPhase":       "build",
			"worktreePath":          root,
			"savedAt":               time.Now().UTC().Format(time.RFC3339),
			"autoResumeAttempts":    0,
			"autoResumeMaxAttempts": 3,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{core.LaneScopeFile: scope, core.CycleStateFile: state} {
		if err := os.WriteFile(filepath.Join(runDir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	o := WorktreeOptions{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve")}

	held, err := o.laneHolds()

	if err != nil || !held.pausedCycles[1700] {
		t.Fatalf("pausedCycles = %v, err = %v, want cycle 1700 held", held.pausedCycles, err)
	}
}

func TestPlanWorktreesFailsWhenALaneHoldSourceIsUnreadable(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "runs"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := WorktreeOptions{
		ProjectRoot: root,
		EvolveDir:   filepath.Join(root, ".evolve"),
		Exec: func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
			return 0, nil
		},
	}

	if _, err := PlanWorktrees(o); err == nil || !strings.Contains(err.Error(), "gc: read paused lanes") {
		t.Fatalf("err = %v, want the paused-lane read failure", err)
	}
}
