package acssuite

// acssuite_adversarial_test.go — cycle-281 test amplification.
// Targets uncovered branches: cycleNumFromDir (66.7%), goLaneTimeout (62.5%),
// predicateEnv (66.7%), goLanePatterns (50.0%), excerpt (75.0%),
// WriteVerdict (83.3%), changedPackagesForCycle (28.6%).

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
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

// TestGoLaneTimeout_AllBranches — adversarial: exercises the three branches of
// goLaneTimeout (opts>0 wins, cfg.GoTimeoutS>0 wins, zero cfg falls back).
func TestGoLaneTimeout_AllBranches(t *testing.T) {
	t.Run("opts > 0 wins over cfg and default", func(t *testing.T) {
		got := goLaneTimeout(15*time.Second, policy.ACSConfig{GoTimeoutS: 3600})
		if got != 15*time.Second {
			t.Errorf("opts must win; got %v", got)
		}
	})
	t.Run("opts=0 cfg.GoTimeoutS=30 yields 30s", func(t *testing.T) {
		got := goLaneTimeout(0, policy.ACSConfig{GoTimeoutS: 30})
		if got != 30*time.Second {
			t.Errorf("cfg=30 must produce 30s; got %v", got)
		}
	})
	t.Run("opts=0 cfg.GoTimeoutS=0 falls back to DefaultTimeout", func(t *testing.T) {
		got := goLaneTimeout(0, policy.ACSConfig{GoTimeoutS: 0})
		if got != DefaultTimeout {
			t.Errorf("zero cfg must fall back to DefaultTimeout; got %v", got)
		}
	})
	t.Run("opts=0 empty ACSConfig falls back to DefaultTimeout", func(t *testing.T) {
		got := goLaneTimeout(0, policy.ACSConfig{})
		if got != DefaultTimeout {
			t.Errorf("empty ACSConfig must fall back to DefaultTimeout; got %v", got)
		}
	})
	t.Run("empty ACSConfig returns positive DefaultTimeout (not zero/panic)", func(t *testing.T) {
		got := goLaneTimeout(0, policy.ACSConfig{})
		if got <= 0 {
			t.Errorf("empty cfg must return positive DefaultTimeout; got %v", got)
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

func TestHasGoACSTree_GoModDirectoryIsNotModule(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "go.mod"), 0o755); err != nil {
		t.Fatal(err)
	}
	if hasGoACSTree(dir) {
		t.Fatal("go.mod directory must not be treated as a Go ACS module")
	}
}

func TestHasGoACSTree_ACSPathMustBeDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "acs"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if hasGoACSTree(dir) {
		t.Fatal("acs file must not be treated as a Go ACS predicate tree")
	}
}

func TestDefaultGoExec_RunsGoTestJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/acs\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(dir, "acs", "cycle1")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	testFile := `package cycle1

import "testing"

func TestPredicate(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "predicate_test.go"), []byte(testFile), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := defaultGoExec(context.Background(), dir, "./acs/cycle1", os.Environ())
	if err != nil {
		t.Fatalf("defaultGoExec: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"Test":"TestPredicate"`) {
		t.Fatalf("go test json missing predicate event: %s", out)
	}
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
