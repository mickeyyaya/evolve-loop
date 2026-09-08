package preflight

import (
	"strings"
	"testing"
)

func TestProbe_MeasuredCapabilityOverridesCodexPathHint(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		for _, tc := range []struct {
			name                   string
			capable, checked, want bool
		}{
			{"applies", true, true, true},
			{"fails", false, true, false},
			{"unknown", true, false, false},
		} {
			t.Run(goos+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				p := Probe(Options{
					ProjectRoot: root, OSType: goos,
					Env:            stubEnv(map[string]string{"HOME": root, "PATH": "/var/run/codex.system/bootstrap/usr/bin:/usr/bin:/bin"}),
					LookPath:       stubLookPath(map[string]string{"sandbox-exec": "/usr/bin/sandbox-exec", "bwrap": "/usr/bin/bwrap"}),
					SandboxCapable: func() (bool, bool) { return tc.capable, tc.checked },
				})
				if !p.ClaudeCode.Nested {
					t.Fatal("fixture must retain the session hint")
				}
				if p.Sandbox.ExpectedToWork != tc.want || p.AutoConfig.InnerSandbox != tc.want {
					t.Fatalf("host report disagrees with measured wrapping: sandbox=%+v auto=%+v", p.Sandbox, p.AutoConfig)
				}
				if tc.want && p.AutoConfig.SandboxFallbackOnEPERM != "0" {
					t.Fatal("working wrapper must not advertise a startup fallback")
				}
				if p.Sandbox.Reason != p.AutoConfig.InnerSandboxReason {
					t.Fatal("host and launch policy explanations diverged")
				}
				if strings.Contains(p.AutoConfig.Reasoning, "hooks suffice") || strings.Contains(p.Sandbox.Reason, "returns EPERM") {
					t.Fatal("report invented outer confinement or an unmeasured EPERM")
				}
			})
		}
	}
}
