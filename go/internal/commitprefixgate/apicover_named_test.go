package commitprefixgate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestResult_NamesManifestAndRuleOnScopeMatch drives a scope-match through Run
// and names the Result, PrefixManifest, and PrefixRule types via the real
// consumer (the GetDiffPaths seam avoids a git subprocess). It pins that a
// matching prefix yields Allowed/scope-matched, the parsed manifest carries the
// declared prefix, and the matched rule carries its required paths.
func TestResult_NamesManifestAndRuleOnScopeMatch(t *testing.T) {
	repo := t.TempDir()
	manifest := `{"prefixes":{"docs":{"required_paths":["docs/**"]}}}`
	if err := os.MkdirAll(filepath.Join(repo, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".evolve", "commit-prefix-scope.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	var res Result
	res, err := Run(Options{
		CommitMsg: "docs: update runtime reference",
		RepoDir:   repo,
		Mode:      ModeStaged,
		GetDiffPaths: func(string, Mode, string) ([]string, error) {
			return []string{"docs/operations/runtime-reference.md"}, nil
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Allowed || res.Reason != "scope-matched" {
		t.Errorf("Result allowed=%v reason=%q, want allowed scope-matched", res.Allowed, res.Reason)
	}
	if res.Prefix != "docs" {
		t.Errorf("Result.Prefix=%q, want docs", res.Prefix)
	}

	var gotManifest *PrefixManifest = res.Manifest
	if gotManifest == nil {
		t.Fatal("Result.Manifest is nil; want parsed PrefixManifest")
	}
	if _, ok := gotManifest.Prefixes["docs"]; !ok {
		t.Errorf("PrefixManifest.Prefixes missing 'docs': %+v", gotManifest.Prefixes)
	}

	var gotRule *PrefixRule = res.PrefixRule
	if gotRule == nil {
		t.Fatal("Result.PrefixRule is nil; want matched PrefixRule")
	}
	if len(gotRule.RequiredPaths) != 1 || gotRule.RequiredPaths[0] != "docs/**" {
		t.Errorf("PrefixRule.RequiredPaths=%v, want [docs/**]", gotRule.RequiredPaths)
	}
}

// TestPrefixRule_AnyPathBypassesScope pins the any_path escape hatch: a rule
// with AnyPath allows any changed path with reason "any-path".
func TestPrefixRule_AnyPathBypassesScope(t *testing.T) {
	repo := t.TempDir()
	manifest := `{"prefixes":{"chore":{"any_path":true}}}`
	if err := os.MkdirAll(filepath.Join(repo, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".evolve", "commit-prefix-scope.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Run(Options{
		CommitMsg: "chore: bump anything",
		RepoDir:   repo,
		Mode:      ModeStaged,
		GetDiffPaths: func(string, Mode, string) ([]string, error) {
			return []string{"go/main.go"}, nil
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Allowed || res.Reason != "any-path" {
		t.Errorf("any_path rule must allow with reason any-path; got allowed=%v reason=%q", res.Allowed, res.Reason)
	}
	if res.PrefixRule == nil || !res.PrefixRule.AnyPath {
		t.Errorf("Result.PrefixRule.AnyPath should be true; got %+v", res.PrefixRule)
	}
}

// TestPrefixRule_RequiredPathsViolationIsScopeError pins that a changed path
// outside the rule's required_paths is rejected as ErrScopeViolation.
func TestPrefixRule_RequiredPathsViolationIsScopeError(t *testing.T) {
	repo := t.TempDir()
	manifest := `{"prefixes":{"feat":{"required_paths":["go/**"]}}}`
	if err := os.MkdirAll(filepath.Join(repo, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".evolve", "commit-prefix-scope.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Run(Options{
		CommitMsg: "feat: add thing",
		RepoDir:   repo,
		Mode:      ModeStaged,
		GetDiffPaths: func(string, Mode, string) ([]string, error) {
			return []string{"docs/only.md"}, nil
		},
	})
	if !errors.Is(err, ErrScopeViolation) {
		t.Errorf("required_paths miss must be ErrScopeViolation; got %v", err)
	}
}
