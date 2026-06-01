package main

import "testing"

func TestPickClassifierCLI(t *testing.T) {
	t.Setenv("EVOLVE_MODELCATALOG_CLASSIFIER_CLI", "")
	tests := []struct {
		name  string
		ready []string
		want  string
	}{
		{"prefers codex", []string{"agy", "ollama", "codex"}, "codex"},
		{"falls to claude when no codex", []string{"agy", "claude"}, "claude"},
		{"falls to agy when only agy of the preferred", []string{"ollama", "agy"}, "agy"},
		{"any ready when none preferred", []string{"ollama"}, "ollama"},
		{"empty when none ready", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickClassifierCLI(tt.ready); got != tt.want {
				t.Fatalf("pickClassifierCLI(%v) = %q, want %q", tt.ready, got, tt.want)
			}
		})
	}
}

func TestPickClassifierCLIEnvOverride(t *testing.T) {
	// Honored when the override names a READY CLI.
	t.Setenv("EVOLVE_MODELCATALOG_CLASSIFIER_CLI", "agy")
	if got := pickClassifierCLI([]string{"codex", "agy"}); got != "agy" {
		t.Fatalf("ready env override = %q, want agy", got)
	}
	// Ignored (falls through to preference) when the override is NOT ready —
	// a stale override must not classify against a blocked CLI.
	t.Setenv("EVOLVE_MODELCATALOG_CLASSIFIER_CLI", "gemini")
	if got := pickClassifierCLI([]string{"codex", "agy"}); got != "codex" {
		t.Fatalf("non-ready env override should fall through to codex, got %q", got)
	}
}
