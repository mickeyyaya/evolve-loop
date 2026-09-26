package profiles

import "testing"

func TestSandboxConfig_DeniesPathsUnderADeniedSubpath(t *testing.T) {
	sb := SandboxConfig{DenySubpaths: []string{".git", ".evolve/profiles", ".evolve/state.json", ".evolve/evals/"}}
	for path, want := range map[string]bool{
		".evolve/profiles/historian.json":  true,
		".evolve/profiles":                 true,
		"./.evolve/profiles/builder.json":  true,
		".evolve/state.json":               true,
		".evolve/evals/x.md":               true,
		".git/config":                      true,
		".evolve/":                         true,
		".evolve/profilesX/a.json":         false,
		".evolve/state.json.bak":           false,
		"go/internal/profiles/profiles.go": false,
		".evolve/runs/cycle-1/report.md":   false,
		"go/":                              false,
		"":                                 false,
	} {
		if got := sb.Denies(path); got != want {
			t.Errorf("Denies(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestSandboxConfig_WithNoDenyListDeniesNothing(t *testing.T) {
	if (SandboxConfig{}).Denies(".evolve/profiles/x.json") {
		t.Fatal("an empty deny list must deny nothing")
	}
}
