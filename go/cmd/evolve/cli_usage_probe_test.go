package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeProbePolicy(t *testing.T, json string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(json), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	return dir
}

// TestUsageProbeEnabled gates the proactive probe on BOTH the policy dial and
// the EVOLVE_CLI_HEALTH master switch: enabled only when policy turns it on AND
// the master switch is not 0. Default (no policy) is off — opt-in.
func TestUsageProbeEnabled(t *testing.T) {
	on := writeProbePolicy(t, `{"cli_health":{"proactive_probe":true}}`)
	off := writeProbePolicy(t, `{}`)

	cases := []struct {
		name      string
		env       map[string]string
		evolveDir string
		want      bool
	}{
		{"policy on, env unset", nil, on, true},
		{"policy on, env=1", map[string]string{"EVOLVE_CLI_HEALTH": "1"}, on, true},
		{"policy on but master off", map[string]string{"EVOLVE_CLI_HEALTH": "0"}, on, false},
		{"policy off (default)", nil, off, false},
		{"no policy dir", nil, t.TempDir(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := usageProbeEnabled(tc.env, tc.evolveDir); got != tc.want {
				t.Errorf("usageProbeEnabled(%v, …) = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}
