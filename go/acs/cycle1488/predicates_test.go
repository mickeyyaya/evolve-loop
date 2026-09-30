//go:build acs

package cycle1488

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runFromGoRoot(t *testing.T, cmd *exec.Cmd) (string, int) {
	t.Helper()
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%v: %v", cmd.Args, err)
	}
	return string(out), code
}

func TestC1488_001_ProbeEligibleRejectsFreshBase(t *testing.T) {
	cases := []struct {
		name      string
		base      string
		candidate string
		want      bool
	}{
		{name: "fresh worktree equals base", base: "tree-aaa", candidate: "tree-aaa", want: false},
		{name: "empty candidate has no identity", base: "tree-aaa", candidate: "", want: false},
		{name: "both identities empty", base: "", candidate: "", want: false},
		{name: "changed worktree", base: "tree-aaa", candidate: "tree-bbb", want: true},
		{name: "unresolvable base stays eligible", base: "", candidate: "tree-bbb", want: true},
	}
	for _, tc := range cases {
		if got := verdictcache.ProbeEligible(tc.base, tc.candidate); got != tc.want {
			t.Errorf("RED %s: ProbeEligible(%q, %q) = %t, want %t",
				tc.name, tc.base, tc.candidate, got, tc.want)
		}
	}
}

func TestC1488_002_ShadowProbeWiredToSharedPredicate(t *testing.T) {
	out, code := runFromGoRoot(t, exec.Command("go", "test", "-tags", "integration", "-count=1",
		"-run", "TestVerdictCacheProbeEligibilityWiring", "./internal/core"))
	if code != 0 {
		t.Errorf("RED: shadow-probe wiring oracle failed (rc=%d):\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("RED: TestVerdictCacheProbeEligibilityWiring did not run:\n%s", out)
	}
	orch := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "core", "orchestrator.go")
	if !acsassert.FileContains(t, orch, "verdictcache.ProbeEligible(") {
		t.Errorf("RED: %s does not call verdictcache.ProbeEligible — the guard is still a local copy", orch)
	}
}

func TestC1488_003_AuditBindingPutWiredToSharedPredicate(t *testing.T) {
	out, code := runFromGoRoot(t, exec.Command("go", "test", "-tags", "integration", "-count=1",
		"-run", "TestVerdictCacheCollisionRegression", "./internal/core"))
	if code != 0 {
		t.Errorf("RED: pre-existing collision regression broke (rc=%d):\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("RED: TestVerdictCacheCollisionRegression did not run — vacuous check:\n%s", out)
	}
	bindings := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "core", "phase_bindings.go")
	if !acsassert.FileContains(t, bindings, "verdictcache.ProbeEligible(") {
		t.Errorf("RED: %s does not call verdictcache.ProbeEligible — the Put guard is still a local copy", bindings)
	}
	if !acsassert.FileNotContains(t, bindings, "worktreeTree == headTree") {
		t.Errorf("RED: %s still holds the duplicated inline comparison alongside the shared predicate", bindings)
	}
}

func TestC1488_004_ProbeEligibleCoveredByPackageSuite(t *testing.T) {
	out, code := runFromGoRoot(t, exec.Command("go", "test", "-count=1",
		"-run", "TestStore|TestProbeEligible", "./internal/verdictcache"))
	if code != 0 {
		t.Errorf("RED: verdictcache suite failed (rc=%d):\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("RED: verdictcache suite selection matched nothing — vacuous check:\n%s", out)
	}
	named := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "verdictcache", "apicover_named_test.go")
	if !acsassert.FileContains(t, named, "ProbeEligible") {
		t.Errorf("RED: %s does not name ProbeEligible — the repo-wide apicover gate (ADR-0069) will reject it", named)
	}
}
