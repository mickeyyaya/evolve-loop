package publishmirror

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShort(t *testing.T) {
	if got := short("abc"); got != "abc" {
		t.Errorf("short(abc) = %q, want abc", got)
	}
	if got := short("0123456789abcdef"); got != "0123456789ab" {
		t.Errorf("short(long) = %q, want first 12", got)
	}
}

func TestTagSuffix(t *testing.T) {
	if tagSuffix("") != "" {
		t.Error("empty tag should yield empty suffix")
	}
	if tagSuffix("v1") != " + v1" {
		t.Errorf("tagSuffix(v1) = %q", tagSuffix("v1"))
	}
}

func TestRun_RejectsBareRemoteName(t *testing.T) {
	// "origin" would resolve against the scratch worktree's inherited private
	// remotes. Run must reject it before touching git.
	_, err := Run(context.Background(), Options{RepoDir: t.TempDir(), Remote: "origin", Push: false})
	if err == nil {
		t.Fatal("a bare remote name must be rejected")
	}
	if !strings.Contains(err.Error(), "bare remote name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestApplyTransforms_NoScopeFileNoReadme_NoOp(t *testing.T) {
	scratch := t.TempDir()
	if err := applyTransforms(Options{ScratchDir: scratch}); err != nil {
		t.Fatalf("applyTransforms with empty scratch should be a no-op, got %v", err)
	}
}

func TestApplyTransforms_MissingPublicReadme_Errors(t *testing.T) {
	scratch := t.TempDir()
	err := applyTransforms(Options{ScratchDir: scratch, PublicReadme: filepath.Join(scratch, "nope.md")})
	if err == nil {
		t.Fatal("a missing --public-readme path should error")
	}
}

func TestApplyTransforms_RemovesPrefixAndSwapsReadme(t *testing.T) {
	scratch := t.TempDir()
	if err := os.MkdirAll(filepath.Join(scratch, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	scopePath := filepath.Join(scratch, ".evolve", "commit-prefix-scope.json")
	if err := os.WriteFile(scopePath,
		[]byte(`{"feat":{"description":"f"},"chore(build)":{"required_paths":["go/evolve"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pub := filepath.Join(scratch, "public.md")
	if err := os.WriteFile(pub, []byte("CONDENSED"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyTransforms(Options{ScratchDir: scratch, PublicReadme: pub}); err != nil {
		t.Fatalf("applyTransforms: %v", err)
	}
	scope, _ := os.ReadFile(scopePath)
	if strings.Contains(string(scope), "chore(build)") {
		t.Error("chore(build) should be removed from the scratch scope file")
	}
	readme, _ := os.ReadFile(filepath.Join(scratch, "README.md"))
	if string(readme) != "CONDENSED" {
		t.Errorf("README not swapped, got %q", readme)
	}
}

func TestApplyTransforms_InvalidScopeJSON_Errors(t *testing.T) {
	scratch := t.TempDir()
	if err := os.MkdirAll(filepath.Join(scratch, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scratch, ".evolve", "commit-prefix-scope.json"),
		[]byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyTransforms(Options{ScratchDir: scratch}); err == nil {
		t.Fatal("invalid commit-prefix-scope JSON should error")
	}
}
