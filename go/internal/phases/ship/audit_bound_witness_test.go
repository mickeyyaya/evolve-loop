package ship

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// assignmentRe matches an assignment to internalAuditBoundTreeSHA: either a
// bare `internalAuditBoundTreeSHA = ...` (package-level helper) or a
// struct-field write `<x>.internalAuditBoundTreeSHA = ...` / `:=` short
// form is impossible for a struct field, so `=` is sufficient. Comparisons
// (`==`, `!=`) and reads are intentionally NOT matched.
var assignmentRe = regexp.MustCompile(`\binternalAuditBoundTreeSHA\s*=[^=]`)

func TestInternalAuditBoundTreeSHA_OnlyAssignedInAuditGo(t *testing.T) {
	dir := "." // go/internal/phases/ship
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	var offenders []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if name == "audit.go" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if assignmentRe.MatchString(line) {
				offenders = append(offenders, name+":"+strconv.Itoa(i+1)+": "+strings.TrimSpace(line))
			}
		}
	}

	if len(offenders) > 0 {
		t.Fatalf("internalAuditBoundTreeSHA must be assigned ONLY in audit.go "+
			"(cycle-583: a rebind elsewhere disarms the post-push integrity guard "+
			"in gitops.go and the ship-binding.json sidecar) — found assignment(s) "+
			"outside audit.go:\n%s", strings.Join(offenders, "\n"))
	}
}

func TestInternalAuditBoundTreeSHA_AuditGoStillAssignsIt(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(".", "audit.go"))
	if err != nil {
		t.Fatalf("read audit.go: %v", err)
	}
	count := 0
	for _, line := range strings.Split(string(body), "\n") {
		if assignmentRe.MatchString(line) {
			count++
		}
	}
	if count == 0 {
		t.Fatal("expected at least one internalAuditBoundTreeSHA assignment in audit.go " +
			"(the sole authorized writer) but found none — the witness-only-in-audit.go " +
			"invariant would be vacuous")
	}
}
