package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

func writeProbePolicy(t *testing.T, json string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(json), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	return dir
}

func TestUsageProbeEnabled(t *testing.T) {
	t.Parallel()
	on := writeProbePolicy(t, `{"cli_health":{"proactive_probe":true}}`)
	off := writeProbePolicy(t, `{}`)
	masterOff := writeProbePolicy(t, `{"cli_health":{"proactive_probe":true,"enabled":false}}`)

	cases := []struct {
		name      string
		evolveDir string
		want      bool
	}{
		{"policy on", on, true},
		{"policy on but master off", masterOff, false},
		{"policy off (default)", off, false},
		{"no policy dir", t.TempDir(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := usageProbeEnabled(tc.evolveDir); got != tc.want {
				t.Errorf("usageProbeEnabled(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func usageFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "internal", "quotastate", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestNewUsageProber_ReadsEachCLIsWindowsThroughItsManifestAndBenchesTheRightFamily(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		family, fixture string
		benched         []string
	}{
		{"agy", "agy_usage_claude_drained.txt", []string{"agy-claude"}},
		{"agy", "agy_usage_gemini_drained.txt", []string{"agy"}},
		{"claude", "claude_usage_session_exhausted.txt", []string{"claude"}},
		{"claude", "claude_usage_week_exhausted.txt", []string{"claude"}},
		{"claude", "claude_usage_fable_exhausted.txt", nil},
		{"claude", "claude_usage_2.1.291.txt", nil},
	} {
		root := t.TempDir()
		evolveDir := filepath.Join(root, ".evolve")
		pane := usageFixture(t, tc.fixture)
		p := newUsageProber(usageProbeDirs{projectRoot: root, evolveDir: evolveDir}, []string{tc.family}, io.Discard)
		p.Probe = func(context.Context, string) (string, error) { return pane, nil }

		p.Run(context.Background())

		var benched []string
		for family := range clihealth.NewStore(root, nil).Active() {
			benched = append(benched, family)
		}
		if !reflect.DeepEqual(benched, tc.benched) {
			t.Errorf("%s %s: benched %v, want %v", tc.family, tc.fixture, benched, tc.benched)
		}
		recorded, err := usageprobe.LoadObservations(evolveDir)
		if err != nil || len(recorded[tc.family].Windows) == 0 {
			t.Errorf("%s %s: recorded %+v (err %v); the probe records every window it read", tc.family, tc.fixture, recorded, err)
		}
	}
}
