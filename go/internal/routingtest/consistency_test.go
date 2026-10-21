package routingtest

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

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

func TestSignalSpec_DualRenderingAgree(t *testing.T) {
	fixtures := []SignalSpec{
		{CycleSize: "trivial"},
		{CycleSize: "medium", ScoutItemCount: 3, ScoutCarryover: 2},
		{TriageSize: "large"},
		{ACSRed: 2, ACSGreen: 5, ACSRegression: 1, BuildVerdict: "PASS"},
		{SeverityMax: "HIGH", FilesTouched: 4, BuildVerdict: "WARN"},
		{AuditVerdict: "PASS", AuditConf: 0.9},
		{CycleSize: "small", ACSRed: 1, SeverityMax: "CRITICAL", FilesTouched: 2, AuditVerdict: "FAIL", AuditRedCount: 3},
		{ScoutBacklog: 7},
		{DiffLOC: 540, BuildVerdict: "PASS"},
		{CycleSize: "large", ScoutBacklog: 12, ScoutCarryover: 3, DiffLOC: 800, FilesTouched: 6, BuildVerdict: "PASS"},
		{ArtifactBytes: 4096},
		{CycleSize: "medium", ScoutBacklog: 9, ScoutCarryover: 2, ArtifactBytes: 1 << 20},
		{},
	}
	for i, f := range fixtures {
		want := f.Signals()
		ws := seedWorkspace(t, t.TempDir(), 1, f.HandoffFiles())
		got, err := router.Digest(ws, presentRoles(f))
		if err != nil {
			t.Fatalf("fixture %d: Digest: %v", i, err)
		}
		got.RunDir = router.RunDirSignals{}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("fixture %d: Digest != Signals\n got=%+v\nwant=%+v", i, got, want)
		}
	}
}

func TestSignalSpec_WrappedDualRenderingAgree(t *testing.T) {
	fixtures := []SignalSpec{
		{CycleSize: "trivial"},
		{CycleSize: "medium", ScoutItemCount: 3, ScoutCarryover: 2},
		{TriageSize: "large"},
		{ACSRed: 2, ACSGreen: 5, ACSRegression: 1, BuildVerdict: "PASS"},
		{SeverityMax: "HIGH", FilesTouched: 4, BuildVerdict: "WARN"},
		{AuditVerdict: "PASS", AuditConf: 0.9},
		{CycleSize: "small", ACSRed: 1, SeverityMax: "CRITICAL", FilesTouched: 2, AuditVerdict: "FAIL", AuditRedCount: 3},
		{ScoutBacklog: 7},
		{DiffLOC: 540, BuildVerdict: "PASS"},
		{CycleSize: "large", ScoutBacklog: 12, ScoutCarryover: 3, DiffLOC: 800, FilesTouched: 6, BuildVerdict: "PASS"},
		{ArtifactBytes: 4096},
		{CycleSize: "medium", ScoutBacklog: 9, ScoutCarryover: 2, ArtifactBytes: 1 << 20},
		{},
	}
	for i, f := range fixtures {
		want := f.Signals()
		ws := seedWorkspace(t, t.TempDir(), 1, f.WrappedHandoffFiles())
		got, err := router.Digest(ws, presentRoles(f))
		if err != nil {
			t.Fatalf("fixture %d: Digest: %v", i, err)
		}
		got.RunDir = router.RunDirSignals{}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("fixture %d: wrapped Digest != Signals\n got=%+v\nwant=%+v", i, got, want)
		}
	}
}

func TestSignalSpec_ArtifactBytesReachesBothRenderings(t *testing.T) {
	const want = 4096
	f := SignalSpec{ArtifactBytes: want}
	pure := f.Signals()
	if !pure.Scout.Present || pure.Scout.ArtifactBytes != want {
		t.Errorf("Signals().Scout = %+v, want Present with ArtifactBytes=%d", pure.Scout, want)
	}
	ws := seedWorkspace(t, t.TempDir(), 1, f.HandoffFiles())
	got, err := router.Digest(ws, presentRoles(f))
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if !got.Scout.Present || got.Scout.ArtifactBytes != want {
		t.Errorf("Digest().Scout = %+v, want Present with ArtifactBytes=%d", got.Scout, want)
	}
}

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
