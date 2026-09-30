//go:build acs

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

func TestC702_001_DigestNewestFirstTruncation(t *testing.T) {
	runGoTest(t, recurrencePkg, "TestWriteDigest_NewestFirstTruncationAtTokenBudget")
}

func TestC702_002_DigestSanitizesControlChars(t *testing.T) {
	runGoTest(t, recurrencePkg, "TestWriteDigest_SanitizesLessonTextControlChars")
}

func TestC702_003_DigestAggregatesGenericPatterns(t *testing.T) {
	runGoTest(t, recurrencePkg, "TestWriteDigest_AggregatesGenericPatternsToOneLine")
}

func TestC702_004_DigestEmptyHistoryWritesNothing(t *testing.T) {
	runGoTest(t, recurrencePkg, "TestWriteDigest_EmptyHistoryWritesNothing")
}

func TestC702_005_ChronicleCompiledDefaults(t *testing.T) {
	runGoTest(t, policyPkg, "TestChronicleConfig_CompiledDefaults")
}

func TestC702_006_ChroniclePolicyOverrides(t *testing.T) {
	runGoTest(t, policyPkg, "TestChronicleConfig_PolicyOverrides")
}

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

func TestC702_008_TouchedPackagesVetClean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", recurrencePkg, policyPkg)
	if code != 0 || err != nil {
		t.Fatalf("go vet exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}
