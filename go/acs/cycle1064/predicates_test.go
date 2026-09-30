//go:build acs

package cycle1064

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-count=1", "-v", "-run", "^"+name+"$", pkg)
	out := stdout + stderr
	if err != nil {
		t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, out)
	}
	if code != 0 {
		t.Fatalf("%s -run %s exited %d\n%s", pkg, name, code, out)
	}
	if !strings.Contains(out, "--- PASS: "+name) {
		t.Fatalf("no PASS line for %s in %s (renamed, skipped, or never ran?)\n%s", name, pkg, out)
	}
}

func TestC1064_001_ManifestGateCodeIsDedicatedAndDualRegistered(t *testing.T) {
	if core.CodeManifestGate != shiperr.CodeManifestGate {
		t.Fatalf("core.CodeManifestGate (%q) must re-export shiperr.CodeManifestGate (%q)",
			core.CodeManifestGate, shiperr.CodeManifestGate)
	}
	if string(core.CodeManifestGate) != "MANIFEST_GATE" {
		t.Errorf("wire string = %q, want %q", core.CodeManifestGate, "MANIFEST_GATE")
	}
	if core.CodeManifestGate == core.CodeGitStageFailed {
		t.Errorf("CodeManifestGate must not alias CodeGitStageFailed (%q)", core.CodeGitStageFailed)
	}
	se, ok := core.AsShipError(core.NewShipError(core.CodeManifestGate,
		core.ShipClassPrecondition, core.StageAtomicShip, "manifest-gate block"))
	if !ok || se.Code != core.CodeManifestGate || se.Class != core.ShipClassPrecondition || se.Stage != core.StageAtomicShip {
		t.Errorf("round trip = %+v (ok=%v), want MANIFEST_GATE/precondition/atomic-ship", se, ok)
	}
}

func TestC1064_002_EnforceBlockCarriesManifestGateCode(t *testing.T) {
	runGoTest(t, "./internal/phases/ship/", "TestReconcileManifest_EnforceCarriesManifestGateCode")
	runGoTest(t, "./internal/phases/ship/", "TestReconcileManifest_ShadowUnaffectedByCodeChange")
}

func TestC1064_003_PolicyManifestGateResolvesFromJSON(t *testing.T) {
	if got := (policy.Policy{}).GatesConfig().ManifestGate; got != "shadow" {
		t.Errorf("default ManifestGate = %q, want %q (behavior-preserving)", got, "shadow")
	}
	for _, tc := range []struct{ raw, want string }{
		{`{"gates":{"manifest_gate":"enforce"}}`, "enforce"},
		{`{"gates":{"manifest_gate":"shadow"}}`, "shadow"},
		{`{"gates":{"manifest_gate":""}}`, "shadow"},
		{`{"gates":{"topn_gate":"off"}}`, "shadow"},
	} {
		var p policy.Policy
		if err := json.Unmarshal([]byte(tc.raw), &p); err != nil {
			t.Fatalf("unmarshal %s: %v", tc.raw, err)
		}
		if got := p.GatesConfig().ManifestGate; got != tc.want {
			t.Errorf("%s → ManifestGate = %q, want %q", tc.raw, got, tc.want)
		}
	}
	var p policy.Policy
	if err := json.Unmarshal([]byte(`{"gates":{"manifest_gate":"enforce"}}`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	g := p.GatesConfig()
	for _, tc := range []struct{ name, have, want string }{
		{"ContractGate", g.ContractGate, "enforce"},
		{"EvalGate", g.EvalGate, "enforce"},
		{"TriageCapGate", g.TriageCapGate, "enforce"},
		{"ReviewGate", g.ReviewGate, "off"},
		{"ReportSizeGate", g.ReportSizeGate, "shadow"},
		{"TopNGate", g.TopNGate, "enforce"},
	} {
		if tc.have != tc.want {
			t.Errorf("%s = %q, want %q (unchanged by the new gate)", tc.name, tc.have, tc.want)
		}
	}
}

func TestC1064_004_ShipPhaseThreadsResolvedGate(t *testing.T) {
	runGoTest(t, "./internal/phases/ship/", "TestShipOptions_ThreadsManifestGate")
	runGoTest(t, "./internal/phases/ship/", "TestManifestGate_PolicyToBlockEndToEnd")
}
