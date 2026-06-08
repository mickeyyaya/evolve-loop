package acssuite

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// goACSDir resolves <repo>/go/acs from this test file's location
// (<repo>/go/internal/acssuite/tagguard_test.go → ../../acs).
func goACSDir(t *testing.T) string {
	t.Helper()
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(self), "..", "..", "acs")
}

// TestAllACSPredicatesAreTagged is the normal-suite guard enforcing the EGPS
// Go-native contract: every go/acs/**/predicates_test.go MUST carry the
// `//go:build acs` constraint. EGPS predicates are state/environment assertions,
// not unit tests — they must NOT run in the normal `go test ./...` / CI suite
// (e.g. cycle106's "no uncommitted changes" assertion is false mid-edit). The
// `acs` tag is what excludes them from the normal suite and includes them in the
// host-side Go predicate lane (`go test -tags acs ./acs/...`).
func TestAllACSPredicatesAreTagged(t *testing.T) {
	acsDir := goACSDir(t)
	var untagged []string
	err := filepath.Walk(acsDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Base(p) != "predicates_test.go" {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		if !hasACSBuildTag(string(b)) {
			rel, _ := filepath.Rel(acsDir, p)
			untagged = append(untagged, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", acsDir, err)
	}
	if len(untagged) > 0 {
		t.Errorf("%d predicate file(s) missing `//go:build acs` (EGPS predicates must be excluded "+
			"from the normal suite):\n  %s", len(untagged), strings.Join(untagged, "\n  "))
	}
}

// hasACSBuildTag reports whether src declares the `//go:build acs` constraint in
// its leading build-constraint block (before the package clause).
func hasACSBuildTag(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			return false // reached package clause without finding the tag
		}
		if strings.HasPrefix(trimmed, "//go:build ") {
			// Constraint expr must reference the acs tag (e.g. "acs", "acs && x").
			expr := strings.TrimPrefix(trimmed, "//go:build ")
			for _, tok := range strings.FieldsFunc(expr, func(r rune) bool {
				return r == ' ' || r == '&' || r == '|' || r == '(' || r == ')' || r == '!'
			}) {
				if tok == "acs" {
					return true
				}
			}
		}
	}
	return false
}
