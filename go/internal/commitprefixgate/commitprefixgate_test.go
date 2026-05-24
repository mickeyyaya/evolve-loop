package commitprefixgate

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// makeRepo writes a manifest at .evolve/commit-prefix-scope.json with the
// given prefix entries. Returns the repo dir.
func makeRepo(t *testing.T, manifestJSON string) string {
	t.Helper()
	d := t.TempDir()
	dir := filepath.Join(d, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if manifestJSON != "" {
		if err := os.WriteFile(filepath.Join(dir, "commit-prefix-scope.json"),
			[]byte(manifestJSON), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return d
}

// stubDiffPaths returns a GetDiffPaths seam that returns a fixed list.
func stubDiffPaths(paths []string) func(string, Mode, string) ([]string, error) {
	return func(string, Mode, string) ([]string, error) {
		return paths, nil
	}
}

// === Happy path: prefix matches scope =====================================
func TestRun_HappyPath(t *testing.T) {
	manifest := `{
  "prefixes": {
    "docs": {
      "required_paths": ["docs/**", "README.md", "CHANGELOG.md"],
      "diff_must_be_subset": true
    }
  }
}`
	repo := makeRepo(t, manifest)
	res, err := Run(Options{
		CommitMsg:    "docs: CHANGELOG update",
		RepoDir:      repo,
		Mode:         ModeStaged,
		GetDiffPaths: stubDiffPaths([]string{"CHANGELOG.md"}),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !res.Allowed {
		t.Error("want Allowed=true")
	}
	if res.Prefix != "docs" {
		t.Errorf("Prefix = %q, want 'docs'", res.Prefix)
	}
}

// === Required-paths violation: NO diff path under required_paths → DENY ====
func TestRun_RequiredPathsViolated(t *testing.T) {
	manifest := `{
  "prefixes": {
    "docs": {"required_paths": ["docs/**"]}
  }
}`
	repo := makeRepo(t, manifest)
	_, err := Run(Options{
		CommitMsg:    "docs: update something",
		RepoDir:      repo,
		Mode:         ModeStaged,
		GetDiffPaths: stubDiffPaths([]string{"go/internal/x.go"}),
	})
	if !errors.Is(err, ErrScopeViolation) {
		t.Fatalf("err = %v, want ErrScopeViolation", err)
	}
}

// === Forbidden-only violation: ALL diff paths under forbidden → DENY ======
func TestRun_ForbiddenOnlyViolated(t *testing.T) {
	manifest := `{
  "prefixes": {
    "feat": {"forbidden_only_paths": ["docs/**", "CHANGELOG.md"]}
  }
}`
	repo := makeRepo(t, manifest)
	_, err := Run(Options{
		CommitMsg:    "feat: docs-only commit mislabeled",
		RepoDir:      repo,
		Mode:         ModeStaged,
		GetDiffPaths: stubDiffPaths([]string{"docs/X.md", "CHANGELOG.md"}),
	})
	if !errors.Is(err, ErrScopeViolation) {
		t.Fatalf("err = %v, want ErrScopeViolation", err)
	}
}

// === Forbidden-only NOT violated when at least one real path ==============
func TestRun_ForbiddenOnly_HasRealPath(t *testing.T) {
	manifest := `{
  "prefixes": {
    "feat": {"forbidden_only_paths": ["docs/**"]}
  }
}`
	repo := makeRepo(t, manifest)
	res, err := Run(Options{
		CommitMsg:    "feat: real code change",
		RepoDir:      repo,
		Mode:         ModeStaged,
		GetDiffPaths: stubDiffPaths([]string{"docs/X.md", "go/internal/x.go"}),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !res.Allowed {
		t.Error("want Allowed=true")
	}
}

// === diff_must_be_subset enforces strict membership =======================
func TestRun_DiffMustBeSubset_Violated(t *testing.T) {
	manifest := `{
  "prefixes": {
    "docs": {
      "required_paths": ["docs/**"],
      "diff_must_be_subset": true
    }
  }
}`
	repo := makeRepo(t, manifest)
	_, err := Run(Options{
		CommitMsg:    "docs: but with code",
		RepoDir:      repo,
		Mode:         ModeStaged,
		GetDiffPaths: stubDiffPaths([]string{"docs/X.md", "go/internal/x.go"}),
	})
	if !errors.Is(err, ErrScopeViolation) {
		t.Fatalf("err = %v, want ErrScopeViolation", err)
	}
}

// === Bypass: manual class permits, cycle class denies =====================
func TestRun_Bypass_ManualClass(t *testing.T) {
	repo := makeRepo(t, "")
	res, err := Run(Options{
		CommitMsg:    "anything: msg",
		RepoDir:      repo,
		BypassEnv:    "1",
		ShipClass:    "manual",
		GetDiffPaths: stubDiffPaths(nil),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !res.Allowed {
		t.Error("want Allowed=true with manual-class bypass")
	}
}

func TestRun_Bypass_CycleClass(t *testing.T) {
	repo := makeRepo(t, "")
	_, err := Run(Options{
		CommitMsg:    "feat: msg",
		RepoDir:      repo,
		BypassEnv:    "1",
		ShipClass:    "cycle",
		GetDiffPaths: stubDiffPaths(nil),
	})
	if !errors.Is(err, ErrScopeViolation) {
		t.Fatalf("err = %v, want ErrScopeViolation (bypass with cycle class)", err)
	}
}

// === Missing manifest = pass-through ======================================
func TestRun_MissingManifest(t *testing.T) {
	repo := makeRepo(t, "") // no manifest
	res, err := Run(Options{
		CommitMsg:    "feat: msg",
		RepoDir:      repo,
		GetDiffPaths: stubDiffPaths([]string{"x.go"}),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !res.Allowed || res.Reason != "manifest-missing" {
		t.Errorf("res = %+v, want allowed pass-through", res)
	}
}

// === Malformed manifest → ErrBadManifest ==================================
func TestRun_MalformedManifest(t *testing.T) {
	repo := makeRepo(t, "{not json")
	_, err := Run(Options{
		CommitMsg:    "feat: msg",
		RepoDir:      repo,
		GetDiffPaths: stubDiffPaths(nil),
	})
	if !errors.Is(err, ErrBadManifest) {
		t.Fatalf("err = %v, want ErrBadManifest", err)
	}
}

// === No conventional prefix = pass-through ================================
// Use a message that does NOT match `^[a-z][a-z-]*(\([a-z0-9-]+\))?!?:`
// (uppercase, no colon, etc.)
func TestRun_NoPrefix(t *testing.T) {
	repo := makeRepo(t, `{"prefixes":{}}`)
	res, err := Run(Options{
		CommitMsg:    "Merge branch X into main (no conventional prefix)",
		RepoDir:      repo,
		GetDiffPaths: stubDiffPaths([]string{"x.go"}),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !res.Allowed || res.Reason != "no-prefix" {
		t.Errorf("res = %+v, want pass-through", res)
	}
}

// === Unknown prefix = pass-through ========================================
func TestRun_UnknownPrefix(t *testing.T) {
	repo := makeRepo(t, `{"prefixes":{"feat":{"any_path":true}}}`)
	res, err := Run(Options{
		CommitMsg:    "chore: cleanup",
		RepoDir:      repo,
		GetDiffPaths: stubDiffPaths([]string{"x.go"}),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !res.Allowed || res.Reason != "unknown-prefix" {
		t.Errorf("res = %+v", res)
	}
}

// === any_path: true is permissive =========================================
func TestRun_AnyPath(t *testing.T) {
	repo := makeRepo(t, `{"prefixes":{"feat":{"any_path":true}}}`)
	res, err := Run(Options{
		CommitMsg:    "feat: anything",
		RepoDir:      repo,
		GetDiffPaths: stubDiffPaths([]string{"docs/X.md"}),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !res.Allowed {
		t.Error("want Allowed=true")
	}
}

// === Empty diff = pass-through ============================================
func TestRun_EmptyDiff(t *testing.T) {
	repo := makeRepo(t, `{"prefixes":{"feat":{"required_paths":["go/**"]}}}`)
	res, err := Run(Options{
		CommitMsg:    "feat: msg",
		RepoDir:      repo,
		GetDiffPaths: stubDiffPaths(nil),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !res.Allowed {
		t.Error("want Allowed=true on empty diff")
	}
}

// === Bad args (missing CommitMsg or RepoDir) → ErrBadArgs ================
func TestRun_BadArgs_MissingCommitMsg(t *testing.T) {
	_, err := Run(Options{RepoDir: t.TempDir()})
	if !errors.Is(err, ErrBadArgs) {
		t.Errorf("err = %v, want ErrBadArgs", err)
	}
}

func TestRun_BadArgs_MissingRepoDir(t *testing.T) {
	_, err := Run(Options{CommitMsg: "feat: x"})
	if !errors.Is(err, ErrBadArgs) {
		t.Errorf("err = %v, want ErrBadArgs", err)
	}
}

// === guards.log appended ==================================================
func TestRun_AppendsGuardsLog(t *testing.T) {
	repo := makeRepo(t, `{"prefixes":{"docs":{"any_path":true}}}`)
	fixedNow := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	_, err := Run(Options{
		CommitMsg:    "docs: x",
		RepoDir:      repo,
		Stderr:       &buf,
		Now:          func() time.Time { return fixedNow },
		GetDiffPaths: stubDiffPaths([]string{"x.md"}),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	body, err := os.ReadFile(filepath.Join(repo, ".evolve", "guards.log"))
	if err != nil {
		t.Fatalf("read guards.log: %v", err)
	}
	if !strings.Contains(string(body), "2026-05-24T12:00:00Z") {
		t.Errorf("guards.log missing fixed timestamp: %q", body)
	}
}

// === matchPath table =======================================================
func TestMatchPath(t *testing.T) {
	cases := []struct {
		pat, path string
		want      bool
	}{
		{"docs/**", "docs/X.md", true},
		{"docs/**", "docs/sub/X.md", true},
		{"docs/**", "go/X.go", false},
		{"**/test.go", "x/y/test.go", true},
		{"**/test.go", "test.go", true},
		{"**/test.go", "x/y/other.go", false},
		{"README.md", "README.md", true},
		{"README.md", "docs/README.md", false},
		{"*.json", "config.json", true},
		{"*.json", "config.json.bak", false},
	}
	for _, tc := range cases {
		t.Run(tc.pat+"/"+tc.path, func(t *testing.T) {
			if got := matchPath(tc.pat, tc.path); got != tc.want {
				t.Errorf("matchPath(%q, %q) = %v, want %v", tc.pat, tc.path, got, tc.want)
			}
		})
	}
}

// === globMatch handles class and ? edge cases ==============================
func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pat, path string
		want      bool
	}{
		{"a?c", "abc", true},
		{"a?c", "ac", false},
		{"a[bcd]c", "abc", true},
		{"a[bcd]c", "aec", false},
		{"a[a-z]c", "abc", true},
		{"a[a-z]c", "aZc", false},
		{"*foo*", "xfooy", true},
	}
	for _, tc := range cases {
		t.Run(tc.pat+"/"+tc.path, func(t *testing.T) {
			if got := globMatch(tc.pat, tc.path); got != tc.want {
				t.Errorf("globMatch(%q, %q) = %v, want %v", tc.pat, tc.path, got, tc.want)
			}
		})
	}
}

// === Mode=Ref with missing diffRef → error returned by default =============
func TestRun_ModeRef_NoDiffRef(t *testing.T) {
	repo := makeRepo(t, `{"prefixes":{"feat":{"required_paths":["go/**"]}}}`)
	getDiffPaths := func(string, Mode, string) ([]string, error) {
		return nil, fmt.Errorf("simulated git failure")
	}
	res, err := Run(Options{
		CommitMsg:    "feat: x",
		RepoDir:      repo,
		Mode:         ModeRef,
		GetDiffPaths: getDiffPaths,
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !res.Allowed {
		t.Error("want Allowed=true on diff lookup failure")
	}
}
