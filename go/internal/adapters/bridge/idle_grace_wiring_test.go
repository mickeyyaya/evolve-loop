package bridge

import "testing"

func TestNewDefault_ProductionDepsCarryTheIdleCheckpointGrace(t *testing.T) {
	for _, tc := range []struct {
		name   string
		policy string
		want   int
	}{
		{"policy value reaches the engine", `{"bridge":{"idle_checkpoint_grace_s":17}}`, 17},
		{"unset key stays zero so the engine default applies", `{}`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writePolicyJson(t, dir, tc.policy)
			got := NewDefault(dir, nil).productionEngineDeps(nil).IdleCheckpointGraceS
			if got != tc.want {
				t.Errorf("IdleCheckpointGraceS = %d, want %d", got, tc.want)
			}
		})
	}
}
