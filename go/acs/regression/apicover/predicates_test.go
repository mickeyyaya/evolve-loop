//go:build acs

// Package apicover is the ADR-0050 Phase 5 ship-gate predicate. It enforces the
// COMPLETENESS half of the public-API coverage invariant — every ./internal/...
// package must appear in the go/.apicover-enforce SSOT, so a newly-added package
// cannot silently escape the apicover gate. The CORRECTNESS half (each enforced
// package is actually apicover-clean: 0 uncovered / 0 false-green) is hard-gated
// by .github/workflows/go.yml's "api-coverage enforce" step, which generates the
// integration coverage profile a per-cycle ship-gate predicate cannot afford to
// re-run. Together they keep the gate COMPLETE (this predicate) and CORRECT (CI).
package apicover

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// TestApicoverEnforce_CoversEveryInternalPackage asserts the .apicover-enforce
// SSOT lists EVERY ./internal/... package (completeness) and lists nothing that
// is not a real internal package (no stale/typo entry). As of Phase 5 completion
// the allowed-unenforced set is EMPTY — every internal package has graduated to
// the public-API DoD. A new internal package added without a graduating
// apicover_named_test.go fails HERE at ship-gate time, not just in CI.
func TestApicoverEnforce_CoversEveryInternalPackage(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")

	// SSOT: the enforced package patterns (strip comments/blank lines) — the same
	// file CI's enforce step and apicover read.
	enfData, err := os.ReadFile(filepath.Join(goDir, ".apicover-enforce"))
	if err != nil {
		t.Fatalf("read .apicover-enforce: %v", err)
	}
	enforced := map[string]bool{}
	for _, line := range strings.Split(string(enfData), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		enforced[line] = true
	}
	if len(enforced) == 0 {
		t.Fatal(".apicover-enforce has no enforced packages")
	}

	// Authoritative internal-package enumeration via `go list` (the same view CI
	// uses) — run from the go module dir.
	modPath := goList(t, goDir, "-m")
	listOut := goListPkgs(t, goDir)

	internal := map[string]bool{}
	var missing []string
	for _, ip := range listOut {
		pat := "." + strings.TrimPrefix(ip, modPath) // import path -> ./internal/foo
		internal[pat] = true
		if !enforced[pat] {
			missing = append(missing, pat)
		}
	}

	// Completeness: every internal package must be enforced (allowed-unenforced is
	// empty post-Phase-5).
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("apicover completeness regression: %d internal package(s) NOT in .apicover-enforce — graduate them (add an apicover_named_test.go) or they escape the public-API gate:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}

	// Integrity: every enforced ./internal/... entry must be a real package.
	var stale []string
	for pat := range enforced {
		if !strings.HasPrefix(pat, "./internal/") {
			continue // the SSOT is internal-only today; ignore any non-internal pattern defensively
		}
		if !internal[pat] {
			stale = append(stale, pat)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("apicover stale entries: %d .apicover-enforce line(s) are not real ./internal/... packages:\n  %s",
			len(stale), strings.Join(stale, "\n  "))
	}
}

// goListPkgs returns the import paths of every ./internal/... package.
func goListPkgs(t *testing.T, goDir string) []string {
	t.Helper()
	out := goListRaw(t, goDir, "./internal/...")
	return strings.Fields(out)
}

// goList runs `go list <args...>` in goDir and returns the trimmed single-line output.
func goList(t *testing.T, goDir string, args ...string) string {
	t.Helper()
	return strings.TrimSpace(goListRaw(t, goDir, args...))
}

// goListRaw runs `go list <args...>` in goDir and returns stdout, failing the test on error.
func goListRaw(t *testing.T, goDir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"list"}, args...)...)
	cmd.Dir = goDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list %v (in %s): %v", args, goDir, err)
	}
	return string(out)
}
