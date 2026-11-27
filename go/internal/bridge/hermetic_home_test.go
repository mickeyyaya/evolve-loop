package bridge

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDoctorProbe_ReadsTheInjectedScratchHomeNotTheOperatorHome(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		withAuth       bool
		wantConfigured bool
	}{
		{name: "an auth file under the scratch home is read", withAuth: true, wantConfigured: true},
		{name: "no auth file under the scratch home is not found elsewhere", withAuth: false, wantConfigured: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			home := t.TempDir()
			if tc.withAuth {
				mkfile(t, filepath.Join(home, ".codex", "auth.json"), "{}")
			}
			eng := doctorEngine(map[string]string{"HOME": home}, map[string]bool{"codex": true}, 0, nil)
			rep, _ := eng.Doctor(context.Background(), "codex", false)
			if len(rep.Results) != 1 || rep.Results[0].Auth.Configured != tc.wantConfigured {
				t.Fatalf("doctor probe auth = %+v, want Configured=%v from the injected scratch home", rep.Results, tc.wantConfigured)
			}
		})
	}
}
