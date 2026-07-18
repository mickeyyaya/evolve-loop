//go:build acs

// Package cycle925 materializes the cycle-925 acceptance criteria for this
// fleet lane's sole committed task, mechanical-scans-to-native (triage top_n
// id: mechanical-scans-to-native).
//
// Goal: convert the two mechanical scan phases (secret-leak-scan,
// flake-rerun-scan) from full LLM boots to native in-process Go phases, per the
// ship kind:"native" precedent and Rule 5 (deterministic work → code, not LLM
// cycles). The prior attempt (cycle-785) left empty placeholder packages under
// go/internal/phases/{secretleakscan,flakererunscan} and a durable eval whose
// evidence pointed at cycle-785 predicates that were never landed; this cycle
// authors the real predicates and re-points the eval.
//
// Every predicate below EXERCISES THE SYSTEM UNDER TEST — it invokes the scan
// packages' functions, the phasespec validator, or the registry loader and
// asserts on their return values. None are source-grep predicates (the cycle-85
// degenerate-predicate failure mode is avoided). RED today is a compile failure
// on the two empty packages plus the validator rejecting kind:"native"; the
// Builder makes them GREEN by implementing the packages, unblocking the
// validator, and adding the registry phases[] entries — WITHOUT modifying this
// file.
//
// SUT CONTRACT the Builder must implement (see test-report.md handoff):
//
//	package secretleakscan
//	    type Finding struct { Rule, Match string }
//	    func ScanDiff(diff string) []Finding          // scans ADDED lines of a unified git diff
//	    func Verdict(findings []Finding) string        // canonical: "PASS" (0) | "FAIL" (>=1)
//
//	package flakererunscan
//	    type Result struct { Runs, Passes, Failures int; Flaky bool }
//	    func (r Result) Verdict() string               // canonical: "PASS" (consistent-pass) | non-PASS (flaky/failed)
//	    func Rerun(runs int, attempt func(i int) bool) Result  // deterministic: identical attempt fn → identical Result
package cycle925

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/flakererunscan"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/secretleakscan"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// canonicalVerdicts mirrors phasespec's canonical set (PASS/FAIL/WARN/SKIPPED),
// the vocabulary downstream gates pattern-match. Duplicated (not imported) to
// keep this ACS leaf import-light.
var canonicalVerdicts = map[string]bool{"PASS": true, "FAIL": true, "WARN": true, "SKIPPED": true}

// -----------------------------------------------------------------------------
// AC-1 — secret-leak-scan native impl emits the contract verdict vocabulary.
// -----------------------------------------------------------------------------

// TestC925_001_SecretLeakScanCleanDiffPasses pins the happy path: a diff whose
// added lines carry no secrets scans clean and yields the canonical PASS
// verdict — the LLM-variant contract (clean diff → PASS, zero tokens). Exercises
// secretleakscan.ScanDiff + Verdict directly.
func TestC925_001_SecretLeakScanCleanDiffPasses(t *testing.T) {
	clean := "diff --git a/foo.go b/foo.go\n" +
		"--- a/foo.go\n+++ b/foo.go\n@@ -1,2 +1,3 @@\n" +
		" package foo\n+\n+var answer = 42\n"
	findings := secretleakscan.ScanDiff(clean)
	if len(findings) != 0 {
		t.Errorf("clean diff produced %d finding(s), want 0: %+v", len(findings), findings)
	}
	if got := secretleakscan.Verdict(findings); got != "PASS" {
		t.Errorf("clean diff verdict = %q, want %q", got, "PASS")
	}
}

// TestC925_002_SecretLeakScanPlantedSecretFails is the NEGATIVE / anti-no-op
// predicate: a diff that ADDS a canonical secret (PEM private-key header + an
// AWS access-key id) MUST produce at least one finding and the canonical FAIL
// verdict. A no-op scanner that always returns PASS fails here. The Builder need
// only detect one of the two planted secrets; the assertion requires >=1
// finding, not a specific rule set.
func TestC925_002_SecretLeakScanPlantedSecretFails(t *testing.T) {
	leaky := "diff --git a/config.go b/config.go\n" +
		"--- a/config.go\n+++ b/config.go\n@@ -1,1 +1,4 @@\n" +
		" package config\n" +
		"+const key = \"-----BEGIN RSA PRIVATE KEY-----\"\n" +
		"+const awsID = \"AKIAIOSFODNN7EXAMPLE\"\n"
	findings := secretleakscan.ScanDiff(leaky)
	if len(findings) == 0 {
		t.Fatalf("planted secret went undetected: ScanDiff returned 0 findings")
	}
	if got := secretleakscan.Verdict(findings); got != "FAIL" {
		t.Errorf("planted-secret verdict = %q, want %q", got, "FAIL")
	}
}

// -----------------------------------------------------------------------------
// AC-2 — flake-rerun-scan native impl reruns N times and diffs deterministically.
// -----------------------------------------------------------------------------

// TestC925_003_FlakeRerunStableIsDeterministicPass pins two things: a stable
// (always-pass) target reruns to a consistent-PASS Result, and Rerun is
// DETERMINISTIC — the same attempt fn yields a byte-identical Result across two
// invocations (the "diffs deterministically" clause). Exercises
// flakererunscan.Rerun + Result.Verdict.
func TestC925_003_FlakeRerunStableIsDeterministicPass(t *testing.T) {
	stable := func(i int) bool { return true }
	got := flakererunscan.Rerun(5, stable)
	if got.Runs != 5 || got.Passes != 5 || got.Failures != 0 {
		t.Errorf("stable Rerun(5) = %+v, want Runs=5 Passes=5 Failures=0", got)
	}
	if got.Flaky {
		t.Errorf("stable target flagged Flaky=true: %+v", got)
	}
	if v := got.Verdict(); v != "PASS" {
		t.Errorf("stable verdict = %q, want %q", v, "PASS")
	}
	// Determinism: identical attempt fn → identical Result.
	again := flakererunscan.Rerun(5, stable)
	if !reflect.DeepEqual(got, again) {
		t.Errorf("Rerun not deterministic: first=%+v second=%+v", got, again)
	}
}

// TestC925_004_FlakeRerunStatefulFlakeDetected is the NEGATIVE / anti-no-op
// predicate: a stateful target whose outcome alternates across reruns MUST be
// flagged Flaky with a canonical, non-PASS verdict. A no-op that always returns
// a consistent-PASS Result fails here.
func TestC925_004_FlakeRerunStatefulFlakeDetected(t *testing.T) {
	alternating := func(i int) bool { return i%2 == 0 } // T,F,T,F,T,F
	got := flakererunscan.Rerun(6, alternating)
	if !got.Flaky {
		t.Errorf("alternating target not flagged Flaky: %+v", got)
	}
	if got.Passes == 0 || got.Failures == 0 {
		t.Errorf("alternating target should record both passes and failures: %+v", got)
	}
	v := got.Verdict()
	if v == "PASS" {
		t.Errorf("flaky verdict = %q, want a non-PASS canonical verdict", v)
	}
	if !canonicalVerdicts[v] {
		t.Errorf("flaky verdict = %q is not in the canonical set PASS/FAIL/WARN/SKIPPED", v)
	}
}

// -----------------------------------------------------------------------------
// AC-3 — registry selects impl per phase (kind:native|llm); LLM path stays valid;
//        no env-flag escape hatch (config/registry-driven only).
// -----------------------------------------------------------------------------

// TestC925_005_RegistryRegistersNativeScanPhases loads the real phase registry
// through phasespec.Load (the exact ingestion path the runtime uses) and asserts
// both scan phases are registered with kind:"native". Exercises the loader; RED
// today because docs/architecture/phase-registry.json has no phases[] entry for
// either name.
func TestC925_005_RegistryRegistersNativeScanPhases(t *testing.T) {
	root := acsassert.RepoRoot(t)
	regPath := filepath.Join(root, "docs", "architecture", "phase-registry.json")
	cat, err := phasespec.Load(regPath)
	if err != nil {
		t.Fatalf("phasespec.Load(%s) failed: %v", regPath, err)
	}
	for _, name := range []string{"secret-leak-scan", "flake-rerun-scan"} {
		spec, ok := cat.Get(name)
		if !ok {
			t.Errorf("phase %q absent from registry — native scan phase not registered", name)
			continue
		}
		if spec.KindOrDefault() != "native" {
			t.Errorf("phase %q kind = %q, want %q", name, spec.KindOrDefault(), "native")
		}
	}
}

// TestC925_006_ValidatorAcceptsNativeRejectsUnknown pins the validator seam that
// unlocks per-phase native dispatch WITHOUT blanket-accepting every kind. It
// asserts (a) a kind:"native" spec for the two scan phases no longer trips the
// "reserved but not yet executable" violation, (b) the kind:"llm" path stays
// valid, and (c) an unknown kind is STILL rejected — the anti-no-op guard that
// the fix didn't just delete the kind switch. Exercises phasespec.ValidateUserSpec.
func TestC925_006_ValidatorAcceptsNativeRejectsUnknown(t *testing.T) {
	const reserved = "reserved but not yet executable"

	for _, name := range []string{"secret-leak-scan", "flake-rerun-scan"} {
		nativeSpec := phasespec.PhaseSpec{Name: name, Kind: "native", Optional: true}
		for _, viol := range phasespec.ValidateUserSpec(nativeSpec) {
			if strings.Contains(viol, reserved) {
				t.Errorf("native kind for %q still rejected as reserved: %q", name, viol)
			}
		}
	}

	// The LLM path must remain valid (no kind violation at all).
	llmSpec := phasespec.PhaseSpec{Name: "secret-leak-scan", Kind: "llm", Optional: true}
	for _, viol := range phasespec.ValidateUserSpec(llmSpec) {
		if strings.Contains(viol, reserved) || strings.Contains(viol, "unknown kind") {
			t.Errorf("kind:\"llm\" spec produced a kind violation: %q", viol)
		}
	}

	// Anti-no-op: an unknown kind is still rejected.
	bogusSpec := phasespec.PhaseSpec{Name: "bogus-scan", Kind: "totally-bogus", Optional: true}
	rejected := false
	for _, viol := range phasespec.ValidateUserSpec(bogusSpec) {
		if strings.Contains(viol, "unknown kind") {
			rejected = true
		}
	}
	if !rejected {
		t.Errorf("unknown kind %q was NOT rejected — validator fix over-broadly accepts all kinds", "totally-bogus")
	}
}
