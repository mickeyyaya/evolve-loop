//go:build acs

package cycle1550

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", "-v", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("binding test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag). exit=%d\ncombined go-test output:\n%s",
				name, pkg, code, out)
		}
	}
}

func TestC1550_001_HaltAutofilerMintsCycleScopedIdentity(t *testing.T) {
	assertDefaultSuiteTestsPass(t, cmdEvolvePkg,
		"TestWritePipelineEscalation_IdentityIncludesCycleNumber",
	)
}

func TestC1550_002_DistinctHaltsNeverCollideOnDisk(t *testing.T) {
	assertDefaultSuiteTestsPass(t, cmdEvolvePkg,
		"TestWritePipelineEscalation_DistinctCyclesNeverCollideOnDisk",
	)
}

func TestC1550_003_PreExistingBehaviourNotWeakened(t *testing.T) {
	assertDefaultSuiteTestsPass(t, cmdEvolvePkg,
		"TestWritePipelineEscalation_WritesDossierAndInboxItem",
	)
}
