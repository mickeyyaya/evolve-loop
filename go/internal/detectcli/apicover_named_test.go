package detectcli

import (
	"errors"
	"testing"
)

// TestCanonical_AliasTable names detectcli.Canonical (the single CLI-alias
// authority, ADR house rule: every export of an apicover-enrolled package must
// be named and executed by a test) and pins its whole contract: exactly one
// alias is rewritten, and every other name — including the empty string the
// callers' "cli unresolved" guards depend on — passes through untouched.
func TestCanonical_AliasTable(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"antigravity", "agy"},
		{"agy", "agy"},
		{"claude", "claude"},
		{"codex", "codex"},
		{"gemini", "gemini"},
		{"", ""},
		{"Antigravity", "Antigravity"}, // case-sensitive: only the exact alias moves
	} {
		if got := Canonical(tc.in); got != tc.want {
			t.Errorf("Canonical(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestResult_DetectExplicitOverride names the detectcli.Result type (returned by
// Detect but never named in a test) and pins the full struct contract for the
// deterministic explicit-override branch: Options.Platform wins and Detect
// reports both the chosen CLI and the matching reason.
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
