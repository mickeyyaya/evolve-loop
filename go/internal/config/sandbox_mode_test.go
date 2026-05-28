package config

import "testing"

// Workstream B: EVOLVE_SANDBOX=auto|on|off → cfg.SandboxMode. Unknown values
// emit a WARN and fall back to the default (auto) rather than silently break
// the trust boundary.

func TestLoad_SandboxMode_DefaultsAuto(t *testing.T) {
	cfg, _ := Load("", map[string]string{})
	if cfg.SandboxMode != SandboxModeAuto {
		t.Errorf("default SandboxMode=%q, want %q", cfg.SandboxMode, SandboxModeAuto)
	}
}

func TestLoad_SandboxMode_EnvOverride(t *testing.T) {
	for _, mode := range []string{SandboxModeAuto, SandboxModeOn, SandboxModeOff} {
		t.Run(mode, func(t *testing.T) {
			cfg, ws := Load("", map[string]string{"EVOLVE_SANDBOX": mode})
			if cfg.SandboxMode != mode {
				t.Errorf("SandboxMode=%q, want %q", cfg.SandboxMode, mode)
			}
			for _, w := range ws {
				if w.Code == "unknown-value" {
					t.Errorf("unexpected warning on valid mode: %+v", w)
				}
			}
		})
	}
}

func TestLoad_SandboxMode_UnknownValueWarnsAndKeepsDefault(t *testing.T) {
	cfg, ws := Load("", map[string]string{"EVOLVE_SANDBOX": "yolo"})
	if cfg.SandboxMode != SandboxModeAuto {
		t.Errorf("unknown value did NOT fall back to auto; got %q", cfg.SandboxMode)
	}
	var sawWarn bool
	for _, w := range ws {
		if w.Code == "unknown-value" {
			sawWarn = true
		}
	}
	if !sawWarn {
		t.Errorf("expected unknown-value warning on EVOLVE_SANDBOX=yolo; got %+v", ws)
	}
}
