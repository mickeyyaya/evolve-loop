package core

import (
	"strings"
	"testing"
)

// Normal-suite (untagged) coverage for the rung-2 scoped-review primitives so
// the apicover gate sees the four exported symbols exercised outside the
// `//go:build acs` predicate suite. Fixtures mirror go/acs/cycle942 so this
// file and the ACS predicates prove the same contract from both the internal
// and external test packages.

// foo.go: audited touches func A() around new line 10; composed touches BOTH
// func A() (overlap) and func B() around new line 51 (disjoint).
const auditedFoo = "diff --git a/foo.go b/foo.go\n" +
	"--- a/foo.go\n+++ b/foo.go\n" +
	"@@ -10,3 +10,4 @@ func A() {\n" +
	" \ta := 1\n \tb := 2\n+\tAUDITED_LINE := 3\n \treturn\n"

const composedFoo = "diff --git a/foo.go b/foo.go\n" +
	"--- a/foo.go\n+++ b/foo.go\n" +
	"@@ -10,3 +10,4 @@ func A() {\n" +
	" \ta := 1\n \tb := 2\n+\tHUNK_A := 3\n \treturn\n" +
	"@@ -50,2 +51,3 @@ func B() {\n" +
	" \tx := 1\n+\tHUNK_B := 2\n \ty := 2\n"

const composedDisjoint = "diff --git a/foo.go b/foo.go\n" +
	"--- a/foo.go\n+++ b/foo.go\n" +
	"@@ -50,2 +51,3 @@ func B() {\n" +
	" \tx := 1\n+\tHUNK_B := 2\n \ty := 2\n"

const resolvedInjected = "diff --git a/foo.go b/foo.go\n" +
	"--- a/foo.go\n+++ b/foo.go\n" +
	"@@ -10,3 +10,5 @@ func A() {\n" +
	" \ta := 1\n \tb := 2\n+\tAUDITED_LINE := 3\n+\tINJECTED := 99\n \treturn\n"

func TestIntersectingHunks_KeepsOverlapDropsDisjoint(t *testing.T) {
	scoped := string(IntersectingHunks([]byte(auditedFoo), []byte(composedFoo)))
	if !strings.Contains(scoped, "HUNK_A") {
		t.Errorf("intersecting hunk HUNK_A dropped:\n%s", scoped)
	}
	if strings.Contains(scoped, "HUNK_B") {
		t.Errorf("disjoint hunk HUNK_B leaked into scoped payload:\n%s", scoped)
	}
	// Header preserved so the payload is a valid unified diff.
	if !strings.Contains(scoped, "diff --git a/foo.go b/foo.go") {
		t.Errorf("scoped payload missing file header:\n%s", scoped)
	}
}

func TestIntersectingHunks_DisjointFootprintIsEmpty(t *testing.T) {
	scoped := IntersectingHunks([]byte(auditedFoo), []byte(composedDisjoint))
	if strings.TrimSpace(string(scoped)) != "" {
		t.Errorf("disjoint footprints produced non-empty scope: %q", scoped)
	}
}

func TestIntersectingHunks_FileOnlyInOneDiffExcluded(t *testing.T) {
	other := "diff --git a/bar.go b/bar.go\n--- a/bar.go\n+++ b/bar.go\n" +
		"@@ -1,1 +1,2 @@ func C() {\n z := 1\n+\tONLY_BAR := 2\n"
	scoped := string(IntersectingHunks([]byte(auditedFoo), []byte(other)))
	if strings.Contains(scoped, "ONLY_BAR") {
		t.Errorf("file present in only the composed diff leaked: %s", scoped)
	}
	if strings.TrimSpace(scoped) != "" {
		t.Errorf("no shared file — want empty scope, got: %s", scoped)
	}
}

func TestScopedReviewVerdict_Composes(t *testing.T) {
	if !ScopedReviewCompatible.Composes() {
		t.Errorf("compatible must compose")
	}
	if ScopedReviewEntangled.Composes() {
		t.Errorf("entangled must not compose")
	}
	if ScopedReviewVerdict("garbage").Composes() {
		t.Errorf("unknown verdict must fail closed")
	}
	if ScopedReviewMethod != "scoped-review" {
		t.Errorf("ScopedReviewMethod = %q, want scoped-review", ScopedReviewMethod)
	}
}

func TestReverifyResolution_MatchAndReject(t *testing.T) {
	ok, err := ReverifyResolution([]byte(auditedFoo), []byte(auditedFoo))
	if err != nil {
		t.Fatalf("identical resolution errored: %v", err)
	}
	if !ok {
		t.Errorf("byte-identical resolution must re-verify (patch-ids match)")
	}

	rejected, err := ReverifyResolution([]byte(auditedFoo), []byte(resolvedInjected))
	if err != nil {
		t.Fatalf("injected resolution errored: %v", err)
	}
	if rejected {
		t.Errorf("resolution that injected a line must be rejected (patch-ids differ)")
	}
}
