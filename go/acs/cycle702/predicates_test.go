//go:build acs

// Package cycle702 materializes the cycle-702 acceptance criteria for the sole
// committed top_n task chronicle-s2-digest-writer (triage-report.md ## top_n;
// this is a fleet-scoped lane — the scout's other selections were left in the
// backlog, so per R9.3 no predicates bind to them and no deferred-floor
// predicates exist).
//
// AC map (1:1):
//
//	AC1 newest-first rendering + token-budget truncation (len/4)   → C702_001
//	AC2 control-char/bullet-forgery sanitization (injection twin)  → C702_002
//	AC3 generic patterns roll up to ONE aggregate line             → C702_003
//	AC4 empty history writes no artifact (anti-no-op negative)     → C702_004
//	AC5 chronicle policy block: compiled defaults + overrides      → C702_005/006
//	AC6 go test -race green + go vet clean on both touched pkgs    → C702_007/008
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit-test contract, which EXERCISES the SUT (WriteDigest against real temp
// workspaces and rendered artifacts; ChronicleConfig against real JSON policy
// docs) — behavioral via subprocess, no source-grep predicates (cycle-85 rule).
// The `-v` + "--- PASS:" guard rejects a rename/no-tests-matched silent green.
package cycle702

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	recurrencePkg = "github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
	policyPkg     = "github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// runGoTest executes the named unit test under -race and requires an explicit
// verbose PASS marker so the predicate fails on: compile failure, test
// failure, a race report, OR the test not existing (rename gaming).
func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test reported no PASS for %s (renamed or not run?)\nstdout:\n%s", name, stdout)
	}
}

// AC1 — newest-first ordering, cfg.Cycles window, and token-budget truncation
// that drops the oldest lines first.
func TestC702_001_DigestNewestFirstTruncation(t *testing.T) {
	runGoTest(t, recurrencePkg, "TestWriteDigest_NewestFirstTruncationAtTokenBudget")
}

// AC2 — the injection negative: LLM-authored lesson/goal text with embedded
// newline bullet forgery renders as one sanitized line.
func TestC702_002_DigestSanitizesControlChars(t *testing.T) {
	runGoTest(t, recurrencePkg, "TestWriteDigest_SanitizesLessonTextControlChars")
}

// AC3 — generic recurrence patterns aggregate to exactly one roll-up line;
// non-generic patterns keep their own PatternStats lines.
func TestC702_003_DigestAggregatesGenericPatterns(t *testing.T) {
	runGoTest(t, recurrencePkg, "TestWriteDigest_AggregatesGenericPatternsToOneLine")
}

// AC4 — the anti-no-op negative: empty history must produce NO artifact.
func TestC702_004_DigestEmptyHistoryWritesNothing(t *testing.T) {
	runGoTest(t, recurrencePkg, "TestWriteDigest_EmptyHistoryWritesNothing")
}

// AC5a — absent/empty chronicle policy block resolves the compiled defaults
// (digest=shadow, 1200 tokens, 10 cycles, escalation=shadow, historian=off).
func TestC702_005_ChronicleCompiledDefaults(t *testing.T) {
	runGoTest(t, policyPkg, "TestChronicleConfig_CompiledDefaults")
}

// AC5b — a present chronicle block overrides via the on-disk JSON shape;
// partial blocks override only the fields they set.
func TestC702_006_ChroniclePolicyOverrides(t *testing.T) {
	runGoTest(t, policyPkg, "TestChronicleConfig_PolicyOverrides")
}

// AC6a — both touched packages green under the race detector.
func TestC702_007_TouchedPackagesRaceClean(t *testing.T) {
	for _, pkg := range []string{recurrencePkg, policyPkg} {
		stdout, stderr, code, err := acsassert.SubprocessOutput(
			"go", "test", "-race", "-count=1", pkg)
		if code != 0 || err != nil {
			t.Fatalf("go test -race %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
				pkg, code, err, stdout, stderr)
		}
	}
}

// AC6b — go vet clean on both touched packages.
func TestC702_008_TouchedPackagesVetClean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", recurrencePkg, policyPkg)
	if code != 0 || err != nil {
		t.Fatalf("go vet exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}
