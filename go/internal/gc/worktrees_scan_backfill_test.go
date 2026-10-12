package gc

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanWorktrees_StatusFailureAbortsTheWholeScan(t *testing.T) {
	base := t.TempDir()
	evolve := t.TempDir()
	for _, leaf := range []string{"cycle-1", "cycle-2"} {
		if err := os.MkdirAll(filepath.Join(base, leaf), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	porcelain := "worktree " + filepath.Join(base, "cycle-1") + "\nbranch refs/heads/b1\n\n" +
		"worktree " + filepath.Join(base, "cycle-2") + "\nbranch refs/heads/b2\n"
	var statusDirs []string
	o := WorktreeOptions{
		WorktreeBase: base,
		EvolveDir:    evolve,
		Exec: func(_ context.Context, _, dir string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
			if len(args) > 0 && args[0] == "status" {
				statusDirs = append(statusDirs, dir)
				if strings.HasSuffix(dir, "cycle-2") {
					return 0, errors.New("git vanished")
				}
			}
			return 0, nil
		},
	}

	items, pool, seen, err := o.scanWorktrees(porcelain, &mergedSet{o: o, ancestors: map[string]bool{"b1": true, "b2": true}, memo: map[string]landing{}})

	if err == nil || !strings.Contains(err.Error(), "gc: git status --porcelain: git vanished") {
		t.Fatalf("err = %v, want the status failure", err)
	}
	if items != nil || pool != nil || seen != nil {
		t.Errorf("got items=%v pool=%v seen=%v, want all nil on error (no partial plan)", items, pool, seen)
	}
	if len(statusDirs) != 2 {
		t.Errorf("status ran in %v, want both worktrees probed before the abort", statusDirs)
	}
}

func TestEligibleAfterGrace(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	minAge := 90 * time.Minute
	cases := []struct {
		name   string
		age    time.Duration
		minAge time.Duration
		want   bool
	}{
		{"older than the grace", minAge + time.Minute, minAge, true},
		{"exactly the grace", minAge, minAge, true},
		{"one second inside the grace", minAge - time.Second, minAge, false},
		{"fresh", 0, minAge, false},
		{"no grace accepts a fresh dir", 0, 0, true},
		{"no grace accepts a future mtime", -time.Hour, 0, true},
		{"negative grace behaves as none", 0, -time.Minute, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cycle-9")
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
			mtime := now.Add(-tc.age)
			if err := os.Chtimes(path, mtime, mtime); err != nil {
				t.Fatal(err)
			}

			got, ok := eligibleAfterGrace(path, "br", now, tc.minAge)

			if ok != tc.want {
				t.Fatalf("ok = %v, want %v", ok, tc.want)
			}
			if ok && (got.path != path || got.branch != "br" || !got.mtime.Equal(mtime)) {
				t.Errorf("eligible = %+v, want path %s branch br mtime %v", got, path, mtime)
			}
			if !ok && got != (eligible{}) {
				t.Errorf("eligible = %+v, want zero when not eligible", got)
			}
		})
	}
}

func TestEligibleAfterGrace_VanishedWorktreeIsNotEligible(t *testing.T) {
	got, ok := eligibleAfterGrace(filepath.Join(t.TempDir(), "gone"), "br", time.Now(), 0)

	if ok || got != (eligible{}) {
		t.Errorf("eligibleAfterGrace(missing) = (%+v, %v), want (zero, false)", got, ok)
	}
}
