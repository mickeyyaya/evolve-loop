package routingtest

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// presentRoles lists the completed-phase names a fixture's handoffs represent,
// so Digest gates them present.
func presentRoles(f SignalSpec) []string {
	var roles []string
	if f.scoutPresent() {
		roles = append(roles, "scout")
	}
	if f.triagePresent() {
		roles = append(roles, "triage")
	}
	if f.buildPresent() {
		roles = append(roles, "build")
	}
	if f.auditPresent() {
		roles = append(roles, "audit")
	}
	return roles
}

// TestSignalSpec_DualRenderingAgree is the framework's keystone self-test: the
// pure rendering (Signals) MUST equal what Digest extracts from the orchestrator
// rendering (HandoffFiles). This guarantees PureKernel and FullOrchestrator
// scenarios see equivalent signals from one fixture.
func TestSignalSpec_DualRenderingAgree(t *testing.T) {
	fixtures := []SignalSpec{
		{CycleSize: "trivial"},
		{CycleSize: "medium", ScoutItemCount: 3, ScoutCarryover: 2},
		{TriageSize: "large"},
		{ACSRed: 2, ACSGreen: 5, ACSRegression: 1, BuildVerdict: "PASS"},
		{SeverityMax: "HIGH", FilesTouched: 4, BuildVerdict: "WARN"},
		{AuditVerdict: "PASS", AuditConf: 0.9},
		{CycleSize: "small", ACSRed: 1, SeverityMax: "CRITICAL", FilesTouched: 2, AuditVerdict: "FAIL", AuditRedCount: 3},
		{}, // empty fixture → all roles absent
	}
	for i, f := range fixtures {
		want := f.Signals()
		ws := seedWorkspace(t, t.TempDir(), 1, f.HandoffFiles())
		got, err := router.Digest(ws, presentRoles(f))
		if err != nil {
			t.Fatalf("fixture %d: Digest: %v", i, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("fixture %d: Digest != Signals\n got=%+v\nwant=%+v", i, got, want)
		}
	}
}

// TestMatrix_CrossProductCount verifies Matrix yields exactly product(dims)
// uniquely-named specs.
func TestMatrix_CrossProductCount(t *testing.T) {
	specs := Matrix([]Brick{Pure()},
		Dim("size", V("trivial", TrivialCycle()), V("medium", MediumCycle())),
		Dim("build", V("green", GreenBuild()), V("red", RedBuild(3)), V("hot", Severity("HIGH"))),
	)
	if len(specs) != 6 {
		t.Fatalf("cross-product count=%d, want 6 (2×3)", len(specs))
	}
	seen := map[string]bool{}
	for _, s := range specs {
		if seen[s.Name] {
			t.Errorf("duplicate scenario name %q", s.Name)
		}
		seen[s.Name] = true
		if s.Surface != PureKernel {
			t.Errorf("%q: base brick Pure() not applied", s.Name)
		}
	}
}
