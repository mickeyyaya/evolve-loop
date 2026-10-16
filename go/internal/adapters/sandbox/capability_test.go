package sandbox

import (
	"context"
	"errors"
	"strings"
	"testing"
)

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

func TestMeasureCapability_FailureReasonSurfaced(t *testing.T) {
	probe := func(ctx context.Context, bin string) error {
		return errors.New("sandbox_apply: Operation not permitted")
	}
	got := measureCapability(ProbeResult{OS: "darwin", Available: true, BinaryPath: "/x"}, probe)
	if !strings.Contains(got.Reason, "Operation not permitted") {
		t.Errorf("Reason should surface the apply failure, got %q", got.Reason)
	}
}

func TestShouldWrap_MeasuredCapability(t *testing.T) {
	base := func(os string) ProbeResult { return ProbeResult{OS: os, Available: true, BinaryPath: "/usr/bin/sb"} }
	capable := func(pr ProbeResult) ProbeResult { pr.CapabilityChecked = true; pr.Capable = true; return pr }
	incapable := func(pr ProbeResult) ProbeResult { pr.CapabilityChecked = true; pr.Capable = false; return pr }

	tests := []struct {
		name     string
		nested   bool
		probe    ProbeResult
		wantWrap bool
	}{
		{"standalone darwin + measured incapable → skip", false, incapable(base("darwin")), false},
		{"standalone linux + measured incapable → skip", false, incapable(base("linux")), false},
		{"standalone darwin + measured capable → wrap", false, capable(base("darwin")), true},
		{"nested darwin + measured capable → wrap", true, capable(base("darwin")), true},
		{"nested linux + measured capable → wrap", true, capable(base("linux")), true},
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
