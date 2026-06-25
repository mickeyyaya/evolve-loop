package preflight

import (
	"strings"
	"testing"
)

// TestProbe_MeasuredCapabilityDemotesExpectedToWork — the SandboxCapable seam
// lets a MEASURED-incapable result demote ExpectedToWork (and InnerSandbox, for
// consistency) to false on a host the standalone guess would call "working". The
// override is subtractive: a measured-capable result never promotes, and a nil
// seam (unmeasured) is byte-identical to the legacy guess.
func TestProbe_MeasuredCapabilityDemotesExpectedToWork(t *testing.T) {
	root := t.TempDir()
	runProbe := func(capable, checked, withSeam, nested bool) Profile {
		opts := Options{
			ProjectRoot: root,
			OSType:      "darwin",
			Env:         stubEnv(map[string]string{"HOME": root}),
			LookPath:    stubLookPath(map[string]string{"sandbox-exec": "/usr/bin/sandbox-exec"}),
			Now:         fixedNow(),
			IsNested:    func() bool { return nested },
		}
		if withSeam {
			opts.SandboxCapable = func() (bool, bool) { return capable, checked }
		}
		return Probe(opts)
	}

	t.Run("measured incapable → demoted to false", func(t *testing.T) {
		p := runProbe(false, true, true, false)
		if p.Sandbox.ExpectedToWork {
			t.Errorf("measured-incapable standalone should demote ExpectedToWork to false; got %+v", p.Sandbox)
		}
		if p.AutoConfig.InnerSandbox {
			t.Error("measured-incapable should also disable InnerSandbox (consistency)")
		}
		if !strings.Contains(p.Sandbox.Reason, "measured") {
			t.Errorf("reason should surface the measurement, got %q", p.Sandbox.Reason)
		}
	})

	t.Run("measured capable → stays true (no promotion needed)", func(t *testing.T) {
		p := runProbe(true, true, true, false)
		if !p.Sandbox.ExpectedToWork {
			t.Error("measured-capable standalone should keep ExpectedToWork true")
		}
		if !p.AutoConfig.InnerSandbox {
			t.Error("measured-capable standalone should keep InnerSandbox true")
		}
	})

	t.Run("nil seam → legacy byte-identical (works)", func(t *testing.T) {
		p := runProbe(false, false, false, false)
		if !p.Sandbox.ExpectedToWork {
			t.Error("nil seam must be byte-identical to legacy: standalone+sandbox-exec → works")
		}
		if !p.AutoConfig.InnerSandbox {
			t.Error("nil seam must keep InnerSandbox true (legacy)")
		}
	})

	// nested+incapable: ExpectedToWork is already false from decideSandbox, so the
	// subtractive override must NOT fire again and double the EPERM reason text.
	t.Run("nested+incapable → false, reason not doubled", func(t *testing.T) {
		p := runProbe(false, true, true, true)
		if p.Sandbox.ExpectedToWork {
			t.Error("nested+incapable: ExpectedToWork must be false")
		}
		if p.AutoConfig.InnerSandbox {
			t.Error("nested+incapable: InnerSandbox must be false")
		}
		if strings.Count(p.Sandbox.Reason, "EPERM") > 1 {
			t.Errorf("reason string doubled the EPERM mention: %q", p.Sandbox.Reason)
		}
	})
}
