// audit_bound_witness_test.go — cycle-585 regression guard for the
// cycle-583 audit finding: a "helpful" rebind of opts.internalAuditBoundTreeSHA
// anywhere outside audit.go disarms the post-push integrity guard
// (gitops.go:493-497) and the ship-binding.json sidecar (gitops.go:528).
//
// Two independent layers, matching preventiveAction #1/#2 of
// cycle-583-audit-bound-sha-rebind-disarms-integrity-guard.yaml:
//
//   - TestInternalAuditBoundTreeSHA_OnlyAssignedInAuditGo: a static source
//     scan (no build tag — no git needed) that fails if the field is EVER
//     assigned outside audit.go. This is the "can't quietly regress"
//     guardrail the cycle-583 audit asked for.
//   - TestShipFromWorktree_PostPushGuard_FiresOnRebind (integration-tagged,
//     below in a sibling file) exercises the REAL post-push guard end to
//     end and asserts it still fires CodeIntegrityTreeDrift when the field
//     holds a value that doesn't match what was actually pushed.
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

// TestInternalAuditBoundTreeSHA_OnlyAssignedInAuditGo is the cycle-583
// regression guard: it source-scans every non-test .go file in this package
// EXCEPT audit.go for an assignment to internalAuditBoundTreeSHA. The
// cycle-583 incident introduced exactly this shape (a rebind in the ship
// dispatch path to the post-merge tree) paired with a sound push-race fix;
// both the adversarial review and the Auditor caught it, but nothing
// mechanical did. This test is that missing mechanical check: it fails RED
// the moment a second assignment site appears anywhere in the package.
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

// TestInternalAuditBoundTreeSHA_AuditGoStillAssignsIt is the sibling
// no-op-guard: if audit.go itself stops assigning the field (e.g. the logic
// is moved wholesale without updating this test's assumption), the scan
// above would trivially pass with zero offenders and give false confidence.
// Confirm the expected single site still exists.
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
