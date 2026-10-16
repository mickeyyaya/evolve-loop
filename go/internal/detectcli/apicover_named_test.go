package detectcli

import (
	"errors"
	"testing"
)

func TestCanonical_AliasTable(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"antigravity", "agy"},
		{"agy", "agy"},
		{"claude", "claude"},
		{"codex", "codex"},
		{"gemini", "gemini"},
		{"", ""},
		{"Antigravity", "Antigravity"},
	} {
		if got := Canonical(tc.in); got != tc.want {
			t.Errorf("Canonical(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResult_DetectExplicitOverride(t *testing.T) {
	got := Detect(Options{
		Platform: "custom",
		Env:      func(string) string { return "" },
		LookPath: func(string) (string, error) { return "", errors.New("unused") },
	})

	want := Result{CLI: "custom", Reason: "explicit override via Options.Platform"}
	if got != want {
		t.Errorf("Detect = %+v, want %+v", got, want)
	}
}
