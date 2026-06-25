package sandbox

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestMeasureCapability — measureCapability records Capable/CapabilityChecked
// from an injected probe seam: applies→capable, EPERM→checked-but-incapable,
// and !Available or nil-probe leave it unmeasured (checked=false).
func TestMeasureCapability(t *testing.T) {
	okProbe := func(ctx context.Context, bin string) error { return nil }
	epermProbe := func(ctx context.Context, bin string) error { return errors.New("sandbox_apply: EPERM") }

	tests := []struct {
		name        string
		in          ProbeResult
		probe       capabilityProbe
		wantChecked bool
		wantCapable bool
	}{
		{"available + applies → capable", ProbeResult{OS: "darwin", Available: true, BinaryPath: "/usr/bin/sandbox-exec"}, okProbe, true, true},
		{"available + EPERM → checked but not capable", ProbeResult{OS: "darwin", Available: true, BinaryPath: "/usr/bin/sandbox-exec"}, epermProbe, true, false},
		{"unavailable → not measured (checked stays false)", ProbeResult{OS: "darwin", Available: false, Reason: "not on PATH"}, epermProbe, false, false},
		{"nil probe → not measured (checked stays false)", ProbeResult{OS: "darwin", Available: true, BinaryPath: "/x"}, nil, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := measureCapability(tt.in, tt.probe)
			if got.CapabilityChecked != tt.wantChecked {
				t.Errorf("CapabilityChecked=%v, want %v", got.CapabilityChecked, tt.wantChecked)
			}
			if got.Capable != tt.wantCapable {
				t.Errorf("Capable=%v, want %v", got.Capable, tt.wantCapable)
			}
		})
	}
}

// TestMeasureCapability_FailureReasonSurfaced — when the apply fails, the
// diagnostic is preserved in Reason so the operator sees WHY inner confinement
// was skipped (not a silent drop).
func TestMeasureCapability_FailureReasonSurfaced(t *testing.T) {
	probe := func(ctx context.Context, bin string) error {
		return errors.New("sandbox_apply: Operation not permitted")
	}
	got := measureCapability(ProbeResult{OS: "darwin", Available: true, BinaryPath: "/x"}, probe)
	if !strings.Contains(got.Reason, "Operation not permitted") {
		t.Errorf("Reason should surface the apply failure, got %q", got.Reason)
	}
}

// TestShouldWrap_CapabilityIsSubtractive — capability only DEMOTES a would-be
// wrap to skip (the broken-standalone hang), never PROMOTES a nested skip to a
// wrap. Non-regression invariant: new_wrap ⟹ old_wrap.
func TestShouldWrap_CapabilityIsSubtractive(t *testing.T) {
	base := func(os string) ProbeResult { return ProbeResult{OS: os, Available: true, BinaryPath: "/usr/bin/sb"} }
	capable := func(pr ProbeResult) ProbeResult { pr.CapabilityChecked = true; pr.Capable = true; return pr }
	incapable := func(pr ProbeResult) ProbeResult { pr.CapabilityChecked = true; pr.Capable = false; return pr }

	tests := []struct {
		name     string
		nested   bool
		probe    ProbeResult
		wantWrap bool
	}{
		// NEW demotion: standalone + measured-incapable is the broken-standalone
		// cell that today wraps → hangs the REPL boot (exit 80). Now → skip.
		{"standalone darwin + measured incapable → skip", false, incapable(base("darwin")), false},
		{"standalone linux + measured incapable → skip", false, incapable(base("linux")), false},
		// standalone + measured-capable → wrap (confirmed-confined path).
		{"standalone darwin + measured capable → wrap", false, capable(base("darwin")), true},
		// Subtractive guarantee: nested + measured-capable must STILL skip —
		// capability never promotes a nested skip to a wrap.
		{"nested darwin + measured capable → still skip", true, capable(base("darwin")), false},
		{"nested linux + measured capable → still skip", true, capable(base("linux")), false},
		// Backward-compat: unchecked capability (every caller before this slice)
		// is unchanged — standalone + available → wrap.
		{"standalone darwin + unchecked → wrap (unchanged)", false, base("darwin"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWrap, reason := ShouldWrap(tt.nested, tt.probe)
			if gotWrap != tt.wantWrap {
				t.Errorf("ShouldWrap(nested=%v, capable=%v checked=%v) wrap=%v, want %v (reason %q)",
					tt.nested, tt.probe.Capable, tt.probe.CapabilityChecked, gotWrap, tt.wantWrap, reason)
			}
			if reason == "" {
				t.Error("ShouldWrap must always return a non-empty reason")
			}
		})
	}
}
