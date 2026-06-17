package acssuite

// acssuite_adversarial_test.go — cycle-281 test amplification.
// Targets uncovered branches: cycleNumFromDir (66.7%), goLaneTimeout (62.5%),
// predicateEnv (66.7%), goLanePatterns (50.0%), excerpt (75.0%),
// WriteVerdict (83.3%), changedPackagesForCycle (28.6%).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCycleNumFromDir_BoundaryAndInvalid — adversarial: exhaustive boundary
// and invalid inputs for the cycle number parser.
func TestCycleNumFromDir_BoundaryAndInvalid(t *testing.T) {
	cases := []struct {
		dir    string
		wantN  int
		wantOK bool
	}{
		{"cycle281", 281, true},
		{"cycle1", 1, true},
		{"cycle0", 0, true},
		{"cycle99999", 99999, true},
		// Edge: "cycle" with nothing after it — empty Atoi → error.
		{"cycle", 0, false},
		// Non-numeric suffix.
		{"cycleabc", 0, false},
		{"cycledefense1", 0, false},
		// Negative cycles: Atoi parses "-1" as -1 (valid but odd).
		{"cycle-1", -1, true},
		// Wrong prefix.
		{"notcycle1", 0, false},
		{"CYCLE1", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		n, ok := cycleNumFromDir(c.dir)
		if ok != c.wantOK || n != c.wantN {
			t.Errorf("cycleNumFromDir(%q) = (%d, %v), want (%d, %v)",
				c.dir, n, ok, c.wantN, c.wantOK)
		}
	}
}

// TestGoLaneTimeout_AllBranches — adversarial: exercises the four branches of
// goLaneTimeout (opts>0 wins, env-valid wins, env-invalid falls back,
// nil-envGet falls back).
func TestGoLaneTimeout_AllBranches(t *testing.T) {
	t.Run("opts > 0 wins over env and default", func(t *testing.T) {
		env := func(string) string { return "3600" }
		got := goLaneTimeout(15*time.Second, env)
		if got != 15*time.Second {
			t.Errorf("opts must win; got %v", got)
		}
	})
	t.Run("opts=0 env valid integer yields that duration", func(t *testing.T) {
		env := func(k string) string {
			if k == "EVOLVE_ACS_GO_TIMEOUT_S" {
				return "30"
			}
			return ""
		}
		got := goLaneTimeout(0, env)
		if got != 30*time.Second {
			t.Errorf("env=30 must produce 30s; got %v", got)
		}
	})
	t.Run("opts=0 env non-integer falls back to DefaultTimeout", func(t *testing.T) {
		env := func(k string) string {
			if k == "EVOLVE_ACS_GO_TIMEOUT_S" {
				return "notanumber"
			}
			return ""
		}
		got := goLaneTimeout(0, env)
		if got != DefaultTimeout {
			t.Errorf("invalid env must fall back to DefaultTimeout; got %v", got)
		}
	})
	t.Run("opts=0 env=0 (not positive) falls back to DefaultTimeout", func(t *testing.T) {
		env := func(k string) string {
			if k == "EVOLVE_ACS_GO_TIMEOUT_S" {
				return "0"
			}
			return ""
		}
		got := goLaneTimeout(0, env)
		if got != DefaultTimeout {
			t.Errorf("env=0 is not positive, must fall back; got %v", got)
		}
	})
	t.Run("opts=0 env empty string falls back to DefaultTimeout", func(t *testing.T) {
		got := goLaneTimeout(0, func(string) string { return "" })
		if got != DefaultTimeout {
			t.Errorf("empty env must fall back; got %v", got)
		}
	})
	t.Run("nil envGet falls back gracefully without panic", func(t *testing.T) {
		got := goLaneTimeout(0, nil)
		if got <= 0 {
			t.Errorf("nil envGet must return positive duration; got %v", got)
		}
	})
}

// TestExcerpt_AllBranches — adversarial: empty, at limit, and over limit.
func TestExcerpt_AllBranches(t *testing.T) {
	t.Run("empty string returns empty", func(t *testing.T) {
		if got := excerpt(""); got != "" {
			t.Errorf(`excerpt("") = %q, want ""`, got)
		}
	})
	t.Run("whitespace-only trims to empty", func(t *testing.T) {
		if got := excerpt("   \n\t  "); got != "" {
			t.Errorf("excerpt(whitespace) = %q, want empty", got)
		}
	})
	t.Run("short string returned verbatim without ellipsis", func(t *testing.T) {
		s := "short content under limit"
		got := excerpt(s)
		if got != s {
			t.Errorf("excerpt(%q) = %q, want verbatim", s, got)
		}
		if strings.HasSuffix(got, "…") {
			t.Errorf("short content must not gain ellipsis: %q", got)
		}
	})
	t.Run("string exactly at limit returned without ellipsis", func(t *testing.T) {
		exact := strings.Repeat("a", evidenceMax)
		got := excerpt(exact)
		if strings.HasSuffix(got, "…") {
			t.Error("string exactly at evidenceMax must not be truncated")
		}
		if got != exact {
			t.Errorf("at-limit string must be returned verbatim")
		}
	})
	t.Run("string over limit gets truncated with ellipsis", func(t *testing.T) {
		long := strings.Repeat("x", evidenceMax+50)
		got := excerpt(long)
		if !strings.HasSuffix(got, "…") {
			t.Errorf("over-limit excerpt must end with ellipsis; got suffix %q", got[max(0, len(got)-5):])
		}
	})
}

// TestPredicateEnv_AllBranches — adversarial: verify every env injection path.
func TestPredicateEnv_AllBranches(t *testing.T) {
	t.Run("empty projectRoot and nil pkgs: neither var injected", func(t *testing.T) {
		env := predicateEnv("", "", nil)
		for _, e := range env {
			if strings.HasPrefix(e, "CHANGED_PACKAGES=") {
				t.Errorf("nil pkgs must not inject CHANGED_PACKAGES; got %q", e)
			}
			if strings.HasPrefix(e, "EVOLVE_PROJECT_ROOT=") {
				// empty root must not inject the var
				v := strings.TrimPrefix(e, "EVOLVE_PROJECT_ROOT=")
				if v == "" {
					t.Errorf("empty projectRoot must not inject EVOLVE_PROJECT_ROOT; got %q", e)
				}
			}
			if strings.HasPrefix(e, "EVOLVE_WORKTREE_ROOT=") {
				t.Errorf("empty worktreeRoot must not inject EVOLVE_WORKTREE_ROOT; got %q", e)
			}
		}
	})
	t.Run("non-empty projectRoot injects EVOLVE_PROJECT_ROOT", func(t *testing.T) {
		env := predicateEnv("/the/root", "", nil)
		var found bool
		for _, e := range env {
			if e == "EVOLVE_PROJECT_ROOT=/the/root" {
				found = true
			}
		}
		if !found {
			t.Error("EVOLVE_PROJECT_ROOT=/the/root not found in env")
		}
	})
	t.Run("non-empty worktreeRoot injects EVOLVE_WORKTREE_ROOT (dual-root)", func(t *testing.T) {
		env := predicateEnv("/main", "/the/worktree", nil)
		var gotProject, gotWorktree bool
		for _, e := range env {
			switch e {
			case "EVOLVE_PROJECT_ROOT=/main":
				gotProject = true
			case "EVOLVE_WORKTREE_ROOT=/the/worktree":
				gotWorktree = true
			}
		}
		if !gotProject {
			t.Error("EVOLVE_PROJECT_ROOT=/main not found in env")
		}
		if !gotWorktree {
			t.Error("EVOLVE_WORKTREE_ROOT=/the/worktree not found in env (source root not exported)")
		}
	})
	t.Run("non-empty changedPkgs injects CHANGED_PACKAGES space-joined", func(t *testing.T) {
		pkgs := []string{"./internal/core", "./internal/bridge"}
		env := predicateEnv("/root", "", pkgs)
		var found string
		for _, e := range env {
			if strings.HasPrefix(e, "CHANGED_PACKAGES=") {
				found = strings.TrimPrefix(e, "CHANGED_PACKAGES=")
				break
			}
		}
		if found == "" {
			t.Fatal("CHANGED_PACKAGES not found in env")
		}
		for _, pkg := range pkgs {
			if !strings.Contains(found, pkg) {
				t.Errorf("CHANGED_PACKAGES=%q missing %q", found, pkg)
			}
		}
	})
}

// TestWriteVerdict_RoundTripAdversarial — adversarial: write a Verdict and
// read it back; the round-trip must be faithful and path under the cycle dir.
func TestWriteVerdict_RoundTripAdversarial(t *testing.T) {
	evolveDir := t.TempDir()
	v := Verdict{
		SchemaVersion: "1",
		Cycle:         99,
		GreenCount:    7,
		RedCount:      0,
		SkipCount:     3,
		Verdict:       "PASS",
		ShipEligible:  true,
	}
	path, err := WriteVerdict(evolveDir, v)
	if err != nil {
		t.Fatalf("WriteVerdict: %v", err)
	}
	if !strings.HasPrefix(path, filepath.Join(evolveDir, "runs", "cycle-99")) {
		t.Errorf("path %q must be under runs/cycle-99", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back verdict file: %v", err)
	}
	var got Verdict
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Cycle != 99 || got.GreenCount != 7 || got.RedCount != 0 || got.Verdict != "PASS" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

// TestChangedPackagesForCycle_NilOnMissingInputs — adversarial: empty root and
// nonexistent root both return nil (best-effort, never an error).
func TestChangedPackagesForCycle_NilOnMissingInputs(t *testing.T) {
	if got := changedPackagesForCycle("", 42); got != nil {
		t.Errorf("empty projectRoot must return nil; got %v", got)
	}
	if got := changedPackagesForCycle("/nonexistent/path/that/cannot/exist", 99); got != nil {
		t.Errorf("nonexistent projectRoot must return nil; got %v", got)
	}
}

// TestGoLanePatterns_NoACSTree — adversarial: a module dir with no acs/
// subtree at all must return an empty pattern slice without panicking.
func TestGoLanePatterns_NoACSTree(t *testing.T) {
	dir := t.TempDir()
	pats := goLanePatterns(dir, 1)
	if len(pats) != 0 {
		t.Errorf("no acs/ tree → empty patterns; got %v", pats)
	}
}

// TestGoLanePatterns_CycleAndRegressionPresent — adversarial: when both the
// cycle package dir and a regression sub-package dir exist, both patterns are
// included.
func TestGoLanePatterns_CycleAndRegressionPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "acs", "cycle5"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "acs", "regression", "sub1"), 0o755); err != nil {
		t.Fatal(err)
	}
	pats := goLanePatterns(dir, 5)
	hasCycle, hasRegression := false, false
	for _, p := range pats {
		if strings.Contains(p, "cycle5") {
			hasCycle = true
		}
		if strings.Contains(p, "regression/sub1") {
			hasRegression = true
		}
	}
	if !hasCycle {
		t.Error("cycle5 dir present → must produce a cycle5 pattern")
	}
	if !hasRegression {
		t.Errorf("regression/sub1 dir present → must produce that regression pattern; pats=%v", pats)
	}
}

// TestGoLanePatterns_OnlyRedteamPresent — adversarial: no cycle dir, no
// regression, but redteam dir exists → exactly the redteam pattern.
func TestGoLanePatterns_OnlyRedteamPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "acs", "redteam"), 0o755); err != nil {
		t.Fatal(err)
	}
	pats := goLanePatterns(dir, 100)
	if len(pats) != 1 || !strings.Contains(pats[0], "redteam") {
		t.Errorf("only redteam present → exactly 1 redteam pattern; got %v", pats)
	}
}
