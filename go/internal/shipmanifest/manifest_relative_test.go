package shipmanifest

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractReportPaths_RelativeDotPrefix(t *testing.T) {
	md := "$ ./go/bin/evolve selfcheck build --worktree <this worktree>   # ADR-0076 mandatory pre-flight\n" +
		"from an older revision still sits in the tree. Running `./go/evolve selfcheck build`\n" +
		"(see ./docs/operations/runtime-reference.md for the flag table)\n"
	got := extractReportPaths(md)
	want := []string{
		"docs/operations/runtime-reference.md",
		"go/bin/evolve",
		"go/evolve",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractReportPaths = %v, want %v", got, want)
	}
}

func TestExtractReportPaths_NeverAbsoluteOrParent(t *testing.T) {
	md := "ok  \tgithub.com/mickeyyaya/evolve-loop/go/cmd/evolve\t0.490s\n" +
		"compare ../sibling-repo/go/thing.go and .../elided/prose.go\n" +
		"ellipsis-glued ..foo/bar and interior a/../../etc/escape.go\n" +
		"$ ./go/bin/evolve selfcheck build\n" +
		"badge https://img.shields.io/badge/coverage-80%25-green\n"
	for _, p := range extractReportPaths(md) {
		if strings.HasPrefix(p, "/") {
			t.Errorf("extracted entry is absolute (git pathspec fatal class): %q", p)
		}
		if !isRepoRelative(p) {
			t.Errorf("extracted entry escapes the repo: %q", p)
		}
		if strings.HasPrefix(p, "..") {
			t.Errorf("ellipsis/parent garbage extracted as manifest entry: %q", p)
		}
	}
}

func TestExtractReportPaths_DotDirsPreserved(t *testing.T) {
	md := ".github/workflows/go.yml and `.evolve/profiles/auditor.json` were regenerated.\n"
	got := extractReportPaths(md)
	want := []string{".evolve/profiles/auditor.json", ".github/workflows/go.yml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractReportPaths = %v, want %v", got, want)
	}
}

func TestIsRepoRelative(t *testing.T) {
	for p, want := range map[string]bool{
		"go/internal/ok.go":   true,
		".github/workflows/x": true,
		"a/../b.go":           true,
		"":                    false,
		"/go/evolve":          false,
		"..":                  false,
		"../x.go":             false,
		"a/../../etc/passwd":  false,
		"go/../..":            false,
		"./":                  false,
	} {
		if got := isRepoRelative(p); got != want {
			t.Errorf("isRepoRelative(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestStagePathspec_RejectsNonRelativeManifestEntries(t *testing.T) {
	manifest := []string{"/go/bin/evolve", "/go/evolve", "../outside.go", "go/internal/ok.go"}
	changed := []string{"go/internal/ok.go"}
	got := pathspec(manifest, changed, func(rel string) bool { return true })
	for _, p := range got {
		if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "../") {
			t.Fatalf("Pathspec passed non-relative entry into git argv: %q (full: %v)", p, got)
		}
	}
	if !reflect.DeepEqual(got, []string{"go/internal/ok.go"}) {
		t.Fatalf("Pathspec = %v, want [go/internal/ok.go]", got)
	}
}
