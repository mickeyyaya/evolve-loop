//go:build acs

// Package cycle1333 materializes the cycle-1333 acceptance criteria for this
// fleet lane's sole assigned todo-id, blocker-breaker-fingerprint-ack (per
// R9.3, no predicates bind to any other lane's items).
//
// Scout's finding: the fingerprint-ack mechanism itself (AckedFingerprints,
// ResolvedFingerprint, LoadResolvedFingerprints, AppendResolvedFingerprint,
// the `evolve loop --reset --fingerprint <fp>` CLI flag) was already
// implemented and merged in cycle-1332 — go build and the fingerprint test
// subset are green. The remaining defect this cycle closes is a
// documentation gap: docs/operations/runtime-reference.md's "Operator
// commands" section documents every other operator-facing mutating command
// but has zero mention of `--fingerprint`, and CHANGELOG.md has no entry
// naming it (cycle-1332's commit was a bare `evolve-cycle:` commit, not a
// `/commit`-attested manual ship, so it never flowed through the changelog
// generator). Per doc_stewardship_policy ("everything learned → docs/ or
// kb/research/"), an undocumented operator-facing CLI flag is itself the
// open defect.
//
// AC map (1:1, from scout-report.md "Acceptance Criteria Summary"):
//
//	AC1 docs/operations/runtime-reference.md names the flag, its --reset
//	    gating, the ledger file .evolve/resolved-fingerprints.json, its
//	    record shape, and which blocker-breaker rule it excludes from
//	    (Rule B, identical-fingerprint).
//	    → C1333_001 (flag + --reset gating + ledger filename)
//	    → C1333_002 (record shape: fingerprint/resolved_at/resolved_by)
//	    → C1333_003 (Rule B / identical-fingerprint exclusion)
//	AC2 CHANGELOG.md has a new entry mentioning `--reset --fingerprint`.
//	    → C1333_004
//	AC3 Zero Go source changes; existing fingerprint/blocker-breaker tests
//	    still pass unchanged (regression — proves the doc-only fix didn't
//	    touch the shipped mechanism).
//	    → C1333_005 (core package, subprocess go test)
//	    → C1333_006 (cmd/evolve package, subprocess go test,
//	      TestRunLoop_FingerprintAck_AppendsLedgerRecord — the real
//	      production-entrypoint caller proof cycle-1332 authored)
//
// Predicate-quality note: AC1/AC2 assert against PROSE (a documentation
// accuracy criterion — the task IS the doc edit, there is no runtime
// behaviour to invoke for "does this doc name this flag"), so they carry
// the `config-check` waiver per go/acs/README.md. This is not the cycle-85
// degenerate shape: the strings asserted (exact flag spelling, exact ledger
// filename, exact JSON field names, "Rule B") are drawn verbatim from the
// real source (go/internal/core/blocker_breaker.go,
// go/cmd/evolve/cmd_loop_args.go) rather than invented, so a Builder cannot
// green these by writing an unrelated sentence containing generic words —
// each substring pins a specific, checkable fact. AC3 (C1333_005/006) is
// fully behavioral: it shells the real `go test` binary against the exact
// package + test names cycle-1332 shipped and requires their PASS marker.
package cycle1333

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// runtimeReferencePath and changelogPath resolve the two target docs under
// the repo root every predicate in this file reads.
func runtimeReferencePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "runtime-reference.md")
}

func changelogPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "CHANGELOG.md")
}

// TestC1333_001_RuntimeReferenceNamesFingerprintFlagAndResetGating verifies
// runtime-reference.md documents the `--fingerprint` flag, gated behind
// `--reset`, and the ledger file it writes to.
//
// acs-predicate: config-check — documentation accuracy criterion (AC1, part
// 1 of 3); see package-level waiver rationale above.
func TestC1333_001_RuntimeReferenceNamesFingerprintFlagAndResetGating(t *testing.T) {
	path := runtimeReferencePath(t)
	acsassert.AllOf(t,
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, path, "--fingerprint") },
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, path, "--reset") },
		func(tb acsassert.TB) bool {
			return acsassert.FileContains(tb, path, "resolved-fingerprints.json")
		},
	)
}

// TestC1333_002_RuntimeReferenceDescribesLedgerRecordShape verifies the doc
// names the ack ledger's record fields (fingerprint, resolved_at,
// resolved_by — the exact JSON tags on core.ResolvedFingerprint).
//
// acs-predicate: config-check — documentation accuracy criterion (AC1, part
// 2 of 3); see package-level waiver rationale above.
func TestC1333_002_RuntimeReferenceDescribesLedgerRecordShape(t *testing.T) {
	path := runtimeReferencePath(t)
	acsassert.AllOf(t,
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, path, "fingerprint") },
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, path, "resolved_at") },
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, path, "resolved_by") },
	)
}

// TestC1333_003_RuntimeReferenceNamesRuleBExclusion verifies the doc states
// which blocker-breaker rule the ack excludes a fingerprint from — Rule B,
// the identical-fingerprint count (core.EvaluateBlockerBreaker).
//
// acs-predicate: config-check — documentation accuracy criterion (AC1, part
// 3 of 3); see package-level waiver rationale above.
func TestC1333_003_RuntimeReferenceNamesRuleBExclusion(t *testing.T) {
	path := runtimeReferencePath(t)
	if !acsassert.FileContainsAny(path, "Rule B", "identical-fingerprint") {
		t.Errorf("runtime-reference.md must name the excluded rule (%q): expected %q or %q",
			path, "Rule B", "identical-fingerprint")
	}
}

// TestC1333_004_ChangelogMentionsResetFingerprintFlag verifies CHANGELOG.md
// gained a dated entry naming the `--reset --fingerprint` flag pair, so
// cycle-1332's undocumented ship leaves a trace in the canonical release
// history (scout's AC2, verifiableBy: `grep -q -- "--reset --fingerprint"
// CHANGELOG.md`).
//
// acs-predicate: config-check — documentation accuracy criterion (AC2); see
// package-level waiver rationale above.
func TestC1333_004_ChangelogMentionsResetFingerprintFlag(t *testing.T) {
	path := changelogPath(t)
	if !acsassert.FileContains(t, path, "--reset --fingerprint") {
		t.Errorf("CHANGELOG.md must contain the literal flag pair %q (%q)",
			"--reset --fingerprint", path)
	}
}

// TestC1333_005_CoreBlockerBreakerFingerprintTestsStillPass is the AC3
// regression predicate for the core package: the doc-only fix must not
// touch (and must not break) the fingerprint-ack mechanism cycle-1332
// shipped. Runs the real `go test` binary — a subprocess, not a direct
// function call — against the exact test names naming the mechanism.
//
// Behavioral (not config-check): invokes the system-under-test's own test
// binary and requires its PASS marker.
func TestC1333_005_CoreBlockerBreakerFingerprintTestsStillPass(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", filepath.Join(root, "go"), "-v",
		"-run", "TestLoadResolvedFingerprints_ReadsLedgerRecords|TestLoadResolvedFingerprints_MissingFileReturnsEmptyNoError|TestEvaluateBlockerBreaker_ExcludesAckedFingerprint|TestEvaluateBlockerBreaker_UnackedIdenticalFingerprintStillHalts|TestAppendResolvedFingerprint_WritesRecord",
		"./internal/core",
	)
	if err != nil || code != 0 {
		t.Fatalf("go test ./internal/core (fingerprint subset) failed: code=%d err=%v\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS:") {
		t.Errorf("expected at least one \"--- PASS:\" marker in go test output, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "--- FAIL:") {
		t.Errorf("unexpected \"--- FAIL:\" marker in go test output:\n%s", stdout)
	}
}

// TestC1333_006_RunLoopFingerprintAckCallerProofStillPasses is the AC3
// regression predicate for the cmd/evolve package: the real production
// caller proof cycle-1332 authored (TestRunLoop_FingerprintAck_AppendsLedgerRecord,
// which drives runLoop — the actual CLI entrypoint, not a direct
// AppendResolvedFingerprint call) must remain green after this cycle's
// docs-only diff.
//
// Behavioral (not config-check): invokes the system-under-test's own test
// binary and requires its PASS marker.
func TestC1333_006_RunLoopFingerprintAckCallerProofStillPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", filepath.Join(root, "go"), "-v",
		"-run", "TestRunLoop_FingerprintAck_AppendsLedgerRecord",
		"./cmd/evolve",
	)
	if err != nil || code != 0 {
		t.Fatalf("go test ./cmd/evolve -run TestRunLoop_FingerprintAck_AppendsLedgerRecord failed: code=%d err=%v\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: TestRunLoop_FingerprintAck_AppendsLedgerRecord") {
		t.Errorf("expected \"--- PASS: TestRunLoop_FingerprintAck_AppendsLedgerRecord\" in go test output, got:\n%s", stdout)
	}
}
