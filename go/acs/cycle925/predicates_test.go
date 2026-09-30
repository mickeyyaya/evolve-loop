//go:build acs

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

var canonicalVerdicts = map[string]bool{"PASS": true, "FAIL": true, "WARN": true, "SKIPPED": true}

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
	again := flakererunscan.Rerun(5, stable)
	if !reflect.DeepEqual(got, again) {
		t.Errorf("Rerun not deterministic: first=%+v second=%+v", got, again)
	}
}

func TestC925_004_FlakeRerunStatefulFlakeDetected(t *testing.T) {
	alternating := func(i int) bool { return i%2 == 0 }
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

	llmSpec := phasespec.PhaseSpec{Name: "secret-leak-scan", Kind: "llm", Optional: true}
	for _, viol := range phasespec.ValidateUserSpec(llmSpec) {
		if strings.Contains(viol, reserved) || strings.Contains(viol, "unknown kind") {
			t.Errorf("kind:\"llm\" spec produced a kind violation: %q", viol)
		}
	}

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
