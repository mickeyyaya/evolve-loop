package acssuite

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func goACSDir(t *testing.T) string {
	t.Helper()
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(self), "..", "..", "acs")
}

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

func hasACSBuildTag(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			return false
		}
		if strings.HasPrefix(trimmed, "//go:build ") {
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
