package bridge

import "testing"

func TestManifestIsTmux(t *testing.T) {
	cases := []struct {
		cli  string
		tmux bool
	}{
		{"claude-tmux", true},
		{"codex-tmux", true},
		{"agy-tmux", true},
		{"ollama-tmux", true},
		{"claude-p", false},
		{"codex", false},
		{"agy", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.cli, func(t *testing.T) {
			m, err := LoadManifest(tc.cli)
			if err != nil {
				t.Fatalf("LoadManifest(%q): %v", tc.cli, err)
			}
			if got := m.IsTmux(); got != tc.tmux {
				t.Errorf("IsTmux()=%v, want %v (Transport=%q)", got, tc.tmux, m.Transport)
			}
			if got := IsTmuxDriver(tc.cli); got != tc.tmux {
				t.Errorf("IsTmuxDriver(%q)=%v, want %v", tc.cli, got, tc.tmux)
			}
		})
	}
}

func TestIsTmuxDriver(t *testing.T) {
	cases := []struct {
		cli  string
		want bool
		note string
	}{
		{"claude-tmux", true, "tmux positive"},
		{"codex-tmux", true, "tmux positive (codex)"},
		{"agy-tmux", true, "tmux positive (agy)"},
		{"ollama-tmux", true, "tmux positive (ollama)"},
		{"claude-p", false, "headless negative"},
		{"codex", false, "headless negative (codex)"},
		{"agy", false, "headless negative (agy)"},
		{"", false, "empty cli -> false (R3 edge)"},
		{"nonesuch", false, "unknown cli -> false (R3 edge)"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.cli+"/"+tc.note, func(t *testing.T) {
			if got := IsTmuxDriver(tc.cli); got != tc.want {
				t.Errorf("IsTmuxDriver(%q)=%v, want %v", tc.cli, got, tc.want)
			}
		})
	}
}

func TestManifestIsTmux_UnknownCLI(t *testing.T) {
	if !IsTmuxDriver("custom-tmux") {
		t.Error("IsTmuxDriver(custom-tmux)=false, want true (suffix fallback)")
	}
	if IsTmuxDriver("custom-headless") {
		t.Error("IsTmuxDriver(custom-headless)=true, want false")
	}
}
