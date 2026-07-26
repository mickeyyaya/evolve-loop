package ship

import (
	"reflect"
	"strings"
	"testing"
)

// manifest_relative_test.go — regression lock for the cycle-1098 ship fatal
// (2026-07-27, first green cycle of batch-12): extractReportPaths ran
// strings.Trim(token, ".") on the matched token, so the ./-prefixed prose the
// ADR-0076 slice-B mandate puts in EVERY build-report ("$ ./go/bin/evolve
// selfcheck build") became the ABSOLUTE-looking manifest entry
// "/go/bin/evolve". stagePathspec's isFile filter resolved it INSIDE the
// worktree (filepath.Join(root, "/go/bin/evolve")), so it survived into
// `git add -A -- /go/bin/evolve ...` → git canonicalized the absolute path →
// `fatal: Invalid path '/go': No such file or directory` (rc=128, reproduced
// verbatim) → 2 futile transient retries → cycle aborted, audited-PASS work
// stranded in the preserved worktree. Deterministic batch-killer: every green
// cycle documents the same pre-flight line.

// TestExtractReportPaths_RelativeDotPrefix pins the REAL cycle-1098
// build-report lines: ./-prefixed tokens must normalize to repo-relative
// paths — never to a leading-slash pathspec.
func TestExtractReportPaths_RelativeDotPrefix(t *testing.T) {
	// Verbatim shapes from .evolve/runs/cycle-1098/build-report.md (lines 104
	// and 144) plus a parenthesized variant.
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

// TestExtractReportPaths_NeverAbsoluteOrParent: whatever the report prose,
// no extracted entry may leave the repo-relative space. A leading-slash entry
// becomes an absolute git pathspec (the rc=128 fatal above); a ../ entry
// would name paths outside the repo. Both must be dropped, not mangled.
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
		// Interior ".." escapes are the same rc=128 class: git canonicalizes
		// "a/../../etc/x" outside the repo ("is outside repository").
		if !isRepoRelative(p) {
			t.Errorf("extracted entry escapes the repo: %q", p)
		}
		if strings.HasPrefix(p, "..") {
			t.Errorf("ellipsis/parent garbage extracted as manifest entry: %q", p)
		}
	}
}

// TestExtractReportPaths_DotDirsPreserved: genuine dot-directory paths are
// real declarations (.github workflows, .evolve profiles). The OLD blind
// Trim(m, ".") silently truncated them ("github/workflows/x.yml") — the exact
// mangling its own left-boundary comment warned against.
func TestExtractReportPaths_DotDirsPreserved(t *testing.T) {
	md := ".github/workflows/go.yml and `.evolve/profiles/auditor.json` were regenerated.\n"
	got := extractReportPaths(md)
	want := []string{".evolve/profiles/auditor.json", ".github/workflows/go.yml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractReportPaths = %v, want %v", got, want)
	}
}

// TestIsRepoRelative pins the SSOT guard, incl. the Clean-based interior-..
// rejection the prefix-only form missed.
func TestIsRepoRelative(t *testing.T) {
	for p, want := range map[string]bool{
		"go/internal/ok.go":   true,
		".github/workflows/x": true,
		"a/../b.go":           true, // stays inside the repo after Clean
		"":                    false,
		"/go/evolve":          false,
		"..":                  false,
		"../x.go":             false,
		"a/../../etc/passwd":  false, // interior escape — Clean → ../etc/passwd
		"go/../..":            false,
		"./":                  false, // Clean → "."
	} {
		if got := isRepoRelative(p); got != want {
			t.Errorf("isRepoRelative(%q) = %v, want %v", p, got, want)
		}
	}
}

// TestStagePathspec_RejectsNonRelativeManifestEntries: defense in depth at the
// staging seam — even if a non-relative entry reaches the manifest (older
// serialized manifests, a continuation union from a pre-fix attempt), it must
// not survive into the git argv.
func TestStagePathspec_RejectsNonRelativeManifestEntries(t *testing.T) {
	manifest := []string{"/go/bin/evolve", "/go/evolve", "../outside.go", "go/internal/ok.go"}
	changed := []string{"go/internal/ok.go"}
	got := stagePathspec(manifest, changed, func(rel string) bool { return true })
	for _, p := range got {
		if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "../") {
			t.Fatalf("stagePathspec passed non-relative entry into git argv: %q (full: %v)", p, got)
		}
	}
	if !reflect.DeepEqual(got, []string{"go/internal/ok.go"}) {
		t.Fatalf("stagePathspec = %v, want [go/internal/ok.go]", got)
	}
}
